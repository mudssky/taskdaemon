package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"taskdaemon/internal/notify"
)

// CapabilityNotification 是 Desktop 原生通知能力名（C-5 / D2）。
const CapabilityNotification = "desktop.notification"

// notificationBackend 抽象 Wails 通知服务，便于单测注入。
type notificationBackend interface {
	RequestNotificationAuthorization() (bool, error)
	CheckNotificationAuthorization() (bool, error)
	SendNotification(options notifications.NotificationOptions) error
}

// notificationCapability 实现 desktop.notification 能力，并作为 notify.DesktopSender。
type notificationCapability struct {
	backend notificationBackend
	// activateMainWindow 点击通知时激活主窗口；可空。
	activateMainWindow func()

	mu sync.RWMutex
	// deniedOnce 记录用户曾拒绝授权，避免自动重弹（仅影响自动路径；显式 request 仍允许）。
	deniedOnce bool
}

// NotificationCapabilityOptions 控制通知能力可选依赖。
type NotificationCapabilityOptions struct {
	// Backend 覆盖默认 Wails 服务；测试注入 stub。
	Backend notificationBackend
	// ActivateMainWindow 点击通知默认动作时调用。
	ActivateMainWindow func()
}

// NewNotificationCapability 创建 Desktop 通知 capability。
//
// 参数:
//   - service: Wails 通知服务；可为 nil（此时需 opts.Backend）。
//   - opts: 可选 backend / 窗口激活回调。
//
// 返回值:
//   - *notificationCapability: 可 Register 且可 Bind 到 notify 的实现。
func NewNotificationCapability(service *notifications.NotificationService, opts NotificationCapabilityOptions) *notificationCapability {
	backend := opts.Backend
	if backend == nil && service != nil {
		backend = service
	}
	return &notificationCapability{
		backend:            backend,
		activateMainWindow: opts.ActivateMainWindow,
	}
}

// Name 返回 capability 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: desktop.notification。
func (c *notificationCapability) Name() string {
	return CapabilityNotification
}

// Available 在后端存在时返回可用。
// 权限状态不在此折叠为 unavailable，以便前端仍能 invoke requestPermission。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 后端可用时 true。
//   - UnavailableReason: 不可用原因。
//   - string: 人类可读说明。
func (c *notificationCapability) Available() (bool, UnavailableReason, string) {
	if c == nil || c.backend == nil {
		return false, ReasonPlatform, "notification service is not available"
	}
	return true, "", ""
}

// notificationInvokePayload 是 capability Invoke 的请求形状。
type notificationInvokePayload struct {
	Action string `json:"action"`
	Title  string `json:"title,omitempty"`
	Body   string `json:"body,omitempty"`
	ID     string `json:"id,omitempty"`
}

// notificationStatusResult 是 status 动作返回。
type notificationStatusResult struct {
	Authorized bool `json:"authorized"`
	// CanRequest 表示是否仍建议发起系统授权请求。
	CanRequest bool `json:"canRequest"`
	DeniedOnce bool `json:"deniedOnce"`
}

// notificationShowResult 是 show / requestPermission 成功返回。
type notificationShowResult struct {
	OK         bool `json:"ok"`
	Authorized bool `json:"authorized,omitempty"`
}

// Invoke 分发 status / requestPermission / show。
//
// 参数:
//   - ctx: 调用上下文。
//   - payload: JSON；空视为 status。
//
// 返回值:
//   - []byte: JSON 结果。
//   - error: 结构化 CapabilityError 或包装错误。
func (c *notificationCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	if c == nil || c.backend == nil {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: "notification service is not available",
		}
	}

	action := "status"
	req := notificationInvokePayload{}
	if len(payload) > 0 && string(payload) != "null" && string(payload) != `""` {
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &CapabilityError{
				Code:    ErrCodeInvokeFailed,
				Message: "invalid notification payload: " + err.Error(),
			}
		}
		if req.Action != "" {
			action = req.Action
		}
	}

	switch action {
	case "status":
		return c.invokeStatus()
	case "requestPermission":
		return c.invokeRequestPermission()
	case "show":
		return c.invokeShow(ctx, req)
	default:
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "unknown notification action: " + action,
		}
	}
}

// invokeStatus 查询授权状态（不弹系统对话框）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []byte: status JSON。
//   - error: 检查失败时返回。
func (c *notificationCapability) invokeStatus() ([]byte, error) {
	authorized, err := c.backend.CheckNotificationAuthorization()
	if err != nil {
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "check notification authorization: " + err.Error(),
		}
	}
	c.mu.RLock()
	denied := c.deniedOnce
	c.mu.RUnlock()
	result := notificationStatusResult{
		Authorized: authorized,
		CanRequest: !authorized,
		DeniedOnce: denied,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// invokeRequestPermission 显式请求系统通知权限。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []byte: 授权结果 JSON。
//   - error: 请求失败时返回。
func (c *notificationCapability) invokeRequestPermission() ([]byte, error) {
	authorized, err := c.backend.RequestNotificationAuthorization()
	if err != nil {
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "request notification authorization: " + err.Error(),
		}
	}
	if !authorized {
		c.mu.Lock()
		c.deniedOnce = true
		c.mu.Unlock()
	}
	data, err := json.Marshal(notificationShowResult{OK: authorized, Authorized: authorized})
	if err != nil {
		return nil, err
	}
	if !authorized {
		return data, &CapabilityError{
			Code:    ErrCodePermissionDenied,
			Message: "notification permission denied",
		}
	}
	return data, nil
}

// invokeShow 发送一条系统通知。
//
// 参数:
//   - ctx: 请求上下文。
//   - req: 含 title/body/id 的请求。
//
// 返回值:
//   - []byte: 成功标记 JSON。
//   - error: 权限或发送失败。
func (c *notificationCapability) invokeShow(ctx context.Context, req notificationInvokePayload) ([]byte, error) {
	if err := c.SendNotification(ctx, req.Title, req.Body, req.ID); err != nil {
		return nil, err
	}
	data, err := json.Marshal(notificationShowResult{OK: true})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SendNotification 实现 notify.DesktopSender，供事件总线 sink 调用。
// 不自动弹出授权对话框；未授权时返回 DESKTOP_PERMISSION_DENIED。
//
// 参数:
//   - ctx: 请求上下文（当前后端 API 不感知，保留形态）。
//   - title: 标题。
//   - body: 正文。
//   - id: 通知 ID。
//
// 返回值:
//   - error: 结构化能力错误或发送错误。
func (c *notificationCapability) SendNotification(ctx context.Context, title, body, id string) error {
	_ = ctx
	if c == nil || c.backend == nil {
		return &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: "notification service is not available",
		}
	}
	authorized, err := c.backend.CheckNotificationAuthorization()
	if err != nil {
		return &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "check notification authorization: " + err.Error(),
		}
	}
	if !authorized {
		return &CapabilityError{
			Code:    ErrCodePermissionDenied,
			Message: "notification permission not granted",
		}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "taskdaemon"
	}
	options := notifications.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  body,
	}
	if err := c.backend.SendNotification(options); err != nil {
		return &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "send notification: " + err.Error(),
		}
	}
	return nil
}

// HandleNotificationResponse 处理用户点击通知；默认动作激活主窗口。
//
// 参数:
//   - result: Wails 通知回调结果。
//
// 返回值:
//   - 无。
func (c *notificationCapability) HandleNotificationResponse(result notifications.NotificationResult) {
	if c == nil {
		return
	}
	if result.Error != nil {
		return
	}
	action := result.Response.ActionIdentifier
	if action != "" &&
		action != notifications.DefaultActionIdentifier &&
		action != "com.apple.UNNotificationDefaultActionIdentifier" {
		return
	}
	if c.activateMainWindow != nil {
		// 防御窗口 API 异常，避免回调路径 panic 杀进程。
		defer func() {
			_ = recover()
		}()
		c.activateMainWindow()
	}
}

// bindAsDesktopSender 将本能力绑定到 notify 总线（进程级）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (c *notificationCapability) bindAsDesktopSender() {
	if c == nil {
		notify.UnbindDesktopSender()
		return
	}
	notify.BindDesktopSender(c)
}

// ensure interface compliance.
var (
	_ Capability           = (*notificationCapability)(nil)
	_ notify.DesktopSender = (*notificationCapability)(nil)
)

// formatAuthDenied 构造权限拒绝说明（测试辅助保留）。
//
// 参数:
//   - detail: 附加说明。
//
// 返回值:
//   - string: 错误消息。
func formatAuthDenied(detail string) string {
	if detail == "" {
		return "notification permission denied"
	}
	return fmt.Sprintf("notification permission denied: %s", detail)
}
