package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/data"
	"taskdaemon/internal/httpapi"
	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
	"taskdaemon/web/embedded"
)

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

	if err := app.migrateStore(ctx, store); err != nil {
		return err
	}

	taskService := scheduler.NewService(store, scheduler.Options{
		Runner: runner.NewExecutor(),
	})
	if err := taskService.RegisterEnabledTasks(ctx); err != nil {
		return fmt.Errorf("register enabled tasks: %w", err)
	}
	if err := taskService.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer func() {
		if err := taskService.Shutdown(); err != nil {
			app.logger.Warn("shutdown scheduler failed", "component", "scheduler", "error", err)
		}
	}()

	server := &http.Server{
		Addr: app.cfg.Server.Address(),
		Handler: httpapi.NewRouter(httpapi.Options{
			EnableSwagger:          app.cfg.Server.Swagger.Enabled,
			Auth:                   auth.New(store, auth.Options{}),
			Tasks:                  taskService,
			Logger:                 app.logger,
			IncludeTraceInResponse: &app.cfg.Observability.TraceID.IncludeInResponse,
			HTTPLog:                app.cfg.Logging.HTTP,
			RuntimeConfig:          app.runtimeConfig,
			ReloadConfig:           app.ReloadRuntimeConfig,
			FrontendFS:             mustFrontendFS(),
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
		if isAddressInUse(err) {
			return fmt.Errorf("http api address %s is already in use; stop the process using it or set TASKDAEMON_SERVER_PORT / server.port to an explicit alternate port: %w", server.Addr, err)
		}
		return err
	}
}

// mustFrontendFS 返回嵌入的前端 dist 文件系统。
//
// 参数:
//   - 无。
//
// 返回值:
//   - fs.FS: dist 根目录文件系统。
func mustFrontendFS() fs.FS {
	frontendFS, err := fs.Sub(embedded.Assets, "dist")
	if err != nil {
		panic(fmt.Errorf("frontend assets unavailable: %w", err))
	}
	return frontendFS
}
