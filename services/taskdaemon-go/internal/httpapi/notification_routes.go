package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/notify"
)

// NotificationService 定义站内通知查询与变更能力。
type NotificationService interface {
	List(ctx context.Context, filter notify.ListFilter) (notify.ListResult, error)
	UnreadCount(ctx context.Context) (int, error)
	MarkRead(ctx context.Context, id int) (*ent.Notification, error)
	MarkReadBatch(ctx context.Context, ids []int) (int, error)
	MarkAllRead(ctx context.Context) (int, error)
	Delete(ctx context.Context, id int) error
	ClearRead(ctx context.Context) (int, error)
}

// registerNotificationRoutes 注册站内通知查询与 sink 状态路由（T2a）。
//
// 参数:
//   - router: Gin engine。
//   - authService: 认证服务接口。
//   - notificationService: 站内通知服务。
//   - bus: 事件总线（用于 sink 状态）；可为 nil。
//
// 返回值:
//   - 无。
func registerNotificationRoutes(
	router *gin.Engine,
	authService AuthService,
	notificationService NotificationService,
	bus *notify.Bus,
) {
	group := router.Group("/api/notifications")
	group.Use(requireSession(authService))

	group.GET("", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		filter, ok := parseNotificationListFilter(ctx)
		if !ok {
			return
		}
		result, err := notificationService.List(ctx.Request.Context(), filter)
		if err != nil {
			if strings.Contains(err.Error(), "invalid notify severity") {
				writeAPIError(ctx, http.StatusBadRequest, "NOTIFY_INVALID_SEVERITY", "Invalid severity filter", gin.H{"field": "severity"})
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_LIST_FAILED", "Notification list query failed", nil)
			return
		}
		items := make([]notificationResponse, 0, len(result.Items))
		for _, record := range result.Items {
			items = append(items, notificationResponseFromEnt(record))
		}
		writeAPIOK(ctx, notificationListResponse{
			Notifications: items,
			Total:         result.Total,
			Page:          result.Page,
			PageSize:      result.PageSize,
		})
	})

	group.GET("/unread-count", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		count, err := notificationService.UnreadCount(ctx.Request.Context())
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_UNREAD_COUNT_FAILED", "Unread count query failed", nil)
			return
		}
		writeAPIOK(ctx, unreadCountResponse{Count: count})
	})

	group.GET("/sinks", func(ctx *gin.Context) {
		if bus == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification bus is unavailable", nil)
			return
		}
		statuses := bus.SinkStatuses()
		items := make([]sinkStatusResponse, 0, len(statuses))
		for _, status := range statuses {
			items = append(items, sinkStatusResponseFromNotify(status))
		}
		writeAPIOK(ctx, gin.H{"sinks": items})
	})

	group.POST("/read", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		var req markReadBatchRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		affected, err := notificationService.MarkReadBatch(ctx.Request.Context(), req.IDs)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_MARK_READ_FAILED", "Batch mark read failed", nil)
			return
		}
		writeAPIOK(ctx, affectedCountResponse{Affected: affected})
	})

	group.POST("/read-all", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		affected, err := notificationService.MarkAllRead(ctx.Request.Context())
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_MARK_READ_FAILED", "Mark all read failed", nil)
			return
		}
		writeAPIOK(ctx, affectedCountResponse{Affected: affected})
	})

	group.DELETE("/read", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		affected, err := notificationService.ClearRead(ctx.Request.Context())
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_CLEAR_READ_FAILED", "Clear read notifications failed", nil)
			return
		}
		writeAPIOK(ctx, affectedCountResponse{Affected: affected})
	})

	group.POST("/:id/read", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		id, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		record, err := notificationService.MarkRead(ctx.Request.Context(), id)
		if err != nil {
			writeNotificationError(ctx, err)
			return
		}
		writeAPIOK(ctx, notificationResponseFromEnt(record))
	})

	group.DELETE("/:id", func(ctx *gin.Context) {
		if notificationService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "NOTIFY_SINK_UNAVAILABLE", "Notification service is unavailable", nil)
			return
		}
		id, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		if err := notificationService.Delete(ctx.Request.Context(), id); err != nil {
			writeNotificationError(ctx, err)
			return
		}
		writeAPIOK(ctx, nil)
	})
}

// parseNotificationListFilter 解析列表 query 参数。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - notify.ListFilter: 筛选条件。
//   - bool: 解析成功时返回 true；失败时已写入错误响应。
func parseNotificationListFilter(ctx *gin.Context) (notify.ListFilter, bool) {
	page := 1
	if raw := ctx.Query("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIError(ctx, http.StatusBadRequest, "NOTIFY_INVALID_PAGE", "Invalid page parameter", gin.H{"field": "page"})
			return notify.ListFilter{}, false
		}
		page = parsed
	}
	pageSize := 20
	if raw := ctx.Query("pageSize"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIError(ctx, http.StatusBadRequest, "NOTIFY_INVALID_PAGE", "Invalid pageSize parameter", gin.H{"field": "pageSize"})
			return notify.ListFilter{}, false
		}
		pageSize = parsed
	}

	filter := notify.ListFilter{Page: page, PageSize: pageSize}
	if raw := ctx.Query("read"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid read parameter", gin.H{"field": "read"})
			return notify.ListFilter{}, false
		}
		filter.Read = &parsed
	}
	if raw := ctx.Query("severity"); raw != "" {
		severity := notify.Severity(raw)
		if !severity.Valid() {
			writeAPIError(ctx, http.StatusBadRequest, "NOTIFY_INVALID_SEVERITY", "Invalid severity filter", gin.H{"field": "severity"})
			return notify.ListFilter{}, false
		}
		filter.Severity = severity
	}
	return filter, true
}

// writeNotificationError 将通知服务错误映射为稳定 API 错误。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - err: 服务错误。
//
// 返回值:
//   - 无。
func writeNotificationError(ctx *gin.Context, err error) {
	if errors.Is(err, notify.ErrNotificationNotFound) {
		writeAPIError(ctx, http.StatusNotFound, "NOTIFY_NOT_FOUND", "Notification not found", nil)
		return
	}
	writeAPIError(ctx, http.StatusInternalServerError, "NOTIFY_OPERATION_FAILED", "Notification operation failed", nil)
}
