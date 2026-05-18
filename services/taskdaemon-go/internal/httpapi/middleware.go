package httpapi

import "github.com/gin-gonic/gin"

// useCoreMiddleware 按固定顺序注册 HTTP API 全局 middleware。
//
// 参数:
//   - router: Gin engine。
//   - opts: router 可选能力。
//
// 返回值:
//   - 无。
func useCoreMiddleware(router *gin.Engine, opts Options) {
	logger := loggerOrDefault(opts.Logger)
	runtimeConfig := runtimeConfigOption(opts)
	router.Use(traceIDMiddleware())
	router.Use(responseOptionsMiddleware(runtimeConfig))
	router.Use(requestLoggerMiddleware(logger, runtimeConfig))
	router.Use(recoveryMiddleware())
}
