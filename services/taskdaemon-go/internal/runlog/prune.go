package runlog

import (
	"context"
	"fmt"
	"time"

	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
)

// prune 执行双维度清理：先按天数，再按总体积从旧到新。
//
// 参数:
//   - ctx: 请求上下文。
//   - service: runlog 服务。
//
// 返回值:
//   - PruneResult: 删除数量与释放字节。
//   - error: 扫描失败时返回错误。
func prune(ctx context.Context, service *Service) (PruneResult, error) {
	result := PruneResult{}
	if !service.cfg.Enabled {
		return result, nil
	}

	entries, err := service.store.ListEntries()
	if err != nil {
		return result, err
	}
	if len(entries) == 0 {
		return result, nil
	}

	// 跳过正在写入的 run
	candidates := make([]FileEntry, 0, len(entries))
	var totalBytes int64
	for _, entry := range entries {
		if service.IsWriting(entry.RunID) {
			continue
		}
		// 跳过 DB 中仍为 running 的 run（防竞态）
		if isRunStillRunning(ctx, service, entry.RunID) {
			continue
		}
		candidates = append(candidates, entry)
		totalBytes += entry.Size
	}

	sortEntriesOldestFirst(candidates)
	now := time.Now()
	retainDays := service.cfg.RetainDays
	maxTotal := service.cfg.MaxTotalBytes

	// 1) 天数淘汰
	if retainDays > 0 {
		cutoff := now.AddDate(0, 0, -retainDays)
		remaining := make([]FileEntry, 0, len(candidates))
		for _, entry := range candidates {
			if entry.ModTime.Before(cutoff) {
				if err := service.markPruned(ctx, entry); err != nil {
					service.logger.Error("runlog prune delete failed", "run_id", entry.RunID, "error", err, "component", "runlog")
					remaining = append(remaining, entry)
					continue
				}
				result.DeletedCount++
				result.FreedBytes += entry.Size
				totalBytes -= entry.Size
				continue
			}
			remaining = append(remaining, entry)
		}
		candidates = remaining
	}

	// 2) 体积淘汰（从旧到新）
	if maxTotal > 0 && totalBytes > maxTotal {
		for _, entry := range candidates {
			if totalBytes <= maxTotal {
				break
			}
			if service.IsWriting(entry.RunID) {
				continue
			}
			if err := service.markPruned(ctx, entry); err != nil {
				service.logger.Error("runlog prune delete failed", "run_id", entry.RunID, "error", err, "component", "runlog")
				continue
			}
			result.DeletedCount++
			result.FreedBytes += entry.Size
			totalBytes -= entry.Size
		}
	}

	if result.DeletedCount > 0 {
		service.logger.Info("runlog pruned",
			"deleted_count", result.DeletedCount,
			"freed_bytes", result.FreedBytes,
			"component", "runlog",
		)
	}
	return result, nil
}

// isRunStillRunning 查询 DB 中 run 是否仍为 running。
//
// 参数:
//   - ctx: 请求上下文。
//   - service: runlog 服务。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - bool: 仍在运行时为 true；查询失败时保守返回 true。
func isRunStillRunning(ctx context.Context, service *Service, runID int) bool {
	runRecord, err := service.data.Client().Run.Get(ctx, runID)
	if err != nil {
		if ent.IsNotFound(err) {
			return false
		}
		// 查询失败时保守跳过，避免误删
		return true
	}
	return runRecord.Status == entrun.StatusRunning
}

// SchedulePrune 在后台异步触发清理，不阻塞调用方。
//
// 参数:
//   - service: runlog 服务。
//
// 返回值:
//   - 无。
func SchedulePrune(service *Service) {
	if service == nil || !service.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := service.Prune(ctx); err != nil {
			service.logger.Error("runlog prune failed", "error", err, "component", "runlog")
		}
	}()
}

// FormatPruneSummary 生成清理摘要（测试与日志复用）。
//
// 参数:
//   - result: 清理结果。
//
// 返回值:
//   - string: 可读摘要。
func FormatPruneSummary(result PruneResult) string {
	return fmt.Sprintf("deleted=%d freed_bytes=%d", result.DeletedCount, result.FreedBytes)
}
