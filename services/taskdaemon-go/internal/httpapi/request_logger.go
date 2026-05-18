package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/config"
)

// requestLoggerMiddleware 记录 HTTP 请求摘要。
//
// 参数:
//   - logger: 结构化 logger。
//   - runtimeConfig: HTTP API 运行时配置。
//
// 返回值:
//   - gin.HandlerFunc: 请求日志 middleware。
func requestLoggerMiddleware(logger *slog.Logger, runtimeConfig *RuntimeConfig) gin.HandlerFunc {
	logger = loggerOrDefault(logger)
	return func(ctx *gin.Context) {
		cfg := normalizedHTTPLogConfig(runtimeConfig.HTTPLog())
		start := time.Now()
		var requestBody loggedBody
		if cfg.IncludeRequestBody {
			requestBody = captureRequestBody(ctx, cfg.MaxBodyBytes)
		}
		var responseBody *loggedResponseWriter
		if cfg.IncludeResponseBody {
			responseBody = newLoggedResponseWriter(ctx.Writer, cfg.MaxBodyBytes)
			ctx.Writer = responseBody
		}

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
		if cfg.IncludeRequestBody {
			attrs = appendBodyLogAttrs(attrs, "request_body", requestBody, cfg.RedactFields)
		}
		if cfg.IncludeResponseBody && responseBody != nil {
			attrs = appendBodyLogAttrs(attrs, "response_body", responseBody.loggedBody(), cfg.RedactFields)
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

// normalizedHTTPLogConfig 补齐 HTTP body 日志配置默认值。
//
// 参数:
//   - cfg: 原始 HTTP 日志配置。
//
// 返回值:
//   - config.LoggingHTTPConfig: 补齐后的 HTTP 日志配置。
func normalizedHTTPLogConfig(cfg config.LoggingHTTPConfig) config.LoggingHTTPConfig {
	defaults := config.Default().Logging.HTTP
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaults.MaxBodyBytes
	}
	if len(cfg.RedactFields) == 0 {
		cfg.RedactFields = defaults.RedactFields
	}
	return cfg
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
