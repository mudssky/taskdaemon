package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/config"
)

// ConfigReloadFunc 定义 daemon 运行时配置重载能力。
type ConfigReloadFunc func(context.Context) (config.ReloadResult, error)

// ConfigSectionWriteFunc 定义按 section 写入配置能力。
//
// 参数:
//   - ctx: 请求上下文。
//   - section: section 名。
//   - body: 原始 JSON body。
//
// 返回值:
//   - ConfigSectionWriteResponse: 成功响应 data。
//   - error: 校验或写入失败。
type ConfigSectionWriteFunc func(ctx context.Context, section string, body []byte) (ConfigSectionWriteResponse, error)

// registerConfigRoutes 注册配置管理 API。
//
// 参数:
//   - router: Gin router。
//   - authService: 认证服务。
//   - reload: 运行时配置重载函数。
//   - runtimeConfig: 运行时配置快照。
//   - writeSection: section 写入函数；nil 时 PUT 返回 503。
//
// 返回值:
//   - 无。
func registerConfigRoutes(
	router *gin.Engine,
	authService AuthService,
	reload ConfigReloadFunc,
	runtimeConfig *RuntimeConfig,
	writeSection ConfigSectionWriteFunc,
) {
	group := router.Group("/api/config")
	group.Use(requireSession(authService))
	group.GET("/audio", func(ctx *gin.Context) {
		writeAPIOK(ctx, audioConfigResponseFromConfig(runtimeConfig.Audio()))
	})
	group.PUT("/:section", func(ctx *gin.Context) {
		section := strings.TrimSpace(ctx.Param("section"))
		if section == "" {
			writeAPIError(ctx, http.StatusNotFound, config.CodeSectionUnknown, "Unknown config section", gin.H{"section": section})
			return
		}
		if !config.IsSectionWritable(section) {
			writeAPIError(ctx, http.StatusNotFound, config.CodeSectionUnknown, "Unknown or non-writable config section", gin.H{"section": section})
			return
		}
		if writeSection == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, config.CodePathUnavailable, "Config write is unavailable", nil)
			return
		}
		body, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, config.CodeInvalidJSON, "Failed to read request body", nil)
			return
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			writeAPIError(ctx, http.StatusBadRequest, config.CodeInvalidJSON, "Request body is required", nil)
			return
		}
		if !json.Valid(body) {
			writeAPIError(ctx, http.StatusBadRequest, config.CodeInvalidJSON, "Request body must be valid JSON", nil)
			return
		}
		result, err := writeSection(ctx.Request.Context(), section, body)
		if err != nil {
			writeConfigWriteError(ctx, err)
			return
		}
		writeAPIOK(ctx, result)
	})
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

// writeConfigWriteError 将配置写入错误映射为 CONFIG_* envelope。
//
// 参数:
//   - ctx: Gin 上下文。
//   - err: 写入错误。
//
// 返回值:
//   - 无。
func writeConfigWriteError(ctx *gin.Context, err error) {
	if ve, ok := config.AsValidationError(err); ok {
		status := http.StatusBadRequest
		details := gin.H{"fields": ve.Fields}
		writeAPIError(ctx, status, ve.Code, ve.Message, details)
		return
	}
	if we, ok := config.AsWriteError(err); ok {
		status := http.StatusInternalServerError
		switch we.Code {
		case config.CodePathUnavailable:
			status = http.StatusServiceUnavailable
		case config.CodeSectionUnknown:
			status = http.StatusNotFound
		case config.CodeWriteFailed, config.CodeReloadFailed:
			status = http.StatusInternalServerError
		}
		writeAPIError(ctx, status, we.Code, we.Message, nil)
		return
	}
	ctx.Error(err)
	writeAPIError(ctx, http.StatusInternalServerError, config.CodeWriteFailed, "Config write failed", nil)
}

// DefaultConfigSectionWriter 基于 RuntimeConfig 与可选 App 钩子构造默认写入器。
// 测试可直接注入 ConfigSectionWriteFunc。
//
// 参数:
//   - writeAudio: 写入 audio 的业务函数。
//
// 返回值:
//   - ConfigSectionWriteFunc: section 分发函数。
func DefaultConfigSectionWriter(writeAudio func(context.Context, config.AudioSectionPatch) (ConfigSectionWriteResponse, error)) ConfigSectionWriteFunc {
	return func(ctx context.Context, section string, body []byte) (ConfigSectionWriteResponse, error) {
		switch section {
		case "audio":
			var req audioConfigWriteRequest
			if err := json.Unmarshal(body, &req); err != nil {
				return ConfigSectionWriteResponse{}, &config.ValidationError{
					Code:    config.CodeInvalidJSON,
					Message: "Request body must be valid JSON object",
				}
			}
			// 拒绝未知顶层字段（仅允许已知键）
			if err := rejectUnknownAudioKeys(body); err != nil {
				return ConfigSectionWriteResponse{}, err
			}
			patch, fieldErrs := audioPatchFromRequest(req)
			if len(fieldErrs) > 0 {
				return ConfigSectionWriteResponse{}, &config.ValidationError{
					Code:    fieldErrs[0].Code,
					Message: "config validation failed",
					Fields:  fieldErrs,
				}
			}
			if writeAudio == nil {
				return ConfigSectionWriteResponse{}, &config.WriteError{
					Code:    config.CodePathUnavailable,
					Message: "audio config writer is not configured",
				}
			}
			return writeAudio(ctx, patch)
		default:
			return ConfigSectionWriteResponse{}, &config.WriteError{
				Code:    config.CodeSectionUnknown,
				Message: "unknown config section",
			}
		}
	}
}

// rejectUnknownAudioKeys 拒绝 audio 请求中的未知顶层键。
//
// 参数:
//   - body: JSON body。
//
// 返回值:
//   - error: 存在未知键时返回 ValidationError。
func rejectUnknownAudioKeys(body []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return &config.ValidationError{Code: config.CodeInvalidJSON, Message: "Request body must be valid JSON object"}
	}
	allowed := map[string]struct{}{
		"autoplay": {},
		"playback": {},
		"inbound":  {},
		"history":  {},
		"ffmpeg":   {},
	}
	var fields []config.FieldError
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			fields = append(fields, config.FieldError{
				Path:   "audio." + key,
				Reason: "unknown or non-writable field",
				Code:   config.CodeFieldNotWritable,
			})
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &config.ValidationError{
		Code:    config.CodeFieldNotWritable,
		Message: "config validation failed",
		Fields:  fields,
	}
}
