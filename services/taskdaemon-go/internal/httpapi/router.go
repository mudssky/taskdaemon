package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
)

const (
	// SessionCookieName 是 HTTP API 使用的管理员 session Cookie 名称。
	SessionCookieName = "taskdaemon_session"
)

// Options 控制 HTTP router 的可选路由。
type Options struct {
	EnableSwagger bool
	Auth          AuthService
}

// AuthService 定义 HTTP handler 依赖的认证服务能力。
type AuthService interface {
	InitializeAdmin(context.Context, string, string) (auth.AdminAccount, error)
	Login(context.Context, string, string, auth.LoginMetadata) (auth.LoginResult, error)
	AuthenticateSession(context.Context, string) (auth.Principal, error)
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

	if opts.EnableSwagger {
		router.GET("/swagger/index.html", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "Swagger UI is enabled. Generated docs will be mounted here.")
		})
	}

	return router
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
