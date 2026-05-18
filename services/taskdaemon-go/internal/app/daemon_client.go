package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"taskdaemon/internal/httpapi"
)

// TaskRunSummary 是应用层暴露给 CLI 查询的执行历史摘要。
type TaskRunSummary struct {
	ID           int    `json:"id"`
	Trigger      string `json:"trigger"`
	Status       string `json:"status"`
	ExitCode     *int   `json:"exitCode"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
	ErrorSummary string `json:"errorSummary"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
}

// TriggerTask 通过正在运行的 daemon HTTP API 手动触发任务。
//
// 参数:
//   - ctx: 控制 HTTP 请求和任务执行生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) TriggerTask(ctx context.Context, taskID int, sessionToken string) error {
	if err := app.callDaemonAPI(ctx, http.MethodPost, fmt.Sprintf("tasks/%d/trigger", taskID), sessionToken, nil, nil, 0, http.StatusOK); err != nil {
		return err
	}
	app.logger.Info("task trigger requested", "task_id", taskID)
	return nil
}

// CancelTask 通过正在运行的 daemon HTTP API 取消任务。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) CancelTask(ctx context.Context, taskID int, sessionToken string) error {
	if err := app.callDaemonAPI(ctx, http.MethodPost, fmt.Sprintf("tasks/%d/cancel", taskID), sessionToken, nil, nil, 10*time.Second, http.StatusOK); err != nil {
		return err
	}
	app.logger.Info("task cancel requested", "task_id", taskID)
	return nil
}

// ListTaskRuns 通过正在运行的 daemon HTTP API 查询任务执行历史。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - []cli.TaskRunSummary: 最近执行历史摘要。
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) ListTaskRuns(ctx context.Context, taskID int, sessionToken string) ([]TaskRunSummary, error) {
	var response struct {
		Runs []TaskRunSummary `json:"runs"`
	}
	if err := app.callDaemonAPI(ctx, http.MethodGet, fmt.Sprintf("tasks/%d/runs", taskID), sessionToken, nil, &response, 10*time.Second, http.StatusOK); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

// callDaemonAPI 调用 daemon HTTP API。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - method: HTTP 方法。
//   - path: API 路径，不含 /api/ 前缀。
//   - sessionToken: 管理员 session token。
//   - requestBody: 请求体；为空时不发送 body。
//   - responseBody: 成功响应 data 的解码目标；为空时不解码。
//   - clientTimeout: HTTP client timeout；0 表示只使用 ctx 控制。
//   - successStatuses: 视为成功的 HTTP 状态码。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) callDaemonAPI(ctx context.Context, method string, path string, sessionToken string, requestBody []byte, responseBody any, clientTimeout time.Duration, successStatuses ...int) error {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return fmt.Errorf("daemon API requires --session-token or TASKDAEMON_SESSION_TOKEN")
	}

	endpoint := fmt.Sprintf("http://%s/api/%s", daemonClientAddress(app.cfg.Server), strings.TrimPrefix(path, "/"))
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create daemon API request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookieName, Value: sessionToken})
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: clientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call daemon API: %w", err)
	}
	defer resp.Body.Close()
	for _, status := range successStatuses {
		if resp.StatusCode == status {
			if responseBody != nil {
				return decodeDaemonAPIData(resp.Body, responseBody)
			}
			return nil
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("daemon API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
}

// decodeDaemonAPIData 从 daemon API envelope 中解出 data。
//
// 参数:
//   - body: HTTP 响应体 reader。
//   - target: data 解码目标。
//
// 返回值:
//   - error: 解码失败或 envelope 非成功时返回错误。
func decodeDaemonAPIData(body io.Reader, target any) error {
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode daemon API response: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("daemon API failed: %s", envelope.Msg)
	}
	if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode daemon API data: %w", err)
	}
	return nil
}
