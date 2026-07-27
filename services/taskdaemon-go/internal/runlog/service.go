package runlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
)

// Service 编排 run 日志落盘、查询、清理。
type Service struct {
	store  *Store
	data   *data.Store
	cfg    config.RunLogConfig
	logger *slog.Logger

	mu   sync.Mutex
	open map[int]*Archive
}

// Options 控制 runlog 服务可选依赖。
type Options struct {
	// Root 覆盖归档根目录；测试注入用。
	Root string
	// Logger 进程日志；nil 时使用 slog.Default。
	Logger *slog.Logger
}

// Meta 是日志元信息查询结果。
type Meta struct {
	RunID         int           `json:"runId"`
	Status        ArchiveStatus `json:"status"`
	SizeBytes     *int64        `json:"sizeBytes"`
	CreatedAt     *time.Time    `json:"createdAt"`
	WriteFailed   bool          `json:"writeFailed"`
	Available     bool          `json:"available"`
	PathHintMonth string        `json:"-"`
}

// New 创建 runlog 服务。
//
// 参数:
//   - dataStore: Ent 数据层。
//   - cfg: runlog 配置。
//   - opts: 可选根目录与 logger。
//
// 返回值:
//   - *Service: 服务实例。
//   - error: 存储初始化失败时返回错误。
func New(dataStore *data.Store, cfg config.RunLogConfig, opts Options) (*Service, error) {
	store, err := NewStore(opts.Root)
	if err != nil {
		return nil, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:  store,
		data:   dataStore,
		cfg:    cfg,
		logger: logger,
		open:   make(map[int]*Archive),
	}, nil
}

// Enabled 返回是否启用落盘。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 配置启用时为 true。
func (service *Service) Enabled() bool {
	return service.cfg.Enabled
}

// Config 返回当前配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.RunLogConfig: 配置。
func (service *Service) Config() config.RunLogConfig {
	return service.cfg
}

// SetConfig 更新运行时配置（保留策略热更新）。
//
// 参数:
//   - cfg: 新配置。
//
// 返回值:
//   - 无。
func (service *Service) SetConfig(cfg config.RunLogConfig) {
	service.cfg = cfg
}

// Begin 打开一次 run 的归档写入；失败不阻断 run。
//
// 参数:
//   - runID: 执行历史 ID。
//   - startedAt: 开始时间。
//
// 返回值:
//   - *Archive: 成功时返回写入器；失败时为 nil。
//   - bool: true 表示打开失败（调用方应标记 log_write_failed）。
func (service *Service) Begin(runID int, startedAt time.Time) (*Archive, bool) {
	if !service.cfg.Enabled {
		return nil, false
	}
	archive, err := OpenArchive(service.store, runID, startedAt, func(writeErr error) {
		service.logger.Error("runlog write failed", "run_id", runID, "error", writeErr, "component", "runlog")
	})
	if err != nil {
		service.logger.Error("runlog open failed", "run_id", runID, "error", err, "component", "runlog")
		return nil, true
	}
	service.mu.Lock()
	service.open[runID] = archive
	service.mu.Unlock()
	return archive, false
}

// Finish 关闭归档并返回落盘结果。
//
// 参数:
//   - archive: Begin 返回的写入器；可为 nil。
//
// 返回值:
//   - ArchiveStatus: 最终归档状态。
//   - int64: 文件大小。
//   - bool: 是否写失败。
func (service *Service) Finish(archive *Archive) (ArchiveStatus, int64, bool) {
	if archive == nil {
		return StatusAbsent, 0, false
	}
	runID := archive.RunID()
	size, err := archive.Close()
	service.mu.Lock()
	delete(service.open, runID)
	service.mu.Unlock()

	if archive.WriteFailed() || err != nil {
		// 写失败时尽量删除残缺文件，状态保持 absent + write_failed
		_ = service.store.Remove(archive.Path())
		return StatusAbsent, 0, true
	}
	return StatusArchived, size, false
}

// IsWriting 判断指定 run 是否正在写入。
//
// 参数:
//   - runID: 执行历史 ID。
//
// 返回值:
//   - bool: 正在写入时为 true。
func (service *Service) IsWriting(runID int) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	_, ok := service.open[runID]
	return ok
}

// GetMeta 查询 run 日志元信息。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - Meta: 元信息。
//   - error: run 不存在时返回 ErrRunNotFound。
func (service *Service) GetMeta(ctx context.Context, runID int) (Meta, error) {
	runRecord, err := service.data.Client().Run.Get(ctx, runID)
	if err != nil {
		if ent.IsNotFound(err) {
			return Meta{}, ErrRunNotFound
		}
		return Meta{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	status := ArchiveStatus(runRecord.LogArchiveStatus)
	meta := Meta{
		RunID:       runID,
		Status:      status,
		WriteFailed: runRecord.LogWriteFailed,
		Available:   false,
	}
	if runRecord.LogSizeBytes != nil {
		size := *runRecord.LogSizeBytes
		meta.SizeBytes = &size
	}
	if status != StatusArchived {
		return meta, nil
	}
	path, err := service.store.PathFor(runID, runRecord.StartedAt)
	if err != nil {
		return meta, nil
	}
	info, err := service.store.Stat(path)
	if err != nil {
		// DB 说 archived 但文件丢失：仍返回 archived，Available=false
		return meta, nil
	}
	size := info.Size()
	meta.SizeBytes = &size
	mod := info.ModTime()
	meta.CreatedAt = &mod
	meta.Available = true
	return meta, nil
}

// OpenDownload 打开完整日志流式下载。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - io.ReadCloser: 文件流。
//   - int64: Content-Length。
//   - string: 建议文件名。
//   - error: 业务错误。
func (service *Service) OpenDownload(ctx context.Context, runID int) (io.ReadCloser, int64, string, error) {
	path, size, err := service.resolveReadablePath(ctx, runID)
	if err != nil {
		return nil, 0, "", err
	}
	file, err := service.store.OpenRead(path)
	if err != nil {
		return nil, 0, "", fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return file, size, fmt.Sprintf("run-%d.log", runID), nil
}

// ReadTail 读取尾部 N 行。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//   - lines: 行数。
//
// 返回值:
//   - []byte: 尾部内容。
//   - error: 业务错误。
func (service *Service) ReadTail(ctx context.Context, runID int, lines int) ([]byte, error) {
	if lines <= 0 {
		return nil, fmt.Errorf("%w: lines", ErrInvalidRange)
	}
	path, _, err := service.resolveReadablePath(ctx, runID)
	if err != nil {
		return nil, err
	}
	data, err := service.store.ReadTail(path, lines)
	if err != nil {
		if err == ErrInvalidRange {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return data, nil
}

// ReadRange 按字节范围读取。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//   - offset: 偏移。
//   - length: 长度。
//
// 返回值:
//   - []byte: 范围内容。
//   - error: 业务错误。
func (service *Service) ReadRange(ctx context.Context, runID int, offset int64, length int64) ([]byte, error) {
	if offset < 0 || length <= 0 {
		return nil, fmt.Errorf("%w: offset/length", ErrInvalidRange)
	}
	path, _, err := service.resolveReadablePath(ctx, runID)
	if err != nil {
		return nil, err
	}
	data, err := service.store.ReadRange(path, offset, length)
	if err != nil {
		if err == ErrInvalidRange {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return data, nil
}

// resolveReadablePath 校验 run 存在且归档可读，返回路径与大小。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - string: 文件路径。
//   - int64: 文件大小。
//   - error: 业务错误。
func (service *Service) resolveReadablePath(ctx context.Context, runID int) (string, int64, error) {
	runRecord, err := service.data.Client().Run.Get(ctx, runID)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", 0, ErrRunNotFound
		}
		return "", 0, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	status := ArchiveStatus(runRecord.LogArchiveStatus)
	if status != StatusArchived {
		return "", 0, ErrArchiveNotFound
	}
	path, err := service.store.PathFor(runID, runRecord.StartedAt)
	if err != nil {
		return "", 0, ErrArchiveNotFound
	}
	info, err := service.store.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, ErrArchiveNotFound
		}
		return "", 0, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return path, info.Size(), nil
}

// Prune 按保留天数与总体积上限清理归档。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - PruneResult: 清理统计。
//   - error: 扫描或删除失败时返回错误。
func (service *Service) Prune(ctx context.Context) (PruneResult, error) {
	return prune(ctx, service)
}

// PruneResult 描述一次清理结果。
type PruneResult struct {
	DeletedCount int
	FreedBytes   int64
}

// ApplyArchiveResult 将落盘结果写回 run 记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//   - status: 归档状态。
//   - size: 文件大小。
//   - writeFailed: 是否写失败。
//
// 返回值:
//   - error: 数据库更新失败时返回错误。
func (service *Service) ApplyArchiveResult(ctx context.Context, runID int, status ArchiveStatus, size int64, writeFailed bool) error {
	update := service.data.Client().Run.UpdateOneID(runID).
		SetLogArchiveStatus(entrun.LogArchiveStatus(status)).
		SetLogWriteFailed(writeFailed)
	if status == StatusArchived {
		update.SetLogSizeBytes(size)
	} else {
		update.ClearLogSizeBytes()
	}
	_, err := update.Save(ctx)
	if err != nil {
		return fmt.Errorf("update run log archive fields: %w", err)
	}
	return nil
}

// markPruned 将 run 标记为 pruned 并删除文件。
//
// 参数:
//   - ctx: 请求上下文。
//   - entry: 文件条目。
//
// 返回值:
//   - error: 删除或更新失败时返回错误。
func (service *Service) markPruned(ctx context.Context, entry FileEntry) error {
	if err := service.store.Remove(entry.Path); err != nil {
		return err
	}
	exists, err := service.data.Client().Run.Query().Where(entrun.IDEQ(entry.RunID)).Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = service.data.Client().Run.UpdateOneID(entry.RunID).
		SetLogArchiveStatus(entrun.LogArchiveStatusPruned).
		ClearLogSizeBytes().
		Save(ctx)
	return err
}

// sortEntriesOldestFirst 按修改时间从旧到新排序。
//
// 参数:
//   - entries: 文件列表。
//
// 返回值:
//   - 无（原地排序）。
func sortEntriesOldestFirst(entries []FileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ModTime.Equal(entries[j].ModTime) {
			return entries[i].RunID < entries[j].RunID
		}
		return entries[i].ModTime.Before(entries[j].ModTime)
	})
}
