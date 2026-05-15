package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
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
//   - runtimeConfig: HTTP API 运行时配置。
//
// 返回值:
//   - gin.HandlerFunc: 响应选项 middleware。
func responseOptionsMiddleware(runtimeConfig *RuntimeConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(includeTraceInResponseKey, runtimeConfig.IncludeTraceInResponse())
		ctx.Next()
	}
}

const redactedLogValue = "[REDACTED]"

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

type loggedBody struct {
	content   []byte
	truncated bool
}

type loggedResponseWriter struct {
	gin.ResponseWriter
	limit     int
	body      bytes.Buffer
	truncated bool
}

// newLoggedResponseWriter 创建会透传响应并缓存有限 body 的 writer。
//
// 参数:
//   - writer: Gin 原始响应 writer。
//   - limit: 最大缓存字节数。
//
// 返回值:
//   - *loggedResponseWriter: 响应体缓存 writer。
func newLoggedResponseWriter(writer gin.ResponseWriter, limit int) *loggedResponseWriter {
	return &loggedResponseWriter{
		ResponseWriter: writer,
		limit:          limit,
	}
}

// Write 写出响应并缓存有限字节用于日志。
//
// 参数:
//   - data: 响应片段。
//
// 返回值:
//   - int: 实际写出的字节数。
//   - error: 底层 writer 返回的错误。
func (writer *loggedResponseWriter) Write(data []byte) (int, error) {
	writer.capture(data)
	return writer.ResponseWriter.Write(data)
}

// WriteString 写出字符串响应并缓存有限字节用于日志。
//
// 参数:
//   - data: 响应字符串片段。
//
// 返回值:
//   - int: 实际写出的字节数。
//   - error: 底层 writer 返回的错误。
func (writer *loggedResponseWriter) WriteString(data string) (int, error) {
	writer.capture([]byte(data))
	return writer.ResponseWriter.WriteString(data)
}

// capture 缓存不超过限制的响应体片段。
//
// 参数:
//   - data: 响应片段。
//
// 返回值:
//   - 无。
func (writer *loggedResponseWriter) capture(data []byte) {
	if writer.limit <= 0 || len(data) == 0 {
		if len(data) > 0 {
			writer.truncated = true
		}
		return
	}
	remaining := writer.limit - writer.body.Len()
	if remaining <= 0 {
		writer.truncated = true
		return
	}
	if len(data) > remaining {
		writer.body.Write(data[:remaining])
		writer.truncated = true
		return
	}
	writer.body.Write(data)
}

// loggedBody 返回缓存到的响应体。
//
// 参数:
//   - 无。
//
// 返回值:
//   - loggedBody: 响应体片段与截断状态。
func (writer *loggedResponseWriter) loggedBody() loggedBody {
	return loggedBody{
		content:   writer.body.Bytes(),
		truncated: writer.truncated,
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

// captureRequestBody 读取并恢复请求体，供日志和后续 handler 共同使用。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - limit: 最大记录字节数。
//
// 返回值:
//   - loggedBody: 请求体片段与截断状态。
func captureRequestBody(ctx *gin.Context, limit int) loggedBody {
	if ctx.Request == nil || ctx.Request.Body == nil {
		return loggedBody{}
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, int64(limit)+1))
	if err != nil {
		ctx.Error(err)
		ctx.Request.Body = newCompositeReadCloser(bytes.NewReader(body), ctx.Request.Body)
		return limitedLoggedBody(body, limit)
	}
	ctx.Request.Body = newCompositeReadCloser(bytes.NewReader(body), ctx.Request.Body)
	return limitedLoggedBody(body, limit)
}

type compositeReadCloser struct {
	reader io.Reader
	closer io.Closer
}

// newCompositeReadCloser 将预读 body 与剩余原始流重新组合。
//
// 参数:
//   - prefix: 已经为了日志预读的 body 片段。
//   - rest: 原始请求体剩余流。
//
// 返回值:
//   - io.ReadCloser: 供后续 handler 正常读取的完整请求体。
func newCompositeReadCloser(prefix io.Reader, rest io.ReadCloser) io.ReadCloser {
	return compositeReadCloser{
		reader: io.MultiReader(prefix, rest),
		closer: rest,
	}
}

// Read 读取组合后的请求体。
//
// 参数:
//   - data: 读取缓冲区。
//
// 返回值:
//   - int: 读取字节数。
//   - error: 读取失败或 EOF 时返回的错误。
func (reader compositeReadCloser) Read(data []byte) (int, error) {
	return reader.reader.Read(data)
}

// Close 关闭底层原始请求体。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 底层 Close 返回的错误。
func (reader compositeReadCloser) Close() error {
	return reader.closer.Close()
}

// limitedLoggedBody 返回限制长度后的 body。
//
// 参数:
//   - body: 原始 body。
//   - limit: 最大记录字节数。
//
// 返回值:
//   - loggedBody: body 片段与截断状态。
func limitedLoggedBody(body []byte, limit int) loggedBody {
	if limit <= 0 {
		return loggedBody{truncated: len(body) > 0}
	}
	if len(body) > limit {
		return loggedBody{
			content:   body[:limit],
			truncated: true,
		}
	}
	return loggedBody{content: body}
}

// appendBodyLogAttrs 追加 body 日志字段。
//
// 参数:
//   - attrs: 已有日志属性。
//   - field: body 字段名前缀。
//   - body: body 片段与截断状态。
//   - redactFields: 需要脱敏的 JSON 字段名。
//
// 返回值:
//   - []any: 追加后的日志属性。
func appendBodyLogAttrs(attrs []any, field string, body loggedBody, redactFields []string) []any {
	attrs = append(attrs, field, sanitizedLogBody(body.content, redactFields))
	if body.truncated {
		attrs = append(attrs, field+"_truncated", true)
	}
	return attrs
}

// sanitizedLogBody 将 body 转为适合日志输出的脱敏值。
//
// 参数:
//   - body: body 字节片段。
//   - redactFields: 需要脱敏的 JSON 字段名。
//
// 返回值:
//   - any: JSON body 返回结构化对象，非 JSON body 返回字符串。
func sanitizedLogBody(body []byte, redactFields []string) any {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return redactedRawBody(string(body), redactFields)
	}
	redactJSONValue(decoded, redactFieldSet(redactFields))
	return decoded
}

// redactJSONValue 递归脱敏 JSON 对象中的敏感字段。
//
// 参数:
//   - value: JSON 解码后的值。
//   - fields: 需要脱敏的字段名集合。
//
// 返回值:
//   - 无。
func redactJSONValue(value any, fields map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if _, ok := fields[strings.ToLower(key)]; ok {
				typed[key] = redactedLogValue
				continue
			}
			redactJSONValue(nested, fields)
		}
	case []any:
		for _, item := range typed {
			redactJSONValue(item, fields)
		}
	}
}

// redactFieldSet 将脱敏字段配置转为大小写不敏感集合。
//
// 参数:
//   - fields: 脱敏字段名列表。
//
// 返回值:
//   - map[string]struct{}: 字段名集合。
func redactFieldSet(fields []string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field != "" {
			set[field] = struct{}{}
		}
	}
	return set
}

// redactedRawBody 对无法解析为 JSON 的 body 文本做保守脱敏。
//
// 参数:
//   - body: body 文本。
//   - fields: 需要脱敏的字段名列表。
//
// 返回值:
//   - string: 脱敏后的 body 文本。
func redactedRawBody(body string, fields []string) string {
	redacted := body
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		fieldPattern := regexp.QuoteMeta(field)
		jsonLikePattern := regexp.MustCompile(`(?i)(["']?` + fieldPattern + `["']?\s*:\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^,\s}\]]+)`)
		formLikePattern := regexp.MustCompile(`(?i)(` + fieldPattern + `\s*=\s*)[^&\s]+`)
		redacted = jsonLikePattern.ReplaceAllString(redacted, `${1}"`+redactedLogValue+`"`)
		redacted = formLikePattern.ReplaceAllString(redacted, `${1}`+redactedLogValue)
	}
	return redacted
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
