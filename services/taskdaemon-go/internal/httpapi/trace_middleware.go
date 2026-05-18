package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
