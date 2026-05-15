package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
)

const (
	// SessionCookieName 是 HTTP API 使用的管理员 session Cookie 名称。
	SessionCookieName = "taskdaemon_session"
)

// Options 控制 HTTP router 的可选路由。
type Options struct {
	EnableSwagger bool
	Auth          AuthService
	Tasks         TaskService
}

// AuthService 定义 HTTP handler 依赖的认证服务能力。
type AuthService interface {
	InitializeAdmin(context.Context, string, string) (auth.AdminAccount, error)
	Login(context.Context, string, string, auth.LoginMetadata) (auth.LoginResult, error)
	AuthenticateSession(context.Context, string) (auth.Principal, error)
}

// TaskService 定义 HTTP handler 依赖的任务调度能力。
type TaskService interface {
	CreateTask(context.Context, scheduler.CreateTaskInput) (*ent.Task, error)
	TriggerTask(context.Context, int) (*ent.Run, error)
	CancelTask(context.Context, int) error
	ListTaskRuns(context.Context, int, int) ([]*ent.Run, error)
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
	router.Use(gin.Recovery())

	router.GET("/api/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	registerAuthRoutes(router, opts.Auth)
	registerTaskRoutes(router, opts.Auth, opts.Tasks)

	if opts.EnableSwagger {
		router.GET("/swagger/index.html", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "Swagger UI is enabled. Generated docs will be mounted here.")
		})
	}

	return router
}

// registerTaskRoutes 注册任务创建、触发、取消和历史路由。
//
// 参数:
//   - router: Gin engine。
//   - authService: 认证服务接口。
//   - taskService: 任务调度服务接口。
//
// 返回值:
//   - 无。
func registerTaskRoutes(router *gin.Engine, authService AuthService, taskService TaskService) {
	group := router.Group("/api/tasks")
	group.Use(requireSession(authService))

	group.POST("", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		var req createTaskRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		taskRecord, err := taskService.CreateTask(ctx.Request.Context(), scheduler.CreateTaskInput{
			Name:                req.Name,
			Description:         req.Description,
			Enabled:             req.Enabled,
			CronExpression:      req.CronExpression,
			Timezone:            req.Timezone,
			ConfirmCronWarnings: req.ConfirmCronWarnings,
			Runner: runner.Config{
				Type:             runner.Type(req.Runner.Type),
				Inline:           req.Runner.Inline,
				ScriptPath:       req.Runner.ScriptPath,
				Args:             req.Runner.Args,
				WorkDir:          req.Runner.WorkDir,
				Env:              req.Runner.Env,
				Timeout:          time.Duration(req.Runner.TimeoutSeconds) * time.Second,
				OutputLimitBytes: req.Runner.OutputLimitBytes,
			},
		})
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "task_invalid", "Task definition is invalid", gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusCreated, taskResponseFromEnt(taskRecord))
	})

	group.POST("/:id/trigger", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		runRecord, err := taskService.TriggerTask(ctx.Request.Context(), taskID)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_trigger_failed", "Task trigger failed", nil)
			return
		}
		ctx.JSON(http.StatusOK, runResponseFromEnt(runRecord))
	})

	group.POST("/:id/cancel", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		if err := taskService.CancelTask(ctx.Request.Context(), taskID); err != nil {
			if errors.Is(err, scheduler.ErrTaskNotRunning) {
				writeAPIError(ctx, http.StatusConflict, "task_not_running", "Task is not running", nil)
				return
			}
			writeAPIError(ctx, http.StatusConflict, "task_cancel_failed", "Task cancel failed", nil)
			return
		}
		ctx.Status(http.StatusNoContent)
	})

	group.GET("/:id/runs", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		runs, err := taskService.ListTaskRuns(ctx.Request.Context(), taskID, 50)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_runs_failed", "Task runs query failed", nil)
			return
		}
		responses := make([]runResponse, 0, len(runs))
		for _, runRecord := range runs {
			responses = append(responses, runResponseFromEnt(runRecord))
		}
		ctx.JSON(http.StatusOK, gin.H{"runs": responses})
	})
}

// requireSession 校验管理员 session。
//
// 参数:
//   - service: 认证服务接口。
//
// 返回值:
//   - gin.HandlerFunc: 未登录时中止请求的 middleware。
func requireSession(service AuthService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if service == nil {
			writeUnauthorized(ctx)
			ctx.Abort()
			return
		}
		cookie, err := ctx.Request.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeUnauthorized(ctx)
			ctx.Abort()
			return
		}
		if _, err := service.AuthenticateSession(ctx.Request.Context(), cookie.Value); err != nil {
			if errors.Is(err, auth.ErrInvalidSession) {
				writeUnauthorized(ctx)
				ctx.Abort()
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Authentication failed", nil)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// parseTaskID 解析路由中的任务 ID。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - int: 任务 ID。
//   - bool: true 表示解析成功。
func parseTaskID(ctx *gin.Context) (int, bool) {
	taskID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || taskID <= 0 {
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid task id", gin.H{"field": "id"})
		return 0, false
	}
	return taskID, true
}

// taskResponseFromEnt 将任务 Ent 实体转换为 API 响应。
//
// 参数:
//   - taskRecord: 任务 Ent 实体。
//
// 返回值:
//   - taskResponse: API 响应 DTO。
func taskResponseFromEnt(taskRecord *ent.Task) taskResponse {
	if taskRecord == nil {
		return taskResponse{}
	}
	return taskResponse{
		ID:             taskRecord.ID,
		Name:           taskRecord.Name,
		CronExpression: taskRecord.CronExpression,
		Timezone:       taskRecord.Timezone,
	}
}

// runResponseFromEnt 将执行历史 Ent 实体转换为 API 响应。
//
// 参数:
//   - runRecord: 执行历史 Ent 实体。
//
// 返回值:
//   - runResponse: API 响应 DTO。
func runResponseFromEnt(runRecord *ent.Run) runResponse {
	if runRecord == nil {
		return runResponse{}
	}
	var exitCode *int
	if runRecord.ExitCode != nil {
		value := *runRecord.ExitCode
		exitCode = &value
	}
	return runResponse{
		ID:           runRecord.ID,
		Status:       runRecord.Status,
		ExitCode:     exitCode,
		DurationMs:   runRecord.DurationMs,
		ErrorSummary: runRecord.ErrorSummary,
		Stdout:       runRecord.Stdout,
		Stderr:       runRecord.Stderr,
	}
}

// registerAuthRoutes 注册管理员初始化、登录和当前用户查询路由。
//
// 参数:
//   - router: Gin engine。
//   - service: 认证服务接口。
//
// 返回值:
//   - 无。
func registerAuthRoutes(router *gin.Engine, service AuthService) {
	router.POST("/api/auth/init", func(ctx *gin.Context) {
		if service == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "auth_unavailable", "Authentication service is unavailable", nil)
			return
		}

		var req initializeAdminRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		admin, err := service.InitializeAdmin(ctx.Request.Context(), req.Username, req.Password)
		if err != nil {
			if errors.Is(err, auth.ErrAdminAlreadyInitialized) {
				writeAPIError(ctx, http.StatusConflict, "admin_already_initialized", "Admin is already initialized", nil)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Admin initialization failed", nil)
			return
		}

		ctx.JSON(http.StatusCreated, initializeAdminResponse{
			AdminID:  admin.ID,
			Username: admin.Username,
		})
	})

	router.POST("/api/auth/login", func(ctx *gin.Context) {
		if service == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "auth_unavailable", "Authentication service is unavailable", nil)
			return
		}

		var req loginRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		result, err := service.Login(ctx.Request.Context(), req.Username, req.Password, auth.LoginMetadata{
			UserAgent: ctx.GetHeader("User-Agent"),
			IP:        ctx.ClientIP(),
		})
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				writeAPIError(ctx, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password", nil)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Authentication failed", nil)
			return
		}

		http.SetCookie(ctx.Writer, &http.Cookie{
			Name:     SessionCookieName,
			Value:    result.SessionToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour),
		})
		ctx.JSON(http.StatusOK, loginResponse{
			AdminID:   result.Principal.AdminID,
			Username:  result.Principal.Username,
			CSRFToken: result.CSRFToken,
		})
	})

	router.GET("/api/auth/me", func(ctx *gin.Context) {
		if service == nil {
			writeUnauthorized(ctx)
			return
		}
		cookie, err := ctx.Request.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeUnauthorized(ctx)
			return
		}
		principal, err := service.AuthenticateSession(ctx.Request.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidSession) {
				writeUnauthorized(ctx)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Authentication failed", nil)
			return
		}
		ctx.JSON(http.StatusOK, meResponse{
			AdminID:  principal.AdminID,
			Username: principal.Username,
		})
	})
}

type initializeAdminRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type initializeAdminResponse struct {
	AdminID  int    `json:"adminId"`
	Username string `json:"username"`
}

// writeUnauthorized 写入稳定的未认证错误响应。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - 无。
func writeUnauthorized(ctx *gin.Context) {
	writeAPIError(ctx, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
}

// writeAPIError 写入统一 API 错误结构。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - status: HTTP 状态码。
//   - code: 稳定机器错误码。
//   - message: 默认错误说明。
//   - details: 可安全返回给调用方的结构化细节。
//
// 返回值:
//   - 无。
func writeAPIError(ctx *gin.Context, status int, code string, message string, details any) {
	ctx.JSON(status, apiErrorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AdminID   int    `json:"adminId"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrfToken"`
}

type meResponse struct {
	AdminID  int    `json:"adminId"`
	Username string `json:"username"`
}

type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

type createTaskRequest struct {
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	Enabled             *bool               `json:"enabled"`
	CronExpression      string              `json:"cronExpression" binding:"required"`
	Timezone            string              `json:"timezone"`
	ConfirmCronWarnings bool                `json:"confirmCronWarnings"`
	Runner              createRunnerRequest `json:"runner" binding:"required"`
}

type createRunnerRequest struct {
	Type             string            `json:"type" binding:"required"`
	Inline           string            `json:"inline"`
	ScriptPath       string            `json:"scriptPath"`
	Args             []string          `json:"args"`
	WorkDir          string            `json:"workDir"`
	Env              map[string]string `json:"env"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	OutputLimitBytes int               `json:"outputLimitBytes"`
}

type taskResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CronExpression string `json:"cronExpression"`
	Timezone       string `json:"timezone"`
}

type runResponse struct {
	ID           int           `json:"id"`
	Status       entrun.Status `json:"status"`
	ExitCode     *int          `json:"exitCode"`
	DurationMs   int64         `json:"durationMs"`
	ErrorSummary string        `json:"errorSummary"`
	Stdout       string        `json:"stdout"`
	Stderr       string        `json:"stderr"`
}
