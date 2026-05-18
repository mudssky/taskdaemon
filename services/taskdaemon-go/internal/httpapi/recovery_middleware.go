package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
