package httpapi

import (
	"time"

	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/notify"
)

// notificationResponse 是站内通知 API 响应 DTO。
type notificationResponse struct {
	ID          int            `json:"id"`
	EventID     string         `json:"eventId"`
	Name        string         `json:"name"`
	Severity    string         `json:"severity"`
	SubjectKind string         `json:"subjectKind,omitempty"`
	SubjectID   string         `json:"subjectId,omitempty"`
	Title       string         `json:"title"`
	Body        string         `json:"body,omitempty"`
	Detail      map[string]any `json:"detail,omitempty"`
	ReadAt      *time.Time     `json:"readAt,omitempty"`
	OccurredAt  time.Time      `json:"occurredAt"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// notificationListResponse 是分页列表响应。
type notificationListResponse struct {
	Notifications []notificationResponse `json:"notifications"`
	Total         int                    `json:"total"`
	Page          int                    `json:"page"`
	PageSize      int                    `json:"pageSize"`
}

// unreadCountResponse 是未读计数响应。
type unreadCountResponse struct {
	Count int `json:"count"`
}

// markReadBatchRequest 是批量已读请求。
type markReadBatchRequest struct {
	IDs []int `json:"ids"`
}

// affectedCountResponse 是影响条数响应。
type affectedCountResponse struct {
	Affected int `json:"affected"`
}

// sinkStatusResponse 是 sink 状态响应 DTO。
type sinkStatusResponse struct {
	Name            string     `json:"name"`
	Enabled         bool       `json:"enabled"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt   *time.Time `json:"lastFailureAt,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	SuccessCount    int64      `json:"successCount"`
	FailureCount    int64      `json:"failureCount"`
	LastDeliveredAt *time.Time `json:"lastDeliveredAt,omitempty"`
}

// notificationResponseFromEnt 将 Ent 通知转换为 API DTO。
//
// 参数:
//   - record: Ent 通知记录。
//
// 返回值:
//   - notificationResponse: API 响应 DTO。
func notificationResponseFromEnt(record *ent.Notification) notificationResponse {
	if record == nil {
		return notificationResponse{}
	}
	return notificationResponse{
		ID:          record.ID,
		EventID:     record.EventID,
		Name:        record.Name,
		Severity:    string(record.Severity),
		SubjectKind: record.SubjectKind,
		SubjectID:   record.SubjectID,
		Title:       record.Title,
		Body:        record.Body,
		Detail:      record.Detail,
		ReadAt:      record.ReadAt,
		OccurredAt:  record.OccurredAt,
		CreatedAt:   record.CreatedAt,
	}
}

// sinkStatusResponseFromNotify 将总线状态转换为 API DTO。
//
// 参数:
//   - status: notify.SinkStatus。
//
// 返回值:
//   - sinkStatusResponse: API 响应 DTO。
func sinkStatusResponseFromNotify(status notify.SinkStatus) sinkStatusResponse {
	return sinkStatusResponse{
		Name:            status.Name,
		Enabled:         status.Enabled,
		LastSuccessAt:   status.LastSuccessAt,
		LastFailureAt:   status.LastFailureAt,
		LastError:       status.LastError,
		SuccessCount:    status.SuccessCount,
		FailureCount:    status.FailureCount,
		LastDeliveredAt: status.LastDeliveredAt,
	}
}
