package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/config"
)

// ConfigReloadFunc 定义 daemon 运行时配置重载能力。
type ConfigReloadFunc func(context.Context) (config.ReloadResult, error)

// registerConfigRoutes 注册配置管理 API。
//
// 参数:
//   - router: Gin router。
//   - authService: 认证服务。
//   - reload: 运行时配置重载函数。
//
// 返回值:
//   - 无。
func registerConfigRoutes(router *gin.Engine, authService AuthService, reload ConfigReloadFunc) {
	group := router.Group("/api/config")
	group.Use(requireSession(authService))
	group.POST("/reload", func(ctx *gin.Context) {
		if reload == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "config_reload_unavailable", "Config reload is unavailable", nil)
			return
		}
		result, err := reload(ctx.Request.Context())
		if err != nil {
			ctx.Error(err)
			writeAPIError(ctx, http.StatusInternalServerError, "config_reload_failed", "Config reload failed", nil)
			return
		}
		writeAPISuccess(ctx, http.StatusOK, result)
	})
}
