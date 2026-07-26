package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/template"
)

// registerTemplateRoutes 注册备份模板 API（T7a）。
//
// 参数:
//   - router: Gin engine。
//   - authService: 认证服务。
//   - templates: 模板服务；nil 时返回 503。
//
// 返回值:
//   - 无。
func registerTemplateRoutes(router *gin.Engine, authService AuthService, templates *template.Service) {
	group := router.Group("/api/templates")
	group.Use(requireSession(authService))

	group.GET("", func(ctx *gin.Context) {
		if templates == nil || templates.Registry == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, template.CodeUnavailable, "Template service is unavailable", nil)
			return
		}
		defs := templates.Registry.List()
		items := make([]templateDefinitionResponse, 0, len(defs))
		for _, def := range defs {
			items = append(items, templateDefinitionFromDomain(def))
		}
		writeAPIOK(ctx, gin.H{"templates": items})
	})

	group.GET("/:id", func(ctx *gin.Context) {
		if templates == nil || templates.Registry == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, template.CodeUnavailable, "Template service is unavailable", nil)
			return
		}
		id := ctx.Param("id")
		def, err := templates.Registry.Get(id)
		if err != nil {
			writeTemplateError(ctx, err)
			return
		}
		writeAPIOK(ctx, templateDefinitionFromDomain(def))
	})

	group.POST("/:id/render", func(ctx *gin.Context) {
		if templates == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, template.CodeUnavailable, "Template service is unavailable", nil)
			return
		}
		id := ctx.Param("id")
		var req templateRenderRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, template.CodeInvalidJSON, "Invalid request body", gin.H{"field": "body"})
			return
		}
		if req.Params == nil {
			req.Params = map[string]any{}
		}
		draft, err := templates.Render(id, req.Params)
		if err != nil {
			writeTemplateError(ctx, err)
			return
		}
		writeAPIOK(ctx, draft)
	})
}

// writeTemplateError 将模板领域错误映射为 TEMPLATE_* envelope。
//
// 参数:
//   - ctx: Gin 上下文。
//   - err: 模板错误。
//
// 返回值:
//   - 无。
func writeTemplateError(ctx *gin.Context, err error) {
	if ve, ok := template.AsValidationError(err); ok {
		writeAPIError(ctx, http.StatusBadRequest, ve.Code, ve.Message, gin.H{"fields": ve.Fields})
		return
	}
	if de, ok := template.AsDomainError(err); ok {
		status := http.StatusBadRequest
		switch de.Code {
		case template.CodeNotFound:
			status = http.StatusNotFound
		case template.CodeUnavailable:
			status = http.StatusServiceUnavailable
		case template.CodeRenderFailed:
			status = http.StatusInternalServerError
		case template.CodeRunnerUnsupported:
			status = http.StatusBadRequest
		}
		writeAPIError(ctx, status, de.Code, de.Message, de.Details)
		return
	}
	ctx.Error(err)
	writeAPIError(ctx, http.StatusInternalServerError, template.CodeRenderFailed, "Template operation failed", nil)
}
