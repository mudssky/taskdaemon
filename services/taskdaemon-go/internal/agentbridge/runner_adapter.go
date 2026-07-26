package agentbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"taskdaemon/internal/runner"
)

// RunnerAdapter 将 Client 适配为 runner.AgentRunner。
type RunnerAdapter struct {
	client *Client
}

// NewRunnerAdapter 创建适配器。
//
// 参数:
//   - client: 出站客户端。
//
// 返回值:
//   - *RunnerAdapter。
func NewRunnerAdapter(client *Client) *RunnerAdapter {
	return &RunnerAdapter{client: client}
}

// CreateAndWaitRun 实现 runner.AgentRunner。
//
// 参数:
//   - ctx: 上下文。
//   - cfg: runner 配置。
//
// 返回值:
//   - runner.Result。
//   - error: 仅配置级错误；网关失败映射到 Result.failed。
func (a *RunnerAdapter) CreateAndWaitRun(ctx context.Context, cfg runner.Config) (runner.Result, error) {
	if a == nil || a.client == nil {
		return runner.Result{
			Status:       runner.StatusFailed,
			ErrorSummary: "agent bridge client is nil",
		}, nil
	}
	started := time.Now()
	traceID := strings.TrimSpace(cfg.AgentTraceID)
	if traceID == "" {
		// 调度层应注入；此处拒绝自行生成，满足「不得重新生成」验收
		return runner.Result{
			Status:       runner.StatusFailed,
			ErrorSummary: "agent traceId missing; scheduler must inject",
			Duration:     time.Since(started),
		}, nil
	}
	result, err := a.client.CreateAndWaitRun(ctx, RunRequest{
		TraceID:   traceID,
		LoopDepth: cfg.AgentLoopDepth,
		RuntimeID: cfg.AgentRuntimeID,
		Profile:   cfg.AgentProfile,
		InputText: firstNonEmpty(cfg.AgentInputText, cfg.Inline),
		ThreadID:  cfg.AgentThreadID,
	})
	duration := time.Since(started)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return runner.Result{
				Status:       runner.StatusCancelled,
				ErrorSummary: "cancelled",
				Duration:     duration,
			}, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return runner.Result{
				Status:       runner.StatusTimeout,
				ErrorSummary: "timeout",
				Duration:     duration,
			}, nil
		}
		// 网关不可用等：映射为 failed Result，便于写执行历史 + C-2 通知
		return runner.Result{
			Status:       runner.StatusFailed,
			ErrorSummary: err.Error(),
			Duration:     duration,
			Stderr:       err.Error(),
		}, nil
	}
	stdout, _ := json.Marshal(map[string]any{
		"threadId": result.ThreadID,
		"runId":    result.RunID,
		"status":   result.Status,
	})
	switch result.Status {
	case "success":
		code := 0
		return runner.Result{
			Status:   runner.StatusSuccess,
			ExitCode: &code,
			Duration: duration,
			Stdout:   string(stdout),
		}, nil
	case "cancelled":
		return runner.Result{
			Status:       runner.StatusCancelled,
			ErrorSummary: firstNonEmpty(result.Error, "cancelled"),
			Duration:     duration,
			Stdout:       string(stdout),
		}, nil
	default:
		return runner.Result{
			Status:       runner.StatusFailed,
			ErrorSummary: firstNonEmpty(result.Error, fmt.Sprintf("agent run status=%s", result.Status)),
			Duration:     duration,
			Stdout:       string(stdout),
			Stderr:       result.Error,
		}, nil
	}
}
