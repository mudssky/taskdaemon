package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/notify"
)

// recordingNotificationService 是通知 API 测试替身。
type recordingNotificationService struct {
	listResult notify.ListResult
	listErr    error
	unread     int
	unreadErr  error
	markRecord *ent.Notification
	markErr    error
	batchIDs   []int
	batchN     int
	batchErr   error
	allN       int
	allErr     error
	deleteID   int
	deleteErr  error
	clearN     int
	clearErr   error
}

// List 返回预设列表。
//
// 参数:
//   - _: context。
//   - filter: 列表筛选。
//
// 返回值:
//   - notify.ListResult: 预设结果。
//   - error: 预设错误。
func (service *recordingNotificationService) List(_ context.Context, filter notify.ListFilter) (notify.ListResult, error) {
	service.listResult.Page = filter.Page
	service.listResult.PageSize = filter.PageSize
	return service.listResult, service.listErr
}

// UnreadCount 返回预设未读数。
//
// 参数:
//   - _: context。
//
// 返回值:
//   - int: 未读数。
//   - error: 预设错误。
func (service *recordingNotificationService) UnreadCount(_ context.Context) (int, error) {
	return service.unread, service.unreadErr
}

// MarkRead 返回预设单条已读结果。
//
// 参数:
//   - _: context。
//   - id: 通知 ID。
//
// 返回值:
//   - *ent.Notification: 预设记录。
//   - error: 预设错误。
func (service *recordingNotificationService) MarkRead(_ context.Context, id int) (*ent.Notification, error) {
	if service.markRecord != nil {
		service.markRecord.ID = id
	}
	return service.markRecord, service.markErr
}

// MarkReadBatch 记录批量已读 ID。
//
// 参数:
//   - _: context。
//   - ids: 通知 ID 列表。
//
// 返回值:
//   - int: 影响条数。
//   - error: 预设错误。
func (service *recordingNotificationService) MarkReadBatch(_ context.Context, ids []int) (int, error) {
	service.batchIDs = append([]int(nil), ids...)
	return service.batchN, service.batchErr
}

// MarkAllRead 返回预设全部已读影响条数。
//
// 参数:
//   - _: context。
//
// 返回值:
//   - int: 影响条数。
//   - error: 预设错误。
func (service *recordingNotificationService) MarkAllRead(_ context.Context) (int, error) {
	return service.allN, service.allErr
}

// Delete 记录删除 ID。
//
// 参数:
//   - _: context。
//   - id: 通知 ID。
//
// 返回值:
//   - error: 预设错误。
func (service *recordingNotificationService) Delete(_ context.Context, id int) error {
	service.deleteID = id
	return service.deleteErr
}

// ClearRead 返回清空已读影响条数。
//
// 参数:
//   - _: context。
//
// 返回值:
//   - int: 影响条数。
//   - error: 预设错误。
func (service *recordingNotificationService) ClearRead(_ context.Context) (int, error) {
	return service.clearN, service.clearErr
}

// TestNotificationsRequireAuth 验证未认证返回 401。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsRequireAuth(t *testing.T) {
	router := NewRouter(Options{Notifications: &recordingNotificationService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "unauthorized", envelope.Error.Code)
	require.NotEmpty(t, envelope.TraceID)
}

// TestNotificationsListFiltersAndPagination 验证列表分页与筛选。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsListFiltersAndPagination(t *testing.T) {
	now := time.Now().UTC()
	service := &recordingNotificationService{
		listResult: notify.ListResult{
			Items: []*ent.Notification{{
				ID: 1, EventID: "e1", Name: string(notify.NameTaskRunSucceeded),
				Title: "ok", OccurredAt: now, CreatedAt: now,
			}},
			Total: 1,
		},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Notifications: service})
	req := authorizedRequest(http.MethodGet, "/api/notifications?page=2&pageSize=10&read=false&severity=error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var data notificationListResponse
	envelope := decodeAPIData(t, rec.Body.Bytes(), &data)
	require.NotEmpty(t, envelope.TraceID)
	require.Equal(t, 1, data.Total)
	require.Equal(t, 2, data.Page)
	require.Equal(t, 10, data.PageSize)
	require.Len(t, data.Notifications, 1)
}

// TestNotificationsInvalidPageAndSeverity 验证非法分页与 severity 错误码。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsInvalidPageAndSeverity(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Notifications: &recordingNotificationService{}})

	req := authorizedRequest(http.MethodGet, "/api/notifications?page=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "NOTIFY_INVALID_PAGE", envelope.Error.Code)

	req = authorizedRequest(http.MethodGet, "/api/notifications?severity=fatal", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope = decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "NOTIFY_INVALID_SEVERITY", envelope.Error.Code)
}

// TestNotificationsUnreadCount 验证未读计数。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsUnreadCount(t *testing.T) {
	service := &recordingNotificationService{unread: 7}
	router := NewRouter(Options{Auth: loggedInAuthService(), Notifications: service})
	req := authorizedRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var data unreadCountResponse
	decodeAPIData(t, rec.Body.Bytes(), &data)
	require.Equal(t, 7, data.Count)
}

// TestNotificationsMarkReadDeleteAndClear 验证已读与删除路径。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsMarkReadDeleteAndClear(t *testing.T) {
	now := time.Now().UTC()
	service := &recordingNotificationService{
		markRecord: &ent.Notification{ID: 3, EventID: "e3", Title: "t", OccurredAt: now, CreatedAt: now, ReadAt: &now},
		batchN:     2,
		allN:       5,
		clearN:     4,
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Notifications: service})

	req := authorizedRequest(http.MethodPost, "/api/notifications/3/read", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, _ := json.Marshal(markReadBatchRequest{IDs: []int{1, 2}})
	req = authorizedRequest(http.MethodPost, "/api/notifications/read", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int{1, 2}, service.batchIDs)

	req = authorizedRequest(http.MethodPost, "/api/notifications/read-all", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = authorizedRequest(http.MethodDelete, "/api/notifications/9", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 9, service.deleteID)

	req = authorizedRequest(http.MethodDelete, "/api/notifications/read", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestNotificationsNotFound 验证 NOTIFY_NOT_FOUND。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsNotFound(t *testing.T) {
	service := &recordingNotificationService{markErr: notify.ErrNotificationNotFound}
	router := NewRouter(Options{Auth: loggedInAuthService(), Notifications: service})
	req := authorizedRequest(http.MethodPost, "/api/notifications/99/read", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "NOTIFY_NOT_FOUND", envelope.Error.Code)
}

// TestNotificationsSinkStatuses 验证 sink 状态查询。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationsSinkStatuses(t *testing.T) {
	bus := notify.NewBus(notifyBusTestConfig(), notify.BusOptions{})
	stub := &routeTestSink{name: "store", enabled: true}
	bus.Register(stub)
	router := NewRouter(Options{Auth: loggedInAuthService(), NotifyBus: bus})
	req := authorizedRequest(http.MethodGet, "/api/notifications/sinks", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var data struct {
		Sinks []sinkStatusResponse `json:"sinks"`
	}
	decodeAPIData(t, rec.Body.Bytes(), &data)
	require.Len(t, data.Sinks, 1)
	require.Equal(t, "store", data.Sinks[0].Name)
}

type routeTestSink struct {
	name    string
	enabled bool
}

func (sink *routeTestSink) Name() string { return sink.name }

func (sink *routeTestSink) Enabled() bool { return sink.enabled }

func (sink *routeTestSink) Deliver(context.Context, notify.Event) error { return nil }

func notifyBusTestConfig() config.NotifyConfig {
	return config.Default().Notify
}
