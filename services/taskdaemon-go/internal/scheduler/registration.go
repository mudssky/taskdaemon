package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	enttask "taskdaemon/internal/data/ent/task"
)

// RegisterEnabledTasks 将已启用的 cron 任务注册到进程内 gocron scheduler。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库查询生命周期的 context。
//
// 返回值:
//   - error: 数据库查询、scheduler 初始化或任务注册失败时返回错误。
func (service *Service) RegisterEnabledTasks(ctx context.Context) error {
	if err := service.ensureScheduler(); err != nil {
		return err
	}
	tasks, err := service.store.Client().Task.Query().
		Where(enttask.EnabledEQ(true), enttask.CronExpressionNotNil()).
		All(ctx)
	if err != nil {
		return fmt.Errorf("list enabled tasks: %w", err)
	}
	for _, taskRecord := range tasks {
		if strings.TrimSpace(taskRecord.CronExpression) == "" {
			continue
		}
		if err := service.removeRegisteredCronTask(taskRecord.ID); err != nil {
			return err
		}
		if err := service.registerCronTask(taskRecord); err != nil {
			return err
		}
	}
	return nil
}

// Start 启动进程内 scheduler。
//
// 参数:
//   - service: 调度服务。
//
// 返回值:
//   - error: scheduler 初始化失败时返回错误。
func (service *Service) Start() error {
	if err := service.ensureScheduler(); err != nil {
		return err
	}
	service.scheduler.Start()
	return nil
}

// Shutdown 关闭进程内 scheduler。
//
// 参数:
//   - service: 调度服务。
//
// 返回值:
//   - error: scheduler 关闭失败时返回错误。
func (service *Service) Shutdown() error {
	service.mu.Lock()
	s := service.scheduler
	service.scheduler = nil
	service.jobs = make(map[int]gocron.Job)
	service.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.Shutdown()
}

// RegisteredTaskIDs 返回当前已注册到 gocron 的任务 ID。
//
// 参数:
//   - service: 调度服务。
//
// 返回值:
//   - []int: 已注册任务 ID 列表。
func (service *Service) RegisteredTaskIDs() []int {
	service.mu.Lock()
	defer service.mu.Unlock()
	ids := make([]int, 0, len(service.jobs))
	for taskID := range service.jobs {
		ids = append(ids, taskID)
	}
	slices.Sort(ids)
	return ids
}

// ensureScheduler 惰性初始化 gocron scheduler。
//
// 参数:
//   - service: 调度服务。
//
// 返回值:
//   - error: 初始化失败时返回错误。
func (service *Service) ensureScheduler() error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.scheduler != nil {
		return nil
	}
	s, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}
	service.scheduler = s
	return nil
}

// registerCronTask 注册单个 cron 任务。
//
// 参数:
//   - service: 调度服务。
//   - taskRecord: 任务定义。
//
// 返回值:
//   - error: cron 校验或 gocron 注册失败时返回错误。
func (service *Service) registerCronTask(taskRecord *ent.Task) error {
	validation, err := ValidateCron(CronValidationInput{
		Expression:      taskRecord.CronExpression,
		Timezone:        taskRecord.Timezone,
		ConfirmWarnings: true,
	})
	if err != nil {
		return err
	}
	expression := taskRecord.CronExpression
	if taskRecord.Timezone != "" && taskRecord.Timezone != "Local" {
		expression = "CRON_TZ=" + taskRecord.Timezone + " " + expression
	}
	job, err := service.scheduler.NewJob(
		gocron.CronJob(expression, validation.WithSeconds),
		gocron.NewTask(func(ctx context.Context) {
			_, _ = service.triggerTaskWithSource(ctx, taskRecord.ID, entrun.TriggerCron)
		}),
		gocron.WithName(taskRecord.Name),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.BeforeJobRunsSkipIfBeforeFuncErrors(func(_ uuid.UUID, _ string) error {
				if service.IsTaskRunning(taskRecord.ID) {
					_, _ = service.recordSkipped(context.Background(), taskRecord.ID, entrun.TriggerCron)
					return ErrTaskAlreadyRunning
				}
				return nil
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("register cron task: %w", err)
	}
	service.mu.Lock()
	service.jobs[taskRecord.ID] = job
	service.mu.Unlock()
	return nil
}

// reconcileRegisteredCronTask 根据任务启停状态同步进程内 cron job。
//
// 参数:
//   - service: 调度服务。
//   - taskRecord: 已持久化的任务定义。
//
// 返回值:
//   - error: 移除或注册 cron job 失败时返回错误。
func (service *Service) reconcileRegisteredCronTask(taskRecord *ent.Task) error {
	service.mu.Lock()
	schedulerReady := service.scheduler != nil
	service.mu.Unlock()
	if !schedulerReady {
		return nil
	}
	if err := service.removeRegisteredCronTask(taskRecord.ID); err != nil {
		return err
	}
	if !taskRecord.Enabled || strings.TrimSpace(taskRecord.CronExpression) == "" {
		return nil
	}
	return service.registerCronTask(taskRecord)
}

// removeRegisteredCronTask 移除指定任务已注册的 cron job。
//
// 参数:
//   - service: 调度服务。
//   - taskID: 任务 ID。
//
// 返回值:
//   - error: gocron 移除失败时返回错误；未注册视为成功。
func (service *Service) removeRegisteredCronTask(taskID int) error {
	service.mu.Lock()
	schedulerRef := service.scheduler
	job, ok := service.jobs[taskID]
	if ok {
		delete(service.jobs, taskID)
	}
	service.mu.Unlock()
	if !ok || schedulerRef == nil {
		return nil
	}
	if err := schedulerRef.RemoveJob(job.ID()); err != nil && !errors.Is(err, gocron.ErrJobNotFound) {
		return fmt.Errorf("remove cron task: %w", err)
	}
	return nil
}
