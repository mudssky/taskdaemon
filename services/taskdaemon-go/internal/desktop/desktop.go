package desktop

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"taskdaemon/internal/app"
	"taskdaemon/internal/config"
	"taskdaemon/internal/httpapi"
	"taskdaemon/internal/notify"
	"taskdaemon/web/embedded"
)

const shutdownTimeout = 5 * time.Second
const apiReadyTimeout = 10 * time.Second

//go:embed assets/appicon.png
var desktopIcon []byte

// App 是暴露给 Wails 前端的桌面绑定根对象。
type App struct {
	cfg           config.Config
	registry      *Registry
	trayHost      *trayHost
	autostartHost *autostartHost
	windowHost    *windowStateHost
}

// newApp 创建带 capability 注册表的 Desktop binding 根对象。
//
// 参数:
//   - cfg: 已加载的应用配置。
//
// 返回值:
//   - *App: 已注册样板与 TD2 能力的 binding 实例。
func newApp(cfg config.Config) *App {
	healthURL := (&url.URL{
		Scheme: "http",
		Host:   desktopClientAddress(cfg.Server),
		Path:   "/api/health",
	}).String()

	trayHost := newTrayHost(healthURL, cfg.Desktop.MinimizeToTray, cfg.Desktop.TrayEnabled)
	autostartHost := newAutostartHost()
	windowHost := newWindowStateHost(cfg.Desktop.WindowStateEnabled, WindowStatePath())

	bindings := &App{
		cfg:           cfg,
		registry:      NewRegistry(),
		trayHost:      trayHost,
		autostartHost: autostartHost,
		windowHost:    windowHost,
	}
	// TD1: 注册 Environment 样板能力；D2/D3 在此下方 append-only 追加 Register。
	bindings.registry.Register(NewEnvironmentCapability(bindings.registry, appVersion))
	// TD2: 托盘 / 自启动 / 窗口状态
	bindings.registry.Register(NewTrayCapability(trayHost))
	bindings.registry.Register(NewAutostartCapability(autostartHost))
	bindings.registry.Register(NewWindowStateCapability(windowHost))
	return bindings
}

// Run 启动 Desktop 壳。
//
// 参数:
//   - ctx: 控制 desktop 生命周期的 context。
//   - cfg: 已加载的应用配置。
//   - logger: 结构化 logger；为空时使用 slog.Default。
//
// 返回值:
//   - error: Wails runtime 启动失败时返回错误。
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 先装配 registry，注入 HTTP 桥，再启动 API——前端经 Vite/API 同域访问 /api/desktop/*。
	// 禁止把业务 API POST 走 wails.localhost AssetServer（WebView2 会丢 body → 登录 400）。
	bindings := newApp(cfg)
	httpapi.SetDesktopBridge(httpBridge{app: bindings})
	defer httpapi.SetDesktopBridge(nil)

	apiErrCh := make(chan error, 1)
	go func() {
		err := app.New(cfg, logger).Serve(runCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			cancel()
		}
		apiErrCh <- err
	}()
	if err := waitForAPIReady(runCtx, cfg, apiErrCh); err != nil {
		cancel()
		return err
	}

	notificationService := notifications.New()
	var desktopApp *application.App
	notifCap := NewNotificationCapability(notificationService, NotificationCapabilityOptions{
		ActivateMainWindow: func() {
			activateMainWindow(desktopApp)
		},
	})
	bindings.registry.Register(notifCap)

	opts := application.Options{
		Name:        "taskdaemon",
		Description: "Cross-platform task scheduling daemon",
		Icon:        desktopIcon,
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(embedded.Assets),
			DisableLogging: true,
		},
		Services: []application.Service{
			application.NewService(bindings),
			application.NewService(notificationService),
		},
	}
	if cfg.Desktop.SingleInstance {
		opts.SingleInstance = &application.SingleInstanceOptions{
			UniqueID: SingleInstanceUniqueID,
			ExitCode: SingleInstanceExitCode,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if bindings.trayHost != nil {
					bindings.trayHost.ShowMainWindow()
				}
			},
		}
	}

	desktopApp = application.New(opts)
	notifCap.bindAsDesktopSender()
	defer notify.UnbindDesktopSender()
	notificationService.OnNotificationResponse(notifCap.HandleNotificationResponse)
	bindings.autostartHost.Attach(desktopApp)

	// 启动时对齐配置中的期望自启动状态（失败仅记日志，不阻塞）。
	if cfg.Desktop.AutostartEnabled {
		if err := bindings.autostartHost.Enable(); err != nil {
			logger.Warn("desktop autostart enable on startup failed", "err", err)
		}
	}

	startURL := desktopStartURL(cfg)
	windowState, useCenter := bindings.windowHost.LoadForStartup(nil)
	windowOpts := application.WebviewWindowOptions{
		Title:     "taskdaemon",
		Width:     windowState.Width,
		Height:    windowState.Height,
		MinWidth:  defaultMinWidth,
		MinHeight: defaultMinHeight,
		URL:       startURL,
	}
	if useCenter {
		windowOpts.InitialPosition = application.WindowCentered
	} else {
		windowOpts.InitialPosition = application.WindowXY
		windowOpts.X = windowState.X
		windowOpts.Y = windowState.Y
	}
	if windowState.Maximised {
		windowOpts.StartState = application.WindowStateMaximised
	}

	mainWindow := desktopApp.Window.NewWithOptions(windowOpts)
	bindings.windowHost.Attach(mainWindow)
	if err := bindings.trayHost.Attach(desktopApp, mainWindow, desktopIcon); err != nil {
		logger.Warn("desktop tray attach failed; continuing without tray", "err", err)
	}

	// 关窗：可配置为最小化到托盘。
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if bindings.trayHost != nil && bindings.trayHost.MinimizeToTrayEnabled() {
			event.Cancel()
			mainWindow.Hide()
		}
	})

	stopQuit := context.AfterFunc(runCtx, desktopApp.Quit)
	defer stopQuit()

	desktopErr := desktopApp.Run()
	_ = bindings.windowHost.CaptureAndSave()
	cancel()
	apiErr := waitForProcessResult("desktop api", apiErrCh)
	if desktopErr != nil {
		if apiErr != nil && !errors.Is(apiErr, context.Canceled) {
			return errors.Join(desktopErr, fmt.Errorf("desktop api: %w", apiErr))
		}
		return desktopErr
	}
	if apiErr != nil && !errors.Is(apiErr, context.Canceled) {
		return fmt.Errorf("desktop api: %w", apiErr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// desktopStartURL 返回桌面窗口入口 URL。
//
// 开发期直连 Vite（FRONTEND_DEVSERVER_URL），发布期直连本机 HTTP API。
// 业务 /api 必须同域，避免经 wails.localhost AssetServer 转发时 WebView2 丢失 POST body。
// Desktop 能力走 /api/desktop/* HTTP 桥，不再依赖 window.wails.Call.ByName。
//
// 参数:
//   - cfg: 已加载的应用配置。
//
// 返回值:
//   - string: 窗口加载 URL。
func desktopStartURL(cfg config.Config) string {
	if devServerURL := os.Getenv("FRONTEND_DEVSERVER_URL"); devServerURL != "" {
		return devServerURL
	}
	return "http://" + desktopClientAddress(cfg.Server)
}

// httpBridge 将 desktop.App 适配为 httpapi.DesktopBridge。
type httpBridge struct {
	app *App
}

// ListDesktopCapabilities 实现 httpapi.DesktopBridge。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []httpapi.DesktopCapabilityDTO: 能力清单。
func (b httpBridge) ListDesktopCapabilities() []httpapi.DesktopCapabilityDTO {
	if b.app == nil {
		return nil
	}
	raw := b.app.ListCapabilities()
	out := make([]httpapi.DesktopCapabilityDTO, 0, len(raw))
	for _, item := range raw {
		out = append(out, httpapi.DesktopCapabilityDTO{
			Name:      item.Name,
			Available: item.Available,
			Reason:    string(item.Reason),
			Message:   item.Message,
		})
	}
	return out
}

// InvokeDesktopCapability 实现 httpapi.DesktopBridge。
//
// 参数:
//   - name: capability 名。
//   - payloadJSON: JSON 字符串负载。
//
// 返回值:
//   - httpapi.DesktopInvokeResultDTO: 结构化调用结果。
func (b httpBridge) InvokeDesktopCapability(name string, payloadJSON string) httpapi.DesktopInvokeResultDTO {
	if b.app == nil {
		return httpapi.DesktopInvokeResultDTO{
			OK:      false,
			Code:    ErrCodeCapabilityNotFound,
			Message: "desktop app is not initialized",
		}
	}
	result := b.app.InvokeCapability(name, payloadJSON)
	return httpapi.DesktopInvokeResultDTO{
		OK:      result.OK,
		Data:    result.Data,
		Code:    result.Code,
		Message: result.Message,
	}
}

// waitForAPIReady 等待桌面内置 HTTP API 可响应。
//
// 参数:
//   - ctx: 控制等待生命周期的 context。
//   - cfg: 已加载的应用配置。
//   - apiErrCh: API server goroutine 的错误通道。
//
// 返回值:
//   - error: API 启动失败、超时或 context 取消时返回错误。
func waitForAPIReady(ctx context.Context, cfg config.Config, apiErrCh <-chan error) error {
	readyURL := url.URL{
		Scheme: "http",
		Host:   desktopClientAddress(cfg.Server),
		Path:   "/api/health",
	}
	deadline := time.NewTimer(apiReadyTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	client := http.Client{Timeout: 500 * time.Millisecond}
	for {
		select {
		case err := <-apiErrCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return fmt.Errorf("desktop api stopped before it became ready")
			}
			return fmt.Errorf("desktop api: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("desktop api did not become ready within %s", apiReadyTimeout)
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, readyURL.String(), nil)
			if err != nil {
				return fmt.Errorf("create desktop api readiness request: %w", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

// desktopClientAddress 返回桌面窗口访问本机 API server 的地址。
//
// 参数:
//   - server: server 监听配置。
//
// 返回值:
//   - string: host:port 格式客户端地址。
func desktopClientAddress(server config.ServerConfig) string {
	host := strings.TrimSpace(server.Host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(server.Port))
}

// APIAddress 返回 Desktop 当前连接的后端 API 地址。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: host:port 格式 API 地址。
func (app *App) APIAddress() string {
	return app.cfg.Server.Address()
}

// ListCapabilities 返回已注册 Desktop 能力清单（供前端缓存）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []CapabilityDescriptor: 能力名、可用性与不可用原因。
func (app *App) ListCapabilities() []CapabilityDescriptor {
	if app == nil || app.registry == nil {
		return nil
	}
	return app.registry.List()
}

// InvokeCapability 按名称调用 Desktop capability。
// 始终返回结构化 InvokeResult；panic 由 Registry 消化为 DESKTOP_INVOKE_FAILED。
//
// 参数:
//   - name: capability 具名常量，例如 desktop.environment。
//   - payloadJSON: JSON 字符串负载；无参时传空字符串。
//
// 返回值:
//   - InvokeResult: ok/data 或 code/message。
func (app *App) InvokeCapability(name string, payloadJSON string) InvokeResult {
	if app == nil || app.registry == nil {
		return InvokeResult{
			OK:      false,
			Code:    ErrCodeCapabilityNotFound,
			Message: "desktop registry is not initialized",
		}
	}
	return invokeAsResult(context.Background(), app.registry, name, payloadJSON)
}

// waitForProcessResult 等待桌面子进程退出，避免关闭时留下孤儿进程。
//
// 参数:
//   - name: 子进程名称，用于错误信息。
//   - errCh: 子进程返回错误的 channel。
//
// 返回值:
//   - error: 子进程错误；超时未退出时返回超时错误。
func waitForProcessResult(name string, errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	case <-time.After(shutdownTimeout):
		return fmt.Errorf("%s did not stop within %s", name, shutdownTimeout)
	}
}

// activateMainWindow 激活主窗口（通知点击默认动作）。
//
// 参数:
//   - desktopApp: Wails 应用实例；nil 时忽略。
//
// 返回值:
//   - 无。
func activateMainWindow(desktopApp *application.App) {
	if desktopApp == nil {
		return
	}
	if window, ok := desktopApp.Window.GetByName("main"); ok && window != nil {
		window.UnMinimise()
		window.Show()
		window.Focus()
		return
	}
	for _, window := range desktopApp.Window.GetAll() {
		if window == nil {
			continue
		}
		window.UnMinimise()
		window.Show()
		window.Focus()
		return
	}
}
