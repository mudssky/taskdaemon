package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"taskdaemon/internal/config"
)

// WebhookSinkName 是 Webhook 出站 sink 的稳定名称。
const WebhookSinkName = "webhook"

// 稳定错误码（T4 / NOTIFY_*）。
const (
	CodeWebhookURLRejected   = "NOTIFY_WEBHOOK_URL_REJECTED"
	CodeWebhookDeliverFailed = "NOTIFY_WEBHOOK_DELIVER_FAILED"
	CodeEmailDeliverFailed   = "NOTIFY_EMAIL_DELIVER_FAILED"
)

// 默认签名 header；接收方可用 HMAC-SHA256 验签。
const defaultSigningHeader = "X-Taskdaemon-Signature"

// WebhookSink 将 C-2 事件以 JSON POST/自定义方法投递到可配置 HTTP 目标。
type WebhookSink struct {
	logger *slog.Logger
	client *http.Client
	// sleep 仅测试注入。
	sleep func(time.Duration)
	// now 可注入时钟，便于测试状态时间戳。
	now func() time.Time

	mu      sync.RWMutex
	cfg     config.NotifyWebhookConfig
	targets map[string]*targetRuntimeStatus
}

// targetRuntimeStatus 记录单个目标的内存投递状态（重启清零）。
type targetRuntimeStatus struct {
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	Enabled         bool       `json:"enabled"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt   *time.Time `json:"lastFailureAt,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	SuccessCount    int64      `json:"successCount"`
	FailureCount    int64      `json:"failureCount"`
	LastDeliveredAt *time.Time `json:"lastDeliveredAt,omitempty"`
}

// WebhookSinkOptions 控制 Webhook sink 可选依赖。
type WebhookSinkOptions struct {
	Logger     *slog.Logger
	HTTPClient *http.Client
	Sleep      func(time.Duration)
	Now        func() time.Time
}

// NewWebhookSink 创建 Webhook 出站 sink。
//
// 参数:
//   - cfg: Webhook 配置。
//   - opts: 可选 logger / HTTP client / 测试时钟。
//
// 返回值:
//   - *WebhookSink: sink 实例。
func NewWebhookSink(cfg config.NotifyWebhookConfig, opts WebhookSinkOptions) *WebhookSink {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &WebhookSink{
		logger:  logger,
		client:  opts.HTTPClient,
		sleep:   opts.Sleep,
		now:     now,
		cfg:     cfg,
		targets: make(map[string]*targetRuntimeStatus),
	}
}

// Name 返回 sink 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 固定为 "webhook"。
func (sink *WebhookSink) Name() string {
	return WebhookSinkName
}

// Enabled 返回 Webhook sink 总开关。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 启用时返回 true。
func (sink *WebhookSink) Enabled() bool {
	if sink == nil {
		return false
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg.Enabled
}

// UpdateConfig 热更新 Webhook 配置段。
//
// 参数:
//   - cfg: 完整通知配置。
//
// 返回值:
//   - 无。
func (sink *WebhookSink) UpdateConfig(cfg config.NotifyConfig) {
	if sink == nil {
		return
	}
	sink.mu.Lock()
	sink.cfg = cfg.Webhook
	sink.mu.Unlock()
}

// webhookConfig 返回当前 Webhook 配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.NotifyWebhookConfig: 当前配置。
func (sink *WebhookSink) webhookConfig() config.NotifyWebhookConfig {
	if sink == nil {
		return config.Default().Notify.Webhook
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg
}

// TargetStatuses 返回各目标投递状态快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []targetRuntimeStatus: 目标状态列表。
func (sink *WebhookSink) TargetStatuses() []targetRuntimeStatus {
	if sink == nil {
		return nil
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	out := make([]targetRuntimeStatus, 0, len(sink.targets))
	for _, status := range sink.targets {
		if status == nil {
			continue
		}
		copyStatus := *status
		out = append(out, copyStatus)
	}
	return out
}

// Deliver 将事件投递到所有已启用目标；任一目标最终失败则返回错误。
//
// 参数:
//   - ctx: 请求上下文。
//   - event: C-2 事件。
//
// 返回值:
//   - error: 目标校验或投递失败时返回 NOTIFY_* 错误。
func (sink *WebhookSink) Deliver(ctx context.Context, event Event) error {
	if sink == nil {
		return fmt.Errorf("%s: webhook sink is nil", CodeWebhookDeliverFailed)
	}
	cfg := sink.webhookConfig()
	if !cfg.Enabled {
		return nil
	}

	payload, err := marshalEventPayload(event)
	if err != nil {
		return fmt.Errorf("%s: marshal payload: %w", CodeWebhookDeliverFailed, err)
	}

	var failures []string
	for _, target := range cfg.Targets {
		if !target.Enabled {
			continue
		}
		if err := sink.deliverTarget(ctx, cfg, target, payload); err != nil {
			// 错误信息已脱敏：不含 secret/header 值。
			failures = append(failures, sanitizeError(err.Error()))
			sink.logger.Error("notify webhook target failed",
				"target", targetDisplayName(target),
				"host", safeURLHost(target.URL),
				"error", sanitizeError(err.Error()),
			)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s: %s", CodeWebhookDeliverFailed, strings.Join(failures, "; "))
	}
	return nil
}

// deliverTarget 对单个目标执行 SSRF 校验、签名、重试投递。
//
// 参数:
//   - ctx: 请求上下文。
//   - cfg: Webhook 全局配置。
//   - target: 目标配置。
//   - payload: 已序列化 JSON body。
//
// 返回值:
//   - error: 失败时返回错误。
func (sink *WebhookSink) deliverTarget(
	ctx context.Context,
	cfg config.NotifyWebhookConfig,
	target config.NotifyWebhookTargetConfig,
	payload []byte,
) error {
	parsed, err := validateWebhookURL(target.URL, cfg)
	if err != nil {
		sink.recordTargetFailure(target, hostOf(parsed), err.Error())
		return err
	}

	method := strings.ToUpper(strings.TrimSpace(target.Method))
	if method == "" {
		method = http.MethodPost
	}
	timeoutSec := target.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = cfg.DefaultTimeoutSec
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	policy := resolveRetry(target.MaxRetries, target.BackoffMs, cfg.DefaultMaxRetries, cfg.DefaultBackoffMs)
	policy.sleep = sink.sleep

	client := sink.httpClient(cfg, timeoutSec)
	err = doWithRetry(ctx, policy, func(attempt int) error {
		req, reqErr := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(payload))
		if reqErr != nil {
			return fmt.Errorf("%s: build request: %v", CodeWebhookDeliverFailed, reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "taskdaemon-notify-webhook/1.0")
		// 自定义 header：值永不写入日志。
		for key, value := range target.Headers {
			if strings.TrimSpace(key) == "" {
				continue
			}
			req.Header.Set(key, value)
		}
		if secret := strings.TrimSpace(target.SigningSecret); secret != "" {
			headerName := strings.TrimSpace(target.SigningHeader)
			if headerName == "" {
				headerName = defaultSigningHeader
			}
			req.Header.Set(headerName, signWebhookPayload(secret, payload))
		}
		resp, doErr := client.Do(req)
		if doErr != nil {
			return fmt.Errorf("%s: request failed (attempt %d): %v", CodeWebhookDeliverFailed, attempt+1, sanitizeError(doErr.Error()))
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("%s: unexpected status %d (attempt %d)", CodeWebhookDeliverFailed, resp.StatusCode, attempt+1)
		}
		return nil
	})
	if err != nil {
		sink.recordTargetFailure(target, hostOf(parsed), err.Error())
		return err
	}
	sink.recordTargetSuccess(target, hostOf(parsed))
	return nil
}

// httpClient 构造带超时与重定向校验的 HTTP client。
//
// 参数:
//   - cfg: Webhook 配置。
//   - timeoutSec: 单次请求超时秒数。
//
// 返回值:
//   - *http.Client: HTTP 客户端。
func (sink *WebhookSink) httpClient(cfg config.NotifyWebhookConfig, timeoutSec int) *http.Client {
	if sink.client != nil {
		// 测试注入 client 时复用，但仍覆盖 CheckRedirect。
		clone := *sink.client
		clone.Timeout = time.Duration(timeoutSec) * time.Second
		clone.CheckRedirect = webhookRedirectPolicy(cfg)
		return &clone
	}
	return &http.Client{
		Timeout:       time.Duration(timeoutSec) * time.Second,
		CheckRedirect: webhookRedirectPolicy(cfg),
	}
}

// webhookRedirectPolicy 限制重定向次数并在每跳复检 SSRF。
//
// 参数:
//   - cfg: Webhook 安全配置。
//
// 返回值:
//   - func(*http.Request, []*http.Request) error: CheckRedirect 回调。
func webhookRedirectPolicy(cfg config.NotifyWebhookConfig) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		maxRedirects := cfg.MaxRedirects
		if maxRedirects < 0 {
			maxRedirects = 0
		}
		if len(via) > maxRedirects {
			return fmt.Errorf("%s: redirects", CodeWebhookURLRejected)
		}
		_, err := validateWebhookURL(req.URL.String(), cfg)
		return err
	}
}

// marshalEventPayload 将 C-2 事件序列化为 JSON，并二次过滤 Detail。
//
// 参数:
//   - event: 待序列化事件。
//
// 返回值:
//   - []byte: JSON body。
//   - error: 序列化失败时返回错误。
func marshalEventPayload(event Event) ([]byte, error) {
	payload := map[string]any{
		"id":         event.ID,
		"name":       string(event.Name),
		"severity":   string(event.Severity),
		"occurredAt": event.OccurredAt.UTC().Format(time.RFC3339Nano),
		"subject": map[string]any{
			"kind": string(event.Subject.Kind),
			"id":   event.Subject.ID,
		},
		"title": event.Title,
		"body":  event.Body,
	}
	if detail := filterDetail(event.Detail); len(detail) > 0 {
		payload["detail"] = detail
	}
	return json.Marshal(payload)
}

// signWebhookPayload 计算 HMAC-SHA256 签名 header 值。
//
// 参数:
//   - secret: 签名密钥。
//   - payload: 原始 body。
//
// 返回值:
//   - string: 形如 "sha256=<hex>" 的签名。
//
// 接收方验签示例（伪代码）:
//
//	expected := hmac_sha256_hex(secret, body)
//	assert header == "sha256="+expected
func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature 供测试与文档对齐的验签辅助。
//
// 参数:
//   - secret: 签名密钥。
//   - payload: 原始 body。
//   - headerValue: 请求头中的签名值。
//
// 返回值:
//   - bool: 验签通过时返回 true。
func VerifyWebhookSignature(secret string, payload []byte, headerValue string) bool {
	expected := signWebhookPayload(secret, payload)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(headerValue)))
}

// recordTargetSuccess 更新目标成功状态。
//
// 参数:
//   - target: 目标配置。
//   - host: 目标 host。
//
// 返回值:
//   - 无。
func (sink *WebhookSink) recordTargetSuccess(target config.NotifyWebhookTargetConfig, host string) {
	now := sink.now()
	sink.mu.Lock()
	defer sink.mu.Unlock()
	status := sink.ensureTargetStatus(target, host)
	status.SuccessCount++
	status.LastSuccessAt = &now
	status.LastDeliveredAt = &now
	status.LastError = ""
	status.Enabled = target.Enabled
}

// recordTargetFailure 更新目标失败状态。
//
// 参数:
//   - target: 目标配置。
//   - host: 目标 host。
//   - message: 失败原因（应已脱敏）。
//
// 返回值:
//   - 无。
func (sink *WebhookSink) recordTargetFailure(target config.NotifyWebhookTargetConfig, host string, message string) {
	now := sink.now()
	sink.mu.Lock()
	defer sink.mu.Unlock()
	status := sink.ensureTargetStatus(target, host)
	status.FailureCount++
	status.LastFailureAt = &now
	status.LastDeliveredAt = &now
	status.LastError = sanitizeError(message)
	status.Enabled = target.Enabled
}

// ensureTargetStatus 获取或创建目标状态槽。
//
// 参数:
//   - target: 目标配置。
//   - host: 目标 host。
//
// 返回值:
//   - *targetRuntimeStatus: 可变状态指针。
func (sink *WebhookSink) ensureTargetStatus(target config.NotifyWebhookTargetConfig, host string) *targetRuntimeStatus {
	key := targetDisplayName(target)
	if sink.targets == nil {
		sink.targets = make(map[string]*targetRuntimeStatus)
	}
	status, ok := sink.targets[key]
	if !ok || status == nil {
		if host == "" {
			host = safeURLHost(target.URL)
		}
		status = &targetRuntimeStatus{
			Name:    key,
			Host:    host,
			Enabled: target.Enabled,
		}
		sink.targets[key] = status
	}
	return status
}

// hostOf 安全提取 hostname。
//
// 参数:
//   - parsed: 已解析 URL。
//
// 返回值:
//   - string: hostname；nil 时返回 unknown。
func hostOf(parsed *url.URL) string {
	if parsed == nil {
		return "unknown"
	}
	return parsed.Hostname()
}

// targetDisplayName 返回日志/状态用的目标名（不含 secret）。
//
// 参数:
//   - target: 目标配置。
//
// 返回值:
//   - string: 目标显示名。
func targetDisplayName(target config.NotifyWebhookTargetConfig) string {
	if name := strings.TrimSpace(target.Name); name != "" {
		return name
	}
	return safeURLHost(target.URL)
}

// safeURLHost 从 URL 提取 host，失败时返回 "unknown"。
//
// 参数:
//   - rawURL: 原始 URL。
//
// 返回值:
//   - string: hostname。
func safeURLHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "unknown"
	}
	// 轻量解析：避免把 query 中的 token 打进日志。
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		rest := rawURL[idx+3:]
		if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
			rest = rest[:slash]
		}
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if host, _, ok := strings.Cut(rest, ":"); ok {
			return host
		}
		return rest
	}
	return "unknown"
}

// sanitizeError 去除错误串中可能夹带的敏感片段。
//
// 参数:
//   - message: 原始错误。
//
// 返回值:
//   - string: 可安全记录的错误摘要。
func sanitizeError(message string) string {
	lower := strings.ToLower(message)
	// 保守：若含常见密钥字段名，整体替换。
	needles := []string{"password=", "token=", "authorization:", "signingsecret", "api_key", "apikey"}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return CodeWebhookDeliverFailed + ": redacted error"
		}
	}
	// 截断过长错误，避免 body 泄漏。
	const maxLen = 240
	if len(message) > maxLen {
		return message[:maxLen] + "..."
	}
	return message
}
