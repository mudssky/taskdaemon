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
		return service.finalizeFailed(ctx, runRecord.ID, started, err.Error())
	}
	result, err := service.runner.Execute(runCtx, cfg)
	if err != nil {
		return service.finalizeFailed(ctx, runRecord.ID, started, err.Error())
	}
	return service.finalizeRun(ctx, runRecord.ID, started, result)
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
	return service.store.Client().Run.Create().
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
}

// finalizeFailed 将无法进入 runner 的执行记录更新为 failed。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - runID: 执行历史 ID。
//   - started: 执行开始时间。
//   - summary: 错误摘要。
//
// 返回值:
//   - *ent.Run: failed 执行历史。
//   - error: 数据库更新失败时返回错误。
func (service *Service) finalizeFailed(ctx context.Context, runID int, started time.Time, summary string) (*ent.Run, error) {
	now := service.now()
	return service.store.Client().Run.UpdateOneID(runID).
		SetStatus(entrun.StatusFailed).
		SetFinishedAt(now).
		SetDurationMs(now.Sub(started).Milliseconds()).
		SetErrorSummary(summary).
		SetUpdatedAt(now).
		Save(ctx)
}

// finalizeRun 将 runner 结果落库为最终执行历史。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库更新生命周期的 context。
//   - runID: 执行历史 ID。
//   - started: 执行开始时间。
//   - result: runner 执行结果。
//
// 返回值:
//   - *ent.Run: 最终执行历史。
//   - error: 数据库更新失败时返回错误。
func (service *Service) finalizeRun(ctx context.Context, runID int, started time.Time, result runner.Result) (*ent.Run, error) {
	now := service.now()
	update := service.store.Client().Run.UpdateOneID(runID).
		SetStatus(toRunStatus(result.Status)).
		SetFinishedAt(now).
		SetDurationMs(maxInt64(result.Duration.Milliseconds(), now.Sub(started).Milliseconds())).
		SetErrorSummary(result.ErrorSummary).
		SetStdout(result.Stdout).
		SetStderr(result.Stderr).
		SetUpdatedAt(now)
	if result.ExitCode != nil {
		update.SetExitCode(*result.ExitCode)
	}
	return update.Save(ctx)
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
