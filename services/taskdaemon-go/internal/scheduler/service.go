package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	enttask "taskdaemon/internal/data/ent/task"
	"taskdaemon/internal/runner"
)

const (
	// WarningSecondLevelCron 表示 cron 表达式启用了秒字段。
	WarningSecondLevelCron = "second_level_cron"
	// WarningHighFrequencyCron 表示 cron 表达式频率较高，需要用户显式确认。
	WarningHighFrequencyCron = "high_frequency_cron"
)

var (
	// ErrCronWarningsNeedConfirmation 表示 cron 校验只有软警告，但调用方尚未确认。
	ErrCronWarningsNeedConfirmation = errors.New("cron warnings need confirmation")
	// ErrTaskAlreadyRunning 表示同一任务已有执行正在运行。
	ErrTaskAlreadyRunning = errors.New("task already running")
	// ErrTaskNotRunning 表示取消目标任务时没有运行中的执行。
	ErrTaskNotRunning = errors.New("task is not running")
)

// CronValidationInput 是 cron 校验请求。
type CronValidationInput struct {
	Expression      string
	Timezone        string
	ConfirmWarnings bool
}

// CronValidationResult 是 cron 校验结果。
type CronValidationResult struct {
	Warnings    []string
	WithSeconds bool
}

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

// Options 控制调度服务的执行器和时间来源。
type Options struct {
	Runner runner.Executor
	Now    func() time.Time
}

// Service 提供任务定义、手动触发、取消和执行历史写入能力。
type Service struct {
	store     *data.Store
	runner    runner.Executor
	now       func() time.Time
	mu        sync.Mutex
	running   map[int]context.CancelFunc
	scheduler gocron.Scheduler
	jobs      map[int]gocron.Job
}

// ValidateCron 校验 cron 表达式和 timezone，并返回软警告。
//
// 参数:
//   - input: cron 表达式、timezone 和软警告确认标记。
//
// 返回值:
//   - CronValidationResult: 是否包含秒字段和软警告列表。
//   - error: cron/timezone 硬错误，或软警告未确认时返回错误。
func ValidateCron(input CronValidationInput) (CronValidationResult, error) {
	expression := strings.TrimSpace(input.Expression)
	if expression == "" {
		return CronValidationResult{}, fmt.Errorf("cron expression is required")
	}
	location, err := time.LoadLocation(normalizeTimezone(input.Timezone))
	if err != nil {
		return CronValidationResult{}, fmt.Errorf("load cron timezone: %w", err)
	}
	fields := strings.Fields(expression)
	withSeconds := len(fields) == 6
	if len(fields) != 5 && len(fields) != 6 {
		return CronValidationResult{}, fmt.Errorf("cron field count must be 5 or 6")
	}

	cron := gocron.NewDefaultCron(withSeconds)
	if err := cron.IsValid(expression, location, time.Now().In(location)); err != nil {
		return CronValidationResult{}, fmt.Errorf("validate cron: %w", err)
	}

	result := CronValidationResult{WithSeconds: withSeconds}
	if withSeconds {
		result.Warnings = append(result.Warnings, WarningSecondLevelCron)
	}
	if isHighFrequency(fields, withSeconds) {
		result.Warnings = append(result.Warnings, WarningHighFrequencyCron)
	}
	if len(result.Warnings) > 0 && !input.ConfirmWarnings {
		return result, ErrCronWarningsNeedConfirmation
	}
	return result, nil
}

// NewService 创建调度服务。
//
// 参数:
//   - store: 已完成 migration 的数据层实例。
//   - opts: runner 执行器和时间来源。
//
// 返回值:
//   - *Service: 可创建、触发和取消任务的调度服务。
func NewService(store *data.Store, opts Options) *Service {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{
		store:   store,
		runner:  opts.Runner,
		now:     opts.Now,
		running: make(map[int]context.CancelFunc),
		jobs:    make(map[int]gocron.Job),
	}
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

// normalizeTimezone 返回默认 timezone。
//
// 参数:
//   - timezone: 调用方提供的 timezone。
//
// 返回值:
//   - string: 标准 timezone 字符串。
func normalizeTimezone(timezone string) string {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return "Local"
	}
	return timezone
}

// isHighFrequency 判断 cron 是否可能高频执行。
//
// 参数:
//   - fields: cron 字段。
//   - withSeconds: 是否包含秒字段。
//
// 返回值:
//   - bool: true 表示需要软警告。
func isHighFrequency(fields []string, withSeconds bool) bool {
	if withSeconds && strings.HasPrefix(fields[0], "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(fields[0], "*/"))
		return err == nil && n > 0 && n < 60
	}
	return false
}

// runnerConfigToJSON 将 runner.Config 转换为可写入 Ent JSON 字段的 map。
//
// 参数:
//   - cfg: runner 配置。
//
// 返回值:
//   - map[string]any: JSON 友好的 runner 配置。
func runnerConfigToJSON(cfg runner.Config) map[string]any {
	return map[string]any{
		"type":             string(cfg.Type),
		"inline":           cfg.Inline,
		"scriptPath":       cfg.ScriptPath,
		"args":             cfg.Args,
		"workDir":          cfg.WorkDir,
		"env":              cfg.Env,
		"timeoutSeconds":   int(math.Ceil(cfg.Timeout.Seconds())),
		"outputLimitBytes": cfg.OutputLimitBytes,
	}
}

// withCronWarnings 在 runner JSON 中附加 cron 软警告，便于后续展示和排查。
//
// 参数:
//   - cfg: runner JSON 配置。
//   - warnings: cron 软警告列表。
//
// 返回值:
//   - map[string]any: 包含 warnings 的配置。
func withCronWarnings(cfg map[string]any, warnings []string) map[string]any {
	if len(warnings) > 0 {
		cfg["cronWarnings"] = warnings
	}
	return cfg
}

// runnerConfigFromTask 从任务定义恢复 runner.Config。
//
// 参数:
//   - taskRecord: Ent 任务记录。
//
// 返回值:
//   - runner.Config: runner 执行配置。
//   - error: 配置类型转换失败时返回错误。
func runnerConfigFromTask(taskRecord *ent.Task) (runner.Config, error) {
	cfg := runner.Config{
		Type:    runner.Type(taskRecord.RunnerType),
		Timeout: time.Duration(taskRecord.TimeoutSeconds) * time.Second,
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "inline"); ok {
		cfg.Inline = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "scriptPath"); ok {
		cfg.ScriptPath = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "workDir"); ok {
		cfg.WorkDir = value
	}
	if value, ok := intValue(taskRecord.RunnerConfig, "outputLimitBytes"); ok {
		cfg.OutputLimitBytes = value
	}
	if args, ok := stringSliceValue(taskRecord.RunnerConfig, "args"); ok {
		cfg.Args = args
	}
	if env, ok := stringMapValue(taskRecord.RunnerConfig, "env"); ok {
		cfg.Env = env
	}
	return cfg, nil
}

// stringValue 从 JSON map 中读取字符串。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - string: 字段值。
//   - bool: true 表示字段存在且类型正确。
func stringValue(values map[string]any, key string) (string, bool) {
	value, ok := values[key].(string)
	return value, ok
}

// intValue 从 JSON map 中读取整数。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - int: 字段值。
//   - bool: true 表示字段存在且类型可转换。
func intValue(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		return value, true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

// stringSliceValue 从 JSON map 中读取字符串切片。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - []string: 字段值。
//   - bool: true 表示字段存在且可转换。
func stringSliceValue(values map[string]any, key string) ([]string, bool) {
	raw, ok := values[key].([]any)
	if !ok {
		if typed, ok := values[key].([]string); ok {
			return typed, true
		}
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

// stringMapValue 从 JSON map 中读取字符串 map。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - map[string]string: 字段值。
//   - bool: true 表示字段存在且可转换。
func stringMapValue(values map[string]any, key string) (map[string]string, bool) {
	if typed, ok := values[key].(map[string]string); ok {
		return typed, true
	}
	raw, ok := values[key].(map[string]any)
	if !ok {
		return nil, false
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		out[key] = text
	}
	return out, true
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
