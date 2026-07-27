package runlog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	entrun "taskdaemon/internal/data/ent/run"
	enttask "taskdaemon/internal/data/ent/task"
)

// openTestService 创建带临时根目录与内存库的 runlog 服务。
//
// 参数:
//   - t: 测试上下文。
//   - cfg: runlog 配置。
//
// 返回值:
//   - *Service: 服务实例。
//   - *data.Store: 数据层。
func openTestService(t *testing.T, cfg config.RunLogConfig) (*Service, *data.Store) {
	t.Helper()
	ctx := context.Background()
	store, err := data.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:runlog-" + t.Name() + "?mode=memory&cache=shared&_fk=1",
	})
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	service, err := New(store, cfg, Options{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service, store
}

// createTaskAndRun 创建任务与一条 run 记录。
//
// 参数:
//   - t: 测试上下文。
//   - store: 数据层。
//   - status: 归档状态。
//
// 返回值:
//   - int: runID。
//   - time.Time: startedAt。
func createTaskAndRun(t *testing.T, store *data.Store, status entrun.LogArchiveStatus) (int, time.Time) {
	t.Helper()
	ctx := context.Background()
	taskRecord, err := store.Client().Task.Create().
		SetName("t-" + t.Name()).
		SetEnabled(true).
		SetCronExpression("0 0 * * *").
		SetTimezone("Local").
		SetRunnerType(enttask.RunnerTypeShell).
		SetRunnerConfig(map[string]any{"type": "shell", "inline": "echo ok"}).
		SetTimeoutSeconds(60).
		Save(ctx)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	started := time.Now().UTC().Truncate(time.Second)
	runRecord, err := store.Client().Run.Create().
		SetTaskID(taskRecord.ID).
		SetTrigger(entrun.TriggerManual).
		SetStatus(entrun.StatusSuccess).
		SetStartedAt(started).
		SetLogArchiveStatus(status).
		Save(ctx)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	return runRecord.ID, started
}

// TestBeginFinishArchived 验证落盘与三态回写。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestBeginFinishArchived(t *testing.T) {
	cfg := config.RunLogConfig{Enabled: true, RetainDays: 30, MaxTotalBytes: 0}
	service, store := openTestService(t, cfg)
	runID, started := createTaskAndRun(t, store, entrun.LogArchiveStatusAbsent)

	archive, openFailed := service.Begin(runID, started)
	if openFailed || archive == nil {
		t.Fatalf("begin failed: openFailed=%v archive=%v", openFailed, archive)
	}
	_, _ = archive.StdoutWriter().Write([]byte("ok\n"))
	status, size, writeFailed := service.Finish(archive)
	if status != StatusArchived || writeFailed || size <= 0 {
		t.Fatalf("finish = %s size=%d failed=%v", status, size, writeFailed)
	}
	if err := service.ApplyArchiveResult(context.Background(), runID, status, size, writeFailed); err != nil {
		t.Fatalf("apply: %v", err)
	}
	meta, err := service.GetMeta(context.Background(), runID)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta.Status != StatusArchived || !meta.Available {
		t.Fatalf("meta = %+v", meta)
	}
}

// TestGetMetaThreeStates 验证 absent/archived/pruned 可区分。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestGetMetaThreeStates(t *testing.T) {
	cfg := config.RunLogConfig{Enabled: true}
	service, store := openTestService(t, cfg)

	absentID, _ := createTaskAndRun(t, store, entrun.LogArchiveStatusAbsent)
	meta, err := service.GetMeta(context.Background(), absentID)
	if err != nil {
		t.Fatalf("absent meta: %v", err)
	}
	if meta.Status != StatusAbsent || meta.Available {
		t.Fatalf("absent meta = %+v", meta)
	}

	prunedID, _ := createTaskAndRun(t, store, entrun.LogArchiveStatusPruned)
	meta, err = service.GetMeta(context.Background(), prunedID)
	if err != nil {
		t.Fatalf("pruned meta: %v", err)
	}
	if meta.Status != StatusPruned {
		t.Fatalf("pruned meta = %+v", meta)
	}

	archivedID, started := createTaskAndRun(t, store, entrun.LogArchiveStatusArchived)
	path, err := service.store.PathFor(archivedID, started)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("[stdout] hi\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	size := int64(len("[stdout] hi\n"))
	_, err = store.Client().Run.UpdateOneID(archivedID).SetLogSizeBytes(size).Save(context.Background())
	if err != nil {
		t.Fatalf("update size: %v", err)
	}
	meta, err = service.GetMeta(context.Background(), archivedID)
	if err != nil {
		t.Fatalf("archived meta: %v", err)
	}
	if meta.Status != StatusArchived || !meta.Available {
		t.Fatalf("archived meta = %+v", meta)
	}
}

// TestPruneByDaysAndBytes 验证双维度清理与 running 跳过。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPruneByDaysAndBytes(t *testing.T) {
	cfg := config.RunLogConfig{Enabled: true, RetainDays: 1, MaxTotalBytes: 20}
	service, store := openTestService(t, cfg)
	ctx := context.Background()

	// 过期文件
	oldID, oldStarted := createTaskAndRun(t, store, entrun.LogArchiveStatusArchived)
	oldPath, _ := service.store.PathFor(oldID, oldStarted)
	_ = os.MkdirAll(filepath.Dir(oldPath), 0o700)
	_ = os.WriteFile(oldPath, []byte("old-content-long-enough"), 0o600)
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(oldPath, oldTime, oldTime)
	_, _ = store.Client().Run.UpdateOneID(oldID).SetLogSizeBytes(int64(len("old-content-long-enough"))).Save(ctx)

	// 新文件但仍超体积
	newID, newStarted := createTaskAndRun(t, store, entrun.LogArchiveStatusArchived)
	newPath, _ := service.store.PathFor(newID, newStarted)
	_ = os.MkdirAll(filepath.Dir(newPath), 0o700)
	_ = os.WriteFile(newPath, []byte("01234567890123456789"), 0o600)
	_, _ = store.Client().Run.UpdateOneID(newID).SetLogSizeBytes(20).Save(ctx)

	// 正在写入应跳过
	writingID, writingStarted := createTaskAndRun(t, store, entrun.LogArchiveStatusArchived)
	writingPath, _ := service.store.PathFor(writingID, writingStarted)
	_ = os.MkdirAll(filepath.Dir(writingPath), 0o700)
	_ = os.WriteFile(writingPath, []byte("writing"), 0o600)
	archive, _ := service.Begin(writingID, writingStarted)
	if archive == nil {
		// Begin truncates; re-register open map by faking open entry
		service.mu.Lock()
		service.open[writingID] = &Archive{runID: writingID, path: writingPath}
		service.mu.Unlock()
	}

	result, err := service.Prune(ctx)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if result.DeletedCount < 1 {
		t.Fatalf("deleted = %d, want >= 1", result.DeletedCount)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old file still exists")
	}
	oldRun, err := store.Client().Run.Get(ctx, oldID)
	if err != nil {
		t.Fatalf("get old run: %v", err)
	}
	if oldRun.LogArchiveStatus != entrun.LogArchiveStatusPruned {
		t.Fatalf("old status = %s", oldRun.LogArchiveStatus)
	}
	if _, err := os.Stat(writingPath); err != nil {
		t.Fatalf("writing file should remain: %v", err)
	}

	// 关闭 open 标记
	service.mu.Lock()
	delete(service.open, writingID)
	service.mu.Unlock()
	if archive != nil {
		_, _ = archive.Close()
	}
}

// TestPruneZeroMeansUnlimited 验证配置 0 表示不限。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPruneZeroMeansUnlimited(t *testing.T) {
	cfg := config.RunLogConfig{Enabled: true, RetainDays: 0, MaxTotalBytes: 0}
	service, store := openTestService(t, cfg)
	runID, started := createTaskAndRun(t, store, entrun.LogArchiveStatusArchived)
	path, _ := service.store.PathFor(runID, started)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte("keep-me"), 0o600)
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	_ = os.Chtimes(path, oldTime, oldTime)

	result, err := service.Prune(context.Background())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if result.DeletedCount != 0 {
		t.Fatalf("deleted = %d, want 0", result.DeletedCount)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file removed unexpectedly: %v", err)
	}
}
