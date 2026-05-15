package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"taskdaemon/internal/auth"
)

const includeTraceInResponseKey = "include_trace_in_response"

// traceIDMiddleware 为每个请求补齐 trace id 并写入响应头。
//
// 参数:
//   - 无。
//
// 返回值:
//   - gin.HandlerFunc: trace id middleware。
func traceIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		traceID := strings.TrimSpace(ctx.GetHeader(traceIDHeader))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		ctx.Set(traceIDKey, traceID)
		ctx.Writer.Header().Set(traceIDHeader, traceID)
		ctx.Next()
	}
}

// responseOptionsMiddleware 保存响应 envelope 的运行时选项。
//
// 参数:
//   - includeTraceInResponse: 是否在响应 body 中写入 traceId。
//
// 返回值:
//   - gin.HandlerFunc: 响应选项 middleware。
func responseOptionsMiddleware(includeTraceInResponse bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(includeTraceInResponseKey, includeTraceInResponse)
		ctx.Next()
	}
}

// requestLoggerMiddleware 记录 HTTP 请求摘要。
//
// 参数:
//   - logger: 结构化 logger。
//
// 返回值:
//   - gin.HandlerFunc: 请求日志 middleware。
func requestLoggerMiddleware(logger *slog.Logger) gin.HandlerFunc {
	logger = loggerOrDefault(logger)
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()

		status := ctx.Writer.Status()
		if shouldSkipRequestLog(ctx.FullPath(), status) {
			return
		}
		attrs := []any{
			"method", ctx.Request.Method,
			"path", requestLogPath(ctx),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"trace_id", traceIDFromContext(ctx),
		}
		if len(ctx.Errors) > 0 {
			attrs = append(attrs, "error", ctx.Errors.String())
		}
		if status >= http.StatusInternalServerError {
			logger.Error("http request", attrs...)
			return
		}
		if status >= http.StatusBadRequest {
			logger.Warn("http request", attrs...)
			return
		}
		logger.Info("http request", attrs...)
	}
}

// recoveryMiddleware 将 panic 转为统一 500 envelope。
//
// 参数:
//   - 无。
//
// 返回值:
//   - gin.HandlerFunc: panic recovery middleware。
func recoveryMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				ctx.Error(errPanicRecovered)
				writeAPIError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
				ctx.Abort()
			}
		}()
		ctx.Next()
	}
}

// shouldSkipRequestLog 判断请求是否可以跳过访问日志。
//
// 参数:
//   - routePath: Gin 匹配到的路由模板。
//   - status: HTTP 响应状态码。
//
// 返回值:
//   - bool: true 表示跳过日志。
func shouldSkipRequestLog(routePath string, status int) bool {
	return routePath == "/api/health" && status < http.StatusBadRequest
}

// requestLogPath 返回用于日志的路径。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - string: 优先使用路由模板，避免高基数 task id 污染日志字段。
func requestLogPath(ctx *gin.Context) string {
	if fullPath := ctx.FullPath(); fullPath != "" {
		return fullPath
	}
	return ctx.Request.URL.Path
}

// traceIDFromContext 读取当前请求 trace id。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - string: 当前请求 trace id，缺失时为空。
func traceIDFromContext(ctx *gin.Context) string {
	traceID, _ := ctx.Get(traceIDKey)
	value, _ := traceID.(string)
	return value
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
