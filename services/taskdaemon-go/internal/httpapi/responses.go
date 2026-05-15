package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	responseEnvelopeSuccessCode = 0
	responseEnvelopeErrorCode   = 1
	responseEnvelopeOKMessage   = "ok"

	traceIDHeader = "X-Trace-Id"
	traceIDKey    = "trace_id"
)

// apiResponse 是 HTTP API 的全局响应 envelope。
type apiResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"msg"`
	Data    any       `json:"data"`
	TraceID string    `json:"traceId,omitempty"`
	Error   *apiError `json:"error,omitempty"`
}

// apiError 是 API 错误对象，承载稳定机器码和安全细节。
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

// writeAPISuccess 写入统一成功响应。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - status: HTTP 状态码。
//   - data: 业务响应数据。
//
// 返回值:
//   - 无。
func writeAPISuccess(ctx *gin.Context, status int, data any) {
	if data == nil {
		data = nil
	}
	ctx.JSON(status, apiResponse{
		Code:    responseEnvelopeSuccessCode,
		Message: responseEnvelopeOKMessage,
		Data:    data,
		TraceID: responseTraceID(ctx),
	})
}

// writeAPIOK 写入 200 OK 成功响应。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - data: 业务响应数据。
//
// 返回值:
//   - 无。
func writeAPIOK(ctx *gin.Context, data any) {
	writeAPISuccess(ctx, http.StatusOK, data)
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
	ctx.JSON(status, apiResponse{
		Code:    responseEnvelopeErrorCode,
		Message: message,
		Data:    nil,
		TraceID: responseTraceID(ctx),
		Error: &apiError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
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

// responseTraceID 返回当前响应是否应包含的 trace id。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - string: 配置允许时返回 trace id，否则返回空字符串。
func responseTraceID(ctx *gin.Context) string {
	if !includeTraceInResponse(ctx) {
		return ""
	}
	return traceIDFromContext(ctx)
}

// includeTraceInResponse 判断当前响应 body 是否包含 traceId。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - bool: true 表示写入响应体。
func includeTraceInResponse(ctx *gin.Context) bool {
	value, ok := ctx.Get(includeTraceInResponseKey)
	if !ok {
		return false
	}
	enabled, ok := value.(bool)
	return ok && enabled
}
