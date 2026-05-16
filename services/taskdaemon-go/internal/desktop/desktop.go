package desktop

import (
	"context"
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

	"taskdaemon/internal/app"
	"taskdaemon/internal/config"
	"taskdaemon/web/embedded"
)

const shutdownTimeout = 5 * time.Second
const apiReadyTimeout = 10 * time.Second

// App 是暴露给 Wails 前端的桌面绑定根对象。
type App struct {
	cfg config.Config
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

	bindings := &App{cfg: cfg}
	desktopApp := application.New(application.Options{
		Name:        "taskdaemon",
		Description: "Cross-platform task scheduling daemon",
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(embedded.Assets),
			DisableLogging: true,
		},
		Services: []application.Service{
			application.NewService(bindings),
		},
	})
	startURL := desktopStartURL(cfg)
	desktopApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "taskdaemon",
		Width:           1200,
		Height:          760,
		MinWidth:        960,
		MinHeight:       600,
		URL:             startURL,
		InitialPosition: application.WindowCentered,
	})

	stopQuit := context.AfterFunc(runCtx, desktopApp.Quit)
	defer stopQuit()

	desktopErr := desktopApp.Run()
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
// 参数:
//   - cfg: 已加载的应用配置。
//
// 返回值:
//   - string: 开发期使用 Vite dev server，发布期使用本机 HTTP API server。
func desktopStartURL(cfg config.Config) string {
	if devServerURL := os.Getenv("FRONTEND_DEVSERVER_URL"); devServerURL != "" {
		return devServerURL
	}
	return "http://" + desktopClientAddress(cfg.Server)
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
