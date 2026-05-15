package scheduler

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	entrun "taskdaemon/internal/data/ent/run"
	"taskdaemon/internal/runner"
)

// TestValidateCronAcceptsFiveAndSixFields 验证 cron 校验支持 5 字段与 6 字段表达式。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestValidateCronAcceptsFiveAndSixFields(t *testing.T) {
	five, err := ValidateCron(CronValidationInput{
		Expression: "30 9 * * *",
		Timezone:   "Asia/Hong_Kong",
	})
	require.NoError(t, err)
	require.Empty(t, five.Warnings)

	six, err := ValidateCron(CronValidationInput{
		Expression:      "30 9 * * * *",
		Timezone:        "Asia/Hong_Kong",
		ConfirmWarnings: true,
	})
	require.NoError(t, err)
	require.Contains(t, six.Warnings, WarningSecondLevelCron)
}

// TestValidateCronRequiresWarningConfirmation 验证秒级 cron 属于可确认软警告。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestValidateCronRequiresWarningConfirmation(t *testing.T) {
	_, err := ValidateCron(CronValidationInput{
		Expression: "*/5 * * * * *",
		Timezone:   "Asia/Hong_Kong",
	})

	require.ErrorIs(t, err, ErrCronWarningsNeedConfirmation)
}

// TestCreateTaskPersistsStructuredRunner 验证任务创建会保存 cron、timezone 和结构化 runner 配置。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCreateTaskPersistsStructuredRunner(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	enabled := false

	created, err := service.CreateTask(ctx, CreateTaskInput{
		Name:                "backup",
		Enabled:             &enabled,
		CronExpression:      "30 9 * * *",
		Timezone:            "Asia/Hong_Kong",
		ConfirmCronWarnings: true,
		Runner: runner.Config{
			Type:    runner.TypeShell,
			Inline:  shellPrintCommand("ok"),
			Timeout: time.Minute,
		},
	})

	require.NoError(t, err)
	require.NotZero(t, created.ID)
	taskRecord, err := store.Client().Task.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "backup", taskRecord.Name)
	require.False(t, taskRecord.Enabled)
	require.Equal(t, "30 9 * * *", taskRecord.CronExpression)
	require.Equal(t, 60, taskRecord.TimeoutSeconds)
	require.Equal(t, "shell", taskRecord.RunnerType.String())
	require.Equal(t, shellPrintCommand("ok"), taskRecord.RunnerConfig["inline"])
}

// TestCreateTaskPersistsDefaultTimeoutInRunnerConfig 验证默认 timeout 会同步写入任务字段和 runner JSON。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCreateTaskPersistsDefaultTimeoutInRunnerConfig(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})

	created, err := service.CreateTask(ctx, CreateTaskInput{
		Name:                "backup",
		CronExpression:      "30 9 * * *",
		Timezone:            "Asia/Hong_Kong",
		ConfirmCronWarnings: true,
		Runner: runner.Config{
			Type:   runner.TypeShell,
			Inline: shellPrintCommand("ok"),
		},
	})

	require.NoError(t, err)
	taskRecord, err := store.Client().Task.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, 3600, taskRecord.TimeoutSeconds)
	require.Equal(t, float64(3600), taskRecord.RunnerConfig["timeoutSeconds"])
}

// TestTriggerTaskRecordsSuccessfulRun 验证手动触发会执行 runner 并写入 success 历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestTriggerTaskRecordsSuccessfulRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:             runner.TypeShell,
		Inline:           shellPrintCommand("abcdef"),
		Timeout:          time.Second,
		OutputLimitBytes: 4,
	})

	runRecord, err := service.TriggerTask(ctx, taskID)

	require.NoError(t, err)
	require.Equal(t, entrun.StatusSuccess, runRecord.Status)
	require.Equal(t, "abcd", runRecord.Stdout)
	require.GreaterOrEqual(t, runRecord.DurationMs, int64(0))
	require.NotNil(t, runRecord.FinishedAt)
	require.NotNil(t, runRecord.ExitCode)
	require.Equal(t, 0, *runRecord.ExitCode)
}

// TestTriggerTaskRecordsTimeout 验证 runner 超时会写入 timeout 历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestTriggerTaskRecordsTimeout(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellSleepCommand(),
		Timeout: 20 * time.Millisecond,
	})

	runRecord, err := service.TriggerTask(ctx, taskID)

	require.NoError(t, err)
	require.Equal(t, entrun.StatusTimeout, runRecord.Status)
	require.Contains(t, runRecord.ErrorSummary, "timeout")
}

// TestCancelRunningTaskRecordsCancelled 验证取消运行中任务会终止进程并写入 cancelled 历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCancelRunningTaskRecordsCancelled(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellSleepCommand(),
		Timeout: time.Second,
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := service.TriggerTask(ctx, taskID)
		errCh <- err
	}()
	require.Eventually(t, func() bool {
		return service.IsTaskRunning(taskID)
	}, time.Second, time.Millisecond)

	require.NoError(t, service.CancelTask(ctx, taskID))
	require.NoError(t, <-errCh)

	runRecord, err := store.Client().Run.Query().Where(entrun.HasTaskWith()).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, entrun.StatusCancelled, runRecord.Status)
}

// TestOverlapTriggerRecordsSkipped 验证同一任务重叠触发会写入 skipped 历史且不启动新进程。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestOverlapTriggerRecordsSkipped(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellSleepCommand(),
		Timeout: time.Second,
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := service.TriggerTask(ctx, taskID)
		errCh <- err
	}()
	require.Eventually(t, func() bool {
		return service.IsTaskRunning(taskID)
	}, time.Second, time.Millisecond)

	skipped, err := service.TriggerTask(ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, entrun.StatusSkipped, skipped.Status)
	require.NoError(t, service.CancelTask(ctx, taskID))
	require.NoError(t, <-errCh)
}

// TestListTaskRunsReturnsNewestFirst 验证执行历史按 ID 倒序返回。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestListTaskRunsReturnsNewestFirst(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellPrintCommand("ok"),
		Timeout: time.Second,
	})
	first, err := store.Client().Run.Create().
		SetTaskID(taskID).
		SetTrigger(entrun.TriggerManual).
		SetStatus(entrun.StatusSuccess).
		Save(ctx)
	require.NoError(t, err)
	second, err := store.Client().Run.Create().
		SetTaskID(taskID).
		SetTrigger(entrun.TriggerManual).
		SetStatus(entrun.StatusFailed).
		Save(ctx)
	require.NoError(t, err)

	runs, err := service.ListTaskRuns(ctx, taskID, 50)

	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, second.ID, runs[0].ID)
	require.Equal(t, first.ID, runs[1].ID)
}

// TestRegisterEnabledTasksCreatesCronJobs 验证已启用 cron 任务可以注册到进程内 scheduler。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestRegisterEnabledTasksCreatesCronJobs(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	_ = createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellPrintCommand("ok"),
		Timeout: time.Second,
	})

	require.NoError(t, service.RegisterEnabledTasks(ctx))

	require.Len(t, service.RegisteredTaskIDs(), 1)
	require.NoError(t, service.Shutdown())
}

// openTestStore 创建已迁移的临时 SQLite Store。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - *data.Store: 已完成 migration 的测试数据层。
func openTestStore(t *testing.T) *data.Store {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "taskdaemon.db")
	store, err := data.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + dbPath + "?_fk=1",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})
	require.NoError(t, store.Migrate(ctx))
	return store
}

// createShellTask 创建测试用 shell 任务并返回任务 ID。
//
// 参数:
//   - t: Go 测试上下文。
//   - ctx: 控制数据库写入生命周期的 context。
//   - service: 调度服务。
//   - cfg: runner 配置。
//
// 返回值:
//   - int: 新建任务 ID。
func createShellTask(t *testing.T, ctx context.Context, service *Service, cfg runner.Config) int {
	t.Helper()
	created, err := service.CreateTask(ctx, CreateTaskInput{
		Name:                "script",
		CronExpression:      "30 9 * * *",
		Timezone:            "Asia/Hong_Kong",
		ConfirmCronWarnings: true,
		Runner:              cfg,
	})
	require.NoError(t, err)
	return created.ID
}

// shellPrintCommand 返回跨平台 stdout 输出命令。
//
// 参数:
//   - stdout: 要写入 stdout 的文本。
//
// 返回值:
//   - string: shell runner 可执行的命令片段。
func shellPrintCommand(stdout string) string {
	if runtime.GOOS == "windows" {
		return "echo " + stdout
	}
	return "printf '" + stdout + "'"
}

// shellSleepCommand 返回跨平台长时间运行命令。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: shell runner 可执行的命令片段。
func shellSleepCommand() string {
	if runtime.GOOS == "windows" {
		return "for /L %i in (1,1,100000000) do @rem"
	}
	return "sleep 1"
}
