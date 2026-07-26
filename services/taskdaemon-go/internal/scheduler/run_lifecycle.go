package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	enttask "taskdaemon/internal/data/ent/task"
	"taskdaemon/internal/notify"
	"taskdaemon/internal/runlog"
	"taskdaemon/internal/runner"
)

var (
	// ErrTaskAlreadyRunning 表示同一任务已有执行正在运行。
	ErrTaskAlreadyRunning = errors.New("task already running")
	// ErrTaskNotRunning 表示取消目标任务时没有运行中的执行。
	ErrTaskNotRunning = errors.New("task is not running")
)

// TriggerTask 手动触发任务并写入执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制任务查询和执行生命周期的 context。
//   - taskID: 任务 ID。
//
// 返回值:
//   - *ent.Run: 最终执行历史。
//   - error: 任务不存在、执行记录写入失败或 runner 配置解析失败时返回错误。
func (service *Service) TriggerTask(ctx context.Context, taskID int) (*ent.Run, error) {
	return service.triggerTaskWithSource(ctx, taskID, entrun.TriggerManual)
}

// triggerTaskWithSource 触发任务并按指定来源写入执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制任务查询和执行生命周期的 context。
//   - taskID: 任务 ID。
//   - trigger: 触发来源。
//
// 返回值:
//   - *ent.Run: 最终执行历史。
//   - error: 任务不存在、执行记录写入失败或 runner 配置解析失败时返回错误。
func (service *Service) triggerTaskWithSource(ctx context.Context, taskID int, trigger entrun.Trigger) (*ent.Run, error) {
	taskRecord, err := service.store.Client().Task.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	runCtx, cancel, alreadyRunning := service.beginRun(ctx, taskID)
	if alreadyRunning {
		return service.recordSkipped(ctx, taskID, trigger)
	}
	defer service.finishRun(taskID)
	defer cancel()

	started := service.now()
	runRecord, err := service.store.Client().Run.Create().
		SetTaskID(taskID).
		SetTrigger(trigger).
		SetStatus(entrun.StatusRunning).
		SetStartedAt(started).
		SetCreatedAt(started).
		SetUpdatedAt(started).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create run record: %w", err)
	}

	cfg, err := runnerConfigFromTask(taskRecord)
	if err != nil {
		return service.finalizeFailed(ctx, taskID, runRecord.ID, started, err.Error())
	}
	runID := runRecord.ID
	archive, openFailed := service.beginRunLog(runID, started)
	if archive != nil {
		cfg.StdoutMirror = archive.StdoutWriter()
		cfg.StderrMirror = archive.StderrWriter()
	}
	result, err := service.runner.Execute(runCtx, cfg)
	if err != nil {
		service.finishRunLog(ctx, runID, archive, openFailed)
		return service.finalizeFailed(ctx, taskID, runID, started, err.Error())
	}
	runRecord, err = service.finalizeRun(ctx, taskID, runID, started, result)
	if err != nil {
		service.finishRunLog(ctx, runID, archive, openFailed)
		return nil, err
	}
	service.finishRunLog(ctx, runID, archive, openFailed)
	return runRecord, nil
}

// CancelTask 取消正在运行的任务。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 调用方 context，当前用于保持 API 形态一致。
//   - taskID: 任务 ID。
//
// 返回值:
//   - error: 任务没有运行时返回 ErrTaskNotRunning。
func (service *Service) CancelTask(ctx context.Context, taskID int) error {
	_ = ctx
	service.mu.Lock()
	cancel, ok := service.running[taskID]
	service.mu.Unlock()
	if !ok {
		return ErrTaskNotRunning
	}
	cancel()
	return nil
}

// ListTaskRuns 查询指定任务的执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库查询生命周期的 context。
//   - taskID: 任务 ID。
//   - limit: 最大返回条数；小于等于 0 时使用默认 50。
//
// 返回值:
//   - []*ent.Run: 按 ID 倒序返回的执行历史。
//   - error: 数据库查询失败时返回错误。
func (service *Service) ListTaskRuns(ctx context.Context, taskID int, limit int) ([]*ent.Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return service.store.Client().Run.Query().
		Where(entrun.HasTaskWith(enttask.IDEQ(taskID))).
		Order(entrun.ByID(entsql.OrderDesc())).
		Limit(limit).
		All(ctx)
}

// IsTaskRunning 返回任务当前是否有运行中的执行。
//
// 参数:
//   - service: 调度服务。
//   - taskID: 任务 ID。
//
// 返回值:
//   - bool: true 表示任务正在运行。
func (service *Service) IsTaskRunning(taskID int) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	_, ok := service.running[taskID]
	return ok
}

// beginRun 尝试登记任务运行态。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 父 context。
//   - taskID: 任务 ID。
//
// 返回值:
//   - context.Context: 本次执行 context。
//   - context.CancelFunc: 取消本次执行的函数。
//   - bool: true 表示任务已在运行。
func (service *Service) beginRun(ctx context.Context, taskID int) (context.Context, context.CancelFunc, bool) {
	runCtx, cancel := context.WithCancel(ctx)
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, exists := service.running[taskID]; exists {
		cancel()
		return nil, nil, true
	}
	service.running[taskID] = cancel
	return runCtx, cancel, false
}

// finishRun 清除任务运行态。
//
// 参数:
//   - service: 调度服务。
//   - taskID: 任务 ID。
//
// 返回值:
//   - 无。
func (service *Service) finishRun(taskID int) {
	service.mu.Lock()
	defer service.mu.Unlock()
	delete(service.running, taskID)
}

// recordSkipped 写入 overlap skipped 执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - taskID: 任务 ID。
//   - trigger: 触发来源。
//
// 返回值:
//   - *ent.Run: skipped 执行历史。
//   - error: 数据库写入失败时返回错误。
func (service *Service) recordSkipped(ctx context.Context, taskID int, trigger entrun.Trigger) (*ent.Run, error) {
	now := service.now()
	runRecord, err := service.store.Client().Run.Create().
		SetTaskID(taskID).
		SetTrigger(trigger).
		SetStatus(entrun.StatusSkipped).
		SetStartedAt(now).
		SetFinishedAt(now).
		SetDurationMs(0).
		SetErrorSummary(ErrTaskAlreadyRunning.Error()).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	// 通知失败不得影响 skipped 落库结果
	service.publishTaskRunEvent(ctx, notify.NameTaskRunSkipped, taskID, runRecord, string(trigger), nil, 0)
	return runRecord, nil
}

// finalizeFailed 将无法进入 runner 的执行记录更新为 failed。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - taskID: 任务 ID。
//   - runID: 执行历史 ID。
//   - started: 执行开始时间。
//   - summary: 错误摘要。
//
// 返回值:
//   - *ent.Run: failed 执行历史。
//   - error: 数据库更新失败时返回错误。
func (service *Service) finalizeFailed(ctx context.Context, taskID int, runID int, started time.Time, summary string) (*ent.Run, error) {
	now := service.now()
	durationMs := now.Sub(started).Milliseconds()
	runRecord, err := service.store.Client().Run.UpdateOneID(runID).
		SetStatus(entrun.StatusFailed).
		SetFinishedAt(now).
		SetDurationMs(durationMs).
		SetErrorSummary(summary).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	// summary 可能含路径/命令片段，不进入通知 payload
	service.publishTaskRunEvent(ctx, notify.NameTaskRunFailed, taskID, runRecord, string(runRecord.Trigger), nil, durationMs)
	return runRecord, nil
}

// finalizeRun 将 runner 结果落库为最终执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库更新生命周期的 context。
//   - taskID: 任务 ID。
//   - runID: 执行历史 ID。
//   - started: 执行开始时间。
//   - result: runner 执行结果。
//
// 返回值:
//   - *ent.Run: 最终执行历史。
//   - error: 数据库更新失败时返回错误。
func (service *Service) finalizeRun(ctx context.Context, taskID int, runID int, started time.Time, result runner.Result) (*ent.Run, error) {
	now := service.now()
	durationMs := maxInt64(result.Duration.Milliseconds(), now.Sub(started).Milliseconds())
	update := service.store.Client().Run.UpdateOneID(runID).
		SetStatus(toRunStatus(result.Status)).
		SetFinishedAt(now).
		SetDurationMs(durationMs).
		SetErrorSummary(result.ErrorSummary).
		SetStdout(result.Stdout).
		SetStderr(result.Stderr).
		SetUpdatedAt(now)
	if result.ExitCode != nil {
		update.SetExitCode(*result.ExitCode)
	}
	runRecord, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	// stdout/stderr/ErrorSummary 可能含敏感命令输出，不进入通知
	service.publishTaskRunEvent(ctx, taskRunEventName(result.Status), taskID, runRecord, string(runRecord.Trigger), result.ExitCode, durationMs)
	return runRecord, nil
}

// toRunStatus 将 runner 状态映射为 Ent run 状态。
//
// 参数:
//   - status: runner 执行状态。
//
// 返回值:
//   - entrun.Status: 执行历史状态。
func toRunStatus(status runner.Status) entrun.Status {
	switch status {
	case runner.StatusSuccess:
		return entrun.StatusSuccess
	case runner.StatusTimeout:
		return entrun.StatusTimeout
	case runner.StatusCancelled:
		return entrun.StatusCancelled
	default:
		return entrun.StatusFailed
	}
}

// taskRunEventName 将 runner 状态映射为通知事件名。
//
// 参数:
//   - status: runner 执行状态。
//
// 返回值:
//   - notify.Name: 对应事件名。
func taskRunEventName(status runner.Status) notify.Name {
	switch status {
	case runner.StatusSuccess:
		return notify.NameTaskRunSucceeded
	case runner.StatusTimeout:
		return notify.NameTaskRunTimeout
	case runner.StatusCancelled:
		return notify.NameTaskRunCancelled
	default:
		return notify.NameTaskRunFailed
	}
}

// publishTaskRunEvent 向通知总线发布任务终态事件；publisher 为空或构造失败时静默跳过。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 调用方 context。
//   - name: 事件名。
//   - taskID: 任务 ID。
//   - runRecord: 已落库的执行记录。
//   - trigger: 触发来源。
//   - exitCode: 可选退出码。
//   - durationMs: 耗时毫秒。
//
// 返回值:
//   - 无。
func (service *Service) publishTaskRunEvent(
	ctx context.Context,
	name notify.Name,
	taskID int,
	runRecord *ent.Run,
	trigger string,
	exitCode *int,
	durationMs int64,
) {
	if service == nil || service.publisher == nil || runRecord == nil {
		return
	}
	occurredAt := service.now()
	if runRecord.FinishedAt != nil {
		occurredAt = *runRecord.FinishedAt
	}
	event, err := notify.BuildTaskRunEvent(name, taskID, runRecord.ID, occurredAt, exitCode, durationMs, trigger)
	if err != nil {
		return
	}
	// Publish 不返回 error，通知失败不得改变调度结果
	service.publisher.Publish(ctx, event)
}

// publishSchedulerLifecycle 发布调度器启停事件。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 调用方 context。
//   - name: started 或 stopped。
//
// 返回值:
//   - 无。
func (service *Service) publishSchedulerLifecycle(ctx context.Context, name notify.Name) {
	if service == nil || service.publisher == nil {
		return
	}
	event, err := notify.BuildSchedulerLifecycleEvent(name, service.now())
	if err != nil {
		return
	}
	service.publisher.Publish(ctx, event)
}

// maxInt64 返回两个 int64 中较大的值。
//
// 参数:
//   - a: 候选值。
//   - b: 候选值。
//
// 返回值:
//   - int64: 较大的值。
func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// beginRunLog 打开 run 日志归档；失败不阻断执行。
//
// 参数:
//   - service: 调度服务。
//   - runID: 执行历史 ID。
//   - startedAt: 开始时间。
//
// 返回值:
//   - *runlog.Archive: 写入器；未启用或打开失败时为 nil。
//   - bool: true 表示打开失败，需要标记 log_write_failed。
func (service *Service) beginRunLog(runID int, startedAt time.Time) (*runlog.Archive, bool) {
	if service == nil || service.runlog == nil {
		return nil, false
	}
	return service.runlog.Begin(runID, startedAt)
}

// finishRunLog 关闭归档、回写状态，并异步触发清理。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 请求上下文。
//   - runID: 执行历史 ID。
//   - archive: 归档写入器。
//   - openFailed: 打开阶段是否失败。
//
// 返回值:
//   - 无。
func (service *Service) finishRunLog(ctx context.Context, runID int, archive *runlog.Archive, openFailed bool) {
	if service == nil || service.runlog == nil {
		return
	}
	if openFailed && archive == nil {
		_ = service.runlog.ApplyArchiveResult(ctx, runID, runlog.StatusAbsent, 0, true)
		return
	}
	status, size, writeFailed := service.runlog.Finish(archive)
	if openFailed {
		writeFailed = true
	}
	if err := service.runlog.ApplyArchiveResult(ctx, runID, status, size, writeFailed); err != nil {
		// 落库失败只记进程侧可观测性，不回滚 run 终态
		return
	}
	runlog.SchedulePrune(service.runlog)
}
