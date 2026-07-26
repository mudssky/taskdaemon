package desktop

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AutostartVsServiceNote 明确区分 Desktop 自启动与 T5 系统服务。
const AutostartVsServiceNote = "此项仅控制 Desktop 图形应用在用户登录时启动，不是 taskdaemon 系统服务（T5）。卸载后请在系统登录项中确认无残留。"

// autostartHost 包装 Wails AutostartManager。
type autostartHost struct {
	mu       sync.RWMutex
	wailsApp *application.App
}

// newAutostartHost 创建未绑定的自启动 host。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *autostartHost: 可 Register 的 host。
func newAutostartHost() *autostartHost {
	return &autostartHost{}
}

// Attach 绑定 Wails 应用。
//
// 参数:
//   - wailsApp: Wails 应用。
//
// 返回值:
//   - 无。
func (h *autostartHost) Attach(wailsApp *application.App) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.wailsApp = wailsApp
}

// Available 判断当前 OS 是否支持 Desktop 登录项自启动。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是否可用。
//   - UnavailableReason: 原因。
//   - string: 消息。
func (h *autostartHost) Available() (bool, UnavailableReason, string) {
	switch runtime.GOOS {
	case "darwin", "windows", "linux":
		return true, "", ""
	default:
		return false, ReasonPlatform, "当前平台不支持 Desktop 登录项自启动"
	}
}

// Status 查询登录项状态。
//
// 参数:
//   - 无。
//
// 返回值:
//   - AutostartStatusInfo: 状态快照。
//   - error: 查询失败。
func (h *autostartHost) Status() (AutostartStatusInfo, error) {
	info := AutostartStatusInfo{Note: AutostartVsServiceNote}
	h.mu.RLock()
	app := h.wailsApp
	h.mu.RUnlock()
	if app == nil || app.Autostart == nil {
		// 未绑定 GUI 时仍可返回结构化空状态（测试/无壳）。
		return info, nil
	}
	st, err := app.Autostart.Status()
	if err != nil {
		return info, err
	}
	info.Enabled = st.Enabled
	info.Path = st.Path
	info.Strategy = string(st.Strategy)
	return info, nil
}

// Enable 注册登录项。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 失败原因。
func (h *autostartHost) Enable() error {
	h.mu.RLock()
	app := h.wailsApp
	h.mu.RUnlock()
	if app == nil || app.Autostart == nil {
		return fmt.Errorf("autostart manager unavailable")
	}
	return app.Autostart.EnableWithOptions(application.AutostartOptions{
		Identifier: SingleInstanceUniqueID,
	})
}

// Disable 注销登录项。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 失败原因。
func (h *autostartHost) Disable() error {
	h.mu.RLock()
	app := h.wailsApp
	h.mu.RUnlock()
	if app == nil || app.Autostart == nil {
		return fmt.Errorf("autostart manager unavailable")
	}
	return app.Autostart.Disable()
}
