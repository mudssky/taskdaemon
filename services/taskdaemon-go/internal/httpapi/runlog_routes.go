package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/runlog"
)

// RunLogService 定义 HTTP handler 依赖的 run 日志能力。
type RunLogService interface {
	GetMeta(ctx context.Context, runID int) (runlog.Meta, error)
	OpenDownload(ctx context.Context, runID int) (io.ReadCloser, int64, string, error)
	ReadTail(ctx context.Context, runID int, lines int) ([]byte, error)
	ReadRange(ctx context.Context, runID int, offset int64, length int64) ([]byte, error)
}

// registerRunLogRoutes 注册 run 日志查询与下载路由。
//
// 参数:
//   - router: Gin engine。
//   - authService: 认证服务。
//   - runLogService: runlog 服务。
//
// 返回值:
//   - 无。
func registerRunLogRoutes(router *gin.Engine, authService AuthService, runLogService RunLogService) {
	if router == nil || runLogService == nil {
		return
	}
	// T6
	group := router.Group("/api/runs")
	if authService != nil {
		group.Use(requireSession(authService))
	}
	group.GET("/:runID/log/meta", func(ctx *gin.Context) {
		runID, ok := parsePositiveID(ctx, "runID")
		if !ok {
			return
		}
		meta, err := runLogService.GetMeta(ctx.Request.Context(), runID)
		if err != nil {
			writeRunLogError(ctx, err)
			return
		}
		writeAPIOK(ctx, runLogMetaFromService(meta))
	})
	group.GET("/:runID/log", func(ctx *gin.Context) {
		runID, ok := parsePositiveID(ctx, "runID")
		if !ok {
			return
		}
		reader, size, filename, err := runLogService.OpenDownload(ctx.Request.Context(), runID)
		if err != nil {
			writeRunLogError(ctx, err)
			return
		}
		defer reader.Close()
		ctx.Header("Content-Type", "text/plain; charset=utf-8")
		ctx.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		ctx.Status(http.StatusOK)
		// 流式写出，不整体读入内存
		_, _ = io.Copy(ctx.Writer, reader)
	})
	group.GET("/:runID/log/tail", func(ctx *gin.Context) {
		runID, ok := parsePositiveID(ctx, "runID")
		if !ok {
			return
		}
		lines := 100
		if raw := ctx.Query("lines"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr != nil || parsed <= 0 {
				writeAPIError(ctx, http.StatusBadRequest, "RUNLOG_INVALID_RANGE", "Invalid lines parameter", gin.H{"lines": raw})
				return
			}
			lines = parsed
		}
		content, err := runLogService.ReadTail(ctx.Request.Context(), runID, lines)
		if err != nil {
			writeRunLogError(ctx, err)
			return
		}
		writeAPIOK(ctx, runLogTailResponse{
			RunID:   runID,
			Lines:   lines,
			Content: string(content),
		})
	})
	group.GET("/:runID/log/range", func(ctx *gin.Context) {
		runID, ok := parsePositiveID(ctx, "runID")
		if !ok {
			return
		}
		offset, err := strconv.ParseInt(ctx.Query("offset"), 10, 64)
		if err != nil || offset < 0 {
			writeAPIError(ctx, http.StatusBadRequest, "RUNLOG_INVALID_RANGE", "Invalid offset parameter", gin.H{"offset": ctx.Query("offset")})
			return
		}
		length, err := strconv.ParseInt(ctx.Query("length"), 10, 64)
		if err != nil || length <= 0 {
			writeAPIError(ctx, http.StatusBadRequest, "RUNLOG_INVALID_RANGE", "Invalid length parameter", gin.H{"length": ctx.Query("length")})
			return
		}
		content, err := runLogService.ReadRange(ctx.Request.Context(), runID, offset, length)
		if err != nil {
			writeRunLogError(ctx, err)
			return
		}
		writeAPIOK(ctx, runLogRangeResponse{
			RunID:   runID,
			Offset:  offset,
			Length:  length,
			Content: string(content),
		})
	})
}

// runLogMetaFromService 将服务 Meta 转为 API DTO。
//
// 参数:
//   - meta: 服务层元信息。
//
// 返回值:
//   - runLogMetaResponse: API 响应。
func runLogMetaFromService(meta runlog.Meta) runLogMetaResponse {
	resp := runLogMetaResponse{
		RunID:       meta.RunID,
		Status:      string(meta.Status),
		SizeBytes:   meta.SizeBytes,
		WriteFailed: meta.WriteFailed,
		Available:   meta.Available,
	}
	if meta.CreatedAt != nil {
		formatted := meta.CreatedAt.UTC().Format(time.RFC3339Nano)
		resp.CreatedAt = &formatted
	}
	return resp
}

// writeRunLogError 将 runlog 错误映射为稳定 RUNLOG_* 响应。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - err: 服务错误。
//
// 返回值:
//   - 无。
func writeRunLogError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, runlog.ErrRunNotFound):
		writeAPIError(ctx, http.StatusNotFound, "RUNLOG_RUN_NOT_FOUND", "Run not found", nil)
	case errors.Is(err, runlog.ErrArchiveNotFound):
		writeAPIError(ctx, http.StatusNotFound, "RUNLOG_ARCHIVE_NOT_FOUND", "Run log archive not found", nil)
	case errors.Is(err, runlog.ErrInvalidRange):
		writeAPIError(ctx, http.StatusBadRequest, "RUNLOG_INVALID_RANGE", "Invalid log range", nil)
	case errors.Is(err, runlog.ErrReadFailed):
		writeAPIError(ctx, http.StatusInternalServerError, "RUNLOG_READ_FAILED", "Failed to read run log", nil)
	default:
		writeAPIError(ctx, http.StatusInternalServerError, "RUNLOG_READ_FAILED", "Failed to read run log", nil)
	}
}
