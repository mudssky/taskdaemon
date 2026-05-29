package audio

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	audiorecord "taskdaemon/internal/data/ent/audiorecord"
)

// TestSubmitUploadArchivesAndPersistsRecord 验证上传音频会归档本地副本并保存历史记录。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitUploadArchivesAndPersistsRecord(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	service := New(store, testAudioConfig("secret"), Options{StorageRoot: t.TempDir()})

	record, err := service.SubmitUpload(ctx, "secret", UploadRequest{
		Reader:   bytes.NewBufferString("audio-content"),
		Source:   "hermes",
		Filename: "hello.mp3",
		MIMEType: "audio/mpeg",
		Size:     int64(len("audio-content")),
	})

	require.NoError(t, err)
	require.Equal(t, audiorecord.SourceKindUpload, record.SourceKind)
	require.Equal(t, "hermes", record.Source)
	require.Equal(t, "audio/mpeg", record.MimeType)
	require.Equal(t, int64(len("audio-content")), record.SizeBytes)
	require.NotEmpty(t, record.Sha256)
	require.FileExists(t, service.absolutePath(record.StoredPath))
}

// TestSubmitURLRejectsPrivateAddressByDefault 验证默认 URL 策略会拒绝本机和私有网络地址。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitURLRejectsPrivateAddressByDefault(t *testing.T) {
	store := testStore(t)
	service := New(store, testAudioConfig("secret"), Options{StorageRoot: t.TempDir()})

	record, err := service.SubmitURL(context.Background(), "secret", URLRequest{URL: "https://127.0.0.1/audio.mp3"})

	require.ErrorIs(t, err, ErrURLRejected)
	require.Nil(t, record)
}

// TestSubmitURLAllowsConfiguredPrivateHTTP 验证配置放开后可从内网 HTTP 地址下载音频。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitURLAllowsConfiguredPrivateHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "audio/wav")
		_, _ = writer.Write([]byte("wav-content"))
	}))
	defer server.Close()
	cfg := testAudioConfig("secret")
	cfg.Inbound.URL.AllowedSchemes = []string{"http", "https"}
	cfg.Inbound.URL.AllowPrivateNetworks = true
	store := testStore(t)
	service := New(store, cfg, Options{StorageRoot: t.TempDir(), HTTPClient: server.Client()})

	record, err := service.SubmitURL(context.Background(), "secret", URLRequest{URL: server.URL + "/audio.wav", Source: "script"})

	require.NoError(t, err)
	require.Equal(t, audiorecord.SourceKindURL, record.SourceKind)
	require.Equal(t, "script", record.Source)
	require.Equal(t, "audio/wav", record.MimeType)
	require.FileExists(t, service.absolutePath(record.StoredPath))
}

// TestSubmitURLRejectsTooManyRedirects 验证 URL 下载会遵守配置的重定向上限。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitURLRejectsTooManyRedirects(t *testing.T) {
	redirects := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirects++
		http.Redirect(writer, request, "/next", http.StatusFound)
	}))
	defer server.Close()
	cfg := testAudioConfig("secret")
	cfg.Inbound.URL.AllowedSchemes = []string{"http"}
	cfg.Inbound.URL.AllowPrivateNetworks = true
	cfg.Inbound.URL.MaxRedirects = 1
	service := New(testStore(t), cfg, Options{StorageRoot: t.TempDir(), HTTPClient: server.Client()})

	record, err := service.SubmitURL(context.Background(), "secret", URLRequest{URL: server.URL + "/start"})

	require.ErrorIs(t, err, ErrURLRejected)
	require.Nil(t, record)
	require.GreaterOrEqual(t, redirects, 2)
}

// TestSubmitUploadRejectsNonAudioMIME 验证上传入口会拒绝非音频 MIME。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitUploadRejectsNonAudioMIME(t *testing.T) {
	service := New(testStore(t), testAudioConfig("secret"), Options{StorageRoot: t.TempDir()})

	record, err := service.SubmitUpload(context.Background(), "secret", UploadRequest{
		Reader:   bytes.NewBufferString("plain-text"),
		Filename: "note.txt",
		MIMEType: "text/plain",
		Size:     int64(len("plain-text")),
	})

	require.ErrorIs(t, err, ErrMIMERejected)
	require.Nil(t, record)
}

// TestQueuePlaysRecordsSequentially 验证播放队列按入队顺序更新状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestQueuePlaysRecordsSequentially(t *testing.T) {
	store := testStore(t)
	root := t.TempDir()
	cfg := testAudioConfig("secret")
	player := &recordingPlayer{done: make(chan struct{}, 2)}
	queue := NewQueue(store, cfg, QueueOptions{Player: player, StorageRoot: root})
	defer queue.Shutdown()
	recordA := createTestAudioRecord(t, store, root, "a.wav")
	recordB := createTestAudioRecord(t, store, root, "b.wav")

	require.NoError(t, queue.Enqueue(context.Background(), recordA.ID))
	require.NoError(t, queue.Enqueue(context.Background(), recordB.ID))
	player.wait(t, 2)

	updatedA := store.Client().AudioRecord.GetX(context.Background(), recordA.ID)
	updatedB := store.Client().AudioRecord.GetX(context.Background(), recordB.ID)
	require.Equal(t, audiorecord.StatusPlayed, updatedA.Status)
	require.Equal(t, audiorecord.StatusPlayed, updatedB.Status)
	require.Equal(t, []string{filepath.Join(root, "a.wav"), filepath.Join(root, "b.wav")}, player.paths)
}

// TestQueueAllowsUnlimitedPendingWhenLimitZero 验证队列上限为 0 时不限制等待数量。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestQueueAllowsUnlimitedPendingWhenLimitZero(t *testing.T) {
	store := testStore(t)
	root := t.TempDir()
	cfg := testAudioConfig("secret")
	cfg.Playback.QueueLimit = 0
	player := &blockingPlayer{release: make(chan struct{}), done: make(chan struct{}, 3)}
	queue := NewQueue(store, cfg, QueueOptions{Player: player, StorageRoot: root})
	defer queue.Shutdown()
	for i := 0; i < 3; i++ {
		record := createTestAudioRecord(t, store, root, fmt.Sprintf("%d.wav", i))
		require.NoError(t, queue.Enqueue(context.Background(), record.ID))
	}

	close(player.release)
	player.wait(t, 3)
}

// TestSubmitUploadMarksQueuedSkippedWhenQueueFull 验证自动播放队列满时仍保存记录并标记 queued_skipped。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSubmitUploadMarksQueuedSkippedWhenQueueFull(t *testing.T) {
	cfg := testAudioConfig("secret")
	cfg.Autoplay.Enabled = true
	cfg.Autoplay.Target = "backend"
	store := testStore(t)
	service := New(store, cfg, Options{StorageRoot: t.TempDir(), Queue: fullQueue{}})

	record, err := service.SubmitUpload(context.Background(), "secret", UploadRequest{
		Reader:   bytes.NewBufferString("audio-content"),
		Filename: "hello.mp3",
		MIMEType: "audio/mpeg",
		Size:     int64(len("audio-content")),
	})

	require.NoError(t, err)
	updated := store.Client().AudioRecord.GetX(context.Background(), record.ID)
	require.Equal(t, audiorecord.StatusQueuedSkipped, updated.Status)
}

// TestAuthenticateInboundTokenRejectsWrongToken 验证入站 Token 使用哈希比对。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAuthenticateInboundTokenRejectsWrongToken(t *testing.T) {
	service := New(nil, testAudioConfig("secret"), Options{})

	require.NoError(t, service.AuthenticateInboundToken("secret"))
	require.ErrorIs(t, service.AuthenticateInboundToken("wrong"), ErrUnauthorized)
}

// testStore 创建带迁移的临时 SQLite 数据层。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - *data.Store: 测试数据层。
func testStore(t *testing.T) *data.Store {
	t.Helper()
	store, err := data.Open(context.Background(), config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.Join(t.TempDir(), "taskdaemon.db") + "?_fk=1",
	})
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})
	return store
}

// testAudioConfig 返回测试用音频配置。
//
// 参数:
//   - token: 入站 Bearer Token 原文。
//
// 返回值:
//   - config.AudioConfig: 测试配置。
func testAudioConfig(token string) config.AudioConfig {
	cfg := config.Default().Audio
	cfg.Inbound.TokenHash = HashToken(token)
	return cfg
}

// createTestAudioRecord 创建测试用音频记录和本地副本。
//
// 参数:
//   - t: Go 测试上下文。
//   - store: 测试数据层。
//   - root: 音频副本目录。
//   - name: 音频文件名。
//
// 返回值:
//   - *ent.AudioRecord: 测试音频记录。
func createTestAudioRecord(t *testing.T, store *data.Store, root string, name string) *ent.AudioRecord {
	t.Helper()
	path := filepath.Join(root, name)
	require.NoError(t, os.WriteFile(path, []byte("audio"), 0o600))
	return store.Client().AudioRecord.Create().
		SetSourceKind(audiorecord.SourceKindUpload).
		SetOriginalFilename(name).
		SetStoredPath(filepath.ToSlash(filepath.Join("audio", "inbound", name))).
		SetMimeType("audio/wav").
		SetSizeBytes(5).
		SetSha256(fmt.Sprintf("%064d", 1)).
		SaveX(context.Background())
}

type recordingPlayer struct {
	mu    sync.Mutex
	paths []string
	done  chan struct{}
}

// Play 记录播放路径并通知测试。
//
// 参数:
//   - ctx: 请求上下文。
//   - path: 本地音频文件绝对路径。
//   - cfg: 音频配置快照。
//
// 返回值:
//   - error: 当前 fake 不返回错误。
func (player *recordingPlayer) Play(_ context.Context, path string, _ config.AudioConfig) error {
	player.mu.Lock()
	player.paths = append(player.paths, path)
	player.mu.Unlock()
	player.done <- struct{}{}
	return nil
}

// wait 等待指定次数播放完成。
//
// 参数:
//   - t: Go 测试上下文。
//   - count: 等待次数。
//
// 返回值:
//   - 无。超时则终止测试。
func (player *recordingPlayer) wait(t *testing.T, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-player.done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audio playback")
		}
	}
}

type fullQueue struct{}

// Enqueue 固定返回队列满错误。
//
// 参数:
//   - ctx: 请求上下文。
//   - recordID: 音频历史记录 ID。
//
// 返回值:
//   - error: 始终为 ErrQueueFull。
func (fullQueue) Enqueue(context.Context, int) error {
	return ErrQueueFull
}

type blockingPlayer struct {
	release chan struct{}
	done    chan struct{}
}

// Play 等待 release 后标记播放完成。
//
// 参数:
//   - ctx: 请求上下文。
//   - path: 本地音频文件绝对路径。
//   - cfg: 音频配置快照。
//
// 返回值:
//   - error: context 取消时返回 context error。
func (player *blockingPlayer) Play(ctx context.Context, _ string, _ config.AudioConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-player.release:
		player.done <- struct{}{}
		return nil
	}
}

// wait 等待指定次数播放完成。
//
// 参数:
//   - t: Go 测试上下文。
//   - count: 等待次数。
//
// 返回值:
//   - 无。超时则终止测试。
func (player *blockingPlayer) wait(t *testing.T, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-player.done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audio playback")
		}
	}
}
