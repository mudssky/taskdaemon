package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// CapabilityWindowState 是窗口状态能力名。
const CapabilityWindowState = "desktop.window-state"

// WindowStateInfo 是 get action 的返回形状。
type WindowStateInfo struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Maximised bool   `json:"maximised"`
	Path      string `json:"path"`
	Enabled   bool   `json:"enabled"`
}

// windowStateActionPayload 是 invoke 请求体。
type windowStateActionPayload struct {
	Action string `json:"action"`
}

// windowStateHost 绑定主窗口并负责持久化。
type windowStateHost struct {
	mu      sync.RWMutex
	enabled bool
	path    string
	window  *application.WebviewWindow
	state   WindowState
}

// newWindowStateHost 创建窗口状态 host。
//
// 参数:
//   - enabled: 是否启用持久化。
//   - path: 状态文件路径。
//
// 返回值:
//   - *windowStateHost: host 实例。
func newWindowStateHost(enabled bool, path string) *windowStateHost {
	return &windowStateHost{
		enabled: enabled,
		path:    path,
		state:   DefaultWindowState(),
	}
}

// LoadForStartup 读取并夹取启动几何（在创建窗口前调用）。
//
// 参数:
//   - displays: 当前显示器工作区；可为空。
//
// 返回值:
//   - WindowState: 用于建窗的状态。
//   - bool: true 表示应使用居中初始位置（无可靠坐标）。
func (h *windowStateHost) LoadForStartup(displays []DisplayRect) (WindowState, bool) {
	if h == nil || !h.enabled {
		return DefaultWindowState(), true
	}
	state, ok := LoadWindowState(h.path)
	if !ok {
		return DefaultWindowState(), true
	}
	clamped, usedDefault := ClampWindowState(state, displays)
	h.mu.Lock()
	h.state = clamped
	h.mu.Unlock()
	return clamped, usedDefault
}

// Attach 绑定主窗口并监听几何变化。
//
// 参数:
//   - window: 主窗口。
//
// 返回值:
//   - 无。
func (h *windowStateHost) Attach(window *application.WebviewWindow) {
	if h == nil || window == nil {
		return
	}
	h.mu.Lock()
	h.window = window
	h.mu.Unlock()
	if !h.enabled {
		return
	}
	persist := func() {
		h.CaptureAndSave()
	}
	window.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		persist()
	})
	window.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
		persist()
	})
	window.OnWindowEvent(events.Common.WindowMaximise, func(_ *application.WindowEvent) {
		persist()
	})
	window.OnWindowEvent(events.Common.WindowUnMaximise, func(_ *application.WindowEvent) {
		persist()
	})
}

// CaptureAndSave 从窗口读取几何并写盘。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 写盘失败。
func (h *windowStateHost) CaptureAndSave() error {
	if h == nil || !h.enabled {
		return nil
	}
	h.mu.RLock()
	window := h.window
	path := h.path
	h.mu.RUnlock()
	if window == nil {
		return nil
	}
	bounds := window.Bounds()
	state := WindowState{
		X:         bounds.X,
		Y:         bounds.Y,
		Width:     bounds.Width,
		Height:    bounds.Height,
		Maximised: window.IsMaximised(),
	}
	if !isSaneWindowState(state) {
		return nil
	}
	h.mu.Lock()
	h.state = state
	h.mu.Unlock()
	return SaveWindowState(path, state)
}

// Snapshot 返回当前状态信息。
//
// 参数:
//   - 无。
//
// 返回值:
//   - WindowStateInfo: 快照。
func (h *windowStateHost) Snapshot() WindowStateInfo {
	if h == nil {
		def := DefaultWindowState()
		return WindowStateInfo{
			X: def.X, Y: def.Y, Width: def.Width, Height: def.Height,
			Path: WindowStatePath(), Enabled: false,
		}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return WindowStateInfo{
		X:         h.state.X,
		Y:         h.state.Y,
		Width:     h.state.Width,
		Height:    h.state.Height,
		Maximised: h.state.Maximised,
		Path:      h.path,
		Enabled:   h.enabled,
	}
}

// Reset 恢复默认布局并写盘。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 失败原因。
func (h *windowStateHost) Reset() error {
	if h == nil {
		return fmt.Errorf("window state host nil")
	}
	def := DefaultWindowState()
	h.mu.Lock()
	h.state = def
	window := h.window
	path := h.path
	enabled := h.enabled
	h.mu.Unlock()
	if window != nil {
		window.UnMaximise()
		window.SetSize(def.Width, def.Height)
		window.Center()
	}
	if enabled {
		return SaveWindowState(path, def)
	}
	return nil
}

// Available 返回能力是否可用。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是否可用。
//   - UnavailableReason: 原因。
//   - string: 消息。
func (h *windowStateHost) Available() (bool, UnavailableReason, string) {
	if h == nil || !h.enabled {
		return false, ReasonPlatform, "窗口状态持久化已关闭"
	}
	return true, "", ""
}

// windowStateController 供 capability 注入。
type windowStateController interface {
	Available() (bool, UnavailableReason, string)
	Snapshot() WindowStateInfo
	Reset() error
}

// windowStateCapability 实现 desktop.window-state。
type windowStateCapability struct {
	host windowStateController
}

// NewWindowStateCapability 创建窗口状态 capability。
//
// 参数:
//   - host: 控制器。
//
// 返回值:
//   - Capability: 可 Register 的实现。
func NewWindowStateCapability(host windowStateController) Capability {
	return &windowStateCapability{host: host}
}

// Name 返回 capability 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: desktop.window-state。
func (c *windowStateCapability) Name() string {
	return CapabilityWindowState
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
func (c *windowStateCapability) Available() (bool, UnavailableReason, string) {
	if c.host == nil {
		return false, ReasonPlatform, "window-state host 未装配"
	}
	return c.host.Available()
}

// Invoke 分发 get/reset。
//
// 参数:
//   - ctx: 调用上下文。
//   - payload: JSON。
//
// 返回值:
//   - []byte: JSON 结果。
//   - error: 结构化错误。
func (c *windowStateCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	if ok, reason, message := c.Available(); !ok {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityUnavailable,
			Message: fmt.Sprintf("%s: %s", reason, message),
		}
	}
	var req windowStateActionPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &CapabilityError{Code: ErrCodeInvokeFailed, Message: "invalid window-state payload"}
		}
	}
	if req.Action == "" {
		req.Action = "get"
	}
	switch req.Action {
	case "get":
		info := c.host.Snapshot()
		return json.Marshal(info)
	case "reset":
		if err := c.host.Reset(); err != nil {
			return nil, &CapabilityError{Code: ErrCodeInvokeFailed, Message: err.Error()}
		}
		info := c.host.Snapshot()
		return json.Marshal(info)
	default:
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: "unknown window-state action: " + req.Action,
		}
	}
}
