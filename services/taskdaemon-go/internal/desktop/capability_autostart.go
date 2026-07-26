package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CapabilityAutostart 是 Desktop 应用自启动能力名。
const CapabilityAutostart = "desktop.autostart"

// AutostartStatusInfo 是 autostart status 的返回形状。
type AutostartStatusInfo struct {
	Enabled  bool   `json:"enabled"`
	Strategy string `json:"strategy,omitempty"`
	Path     string `json:"path,omitempty"`
	Note     string `json:"note"`
}

// autostartActionPayload 是 invoke 请求体。
type autostartActionPayload struct {
	Action string `json:"action"`
}

// autostartController 抽象登录项控制。
type autostartController interface {
	Available() (bool, UnavailableReason, string)
	Status() (AutostartStatusInfo, error)
	Enable() error
	Disable() error
}

// autostartCapability 实现 desktop.autostart。
type autostartCapability struct {
	host autostartController
}

// NewAutostartCapability 创建自启动 capability。
//
// 参数:
//   - host: 自启动控制器。
//
// 返回值:
//   - Capability: 可 Register 的实现。
func NewAutostartCapability(host autostartController) Capability {
	return &autostartCapability{host: host}
}

// Name 返回 capability 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: desktop.autostart。
func (c *autostartCapability) Name() string {
	return CapabilityAutostart
}

// Available 委托 host。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是否可用。
//   - UnavailableReason: 原因。
//   - string: 消息。
func (c *autostartCapability) Available() (bool, UnavailableReason, string) {
	if c.host == nil {
		return false, ReasonPlatform, "autostart host 未装配"
	}
	return c.host.Available()
}

// Invoke 分发 enable/disable/status。
//
// 参数:
//   - ctx: 调用上下文。
//   - payload: JSON。
//
// 返回值:
//   - []byte: JSON 结果。
//   - error: 结构化错误。
func (c *autostartCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	if ok, reason, message := c.Available(); !ok {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: fmt.Sprintf("%s: %s", reason, message),
		}
	}
	var req autostartActionPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &CapabilityError{Code: ErrCodeInvokeFailed, Message: "invalid autostart payload"}
		}
	}
	if req.Action == "" {
		req.Action = "status"
	}
	switch req.Action {
	case "status":
		info, err := c.host.Status()
		if err != nil {
			return nil, mapAutostartError(err)
		}
		if info.Note == "" {
			info.Note = AutostartVsServiceNote
		}
		return json.Marshal(info)
	case "enable":
		if err := c.host.Enable(); err != nil {
			return nil, mapAutostartError(err)
		}
		info, _ := c.host.Status()
		if info.Note == "" {
			info.Note = AutostartVsServiceNote
		}
		return json.Marshal(info)
	case "disable":
		if err := c.host.Disable(); err != nil {
			return nil, mapAutostartError(err)
		}
		info, _ := c.host.Status()
		if info.Note == "" {
			info.Note = AutostartVsServiceNote
		}
		return json.Marshal(info)
	default:
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "unknown autostart action: " + req.Action,
		}
	}
}

// mapAutostartError 将底层错误映射为 DESKTOP_* 码。
//
// 参数:
//   - err: 原始错误。
//
// 返回值:
//   - error: CapabilityError。
func mapAutostartError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, application.ErrAutostartNotSupported) {
		return &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: err.Error(),
		}
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "permission") || strings.Contains(lower, "denied") || strings.Contains(lower, "not authorized") {
		return &CapabilityError{Code: ErrCodePermissionDenied, Message: msg}
	}
	return &CapabilityError{Code: ErrCodeInvokeFailed, Message: msg}
}
