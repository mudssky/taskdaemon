package httpapi

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/audio"
	"taskdaemon/internal/data/ent"
)

const defaultAudioHistoryLimit = 50

// registerAudioRoutes 注册外部音频播放请求、历史和重放路由。
//
// 参数:
//   - router: Gin router。
//   - authService: 认证服务接口。
//   - audioService: 音频服务接口。
//
// 返回值:
//   - 无。
func registerAudioRoutes(router *gin.Engine, authService AuthService, audioService AudioService) {
	inbound := router.Group("/api/inbound/audio-play-requests")
	inbound.POST("", func(ctx *gin.Context) {
		if audioService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "audio_service_unavailable", "Audio service is unavailable", nil)
			return
		}
		var req submitAudioURLRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		record, err := audioService.SubmitURL(ctx.Request.Context(), bearerToken(ctx), audioURLInputFromRequest(req))
		writeAudioRecordResult(ctx, record, err, http.StatusCreated)
	})
	inbound.POST("/upload", func(ctx *gin.Context) {
		if audioService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "audio_service_unavailable", "Audio service is unavailable", nil)
			return
		}
		file, header, err := ctx.Request.FormFile("file")
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Audio file is required", gin.H{"field": "file"})
			return
		}
		defer file.Close()
		record, err := audioService.SubmitUpload(ctx.Request.Context(), bearerToken(ctx), uploadInputFromForm(ctx, file, header))
		writeAudioRecordResult(ctx, record, err, http.StatusCreated)
	})

	history := router.Group("/api/audio/history")
	history.Use(requireSession(authService))
	history.GET("", func(ctx *gin.Context) {
		if audioService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "audio_service_unavailable", "Audio service is unavailable", nil)
			return
		}
		records, err := audioService.ListHistory(ctx.Request.Context(), audioHistoryLimit(ctx))
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "audio_history_failed", "Audio history query failed", nil)
			return
		}
		responses := make([]audioRecordResponse, 0, len(records))
		for _, record := range records {
			responses = append(responses, audioRecordResponseFromEnt(record))
		}
		writeAPIOK(ctx, gin.H{"records": responses})
	})
	history.POST("/:id/replay", func(ctx *gin.Context) {
		if audioService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "audio_service_unavailable", "Audio service is unavailable", nil)
			return
		}
		recordID, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		record, err := audioService.Replay(ctx.Request.Context(), recordID)
		writeAudioRecordResult(ctx, record, err, http.StatusOK)
	})
}

// uploadInputFromForm 将 multipart form 转为音频上传输入。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - file: 上传文件 reader。
//   - header: 上传文件头。
//
// 返回值:
//   - audio.UploadRequest: 音频服务输入。
func uploadInputFromForm(ctx *gin.Context, file multipart.File, header *multipart.FileHeader) audio.UploadRequest {
	return audio.UploadRequest{
		Reader:   file,
		Source:   ctx.PostForm("source"),
		Filename: header.Filename,
		MIMEType: header.Header.Get("Content-Type"),
		Size:     header.Size,
	}
}

// writeAudioRecordResult 写入音频记录结果或映射稳定 API 错误。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - record: 音频记录。
//   - err: 音频服务错误。
//   - successStatus: 成功 HTTP 状态码。
//
// 返回值:
//   - 无。
func writeAudioRecordResult(ctx *gin.Context, record *ent.AudioRecord, err error, successStatus int) {
	if err == nil {
		writeAPISuccess(ctx, successStatus, audioRecordResponseFromEnt(record))
		return
	}
	writeAudioError(ctx, err)
}

// writeAudioError 将音频服务错误映射为稳定 API 错误。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - err: 音频服务错误。
//
// 返回值:
//   - 无。
func writeAudioError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, audio.ErrUnauthorized):
		writeAPIError(ctx, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
	case errors.Is(err, audio.ErrInvalidInput):
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid audio play request", nil)
	case errors.Is(err, audio.ErrFileTooLarge):
		writeAPIError(ctx, http.StatusRequestEntityTooLarge, "audio_file_too_large", "Audio file is too large", nil)
	case errors.Is(err, audio.ErrURLRejected):
		writeAPIError(ctx, http.StatusBadRequest, "audio_url_rejected", "Audio URL is rejected", nil)
	case errors.Is(err, audio.ErrMIMERejected):
		writeAPIError(ctx, http.StatusBadRequest, "audio_mime_rejected", "Audio MIME type is rejected", nil)
	case errors.Is(err, audio.ErrRecordNotFound):
		writeAPIError(ctx, http.StatusNotFound, "audio_record_not_found", "Audio record not found", nil)
	case errors.Is(err, audio.ErrQueueFull):
		writeAPIError(ctx, http.StatusConflict, "audio_queue_full", "Audio playback queue is full", nil)
	default:
		writeAPIError(ctx, http.StatusInternalServerError, "audio_request_failed", "Audio play request failed", nil)
	}
}

// bearerToken 提取 Authorization Bearer Token。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - string: Bearer Token 原文；缺失或格式不匹配时返回空字符串。
func bearerToken(ctx *gin.Context) string {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if header == "" {
		return ""
	}
	kind, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(kind, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// audioHistoryLimit 解析历史查询 limit。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - int: 查询条数限制。
func audioHistoryLimit(ctx *gin.Context) int {
	value := strings.TrimSpace(ctx.Query("limit"))
	if value == "" {
		return defaultAudioHistoryLimit
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 0 {
		return defaultAudioHistoryLimit
	}
	return limit
}

// parsePositiveID 解析正整数 path 参数。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - name: path 参数名。
//
// 返回值:
//   - int: 解析后的 ID。
//   - bool: 解析成功时返回 true；失败时已写入 API 错误。
func parsePositiveID(ctx *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(ctx.Param(name))
	if err != nil || value <= 0 {
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid path parameter", gin.H{"field": name})
		return 0, false
	}
	return value, true
}
