package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/scheduler"
)

var errPanicRecovered = errors.New("panic recovered")

const (
	// SessionCookieName 是 HTTP API 使用的管理员 session Cookie 名称。
	SessionCookieName = "taskdaemon_session"
)

// Options 控制 HTTP router 的可选路由。
type Options struct {
	EnableSwagger          bool
	Auth                   AuthService
	Tasks                  TaskService
	Logger                 *slog.Logger
	IncludeTraceInResponse *bool
	HTTPLog                config.LoggingHTTPConfig
	RuntimeConfig          *RuntimeConfig
	ReloadConfig           func(context.Context) (config.ReloadResult, error)
}

// AuthService 定义 HTTP handler 依赖的认证服务能力。
type AuthService interface {
	InitializeAdminWithSession(context.Context, string, string, auth.LoginMetadata) (auth.LoginResult, error)
	IsAdminInitialized(context.Context) (bool, error)
	Login(context.Context, string, string, auth.LoginMetadata) (auth.LoginResult, error)
	AuthenticateSession(context.Context, string) (auth.Principal, error)
	Logout(context.Context, string) error
}

// TaskService 定义 HTTP handler 依赖的任务调度能力。
type TaskService interface {
	CreateTask(context.Context, scheduler.CreateTaskInput) (*ent.Task, error)
	ListTasks(context.Context, int) ([]*ent.Task, error)
	UpdateTask(context.Context, int, scheduler.CreateTaskInput) (*ent.Task, error)
	SetTaskEnabled(context.Context, int, bool) (*ent.Task, error)
	TriggerTask(context.Context, int) (*ent.Run, error)
	CancelTask(context.Context, int) error
	ListTaskRuns(context.Context, int, int) ([]*ent.Run, error)
	IsTaskRunning(int) bool
}

// NewRouter 创建 taskdaemon HTTP API router。
//
// 参数:
//   - opts: router 可选能力，例如是否注册 swagger route。
//
// 返回值:
//   - http.Handler: 可被 http.Server 或测试直接使用的 handler。
func NewRouter(opts Options) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logger := loggerOrDefault(opts.Logger)
	runtimeConfig := runtimeConfigOption(opts)
	router.Use(traceIDMiddleware())
	router.Use(responseOptionsMiddleware(runtimeConfig))
	router.Use(requestLoggerMiddleware(logger, runtimeConfig))
	router.Use(recoveryMiddleware())

	router.GET("/api/health", func(ctx *gin.Context) {
		writeAPISuccess(ctx, http.StatusOK, gin.H{"status": "ok"})
	})

	registerAuthRoutes(router, opts.Auth)
	registerTaskRoutes(router, opts.Auth, opts.Tasks)
	registerConfigRoutes(router, opts.Auth, opts.ReloadConfig)

	if opts.EnableSwagger {
		router.GET("/swagger/index.html", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "Swagger UI is enabled. Generated docs will be mounted here.")
		})
	}

	return router
}

// loggerOrDefault 返回传入 logger 或 slog.Default。
//
// 参数:
//   - logger: 调用方注入的 logger。
//
// 返回值:
//   - *slog.Logger: 可用 logger。
func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}

// includeTraceInResponseOption 返回响应体 traceId 开关，未配置时默认开启。
//
// 参数:
//   - value: 可选配置值。
//
// 返回值:
//   - bool: true 表示响应 body 包含 traceId。
func includeTraceInResponseOption(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

// runtimeConfigOption 返回 router 使用的运行时配置。
//
// 参数:
//   - opts: router 可选能力。
//
// 返回值:
//   - *RuntimeConfig: 可供 middleware 读取的运行时配置。
func runtimeConfigOption(opts Options) *RuntimeConfig {
	if opts.RuntimeConfig != nil {
		return opts.RuntimeConfig
	}
	cfg := config.Default()
	cfg.Logging.HTTP = opts.HTTPLog
	cfg.Observability.TraceID.IncludeInResponse = includeTraceInResponseOption(opts.IncludeTraceInResponse)
	return NewRuntimeConfig(cfg)
}
