package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/audio"
	"taskdaemon/internal/auth"
	"taskdaemon/internal/data/ent"
	audiorecord "taskdaemon/internal/data/ent/audiorecord"
)

// TestSubmitAudioURLUsesBearerToken 验证 URL 播放请求使用外部 Bearer Token 而非管理员 session。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitAudioURLUsesBearerToken(t *testing.T) {
	service := &recordingAudioService{record: &ent.AudioRecord{ID: 3, SourceKind: audiorecord.SourceKindURL, Status: audiorecord.StatusReceived}}
	router := NewRouter(Options{Audio: service})
	body := bytes.NewBufferString(`{"url":"https://example.com/a.mp3","source":"hermes"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/inbound/audio-play-requests", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "secret", service.urlToken)
	require.Equal(t, "https://example.com/a.mp3", service.urlInput.URL)
	require.Equal(t, "hermes", service.urlInput.Source)
	var response audioRecordResponse
	decodeAPIData(t, rec.Body.Bytes(), &response)
	require.Equal(t, 3, response.ID)
}

// TestSubmitAudioUploadAcceptsMultipartFile 验证上传播放请求绑定 multipart 文件。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitAudioUploadAcceptsMultipartFile(t *testing.T) {
	service := &recordingAudioService{record: &ent.AudioRecord{ID: 4, SourceKind: audiorecord.SourceKindUpload, Status: audiorecord.StatusReceived}}
	router := NewRouter(Options{Audio: service})
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("source", "script"))
	part, err := writer.CreateFormFile("file", "hello.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("audio-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/inbound/audio-play-requests/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "secret", service.uploadToken)
	require.Equal(t, "script", service.uploadInput.Source)
	require.Equal(t, "hello.mp3", service.uploadInput.Filename)
}

// TestAudioHistoryRequiresAdminSession 验证历史查询走管理员 session 认证。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAudioHistoryRequiresAdminSession(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{authErr: auth.ErrInvalidSession}, Audio: &recordingAudioService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/audio/history", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "unauthorized", envelope.Error.Code)
}

// TestAudioReplayCallsService 验证管理员可对历史记录发起重放。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAudioReplayCallsService(t *testing.T) {
	service := &recordingAudioService{record: &ent.AudioRecord{ID: 8, SourceKind: audiorecord.SourceKindUpload, Status: audiorecord.StatusQueued}}
	router := NewRouter(Options{Auth: loggedInAuthService(), Audio: service})
	req := authorizedRequest(http.MethodPost, "/api/audio/history/8/replay", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 8, service.replayID)
}

// recordingAudioService 是音频 API 测试使用的服务替身。
type recordingAudioService struct {
	urlToken    string
	urlInput    audio.URLRequest
	uploadToken string
	uploadInput audio.UploadRequest
	replayID    int
	record      *ent.AudioRecord
	records     []*ent.AudioRecord
	err         error
}

// SubmitURL 记录 URL 播放请求输入。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - token: Bearer Token。
//   - input: URL 播放请求输入。
//
// 返回值:
//   - *ent.AudioRecord: 预设音频记录。
//   - error: 预设错误。
func (service *recordingAudioService) SubmitURL(_ context.Context, token string, input audio.URLRequest) (*ent.AudioRecord, error) {
	service.urlToken = token
	service.urlInput = input
	return service.record, service.err
}

// SubmitUpload 记录上传播放请求输入。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - token: Bearer Token。
//   - input: 上传播放请求输入。
//
// 返回值:
//   - *ent.AudioRecord: 预设音频记录。
//   - error: 预设错误。
func (service *recordingAudioService) SubmitUpload(_ context.Context, token string, input audio.UploadRequest) (*ent.AudioRecord, error) {
	service.uploadToken = token
	service.uploadInput = input
	return service.record, service.err
}

// ListHistory 返回预设历史记录。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 查询限制，测试替身不使用。
//
// 返回值:
//   - []*ent.AudioRecord: 预设记录。
//   - error: 预设错误。
func (service *recordingAudioService) ListHistory(_ context.Context, _ int) ([]*ent.AudioRecord, error) {
	return service.records, service.err
}

// Replay 记录重放历史 ID。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - recordID: 历史记录 ID。
//
// 返回值:
//   - *ent.AudioRecord: 预设记录。
//   - error: 预设错误。
func (service *recordingAudioService) Replay(_ context.Context, recordID int) (*ent.AudioRecord, error) {
	service.replayID = recordID
	return service.record, service.err
}
