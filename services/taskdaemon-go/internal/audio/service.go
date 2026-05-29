package audio

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	audiorecord "taskdaemon/internal/data/ent/audiorecord"
)

const StatusQueuedSkipped = string(audiorecord.StatusQueuedSkipped)

// PlaybackQueue 定义音频服务依赖的播放队列能力。
type PlaybackQueue interface {
	Enqueue(context.Context, int) error
}

// Service 处理外部音频播放请求、历史记录和本地文件归档。
type Service struct {
	store       *data.Store
	mu          sync.RWMutex
	cfg         config.AudioConfig
	storageRoot string
	httpClient  *http.Client
	queue       PlaybackQueue
}

// Options 定义音频服务可选依赖。
type Options struct {
	StorageRoot string
	HTTPClient  *http.Client
	Queue       PlaybackQueue
}

// URLRequest 描述 URL 来源的音频播放请求。
type URLRequest struct {
	URL      string
	Source   string
	Filename string
	MIMEType string
}

// UploadRequest 描述上传来源的音频播放请求。
type UploadRequest struct {
	Reader   io.Reader
	Source   string
	Filename string
	MIMEType string
	Size     int64
}

// New 创建音频服务。
//
// 参数:
//   - store: 数据层实例。
//   - cfg: 音频相关配置。
//   - opts: 可选依赖和路径覆盖。
//
// 返回值:
//   - *Service: 音频服务实例。
func New(store *data.Store, cfg config.AudioConfig, opts Options) *Service {
	client := opts.HTTPClient
	if client == nil {
		client = httpClient(cfg, nil)
	}
	service := &Service{
		store:       store,
		cfg:         cfg,
		storageRoot: opts.StorageRoot,
		httpClient:  client,
		queue:       opts.Queue,
	}
	if service.httpClient.CheckRedirect == nil {
		service.httpClient.CheckRedirect = redirectPolicy(service)
	}
	return service
}

// UpdateConfig 更新音频服务运行时配置。
//
// 参数:
//   - cfg: 新的音频配置快照。
//
// 返回值:
//   - 无。
func (service *Service) UpdateConfig(cfg config.AudioConfig) {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.cfg = cfg
	if service.httpClient != nil {
		service.httpClient = httpClient(cfg, service)
	}
	service.mu.Unlock()
}

// config 返回当前音频配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.AudioConfig: 当前配置。
func (service *Service) config() config.AudioConfig {
	if service == nil {
		return config.Default().Audio
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.cfg
}

// client 返回当前 URL 下载 HTTP client。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *http.Client: 当前下载 client。
func (service *Service) client() *http.Client {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.httpClient
}

// SubmitURL 接收 URL 来源音频，下载到本地后持久化历史记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - bearerToken: Authorization Bearer Token 原文。
//   - input: URL 来源请求。
//
// 返回值:
//   - *ent.AudioRecord: 持久化后的音频记录。
//   - error: 认证、下载、归档或持久化失败时返回错误。
func (service *Service) SubmitURL(ctx context.Context, bearerToken string, input URLRequest) (*ent.AudioRecord, error) {
	if err := service.AuthenticateInboundToken(bearerToken); err != nil {
		return nil, err
	}
	parsedURL, err := service.validateURL(input.URL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create audio download request: %w", err)
	}
	resp, err := service.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: status %d", ErrURLRejected, resp.StatusCode)
	}
	mimeType := strings.TrimSpace(input.MIMEType)
	if mimeType == "" {
		mimeType = resp.Header.Get("Content-Type")
	}
	if err := validateMIMEType(mimeType); err != nil {
		return nil, err
	}
	filename := input.Filename
	if filename == "" {
		filename = filepath.Base(parsedURL.Path)
	}
	archive, err := service.archiveReader(ctx, resp.Body, filename, service.maxBytes(), false)
	if err != nil {
		return nil, err
	}
	record, err := service.createRecord(ctx, createRecordInput{
		SourceKind:       audiorecord.SourceKindURL,
		Source:           input.Source,
		SourceURL:        parsedURL.String(),
		OriginalFilename: filename,
		StoredPath:       archive.RelativePath,
		MIMEType:         mimeType,
		SizeBytes:        archive.SizeBytes,
		SHA256:           archive.SHA256,
	})
	if err != nil {
		_ = os.Remove(archive.AbsolutePath)
		return nil, err
	}
	return record, service.enqueueIfNeeded(ctx, record)
}

// SubmitUpload 接收上传音频，归档到本地后持久化历史记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - bearerToken: Authorization Bearer Token 原文。
//   - input: 上传来源请求。
//
// 返回值:
//   - *ent.AudioRecord: 持久化后的音频记录。
//   - error: 认证、归档或持久化失败时返回错误。
func (service *Service) SubmitUpload(ctx context.Context, bearerToken string, input UploadRequest) (*ent.AudioRecord, error) {
	if err := service.AuthenticateInboundToken(bearerToken); err != nil {
		return nil, err
	}
	if input.Reader == nil {
		return nil, fmt.Errorf("%w: file", ErrInvalidInput)
	}
	if err := validateMIMEType(input.MIMEType); err != nil {
		return nil, err
	}
	archive, err := service.archiveReader(ctx, input.Reader, input.Filename, service.maxBytes(), input.Size > service.maxBytes())
	if err != nil {
		return nil, err
	}
	record, err := service.createRecord(ctx, createRecordInput{
		SourceKind:       audiorecord.SourceKindUpload,
		Source:           input.Source,
		OriginalFilename: input.Filename,
		StoredPath:       archive.RelativePath,
		MIMEType:         input.MIMEType,
		SizeBytes:        archive.SizeBytes,
		SHA256:           archive.SHA256,
	})
	if err != nil {
		_ = os.Remove(archive.AbsolutePath)
		return nil, err
	}
	return record, service.enqueueIfNeeded(ctx, record)
}

// ListHistory 返回最近音频播放请求记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - limit: 查询条数；小于等于 0 时使用配置上限或默认值。
//
// 返回值:
//   - []*ent.AudioRecord: 按创建时间倒序排列的记录。
//   - error: 查询失败时返回错误。
func (service *Service) ListHistory(ctx context.Context, limit int) ([]*ent.AudioRecord, error) {
	if service == nil || service.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = service.config().History.Limit
		if limit <= 0 {
			limit = 100
		}
	}
	return service.store.Client().AudioRecord.Query().
		Order(audiorecord.ByCreatedAt(sql.OrderDesc())).
		Limit(limit).
		All(ctx)
}

// Replay 将指定历史记录重新加入播放队列。
//
// 参数:
//   - ctx: 请求上下文。
//   - recordID: 音频历史记录 ID。
//
// 返回值:
//   - *ent.AudioRecord: 重新排队后的记录。
//   - error: 记录不存在或队列已满时返回错误。
func (service *Service) Replay(ctx context.Context, recordID int) (*ent.AudioRecord, error) {
	record, err := service.store.Client().AudioRecord.Get(ctx, recordID)
	if ent.IsNotFound(err) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get audio record: %w", err)
	}
	if service.queue == nil {
		return record, nil
	}
	if err := service.queue.Enqueue(ctx, record.ID); err != nil {
		if errors.Is(err, ErrQueueFull) {
			return nil, err
		}
		return nil, fmt.Errorf("enqueue audio replay: %w", err)
	}
	return record.Update().SetStatus(audiorecord.StatusQueued).Save(ctx)
}

// AuthenticateInboundToken 校验外部入站 Bearer Token。
//
// 参数:
//   - token: 请求中提取出的 Bearer Token。
//
// 返回值:
//   - error: Token 缺失或不匹配时返回 ErrUnauthorized。
func (service *Service) AuthenticateInboundToken(token string) error {
	token = strings.TrimSpace(token)
	cfg := service.config()
	if service == nil || cfg.Inbound.TokenHash == "" || token == "" {
		return ErrUnauthorized
	}
	sum := sha256.Sum256([]byte(token))
	got := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(cfg.Inbound.TokenHash)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

// HashToken 返回入站 Bearer Token 的 SHA-256 十六进制摘要。
//
// 参数:
//   - token: Bearer Token 原文。
//
// 返回值:
//   - string: SHA-256 十六进制摘要。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type createRecordInput struct {
	SourceKind       audiorecord.SourceKind
	Source           string
	SourceURL        string
	OriginalFilename string
	StoredPath       string
	MIMEType         string
	SizeBytes        int64
	SHA256           string
}

// createRecord 持久化音频历史记录并执行保留上限清理。
//
// 参数:
//   - ctx: 请求上下文。
//   - input: 已归档音频元数据。
//
// 返回值:
//   - *ent.AudioRecord: 新建记录。
//   - error: 数据库写入失败时返回错误。
func (service *Service) createRecord(ctx context.Context, input createRecordInput) (*ent.AudioRecord, error) {
	record, err := service.store.Client().AudioRecord.Create().
		SetSourceKind(input.SourceKind).
		SetSource(input.Source).
		SetSourceURL(input.SourceURL).
		SetOriginalFilename(input.OriginalFilename).
		SetStoredPath(input.StoredPath).
		SetMimeType(input.MIMEType).
		SetSizeBytes(input.SizeBytes).
		SetSha256(input.SHA256).
		SetStatus(audiorecord.StatusReceived).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create audio record: %w", err)
	}
	if err := service.pruneHistory(ctx); err != nil {
		return nil, err
	}
	return record, nil
}

// enqueueIfNeeded 根据自动播放配置将记录加入播放队列。
//
// 参数:
//   - ctx: 请求上下文。
//   - record: 已保存的音频记录。
//
// 返回值:
//   - error: 队列入队失败时返回错误。
func (service *Service) enqueueIfNeeded(ctx context.Context, record *ent.AudioRecord) error {
	cfg := service.config()
	if !cfg.Autoplay.Enabled || cfg.Autoplay.Target != "backend" || service.queue == nil {
		return nil
	}
	if err := service.queue.Enqueue(ctx, record.ID); err != nil {
		if errors.Is(err, ErrQueueFull) {
			_, updateErr := record.Update().SetStatus(audiorecord.StatusQueuedSkipped).Save(ctx)
			if updateErr != nil {
				return fmt.Errorf("mark audio queue skipped: %w", updateErr)
			}
			return nil
		}
		return fmt.Errorf("enqueue audio: %w", err)
	}
	_, err := record.Update().SetStatus(audiorecord.StatusQueued).Save(ctx)
	return err
}

// pruneHistory 清理超过保留上限的最旧记录及其本地副本。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - error: 查询或删除失败时返回错误。
func (service *Service) pruneHistory(ctx context.Context) error {
	limit := service.config().History.Limit
	if limit <= 0 {
		return nil
	}
	count, err := service.store.Client().AudioRecord.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("count audio records: %w", err)
	}
	if count <= limit {
		return nil
	}
	records, err := service.store.Client().AudioRecord.Query().
		Order(audiorecord.ByCreatedAt(sql.OrderAsc())).
		Limit(count - limit).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query old audio records: %w", err)
	}
	for _, record := range records {
		if err := service.store.Client().AudioRecord.DeleteOneID(record.ID).Exec(ctx); err != nil {
			return fmt.Errorf("delete old audio record: %w", err)
		}
		_ = os.Remove(service.absolutePath(record.StoredPath))
	}
	return nil
}

// validateURL 校验 URL 协议、Host 和私有网络策略。
//
// 参数:
//   - rawURL: 请求传入的 URL 字符串。
//
// 返回值:
//   - *url.URL: 解析后的 URL。
//   - error: URL 不符合策略时返回 ErrURLRejected。
func (service *Service) validateURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("%w: url", ErrInvalidInput)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: url", ErrInvalidInput)
	}
	cfg := service.config()
	if !containsFold(cfg.Inbound.URL.AllowedSchemes, parsed.Scheme) {
		return nil, fmt.Errorf("%w: scheme", ErrURLRejected)
	}
	host := strings.ToLower(parsed.Hostname())
	if len(cfg.Inbound.URL.AllowedHosts) > 0 && !containsFold(cfg.Inbound.URL.AllowedHosts, host) {
		return nil, fmt.Errorf("%w: host", ErrURLRejected)
	}
	if !cfg.Inbound.URL.AllowPrivateNetworks && isPrivateHost(host) {
		return nil, fmt.Errorf("%w: private network", ErrURLRejected)
	}
	if !cfg.Inbound.URL.AllowPrivateNetworks && resolvesToPrivateHost(host) {
		return nil, fmt.Errorf("%w: private network", ErrURLRejected)
	}
	return parsed, nil
}

type archiveResult struct {
	RelativePath string
	AbsolutePath string
	SizeBytes    int64
	SHA256       string
}

// archiveReader 将音频流保存到受控目录并计算摘要。
//
// 参数:
//   - ctx: 请求上下文。
//   - reader: 音频内容 reader。
//   - filename: 原始文件名。
//   - maxBytes: 最大允许字节数。
//   - alreadyTooLarge: 调用方已知大小超限时为 true。
//
// 返回值:
//   - archiveResult: 归档路径、大小和摘要。
//   - error: 写入失败或大小超限时返回错误。
func (service *Service) archiveReader(ctx context.Context, reader io.Reader, filename string, maxBytes int64, alreadyTooLarge bool) (archiveResult, error) {
	if alreadyTooLarge {
		return archiveResult{}, ErrFileTooLarge
	}
	if err := ctx.Err(); err != nil {
		return archiveResult{}, err
	}
	root, err := service.resolveStorageRoot()
	if err != nil {
		return archiveResult{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return archiveResult{}, fmt.Errorf("create audio storage dir: %w", err)
	}
	tmp, err := os.CreateTemp(root, "audio-*")
	if err != nil {
		return archiveResult{}, fmt.Errorf("create audio temp file: %w", err)
	}
	hasher := sha256.New()
	limited := &io.LimitedReader{R: reader, N: maxBytes + 1}
	written, err := io.Copy(io.MultiWriter(tmp, hasher), limited)
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return archiveResult{}, fmt.Errorf("archive audio file: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmp.Name())
		return archiveResult{}, fmt.Errorf("close audio temp file: %w", closeErr)
	}
	if written > maxBytes {
		_ = os.Remove(tmp.Name())
		return archiveResult{}, ErrFileTooLarge
	}
	ext := filepath.Ext(filename)
	finalName := strings.TrimSuffix(filepath.Base(tmp.Name()), filepath.Ext(tmp.Name())) + ext
	if ext == "" {
		finalName = filepath.Base(tmp.Name())
	}
	finalPath := filepath.Join(root, finalName)
	if err := os.Rename(tmp.Name(), finalPath); err != nil {
		_ = os.Remove(tmp.Name())
		return archiveResult{}, fmt.Errorf("finalize audio file: %w", err)
	}
	return archiveResult{
		RelativePath: filepath.ToSlash(filepath.Join("audio", "inbound", finalName)),
		AbsolutePath: finalPath,
		SizeBytes:    written,
		SHA256:       hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

// resolveStorageRoot 返回音频副本存储目录。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 音频副本目录。
//   - error: 用户配置目录不可用时返回错误。
func (service *Service) resolveStorageRoot() (string, error) {
	return resolveStorageRoot(service.storageRoot)
}

// absolutePath 将数据库中的相对路径转换为当前本地副本绝对路径。
//
// 参数:
//   - relativePath: 数据库保存的相对路径。
//
// 返回值:
//   - string: 绝对路径，无法解析时返回空字符串。
func (service *Service) absolutePath(relativePath string) string {
	root, err := service.resolveStorageRoot()
	if err != nil {
		return ""
	}
	return absolutePathFor(root, relativePath)
}

// httpClient 创建带下载超时和重定向策略的 HTTP client。
//
// 参数:
//   - cfg: 音频配置快照。
//
// 返回值:
//   - *http.Client: URL 下载 client。
func httpClient(cfg config.AudioConfig, service *Service) *http.Client {
	client := &http.Client{Timeout: downloadTimeout(cfg)}
	if service != nil {
		client.CheckRedirect = redirectPolicy(service)
	}
	return client
}

// downloadTimeout 返回 URL 下载超时。
//
// 参数:
//   - cfg: 音频配置快照。
//
// 返回值:
//   - time.Duration: 下载超时。
func downloadTimeout(cfg config.AudioConfig) time.Duration {
	timeout := time.Duration(cfg.Inbound.URL.DownloadTimeoutSeconds) * time.Second
	if timeout <= 0 {
		return 60 * time.Second
	}
	return timeout
}

// redirectPolicy 返回 URL 下载重定向校验函数。
//
// 参数:
//   - service: 音频服务实例。
//
// 返回值:
//   - func(*http.Request, []*http.Request) error: HTTP client 使用的重定向策略。
func redirectPolicy(service *Service) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		cfg := service.config()
		maxRedirects := cfg.Inbound.URL.MaxRedirects
		if maxRedirects < 0 {
			maxRedirects = 0
		}
		if len(via) > maxRedirects {
			return fmt.Errorf("%w: redirects", ErrURLRejected)
		}
		_, err := service.validateURL(req.URL.String())
		return err
	}
}

// maxBytes 返回当前音频文件大小上限。
//
// 参数:
//   - 无。
//
// 返回值:
//   - int64: 最大字节数。
func (service *Service) maxBytes() int64 {
	cfg := service.config()
	if cfg.Inbound.MaxBytes <= 0 {
		return 209715200
	}
	return cfg.Inbound.MaxBytes
}

// containsFold 判断字符串切片中是否包含指定值。
//
// 参数:
//   - values: 候选值。
//   - target: 目标值。
//
// 返回值:
//   - bool: 包含时返回 true。
func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

// isPrivateHost 判断 Host 是否属于本机或私有网络。
//
// 参数:
//   - host: URL hostname。
//
// 返回值:
//   - bool: 属于本机、回环、链路本地或私有地址时返回 true。
func isPrivateHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// resolvesToPrivateHost 判断域名解析结果是否包含私有地址。
//
// 参数:
//   - host: URL hostname。
//
// 返回值:
//   - bool: 解析到本机、回环、链路本地或私有地址时返回 true。
func resolvesToPrivateHost(host string) bool {
	if net.ParseIP(host) != nil {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}
	return false
}

// validateMIMEType 校验第一版允许的音频 MIME 类型。
//
// 参数:
//   - mimeType: 请求或下载响应中的 MIME 类型。
//
// 返回值:
//   - error: MIME 类型非法时返回 ErrMIMERejected。
func validateMIMEType(mimeType string) error {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if mediaType == "" {
		return nil
	}
	if mediaType == "application/octet-stream" || strings.HasPrefix(mediaType, "audio/") {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrMIMERejected, mediaType)
}

// resolveStorageRoot 返回音频副本存储目录。
//
// 参数:
//   - override: 测试或装配传入的目录覆盖。
//
// 返回值:
//   - string: 音频副本目录。
//   - error: 用户配置目录不可用时返回错误。
func resolveStorageRoot(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "taskdaemon", "audio", "inbound"), nil
}

// absolutePathFor 将数据库相对路径转换为音频副本绝对路径。
//
// 参数:
//   - root: 音频副本根目录。
//   - relativePath: 数据库保存的相对路径。
//
// 返回值:
//   - string: 绝对路径，无法解析时返回空字符串。
func absolutePathFor(root string, relativePath string) string {
	name := filepath.Base(relativePath)
	if root == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	return filepath.Join(root, name)
}
