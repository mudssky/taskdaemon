package desktop

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TrayServiceStatus 是托盘展示的服务三态。
type TrayServiceStatus string

const (
	// TrayServiceRunning 本机 API 健康。
	TrayServiceRunning TrayServiceStatus = "running"
	// TrayServiceStopped API 不可达（桌面壳仍在但后端停）。
	TrayServiceStopped TrayServiceStatus = "stopped"
	// TrayServiceError 探测异常或非 2xx。
	TrayServiceError TrayServiceStatus = "error"
)

// trayHost 持有托盘与主窗口的运行时引用，供 capability 与菜单回调使用。
type trayHost struct {
	mu             sync.RWMutex
	enabled        bool
	ready          bool
	minimizeToTray atomic.Bool
	healthURL      string
	wailsApp       *application.App
	window         *application.WebviewWindow
	tray           *application.SystemTray
	statusItem     *application.MenuItem
	client         *http.Client
}

// newTrayHost 创建未绑定窗口的托盘 host。
//
// 参数:
//   - healthURL: 例如 http://127.0.0.1:39245/api/health。
//   - minimizeToTray: 初始关窗行为。
//   - enabled: 配置是否启用托盘。
//
// 返回值:
//   - *trayHost: 可 Register 的 host。
func newTrayHost(healthURL string, minimizeToTray bool, enabled bool) *trayHost {
	h := &trayHost{
		enabled:   enabled,
		healthURL: healthURL,
		client: &http.Client{
			Timeout: 1500 * time.Millisecond,
		},
	}
	h.minimizeToTray.Store(minimizeToTray)
	return h
}

// Attach 在 Wails 应用与主窗口就绪后装配托盘。
//
// 参数:
//   - wailsApp: Wails 应用实例。
//   - window: 主窗口。
//   - icon: 托盘图标字节。
//
// 返回值:
//   - error: 创建失败时返回；调用方应降级继续启动。
func (h *trayHost) Attach(wailsApp *application.App, window *application.WebviewWindow, icon []byte) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.wailsApp = wailsApp
	h.window = window
	if !h.enabled || wailsApp == nil || window == nil {
		h.ready = false
		return nil
	}

	menu := wailsApp.NewMenu()
	showItem := menu.Add("显示主窗口")
	showItem.OnClick(func(_ *application.Context) {
		h.ShowMainWindow()
	})
	statusItem := menu.Add("运行状态：检测中…")
	statusItem.SetEnabled(false)
	h.statusItem = statusItem
	menu.AddSeparator()
	quitItem := menu.Add("退出")
	quitItem.OnClick(func(_ *application.Context) {
		// 真正退出：先关闭 minimize 语义再 Quit。
		h.minimizeToTray.Store(false)
		if h.wailsApp != nil {
			h.wailsApp.Quit()
		}
	})

	tray := wailsApp.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
		tray.SetTemplateIcon(icon)
	}
	tray.SetTooltip("taskdaemon")
	tray.SetMenu(menu)
	tray.OnClick(func() {
		h.ShowMainWindow()
	})
	h.tray = tray
	h.ready = true
	h.refreshStatusLocked()
	return nil
}

// Available 返回托盘能力是否可用。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是否可用。
//   - UnavailableReason: 不可用原因。
//   - string: 人类可读说明。
func (h *trayHost) Available() (bool, UnavailableReason, string) {
	if h == nil || !h.enabled {
		return false, ReasonPlatform, "托盘已在配置中关闭"
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.ready {
		return false, ReasonPlatform, "托盘尚未装配或当前环境不支持"
	}
	return true, "", ""
}

// ShowMainWindow 显示并聚焦主窗口。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (h *trayHost) ShowMainWindow() {
	if h == nil {
		return
	}
	h.mu.RLock()
	window := h.window
	h.mu.RUnlock()
	if window == nil {
		return
	}
	window.Show().Focus()
}

// MinimizeToTrayEnabled 返回关窗是否最小化到托盘。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是否启用。
func (h *trayHost) MinimizeToTrayEnabled() bool {
	if h == nil {
		return false
	}
	return h.minimizeToTray.Load()
}

// SetMinimizeToTray 更新关窗行为。
//
// 参数:
//   - enabled: 是否最小化到托盘。
//
// 返回值:
//   - 无。
func (h *trayHost) SetMinimizeToTray(enabled bool) {
	if h == nil {
		return
	}
	h.minimizeToTray.Store(enabled)
}

// ProbeServiceStatus 探测本机 API 健康状态。
//
// 参数:
//   - 无。
//
// 返回值:
//   - TrayServiceStatus: 三态之一。
func (h *trayHost) ProbeServiceStatus() TrayServiceStatus {
	if h == nil || h.healthURL == "" {
		return TrayServiceError
	}
	resp, err := h.client.Get(h.healthURL)
	if err != nil {
		return TrayServiceStopped
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return TrayServiceRunning
	}
	return TrayServiceError
}

// StatusSnapshot 返回托盘状态快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - TrayStatusInfo: 供 capability 序列化。
func (h *trayHost) StatusSnapshot() TrayStatusInfo {
	status := h.ProbeServiceStatus()
	h.mu.Lock()
	if h.statusItem != nil {
		h.statusItem.SetLabel("运行状态：" + trayStatusLabel(status))
	}
	h.mu.Unlock()
	return TrayStatusInfo{
		ServiceStatus:  status,
		MinimizeToTray: h.MinimizeToTrayEnabled(),
		Label:          trayStatusLabel(status),
	}
}

// refreshStatusLocked 在持锁时刷新菜单文案（Attach 内调用）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (h *trayHost) refreshStatusLocked() {
	status := TrayServiceStopped
	if h.healthURL != "" && h.client != nil {
		// 避免在持锁时做 HTTP；仅用默认文案，首次菜单打开前由 StatusSnapshot 更新。
		status = TrayServiceRunning
	}
	if h.statusItem != nil {
		h.statusItem.SetLabel("运行状态：" + trayStatusLabel(status))
	}
}

// trayStatusLabel 将状态映射为中文。
//
// 参数:
//   - status: 服务状态。
//
// 返回值:
//   - string: UI 文案。
func trayStatusLabel(status TrayServiceStatus) string {
	switch status {
	case TrayServiceRunning:
		return "运行中"
	case TrayServiceStopped:
		return "已停止"
	case TrayServiceError:
		return "异常"
	default:
		return fmt.Sprintf("未知(%s)", status)
	}
}
