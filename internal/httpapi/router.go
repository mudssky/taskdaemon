package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Options 控制 HTTP router 的可选路由。
type Options struct {
	EnableSwagger bool
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

	if opts.EnableSwagger {
		router.GET("/swagger/index.html", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "Swagger UI is enabled. Generated docs will be mounted here.")
		})
	}

	return router
}
