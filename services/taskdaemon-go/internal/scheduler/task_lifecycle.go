package scheduler

import (
	"context"
	"fmt"
	"math"
	"strings"

	entsql "entgo.io/ent/dialect/sql"

	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	enttask "taskdaemon/internal/data/ent/task"
	"taskdaemon/internal/runner"
)

// CreateTaskInput 保存创建任务需要的业务字段。
type CreateTaskInput struct {
	Name                string
	Description         string
	Enabled             *bool
	CronExpression      string
	Timezone            string
	ConfirmCronWarnings bool
	Runner              runner.Config
}

// taskValues 是通过业务校验后可直接落库的任务字段集合。
type taskValues struct {
	Name           string
	Description    string
	Enabled        bool
	CronExpression string
	Timezone       string
	RunnerType     enttask.RunnerType
	RunnerConfig   map[string]any
	TimeoutSeconds int
}

// CreateTask 校验并持久化任务定义。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - input: 任务定义输入。
//
// 返回值:
//   - *ent.Task: 已保存的任务定义。
//   - error: cron、runner 配置或数据库写入失败时返回错误。
func (service *Service) CreateTask(ctx context.Context, input CreateTaskInput) (*ent.Task, error) {
	values, err := validateTaskInput(input)
	if err != nil {
		return nil, err
	}
	now := service.now()
	taskRecord, err := service.store.Client().Task.Create().
		SetName(values.Name).
		SetDescription(values.Description).
		SetEnabled(values.Enabled).
		SetCronExpression(values.CronExpression).
		SetTimezone(values.Timezone).
		SetRunnerType(values.RunnerType).
		SetTimeoutSeconds(values.TimeoutSeconds).
		SetOverlapPolicy(enttask.OverlapPolicySkip).
		SetUpdatedAt(now).
		SetCreatedAt(now).
		SetRunnerConfig(values.RunnerConfig).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.reconcileRegisteredCronTask(taskRecord); err != nil {
		return nil, err
	}
	return taskRecord, nil
}

// ListTasks 查询任务定义列表。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库查询生命周期的 context。
//   - limit: 最大返回条数；小于等于 0 时使用默认 100。
//
// 返回值:
//   - []*ent.Task: 按 ID 倒序返回的任务定义列表。
//   - error: 数据库查询失败时返回错误。
func (service *Service) ListTasks(ctx context.Context, limit int) ([]*ent.Task, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return service.store.Client().Task.Query().
		Order(enttask.ByID(entsql.OrderDesc())).
		Limit(limit).
		All(ctx)
}

// UpdateTask 校验并更新任务定义。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - taskID: 任务 ID。
//   - input: 完整任务定义输入。
//
// 返回值:
//   - *ent.Task: 更新后的任务定义。
//   - error: cron、runner 配置、数据库写入或 scheduler 重注册失败时返回错误。
func (service *Service) UpdateTask(ctx context.Context, taskID int, input CreateTaskInput) (*ent.Task, error) {
	values, err := validateTaskInput(input)
	if err != nil {
		return nil, err
	}
	taskRecord, err := service.store.Client().Task.UpdateOneID(taskID).
		SetName(values.Name).
		SetDescription(values.Description).
		SetEnabled(values.Enabled).
		SetCronExpression(values.CronExpression).
		SetTimezone(values.Timezone).
		SetRunnerType(values.RunnerType).
		SetTimeoutSeconds(values.TimeoutSeconds).
		SetOverlapPolicy(enttask.OverlapPolicySkip).
		SetRunnerConfig(values.RunnerConfig).
		SetUpdatedAt(service.now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.reconcileRegisteredCronTask(taskRecord); err != nil {
		return nil, err
	}
	return taskRecord, nil
}

// SetTaskEnabled 更新任务启停状态，并同步进程内 cron 注册。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - taskID: 任务 ID。
//   - enabled: 是否启用任务。
//
// 返回值:
//   - *ent.Task: 更新后的任务定义。
//   - error: 数据库写入或 scheduler 重注册失败时返回错误。
func (service *Service) SetTaskEnabled(ctx context.Context, taskID int, enabled bool) (*ent.Task, error) {
	taskRecord, err := service.store.Client().Task.UpdateOneID(taskID).
		SetEnabled(enabled).
		SetUpdatedAt(service.now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.reconcileRegisteredCronTask(taskRecord); err != nil {
		return nil, err
	}
	return taskRecord, nil
}

// DeleteTask 删除任务定义、执行历史和进程内 cron 注册。
//
// 参数:
//   - service: 调度服务。
//   - ctx: 控制数据库写入生命周期的 context。
//   - taskID: 任务 ID。
//
// 返回值:
//   - error: 任务正在运行、cron job 移除失败或数据库删除失败时返回错误。
func (service *Service) DeleteTask(ctx context.Context, taskID int) error {
	if service.IsTaskRunning(taskID) {
		return ErrTaskAlreadyRunning
	}
	if err := service.removeRegisteredCronTask(taskID); err != nil {
		return err
	}
	if _, err := service.store.Client().Run.Delete().
		Where(entrun.HasTaskWith(enttask.IDEQ(taskID))).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete task runs: %w", err)
	}
	if err := service.store.Client().Task.DeleteOneID(taskID).Exec(ctx); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// validateTaskInput 校验任务输入并转换为可落库字段。
//
// 参数:
//   - input: 创建或更新任务时提交的完整任务定义。
//
// 返回值:
//   - taskValues: 已标准化的任务字段。
//   - error: cron 或 runner 配置非法时返回错误。
func validateTaskInput(input CreateTaskInput) (taskValues, error) {
	cronResult, err := ValidateCron(CronValidationInput{
		Expression:      input.CronExpression,
		Timezone:        input.Timezone,
		ConfirmWarnings: input.ConfirmCronWarnings,
	})
	if err != nil {
		return taskValues{}, err
	}
	if err := runner.Validate(input.Runner); err != nil {
		return taskValues{}, err
	}

	timeoutSeconds := int(math.Ceil(input.Runner.Timeout.Seconds()))
	if timeoutSeconds <= 0 {
		timeoutSeconds = 3600
	}
	runnerConfig := runnerConfigToJSON(input.Runner)
	runnerConfig["timeoutSeconds"] = timeoutSeconds
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	return taskValues{
		Name:           strings.TrimSpace(input.Name),
		Description:    input.Description,
		Enabled:        enabled,
		CronExpression: strings.TrimSpace(input.CronExpression),
		Timezone:       normalizeTimezone(input.Timezone),
		RunnerType:     enttask.RunnerType(input.Runner.Type),
		TimeoutSeconds: timeoutSeconds,
		RunnerConfig:   withCronWarnings(runnerConfig, cronResult.Warnings),
	}, nil
}
