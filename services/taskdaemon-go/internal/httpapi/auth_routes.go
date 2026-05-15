package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
)

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
		username, password, ok := validateAuthCredentials(ctx, req.Username, req.Password)
		if !ok {
			return
		}
		result, err := service.InitializeAdminWithSession(ctx.Request.Context(), username, password, auth.LoginMetadata{
			UserAgent: ctx.GetHeader("User-Agent"),
			IP:        ctx.ClientIP(),
		})
		if err != nil {
			if errors.Is(err, auth.ErrAdminAlreadyInitialized) {
				writeAPIError(ctx, http.StatusConflict, "admin_already_initialized", "Admin is already initialized", nil)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Admin initialization failed", nil)
			return
		}

		writeSessionCookie(ctx, result.SessionToken)
		writeAPISuccess(ctx, http.StatusCreated, initializeAdminResponse{
			AdminID:   result.Principal.AdminID,
			Username:  result.Principal.Username,
			CSRFToken: result.CSRFToken,
		})
	})

	router.GET("/api/auth/status", func(ctx *gin.Context) {
		if service == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "auth_unavailable", "Authentication service is unavailable", nil)
			return
		}
		initialized, err := service.IsAdminInitialized(ctx.Request.Context())
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Authentication status query failed", nil)
			return
		}
		response := authStatusResponse{Initialized: initialized}
		if !initialized {
			writeAPIOK(ctx, response)
			return
		}

		cookie, err := ctx.Request.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIOK(ctx, response)
			return
		}
		principal, err := service.AuthenticateSession(ctx.Request.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidSession) {
				writeAPIOK(ctx, response)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Authentication failed", nil)
			return
		}
		response.Authenticated = true
		response.Admin = &authAdminResponse{AdminID: principal.AdminID, Username: principal.Username}
		writeAPIOK(ctx, response)
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
		username, password, ok := validateAuthCredentials(ctx, req.Username, req.Password)
		if !ok {
			return
		}
		result, err := service.Login(ctx.Request.Context(), username, password, auth.LoginMetadata{
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

		writeSessionCookie(ctx, result.SessionToken)
		writeAPIOK(ctx, loginResponse{
			AdminID:   result.Principal.AdminID,
			Username:  result.Principal.Username,
			CSRFToken: result.CSRFToken,
		})
	})

	router.POST("/api/auth/logout", func(ctx *gin.Context) {
		if service == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "auth_unavailable", "Authentication service is unavailable", nil)
			return
		}
		cookie, err := ctx.Request.Cookie(SessionCookieName)
		if err == nil && cookie.Value != "" {
			if err := service.Logout(ctx.Request.Context(), cookie.Value); err != nil {
				writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Logout failed", nil)
				return
			}
		}
		clearSessionCookie(ctx)
		writeAPIOK(ctx, nil)
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
		writeAPIOK(ctx, meResponse{
			AdminID:  principal.AdminID,
			Username: principal.Username,
		})
	})
}

// writeSessionCookie 写入管理员 session cookie。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - token: 原始 session token。
//
// 返回值:
//   - 无。
func writeSessionCookie(ctx *gin.Context, token string) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

// clearSessionCookie 让浏览器立即移除管理员 session cookie。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - 无。
func clearSessionCookie(ctx *gin.Context) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// validateAuthCredentials 校验认证请求的必填凭证字段，并写入字段级错误响应。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - username: 请求中的管理员用户名。
//   - password: 请求中的管理员密码。
//
// 返回值:
//   - string: 去除首尾空白后的用户名。
//   - string: 原始密码。
//   - bool: true 表示凭证字段完整，可继续调用认证服务。
func validateAuthCredentials(ctx *gin.Context, username string, password string) (string, string, bool) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Username is required", gin.H{"field": "username"})
		return "", "", false
	}
	if password == "" {
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Password is required", gin.H{"field": "password"})
		return "", "", false
	}
	return trimmedUsername, password, true
}
