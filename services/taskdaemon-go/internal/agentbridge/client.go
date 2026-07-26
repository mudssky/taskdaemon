// Package agentbridge 实现 G6 双引擎互调：taskdaemon → agent-gateway 出站客户端，
// 以及入站 token / 环路 / 任务白名单校验。
// 不使用管理员 cookie；凭据不进入日志。
package agentbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taskdaemon/internal/config"
)

// 环路深度请求头（双向约定）。
const HeaderLoopDepth = "X-Agent-Loop-Depth"

// loopDepthContextKey 在 context 中传递环路深度。
type loopDepthContextKey struct{}

// WithLoopDepth 将深度写入 context。
//
// 参数:
//   - ctx: 父 context。
//   - depth: 深度。
//
// 返回值:
//   - context.Context。
func WithLoopDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, loopDepthContextKey{}, depth)
}

// LoopDepthFromContext 读取深度。
//
// 参数:
//   - ctx: context。
//
// 返回值:
//   - int: 深度。
//   - bool: 是否存在。
func LoopDepthFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(loopDepthContextKey{})
	if v == nil {
		return 0, false
	}
	n, ok := v.(int)
	return n, ok
}

// 错误定义。
var (
	// ErrUnauthorized 表示入站 Bearer 无效。
	ErrUnauthorized = errors.New("agent bridge token is invalid")
	// ErrDisabled 表示桥接未启用。
	ErrDisabled = errors.New("agent bridge is disabled")
	// ErrTaskNotAllowed 表示任务不在 agent 可触发白名单。
	ErrTaskNotAllowed = errors.New("task is not allowed for agent trigger")
	// ErrLoopDetected 表示环路深度超限。
	ErrLoopDetected = errors.New("agent bridge loop depth exceeded")
	// ErrGatewayUnavailable 表示 agent-gateway 不可达或返回错误。
	ErrGatewayUnavailable = errors.New("agent gateway unavailable")
)

// HashToken 返回 Bearer Token 的 SHA-256 hex。
//
// 参数:
//   - token: 明文 token。
//
// 返回值:
//   - string: hex 摘要。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// AuthenticateInbound 校验入站 Bearer Token（机器认证，非管理员 cookie）。
//
// 参数:
//   - cfg: AgentBridge 配置。
//   - token: Bearer 原文。
//
// 返回值:
//   - error: 未启用、未配置 hash、token 不匹配时返回错误。
func AuthenticateInbound(cfg config.AgentBridgeConfig, token string) error {
	if !cfg.Enabled {
		return ErrDisabled
	}
	token = strings.TrimSpace(token)
	if cfg.InboundTokenHash == "" || token == "" {
		return ErrUnauthorized
	}
	sum := sha256.Sum256([]byte(token))
	got := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(cfg.InboundTokenHash)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

// AssertTaskAllowed 检查任务是否允许被 agent 触发。
//
// 参数:
//   - cfg: 配置。
//   - taskID: 任务 ID。
//
// 返回值:
//   - error: 不在白名单时返回 ErrTaskNotAllowed。
func AssertTaskAllowed(cfg config.AgentBridgeConfig, taskID int) error {
	if !cfg.Enabled {
		return ErrDisabled
	}
	if len(cfg.AllowedTaskIDs) == 0 {
		return ErrTaskNotAllowed
	}
	for _, id := range cfg.AllowedTaskIDs {
		if id == taskID {
			return nil
		}
	}
	return ErrTaskNotAllowed
}

// AssertLoopDepth 校验环路深度。
// depth >= MaxLoopDepth 时拒绝（默认 MaxLoopDepth=1：depth 0 可入，1 及以上拒绝）。
//
// 参数:
//   - cfg: 配置。
//   - depth: 请求携带的深度（缺省 0）。
//
// 返回值:
//   - error: 超限时返回 ErrLoopDetected。
func AssertLoopDepth(cfg config.AgentBridgeConfig, depth int) error {
	max := cfg.MaxLoopDepth
	if max <= 0 {
		max = 1
	}
	if depth < 0 {
		depth = 0
	}
	if depth >= max {
		return ErrLoopDetected
	}
	return nil
}

// ParseLoopDepth 解析环路深度头。
//
// 参数:
//   - raw: 头值。
//
// 返回值:
//   - int: 深度，非法时 0。
func ParseLoopDepth(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// RunRequest 描述一次 taskdaemon → agent-gateway 的 run 触发。
type RunRequest struct {
	// TraceID 必须透传，禁止在客户端重新生成覆盖。
	TraceID string
	// LoopDepth 当前环路深度，写入请求头。
	LoopDepth int
	// RuntimeID agent runtime。
	RuntimeID string
	// Profile coding|general。
	Profile string
	// InputText 用户输入。
	InputText string
	// ThreadID 可选已有 thread。
	ThreadID string
}

// RunResult agent run 终态摘要。
type RunResult struct {
	ThreadID string
	RunID    string
	Status   string
	Error    string
}

// Client 出站 HTTP 客户端。
type Client struct {
	cfg        config.AgentBridgeConfig
	httpClient *http.Client
}

// NewClient 创建出站客户端。
//
// 参数:
//   - cfg: AgentBridge 配置。
//
// 返回值:
//   - *Client。
func NewClient(cfg config.AgentBridgeConfig) *Client {
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// CreateAndWaitRun 创建 thread（如需）并执行 run，轮询至终态。
//
// 参数:
//   - ctx: 上下文。
//   - req: 运行请求。
//
// 返回值:
//   - RunResult: 终态。
//   - error: 网关不可用或协议错误。
func (c *Client) CreateAndWaitRun(ctx context.Context, req RunRequest) (RunResult, error) {
	if c == nil || !c.cfg.Enabled {
		return RunResult{}, ErrDisabled
	}
	if strings.TrimSpace(c.cfg.GatewayBaseURL) == "" {
		return RunResult{}, fmt.Errorf("%w: empty gateway base url", ErrGatewayUnavailable)
	}
	if strings.TrimSpace(req.TraceID) == "" {
		return RunResult{}, fmt.Errorf("traceId is required for agent bridge")
	}

	threadID := strings.TrimSpace(req.ThreadID)
	if threadID == "" {
		id, err := c.createThread(ctx, req)
		if err != nil {
			return RunResult{}, err
		}
		threadID = id
	}

	runID, err := c.createRun(ctx, threadID, req)
	if err != nil {
		return RunResult{}, err
	}

	return c.waitRun(ctx, threadID, runID, req.TraceID, req.LoopDepth)
}

func (c *Client) createThread(ctx context.Context, req RunRequest) (string, error) {
	body := map[string]any{
		"runtimeId": firstNonEmpty(req.RuntimeID, "mock-high"),
		"profile":   firstNonEmpty(req.Profile, "general"),
	}
	var out struct {
		ThreadID string `json:"threadId"`
		Error    *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/threads", req.TraceID, req.LoopDepth, body, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("%w: %s", ErrGatewayUnavailable, out.Error.Message)
	}
	if out.ThreadID == "" {
		return "", fmt.Errorf("%w: empty threadId", ErrGatewayUnavailable)
	}
	return out.ThreadID, nil
}

func (c *Client) createRun(ctx context.Context, threadID string, req RunRequest) (string, error) {
	body := map[string]any{
		"input": map[string]any{
			"text": req.InputText,
		},
	}
	var out struct {
		RunID  string `json:"runId"`
		Status string `json:"status"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	path := fmt.Sprintf("/v1/threads/%s/runs", threadID)
	if err := c.doJSON(ctx, http.MethodPost, path, req.TraceID, req.LoopDepth, body, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("%w: %s", ErrGatewayUnavailable, out.Error.Message)
	}
	if out.RunID == "" {
		return "", fmt.Errorf("%w: empty runId", ErrGatewayUnavailable)
	}
	return out.RunID, nil
}

func (c *Client) waitRun(ctx context.Context, threadID, runID, traceID string, depth int) (RunResult, error) {
	path := fmt.Sprintf("/v1/threads/%s/runs/%s", threadID, runID)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var out struct {
			RunID  string `json:"runId"`
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := c.doJSON(ctx, http.MethodGet, path, traceID, depth, nil, &out); err != nil {
			return RunResult{}, err
		}
		switch out.Status {
		case "success", "error", "cancelled", "interrupted":
			msg := ""
			if out.Error != nil {
				msg = out.Error.Message
			}
			return RunResult{
				ThreadID: threadID,
				RunID:    runID,
				Status:   out.Status,
				Error:    msg,
			}, nil
		}
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, path, traceID string, depth int, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	url := strings.TrimRight(c.cfg.GatewayBaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// 机器身份头；禁止使用管理员 cookie
	req.Header.Set("x-auth-subject", firstNonEmpty(c.cfg.GatewaySubject, "taskdaemon-service"))
	req.Header.Set("x-tenant-id", firstNonEmpty(c.cfg.GatewayTenantID, "system"))
	req.Header.Set("x-trace-id", traceID)
	req.Header.Set(HeaderLoopDepth, strconv.Itoa(depth))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGatewayUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", ErrGatewayUnavailable, err)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: status %d", ErrGatewayUnavailable, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrGatewayUnavailable, err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: status %d body=%s", ErrGatewayUnavailable, resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
