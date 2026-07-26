package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	entrun "taskdaemon/internal/data/ent/run"
	"taskdaemon/internal/notify"
	"taskdaemon/internal/runner"
)

// recordingPublisher 记录调度器发布的通知事件。
type recordingPublisher struct {
	mu     sync.Mutex
	events []notify.Event
}

// Publish 记录事件。
//
// 参数:
//   - _: context。
//   - event: 通知事件。
//
// 返回值:
//   - 无。
func (publisher *recordingPublisher) Publish(_ context.Context, event notify.Event) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = append(publisher.events, event)
}

// snapshot 返回事件快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []notify.Event: 已发布事件。
func (publisher *recordingPublisher) snapshot() []notify.Event {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return append([]notify.Event(nil), publisher.events...)
}

// lastByName 返回指定事件名的最后一条事件。
//
// 参数:
//   - name: 事件名。
//
// 返回值:
//   - notify.Event: 事件。
//   - bool: 是否找到。
func (publisher *recordingPublisher) lastByName(name notify.Name) (notify.Event, bool) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	for i := len(publisher.events) - 1; i >= 0; i-- {
		if publisher.events[i].Name == name {
			return publisher.events[i], true
		}
	}
	return notify.Event{}, false
}

// TestTriggerPublishesSucceededFailedTimeoutCancelled 验证四类终态产生正确事件。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestTriggerPublishesSucceededFailedTimeoutCancelled(t *testing.T) {
	ctx := context.Background()

	t.Run("succeeded", func(t *testing.T) {
		store := openTestStore(t)
		publisher := &recordingPublisher{}
		service := NewService(store, Options{Publisher: publisher})
		taskID := createShellTask(t, ctx, service, runner.Config{
			Type:    runner.TypeShell,
			Inline:  shellPrintCommand("password=should-not-appear-in-notify"),
			Timeout: time.Second,
		})
		runRecord, err := service.TriggerTask(ctx, taskID)
		require.NoError(t, err)
		require.Equal(t, entrun.StatusSuccess, runRecord.Status)

		event, ok := publisher.lastByName(notify.NameTaskRunSucceeded)
		require.True(t, ok)
		require.Equal(t, notify.TaskRunEventID(notify.NameTaskRunSucceeded, runRecord.ID), event.ID)
		require.NotContains(t, event.Title, "password")
		require.NotContains(t, event.Body, "password")
		require.NotContains(t, event.Body, "should-not-appear")
		require.Equal(t, float64(taskID), toFloat(event.Detail["taskId"]))
	})

	t.Run("failed", func(t *testing.T) {
		store := openTestStore(t)
		publisher := &recordingPublisher{}
		service := NewService(store, Options{Publisher: publisher})
		taskID := createShellTask(t, ctx, service, runner.Config{
			Type:    runner.TypeShell,
			Inline:  shellFailCommand(),
			Timeout: time.Second,
		})
		runRecord, err := service.TriggerTask(ctx, taskID)
		require.NoError(t, err)
		require.Equal(t, entrun.StatusFailed, runRecord.Status)

		event, ok := publisher.lastByName(notify.NameTaskRunFailed)
		require.True(t, ok)
		require.Equal(t, notify.TaskRunEventID(notify.NameTaskRunFailed, runRecord.ID), event.ID)
		// ErrorSummary 不应泄漏进通知正文
		require.NotContains(t, event.Body, runRecord.ErrorSummary)
	})

	t.Run("timeout", func(t *testing.T) {
		store := openTestStore(t)
		publisher := &recordingPublisher{}
		service := NewService(store, Options{Publisher: publisher})
		taskID := createShellTask(t, ctx, service, runner.Config{
			Type:    runner.TypeShell,
			Inline:  shellSleepCommand(),
			Timeout: 20 * time.Millisecond,
		})
		runRecord, err := service.TriggerTask(ctx, taskID)
		require.NoError(t, err)
		require.Equal(t, entrun.StatusTimeout, runRecord.Status)

		event, ok := publisher.lastByName(notify.NameTaskRunTimeout)
		require.True(t, ok)
		require.Equal(t, notify.TaskRunEventID(notify.NameTaskRunTimeout, runRecord.ID), event.ID)
	})

	t.Run("cancelled", func(t *testing.T) {
		store := openTestStore(t)
		publisher := &recordingPublisher{}
		service := NewService(store, Options{Publisher: publisher})
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
		require.Eventually(t, func() bool { return service.IsTaskRunning(taskID) }, time.Second, time.Millisecond)
		require.NoError(t, service.CancelTask(ctx, taskID))
		require.NoError(t, <-errCh)

		event, ok := publisher.lastByName(notify.NameTaskRunCancelled)
		require.True(t, ok)
		require.Equal(t, notify.NameTaskRunCancelled, event.Name)
	})
}

// TestTriggerWithoutPublisherStillWorks 验证无 Publisher 时调度器正常。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestTriggerWithoutPublisherStillWorks(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	service := NewService(store, Options{})
	taskID := createShellTask(t, ctx, service, runner.Config{
		Type:    runner.TypeShell,
		Inline:  shellPrintCommand("ok"),
		Timeout: time.Second,
	})
	runRecord, err := service.TriggerTask(ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, entrun.StatusSuccess, runRecord.Status)
}

// TestSkippedPublishesEvent 验证 overlap skipped 产生 skipped 事件。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestSkippedPublishesEvent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	publisher := &recordingPublisher{}
	service := NewService(store, Options{Publisher: publisher})
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
	require.Eventually(t, func() bool { return service.IsTaskRunning(taskID) }, time.Second, time.Millisecond)

	skipped, err := service.TriggerTask(ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, entrun.StatusSkipped, skipped.Status)
	require.NoError(t, service.CancelTask(ctx, taskID))
	require.NoError(t, <-errCh)

	event, ok := publisher.lastByName(notify.NameTaskRunSkipped)
	require.True(t, ok)
	require.Equal(t, notify.TaskRunEventID(notify.NameTaskRunSkipped, skipped.ID), event.ID)
}

// shellFailCommand 返回跨平台失败命令。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: shell 命令。
func shellFailCommand() string {
	// 使用 false / exit 1，避免在错误摘要里写敏感字面量
	if shellPrintCommand("x") == "echo x" {
		return "exit /b 1"
	}
	return "exit 1"
}

// toFloat 将 detail 中的数值转为 float64（JSON 数字解码形态）。
//
// 参数:
//   - value: detail 值。
//
// 返回值:
//   - float64: 数值。
func toFloat(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	default:
		return 0
	}
}
