package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
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
	store, err := data.Open(ctx, app.cfg.Database)
	if err != nil {
		return fmt.Errorf("open data store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			app.logger.Warn("close data store failed", "component", "data", "error", err)
		}
	}()

	server := &http.Server{
		Addr: app.cfg.Server.Address(),
		Handler: httpapi.NewRouter(httpapi.Options{
			EnableSwagger: app.cfg.Server.Swagger.Enabled,
			Auth:          auth.New(store, auth.Options{}),
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

// MigrateSchema 对当前配置数据库执行 schema migration。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//
// 返回值:
//   - error: 数据库打开、migration 或关闭连接失败时返回错误。
func (app *App) MigrateSchema(ctx context.Context) error {
	store, err := data.Open(ctx, app.cfg.Database)
	if err != nil {
		return fmt.Errorf("open data store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			app.logger.Warn("close data store failed", "component", "data", "error", err)
		}
	}()

	app.logger.Info("schema migration started", "driver", app.cfg.Database.Driver)
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	app.logger.Info("schema migration completed", "driver", app.cfg.Database.Driver)
	return nil
}
