package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"taskdaemon/internal/config"
	"taskdaemon/internal/httpapi"
)

// App 组合 taskdaemon 的共享后端能力。
type App struct {
	cfg    config.Config
	logger *slog.Logger
}

// New 创建应用装配实例。
//
// 参数:
//   - cfg: 已加载的应用配置。
//   - logger: 结构化 logger；为空时使用 slog.Default。
//
// 返回值:
//   - *App: 应用装配实例。
func New(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		cfg:    cfg,
		logger: logger,
	}
}

// Serve 启动 HTTP API server，并在 context 取消时尝试优雅关闭。
//
// 参数:
//   - ctx: 控制 server 生命周期的 context。
//
// 返回值:
//   - error: server 启动或关闭失败时返回错误。
func (app *App) Serve(ctx context.Context) error {
	server := &http.Server{
		Addr: app.cfg.Server.Address(),
		Handler: httpapi.NewRouter(httpapi.Options{
			EnableSwagger: app.cfg.Server.Swagger.Enabled,
		}),
	}

	errCh := make(chan error, 1)
	go func() {
		app.logger.Info("starting http api", "addr", server.Addr, "swagger", app.cfg.Server.Swagger.Enabled)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// MigrateSchema 预留当前数据库 schema migration 入口。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//
// 返回值:
//   - error: 当前骨架阶段无迁移实现，始终返回 nil。
func (app *App) MigrateSchema(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	app.logger.Info("schema migration command reached", "driver", app.cfg.Database.Driver)
	return nil
}
