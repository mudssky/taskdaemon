package desktop

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"taskdaemon/internal/config"
	"taskdaemon/web/embedded"
)

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
	desktopApp.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:           "taskdaemon",
		Width:           1200,
		Height:          760,
		MinWidth:        960,
		MinHeight:       600,
		URL:             "/",
		InitialPosition: application.WindowCentered,
	})

	stopQuit := context.AfterFunc(ctx, desktopApp.Quit)
	defer stopQuit()

	return desktopApp.Run()
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
