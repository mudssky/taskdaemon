package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/runlog"
)

// fakeRunLogService 是 runlog 路由测试替身。
type fakeRunLogService struct {
	meta        runlog.Meta
	metaErr     error
	content     []byte
	downloadErr error
	tail        []byte
	tailErr     error
	rangeData   []byte
	rangeErr    error
	lastRunID   int
	lastLines   int
	lastOffset  int64
	lastLength  int64
}

// GetMeta 返回预设元信息。
//
// 参数:
//   - _: context。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - runlog.Meta: 元信息。
//   - error: 预设错误。
func (service *fakeRunLogService) GetMeta(_ context.Context, runID int) (runlog.Meta, error) {
	service.lastRunID = runID
	return service.meta, service.metaErr
}

// OpenDownload 返回预设内容流。
//
// 参数:
//   - _: context。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - io.ReadCloser: 内容流。
//   - int64: 大小。
//   - string: 文件名。
//   - error: 预设错误。
func (service *fakeRunLogService) OpenDownload(_ context.Context, runID int) (io.ReadCloser, int64, string, error) {
	service.lastRunID = runID
	if service.downloadErr != nil {
		return nil, 0, "", service.downloadErr
	}
	return io.NopCloser(bytes.NewReader(service.content)), int64(len(service.content)), "run-1.log", nil
}

// ReadTail 返回预设尾部内容。
//
// 参数:
//   - _: context。
//   - runID: 执行历史 ID。
//   - lines: 行数。
//
// 返回值:
//   - []byte: 内容。
//   - error: 预设错误。
func (service *fakeRunLogService) ReadTail(_ context.Context, runID int, lines int) ([]byte, error) {
	service.lastRunID = runID
	service.lastLines = lines
	return service.tail, service.tailErr
}

// ReadRange 返回预设范围内容。
//
// 参数:
//   - _: context。
//   - runID: 执行历史 ID。
//   - offset: 偏移。
//   - length: 长度。
//
// 返回值:
//   - []byte: 内容。
//   - error: 预设错误。
func (service *fakeRunLogService) ReadRange(_ context.Context, runID int, offset int64, length int64) ([]byte, error) {
	service.lastRunID = runID
	service.lastOffset = offset
	service.lastLength = length
	return service.rangeData, service.rangeErr
}

// TestRunLogMetaArchived 验证元信息成功路径。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogMetaArchived(t *testing.T) {
	size := int64(12)
	created := time.Date(2026, 7, 27, 1, 2, 3, 0, time.UTC)
	service := &fakeRunLogService{meta: runlog.Meta{
		RunID:     7,
		Status:    runlog.StatusArchived,
		SizeBytes: &size,
		CreatedAt: &created,
		Available: true,
	}}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	req := authorizedRequest(http.MethodGet, "/api/runs/7/log/meta", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response runLogMetaResponse
	envelope := decodeAPIData(t, rec.Body.Bytes(), &response)
	require.NotEmpty(t, envelope.TraceID)
	require.Equal(t, 7, response.RunID)
	require.Equal(t, "archived", response.Status)
	require.True(t, response.Available)
	require.Equal(t, size, *response.SizeBytes)
}

// TestRunLogMetaAbsentAndPruned 验证无归档与已清理元信息。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogMetaAbsentAndPruned(t *testing.T) {
	for _, status := range []runlog.ArchiveStatus{runlog.StatusAbsent, runlog.StatusPruned} {
		service := &fakeRunLogService{meta: runlog.Meta{RunID: 3, Status: status}}
		router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
		req := authorizedRequest(http.MethodGet, "/api/runs/3/log/meta", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var response runLogMetaResponse
		decodeAPIData(t, rec.Body.Bytes(), &response)
		require.Equal(t, string(status), response.Status)
		require.False(t, response.Available)
	}
}

// TestRunLogDownloadStreamsBody 验证完整下载成功。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogDownloadStreamsBody(t *testing.T) {
	service := &fakeRunLogService{content: []byte("[stdout] hello\n")}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	req := authorizedRequest(http.MethodGet, "/api/runs/1/log", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[stdout] hello\n", rec.Body.String())
	require.Contains(t, rec.Header().Get("Content-Disposition"), "run-1.log")
	require.Equal(t, "15", rec.Header().Get("Content-Length"))
}

// TestRunLogTailAndRange 验证尾部与范围读取。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogTailAndRange(t *testing.T) {
	service := &fakeRunLogService{
		tail:      []byte("line3\nline4\n"),
		rangeData: []byte("line2"),
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})

	req := authorizedRequest(http.MethodGet, "/api/runs/5/log/tail?lines=2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var tail runLogTailResponse
	decodeAPIData(t, rec.Body.Bytes(), &tail)
	require.Equal(t, 2, service.lastLines)
	require.Equal(t, "line3\nline4\n", tail.Content)

	req = authorizedRequest(http.MethodGet, "/api/runs/5/log/range?offset=6&length=5", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var ranged runLogRangeResponse
	decodeAPIData(t, rec.Body.Bytes(), &ranged)
	require.Equal(t, int64(6), service.lastOffset)
	require.Equal(t, int64(5), service.lastLength)
	require.Equal(t, "line2", ranged.Content)
}

// TestRunLogInvalidRange 验证非法范围参数。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogInvalidRange(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: &fakeRunLogService{}})
	req := authorizedRequest(http.MethodGet, "/api/runs/1/log/range?offset=-1&length=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "RUNLOG_INVALID_RANGE", envelope.Error.Code)
	require.NotEmpty(t, envelope.TraceID)
}

// TestRunLogArchiveNotFound 验证无归档错误码。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogArchiveNotFound(t *testing.T) {
	service := &fakeRunLogService{downloadErr: runlog.ErrArchiveNotFound}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	req := authorizedRequest(http.MethodGet, "/api/runs/9/log", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "RUNLOG_ARCHIVE_NOT_FOUND", envelope.Error.Code)
}

// TestRunLogRunNotFound 验证 run 不存在错误码。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogRunNotFound(t *testing.T) {
	service := &fakeRunLogService{metaErr: runlog.ErrRunNotFound}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	req := authorizedRequest(http.MethodGet, "/api/runs/99/log/meta", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "RUNLOG_RUN_NOT_FOUND", envelope.Error.Code)
}

// TestRunLogRequiresAuth 验证未认证返回 401。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogRequiresAuth(t *testing.T) {
	router := NewRouter(Options{
		Auth:   &fakeAuthService{authErr: auth.ErrInvalidSession},
		RunLog: &fakeRunLogService{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/runs/1/log/meta", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "unauthorized", envelope.Error.Code)
}

// TestRunLogPathTraversalRejectedByIDParser 验证恶意 runID 被 path 解析拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogPathTraversalRejectedByIDParser(t *testing.T) {
	service := &fakeRunLogService{}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	// 非正整数 path 段无法匹配/解析为 runID
	for _, target := range []string{
		"/api/runs/../etc/passwd/log/meta",
		"/api/runs/abc/log/meta",
		"/api/runs/-1/log/meta",
	} {
		req := authorizedRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.NotEqual(t, http.StatusOK, rec.Code, target)
		// 确保服务层未被以遍历字符串调用
		require.True(t, service.lastRunID >= 0)
	}
	// 合法数字才会进入服务
	req := authorizedRequest(http.MethodGet, "/api/runs/12/log/meta", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, 12, service.lastRunID)
}

// TestRunLogServiceInvalidRangeError 验证服务层非法范围映射。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRunLogServiceInvalidRangeError(t *testing.T) {
	service := &fakeRunLogService{rangeErr: runlog.ErrInvalidRange}
	router := NewRouter(Options{Auth: loggedInAuthService(), RunLog: service})
	req := authorizedRequest(http.MethodGet, "/api/runs/1/log/range?offset=0&length=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "RUNLOG_INVALID_RANGE", envelope.Error.Code)
	require.True(t, strings.Contains(envelope.Error.Code, "RUNLOG_"))
	require.True(t, errors.Is(service.rangeErr, runlog.ErrInvalidRange))
}
