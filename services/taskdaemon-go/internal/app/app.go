package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/httpapi"
	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
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
			EnableSwagger: app.cfg.Server.Swagger.Enabled,
			Auth:          auth.New(store, auth.Options{}),
			Tasks:         taskService,
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

// TriggerTask 通过正在运行的 daemon HTTP API 手动触发任务。
//
// 参数:
//   - ctx: 控制 HTTP 请求和任务执行生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) TriggerTask(ctx context.Context, taskID int, sessionToken string) error {
	if err := app.callTaskAction(ctx, taskID, "trigger", sessionToken, 0, http.StatusOK); err != nil {
		return err
	}
	app.logger.Info("task trigger requested", "task_id", taskID)
	return nil
}

// CancelTask 通过正在运行的 daemon HTTP API 取消任务。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) CancelTask(ctx context.Context, taskID int, sessionToken string) error {
	if err := app.callTaskAction(ctx, taskID, "cancel", sessionToken, 10*time.Second, http.StatusNoContent, http.StatusOK); err != nil {
		return err
	}
	app.logger.Info("task cancel requested", "task_id", taskID)
	return nil
}

// callTaskAction 调用 daemon HTTP API 中的任务动作端点。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - taskID: 任务 ID。
//   - action: 动作名称，例如 trigger 或 cancel。
//   - sessionToken: 管理员 session token。
//   - clientTimeout: HTTP client timeout；0 表示只使用 ctx 控制。
//   - successStatuses: 视为成功的 HTTP 状态码。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) callTaskAction(ctx context.Context, taskID int, action string, sessionToken string, clientTimeout time.Duration, successStatuses ...int) error {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return fmt.Errorf("task %s requires --session-token or TASKDAEMON_SESSION_TOKEN", action)
	}

	endpoint := fmt.Sprintf("http://%s/api/tasks/%d/%s", daemonClientAddress(app.cfg.Server), taskID, action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create %s request: %w", action, err)
	}
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookieName, Value: sessionToken})

	client := &http.Client{Timeout: clientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call daemon %s API: %w", action, err)
	}
	defer resp.Body.Close()
	for _, status := range successStatuses {
		if resp.StatusCode == status {
			return nil
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("daemon %s API returned %s: %s", action, resp.Status, strings.TrimSpace(string(body)))
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

// daemonClientAddress 返回 CLI 访问本机 daemon API 时使用的地址。
//
// 参数:
//   - server: server 监听配置。
//
// 返回值:
//   - string: host:port 格式客户端地址。
func daemonClientAddress(server config.ServerConfig) string {
	host := strings.TrimSpace(server.Host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(server.Port))
}
