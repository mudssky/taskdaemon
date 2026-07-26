package desktop

import (
	"context"
	"encoding/json"
	"fmt"
)

// CapabilityTray 是托盘能力名。
const CapabilityTray = "desktop.tray"

// TrayStatusInfo 是 tray status action 的返回形状。
type TrayStatusInfo struct {
	ServiceStatus  TrayServiceStatus `json:"serviceStatus"`
	MinimizeToTray bool              `json:"minimizeToTray"`
	Label          string            `json:"label"`
}

// trayActionPayload 是 tray invoke 的请求体。
type trayActionPayload struct {
	Action  string `json:"action"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// trayController 抽象托盘运行时，便于单测注入。
type trayController interface {
	Available() (bool, UnavailableReason, string)
	ShowMainWindow()
	MinimizeToTrayEnabled() bool
	SetMinimizeToTray(enabled bool)
	StatusSnapshot() TrayStatusInfo
}

// trayCapability 实现 desktop.tray。
type trayCapability struct {
	host trayController
}

// NewTrayCapability 创建托盘 capability。
//
// 参数:
//   - host: 托盘控制器；nil 时始终 platform 不可用。
//
// 返回值:
//   - Capability: 可 Register 的实现。
func NewTrayCapability(host trayController) Capability {
	return &trayCapability{host: host}
}

// Name 返回 capability 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: desktop.tray。
func (c *trayCapability) Name() string {
	return CapabilityTray
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
func (c *trayCapability) Available() (bool, UnavailableReason, string) {
	if c.host == nil {
		return false, ReasonPlatform, "托盘 host 未装配"
	}
	return c.host.Available()
}

// Invoke 分发 tray action。
//
// 参数:
//   - ctx: 调用上下文。
//   - payload: JSON，含 action。
//
// 返回值:
//   - []byte: JSON 结果。
//   - error: 结构化 CapabilityError。
func (c *trayCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	if ok, reason, message := c.Available(); !ok {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: fmt.Sprintf("%s: %s", reason, message),
		}
	}
	var req trayActionPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &CapabilityError{Code: ErrCodeInvokeFailed, Message: "invalid tray payload"}
		}
	}
	if req.Action == "" {
		req.Action = "status"
	}
	switch req.Action {
	case "status":
		info := c.host.StatusSnapshot()
		return json.Marshal(info)
	case "showWindow":
		c.host.ShowMainWindow()
		return json.Marshal(map[string]any{"ok": true})
	case "setMinimizeToTray":
		if req.Enabled == nil {
			return nil, &CapabilityError{Code: ErrCodeInvokeFailed, Message: "enabled required"}
		}
		c.host.SetMinimizeToTray(*req.Enabled)
		return json.Marshal(map[string]any{
			"ok":             true,
			"minimizeToTray": c.host.MinimizeToTrayEnabled(),
		})
	default:
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "unknown tray action: " + req.Action,
		}
	}
}
