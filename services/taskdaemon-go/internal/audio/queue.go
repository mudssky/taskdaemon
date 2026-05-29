package audio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	audiorecord "taskdaemon/internal/data/ent/audiorecord"
)

// Queue 顺序执行后端音频播放请求。
type Queue struct {
	store  *data.Store
	player Player
	logger *slog.Logger

	mu          sync.RWMutex
	cfg         config.AudioConfig
	storageRoot string
	pending     []int
	wake        chan struct{}
	closed      chan struct{}
}

// QueueOptions 定义播放队列可选依赖。
type QueueOptions struct {
	Player      Player
	Logger      *slog.Logger
	StorageRoot string
}

// NewQueue 创建后端播放队列并启动后台 worker。
//
// 参数:
//   - store: 数据层实例。
//   - cfg: 音频配置快照。
//   - opts: 播放器、logger 和存储路径覆盖。
//
// 返回值:
//   - *Queue: 可接收入队请求的播放队列。
func NewQueue(store *data.Store, cfg config.AudioConfig, opts QueueOptions) *Queue {
	player := opts.Player
	if player == nil {
		player = NewFFmpegOtoPlayer()
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	queue := &Queue{
		store:       store,
		player:      player,
		logger:      logger,
		cfg:         cfg,
		storageRoot: opts.StorageRoot,
		wake:        make(chan struct{}, 1),
		closed:      make(chan struct{}),
	}
	go queue.run()
	return queue
}

// UpdateConfig 更新播放队列运行时配置。
//
// 参数:
//   - cfg: 新的音频配置快照。
//
// 返回值:
//   - 无。
func (queue *Queue) UpdateConfig(cfg config.AudioConfig) {
	if queue == nil {
		return
	}
	queue.mu.Lock()
	queue.cfg = cfg
	queue.mu.Unlock()
}

// Enqueue 尝试将音频记录加入播放队列。
//
// 参数:
//   - ctx: 请求上下文。
//   - recordID: 音频历史记录 ID。
//
// 返回值:
//   - error: 队列关闭、上下文取消或队列已满时返回错误。
func (queue *Queue) Enqueue(ctx context.Context, recordID int) error {
	if queue == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	select {
	case <-queue.closed:
		return fmt.Errorf("%w: closed", ErrPlaybackFailed)
	default:
	}
	if queue.cfg.Playback.QueueLimit > 0 && len(queue.pending) >= queue.cfg.Playback.QueueLimit {
		return ErrQueueFull
	}
	queue.pending = append(queue.pending, recordID)
	queue.notify()
	return nil
}

// Shutdown 停止播放队列。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (queue *Queue) Shutdown() {
	if queue == nil {
		return
	}
	select {
	case <-queue.closed:
	default:
		close(queue.closed)
	}
}

// config 返回当前音频配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.AudioConfig: 当前配置。
func (queue *Queue) config() config.AudioConfig {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	return queue.cfg
}

// run 消费队列并顺序播放音频。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (queue *Queue) run() {
	for {
		recordID, ok := queue.next()
		if !ok {
			return
		}
		queue.playRecord(recordID)
	}
}

// next 取出下一条待播放记录；没有记录时等待唤醒或关闭。
//
// 参数:
//   - 无。
//
// 返回值:
//   - int: 下一条音频记录 ID。
//   - bool: 队列仍可继续工作时为 true；关闭时为 false。
func (queue *Queue) next() (int, bool) {
	for {
		queue.mu.Lock()
		if len(queue.pending) > 0 {
			recordID := queue.pending[0]
			queue.pending = queue.pending[1:]
			queue.mu.Unlock()
			return recordID, true
		}
		queue.mu.Unlock()
		select {
		case <-queue.closed:
			return 0, false
		case <-queue.wake:
		}
	}
}

// playRecord 播放指定音频记录，并持久化播放状态。
//
// 参数:
//   - recordID: 音频历史记录 ID。
//
// 返回值:
//   - 无。
func (queue *Queue) playRecord(recordID int) {
	ctx := context.Background()
	record, err := queue.store.Client().AudioRecord.Get(ctx, recordID)
	if err != nil {
		queue.logger.Warn("audio record unavailable", "component", "audio", "recordId", recordID, "error", err)
		return
	}
	now := time.Now()
	record, err = record.Update().SetStatus(audiorecord.StatusPlaying).Save(ctx)
	if err != nil {
		queue.logger.Warn("mark audio playing failed", "component", "audio", "recordId", recordID, "error", err)
		return
	}
	path := absolutePathFor(queue.resolvedStorageRoot(), record.StoredPath)
	err = queue.player.Play(ctx, path, queue.config())
	if err != nil {
		summary := truncate(err.Error(), 512)
		if updateErr := record.Update().SetStatus(audiorecord.StatusFailed).SetErrorSummary(summary).SetPlayedAt(now).Exec(ctx); updateErr != nil {
			queue.logger.Warn("mark audio failed failed", "component", "audio", "recordId", recordID, "error", updateErr)
		}
		if !errors.Is(err, ErrFFmpegUnavailable) {
			queue.logger.Warn("audio playback failed", "component", "audio", "recordId", recordID, "error", err)
		}
		return
	}
	if err := record.Update().SetStatus(audiorecord.StatusPlayed).ClearErrorSummary().SetPlayedAt(time.Now()).Exec(ctx); err != nil {
		queue.logger.Warn("mark audio played failed", "component", "audio", "recordId", recordID, "error", err)
	}
}

// storageRoot 返回队列读取本地音频副本时使用的目录。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 本地音频副本目录。
func (queue *Queue) resolvedStorageRoot() string {
	root, err := resolveStorageRoot(queue.storageRoot)
	if err != nil {
		return ""
	}
	return root
}

// notify 唤醒后台 worker。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (queue *Queue) notify() {
	select {
	case queue.wake <- struct{}{}:
	default:
	}
}
