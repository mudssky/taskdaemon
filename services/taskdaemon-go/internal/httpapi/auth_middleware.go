package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
)

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
