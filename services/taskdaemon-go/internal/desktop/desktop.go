package desktop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"taskdaemon/internal/app"
	"taskdaemon/internal/config"
	"taskdaemon/web/embedded"
)

const shutdownTimeout = 5 * time.Second

// App 是暴露给 Wails 前端的桌面绑定根对象。
type App struct {
	cfg config.Config
}

// Run 启动 Desktop 壳。
//
// 参数:
//   - ctx: 控制 desktop 生命周期的 context。
//   - cfg: 已加载的应用配置。
//
// 返回值:
//   - error: Wails runtime 启动失败时返回错误。
func Run(ctx context.Context, cfg config.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	desktopApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "taskdaemon",
		Width:           1200,
		Height:          760,
		MinWidth:        960,
		MinHeight:       600,
		URL:             "/",
		InitialPosition: application.WindowCentered,
	})

	stopQuit := context.AfterFunc(runCtx, desktopApp.Quit)
	defer stopQuit()

	apiErrCh := make(chan error, 1)
	go func() {
		err := app.New(cfg, nil).Serve(runCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			cancel()
			desktopApp.Quit()
		}
		apiErrCh <- err
	}()

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
