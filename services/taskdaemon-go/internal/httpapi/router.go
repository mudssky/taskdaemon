package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/audio"
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
	Audio                  AudioService
	Logger                 *slog.Logger
	IncludeTraceInResponse *bool
	HTTPLog                config.LoggingHTTPConfig
	RuntimeConfig          *RuntimeConfig
	ReloadConfig           func(context.Context) (config.ReloadResult, error)
	FrontendFS             fs.FS
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
	DeleteTask(context.Context, int) error
	TriggerTask(context.Context, int) (*ent.Run, error)
	CancelTask(context.Context, int) error
	ListTaskRuns(context.Context, int, int) ([]*ent.Run, error)
	IsTaskRunning(int) bool
}

// AudioService 定义 HTTP handler 依赖的音频播放请求能力。
type AudioService interface {
	SubmitURL(context.Context, string, audio.URLRequest) (*ent.AudioRecord, error)
	SubmitUpload(context.Context, string, audio.UploadRequest) (*ent.AudioRecord, error)
	ListHistory(context.Context, int) ([]*ent.AudioRecord, error)
	Replay(context.Context, int) (*ent.AudioRecord, error)
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

	useCoreMiddleware(router, opts)

	router.GET("/api/health", func(ctx *gin.Context) {
		writeAPISuccess(ctx, http.StatusOK, gin.H{"status": "ok"})
	})

	registerAuthRoutes(router, opts.Auth)
	registerTaskRoutes(router, opts.Auth, opts.Tasks)
	registerConfigRoutes(router, opts.Auth, opts.ReloadConfig, runtimeConfigOption(opts))
	registerAudioRoutes(router, opts.Auth, opts.Audio)

	if opts.EnableSwagger {
		router.GET("/swagger/index.html", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "Swagger UI is enabled. Generated docs will be mounted here.")
		})
	}
	if opts.FrontendFS != nil {
		registerFrontendRoutes(router, opts.FrontendFS)
	}

	return router
}

// registerFrontendRoutes 注册管理台静态资源和 SPA fallback。
//
// 参数:
//   - router: Gin engine。
//   - frontendFS: 已定位到 dist 根目录的前端静态资源文件系统。
//
// 返回值:
//   - 无。
func registerFrontendRoutes(router *gin.Engine, frontendFS fs.FS) {
	router.NoRoute(func(ctx *gin.Context) {
		requestPath := ctx.Request.URL.Path
		if isReservedBackendPath(requestPath) {
			writeAPIError(ctx, http.StatusNotFound, "not_found", "Not found", gin.H{"path": requestPath})
			return
		}
		if serveFrontendFile(ctx, frontendFS, requestPath) {
			return
		}
		if path.Ext(requestPath) != "" {
			ctx.Status(http.StatusNotFound)
			return
		}
		if serveFrontendFile(ctx, frontendFS, "/index.html") {
			return
		}
		ctx.Status(http.StatusNotFound)
	})
}

// serveFrontendFile 尝试从前端静态资源中写出指定文件。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - frontendFS: 前端静态资源文件系统。
//   - requestPath: 请求路径。
//
// 返回值:
//   - bool: true 表示文件存在且已经写入响应。
func serveFrontendFile(ctx *gin.Context, frontendFS fs.FS, requestPath string) bool {
	cleanPath := cleanFrontendPath(requestPath)
	info, err := fs.Stat(frontendFS, cleanPath)
	if err != nil || info.IsDir() {
		return false
	}
	content, err := fs.ReadFile(frontendFS, cleanPath)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return true
	}
	ctx.Data(http.StatusOK, frontendContentType(cleanPath), content)
	return true
}

// frontendContentType 返回前端静态资源的 Content-Type。
//
// 参数:
//   - filename: 静态资源文件名。
//
// 返回值:
//   - string: HTTP Content-Type，无法识别时为空字符串。
func frontendContentType(filename string) string {
	if contentType := mime.TypeByExtension(path.Ext(filename)); contentType != "" {
		return contentType
	}
	return ""
}

// cleanFrontendPath 将 URL path 转为 fs.FS 内的安全相对路径。
//
// 参数:
//   - requestPath: URL path。
//
// 返回值:
//   - string: 可用于 fs.FS 读取的相对路径。
func cleanFrontendPath(requestPath string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return "index.html"
	}
	return cleaned
}

// isReservedBackendPath 判断路径是否应由后端路由负责。
//
// 参数:
//   - requestPath: URL path。
//
// 返回值:
//   - bool: true 表示不能回退到前端 index.html。
func isReservedBackendPath(requestPath string) bool {
	return requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/swagger" ||
		strings.HasPrefix(requestPath, "/swagger/")
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
