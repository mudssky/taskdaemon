package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"taskdaemon/web/embedded"
)

// App 组合 taskdaemon 的共享后端能力。
type App struct {
	cfg           config.Config
	logger        *slog.Logger
	loadConfig    func(config.LoadOptions) (config.Config, error)
	loadOptions   config.LoadOptions
	runtimeConfig *httpapi.RuntimeConfig
}

// TaskRunSummary 是应用层暴露给 CLI 查询的执行历史摘要。
type TaskRunSummary struct {
	ID           int    `json:"id"`
	Trigger      string `json:"trigger"`
	Status       string `json:"status"`
	ExitCode     *int   `json:"exitCode"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
	ErrorSummary string `json:"errorSummary"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
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
		cfg:           cfg,
		logger:        logger,
		loadConfig:    config.Load,
		runtimeConfig: httpapi.NewRuntimeConfig(cfg),
	}
}

// WithConfigReload 配置 daemon 运行时重载使用的配置加载器。
//
// 参数:
//   - loader: 配置加载函数；为空时使用 config.Load。
//   - opts: 配置加载选项。
//
// 返回值:
//   - *App: 当前应用实例，便于链式调用。
func (app *App) WithConfigReload(loader func(config.LoadOptions) (config.Config, error), opts config.LoadOptions) *App {
	if loader == nil {
		loader = config.Load
	}
	app.loadConfig = loader
	app.loadOptions = opts
	return app
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

// ReloadRuntimeConfig 重新加载配置文件并应用可热更新配置。
//
// 参数:
//   - ctx: 控制配置加载生命周期的 context。
//
// 返回值:
//   - config.ReloadResult: 已应用和需要重启的配置分组。
//   - error: 配置加载失败时返回错误。
func (app *App) ReloadRuntimeConfig(ctx context.Context) (config.ReloadResult, error) {
	if err := ctx.Err(); err != nil {
		return config.ReloadResult{}, err
	}
	loader := app.loadConfig
	if loader == nil {
		loader = config.Load
	}
	cfg, err := loader(app.loadOptions)
	if err != nil {
		return config.ReloadResult{}, err
	}
	result := app.runtimeConfig.Apply(cfg)
	app.logger.Info("runtime config reloaded", "applied", result.Applied, "restart_required", result.RestartRequired)
	return result, nil
}

// ReloadDaemonConfig 调用正在运行的 daemon API 重载配置。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - config.ReloadResult: daemon 返回的重载结果。
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) ReloadDaemonConfig(ctx context.Context, sessionToken string) (config.ReloadResult, error) {
	var result config.ReloadResult
	if err := app.callDaemonAPI(ctx, http.MethodPost, "config/reload", sessionToken, nil, &result, 10*time.Second, http.StatusOK); err != nil {
		return config.ReloadResult{}, err
	}
	app.logger.Info("config reload requested")
	return result, nil
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
	if err := app.callDaemonAPI(ctx, http.MethodPost, fmt.Sprintf("tasks/%d/trigger", taskID), sessionToken, nil, nil, 0, http.StatusOK); err != nil {
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
	if err := app.callDaemonAPI(ctx, http.MethodPost, fmt.Sprintf("tasks/%d/cancel", taskID), sessionToken, nil, nil, 10*time.Second, http.StatusOK); err != nil {
		return err
	}
	app.logger.Info("task cancel requested", "task_id", taskID)
	return nil
}

// ListTaskRuns 通过正在运行的 daemon HTTP API 查询任务执行历史。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - taskID: 任务 ID。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - []cli.TaskRunSummary: 最近执行历史摘要。
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) ListTaskRuns(ctx context.Context, taskID int, sessionToken string) ([]TaskRunSummary, error) {
	var response struct {
		Runs []TaskRunSummary `json:"runs"`
	}
	if err := app.callDaemonAPI(ctx, http.MethodGet, fmt.Sprintf("tasks/%d/runs", taskID), sessionToken, nil, &response, 10*time.Second, http.StatusOK); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

// callDaemonAPI 调用 daemon HTTP API。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - method: HTTP 方法。
//   - path: API 路径，不含 /api/ 前缀。
//   - sessionToken: 管理员 session token。
//   - requestBody: 请求体；为空时不发送 body。
//   - responseBody: 成功响应 data 的解码目标；为空时不解码。
//   - clientTimeout: HTTP client timeout；0 表示只使用 ctx 控制。
//   - successStatuses: 视为成功的 HTTP 状态码。
//
// 返回值:
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) callDaemonAPI(ctx context.Context, method string, path string, sessionToken string, requestBody []byte, responseBody any, clientTimeout time.Duration, successStatuses ...int) error {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return fmt.Errorf("daemon API requires --session-token or TASKDAEMON_SESSION_TOKEN")
	}

	endpoint := fmt.Sprintf("http://%s/api/%s", daemonClientAddress(app.cfg.Server), strings.TrimPrefix(path, "/"))
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create daemon API request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookieName, Value: sessionToken})
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: clientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call daemon API: %w", err)
	}
	defer resp.Body.Close()
	for _, status := range successStatuses {
		if resp.StatusCode == status {
			if responseBody != nil {
				return decodeDaemonAPIData(resp.Body, responseBody)
			}
			return nil
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("daemon API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
}

// decodeDaemonAPIData 从 daemon API envelope 中解出 data。
//
// 参数:
//   - body: HTTP 响应体 reader。
//   - target: data 解码目标。
//
// 返回值:
//   - error: 解码失败或 envelope 非成功时返回错误。
func decodeDaemonAPIData(body io.Reader, target any) error {
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode daemon API response: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("daemon API failed: %s", envelope.Msg)
	}
	if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode daemon API data: %w", err)
	}
	return nil
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

	if err := app.migrateStore(ctx, store); err != nil {
		return err
	}
	return nil
}

// migrateStore 对已打开的数据层执行 schema migration。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//   - store: 已打开的数据层连接。
//
// 返回值:
//   - error: migration 失败时返回带上下文的错误。
func (app *App) migrateStore(ctx context.Context, store *data.Store) error {
	app.logger.Info("schema migration started", "driver", app.cfg.Database.Driver)
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
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

// isAddressInUse 判断 HTTP server 启动错误是否为监听地址已被占用。
//
// 参数:
//   - err: server 启动返回的错误。
//
// 返回值:
//   - bool: true 表示错误来自地址占用。
func isAddressInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	if opErr.Op != "listen" {
		return false
	}
	return strings.Contains(strings.ToLower(opErr.Err.Error()), "address already in use") ||
		strings.Contains(strings.ToLower(opErr.Err.Error()), "only one usage of each socket address")
}
