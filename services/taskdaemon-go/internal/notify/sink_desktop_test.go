package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

type stubDesktopSender struct {
	mu      sync.Mutex
	calls   []desktopSendCall
	err     error
	blocked chan struct{}
}

type desktopSendCall struct {
	title string
	body  string
	id    string
}

func (s *stubDesktopSender) SendNotification(_ context.Context, title, body, id string) error {
	if s.blocked != nil {
		<-s.blocked
	}
	s.mu.Lock()
	s.calls = append(s.calls, desktopSendCall{title: title, body: body, id: id})
	s.mu.Unlock()
	return s.err
}

func (s *stubDesktopSender) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// TestDesktopSinkDisabledWithoutSender 验证纯 server（无 sender）自动禁用。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkDisabledWithoutSender(t *testing.T) {
	t.Cleanup(UnbindDesktopSender)
	UnbindDesktopSender()

	sink := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{})
	require.False(t, sink.Enabled())
	require.NoError(t, sink.Deliver(context.Background(), testEvent("d1")))
}

// TestDesktopSinkDeliverRendersTitleBody 验证标题正文来自 C-2 事件。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkDeliverRendersTitleBody(t *testing.T) {
	sender := &stubDesktopSender{}
	sink := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{Sender: sender})
	require.True(t, sink.Enabled())

	event := Event{
		ID:       "evt-1",
		Name:     NameTaskRunFailed,
		Severity: SeverityError,
		Title:    "任务失败",
		Body:     "exit 1",
	}
	require.NoError(t, sink.Deliver(context.Background(), event))
	require.Equal(t, 1, sender.callCount())
	require.Equal(t, "任务失败", sender.calls[0].title)
	require.Equal(t, "exit 1", sender.calls[0].body)
	require.Equal(t, "evt-1", sender.calls[0].id)
}

// TestDesktopSinkSeverityFilter 验证最低严重级别过滤。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkSeverityFilter(t *testing.T) {
	sender := &stubDesktopSender{}
	sink := NewDesktopSink(config.NotifyDesktopConfig{
		Enabled:     true,
		MinSeverity: "error",
	}, DesktopSinkOptions{Sender: sender})

	info := testEvent("info-1")
	info.Severity = SeverityInfo
	require.NoError(t, sink.Deliver(context.Background(), info))
	require.Equal(t, 0, sender.callCount())

	errEvt := testEvent("err-1")
	errEvt.Severity = SeverityError
	errEvt.Title = "e"
	require.NoError(t, sink.Deliver(context.Background(), errEvt))
	require.Equal(t, 1, sender.callCount())
}

// TestDesktopSinkConfigToggle 验证热更新开关。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkConfigToggle(t *testing.T) {
	sender := &stubDesktopSender{}
	sink := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{Sender: sender})
	require.True(t, sink.Enabled())

	cfg := config.Default().Notify
	cfg.Desktop.Enabled = false
	sink.UpdateConfig(cfg)
	require.False(t, sink.Enabled())
	require.NoError(t, sink.Deliver(context.Background(), testEvent("off")))
	require.Equal(t, 0, sender.callCount())
}

// TestDesktopSinkFailureDoesNotBlockOtherSinks 验证 desktop 失败不影响其他 sink。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkFailureDoesNotBlockOtherSinks(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	failingSender := &stubDesktopSender{err: errors.New("os denied")}
	desktop := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{Sender: failingSender})
	ok := newStubSink("ok")
	bus.Register(desktop)
	bus.Register(ok)

	bus.Publish(context.Background(), testEvent("iso-1"))
	require.Eventually(t, func() bool {
		return ok.delivered.Load() == 1
	}, time.Second, 10*time.Millisecond)

	var desktopFailed bool
	for _, status := range bus.SinkStatuses() {
		if status.Name == DesktopSinkName && status.FailureCount >= 1 {
			desktopFailed = true
		}
	}
	require.True(t, desktopFailed)
	require.Equal(t, int64(1), ok.delivered.Load())
}

// TestBindDesktopSenderProcessScope 验证进程级 Bind/Unbind。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestBindDesktopSenderProcessScope(t *testing.T) {
	t.Cleanup(UnbindDesktopSender)
	UnbindDesktopSender()

	sender := &stubDesktopSender{}
	BindDesktopSender(sender)
	sink := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{})
	require.True(t, sink.Enabled())
	require.NoError(t, sink.Deliver(context.Background(), testEvent("bound")))
	require.Equal(t, 1, sender.callCount())

	UnbindDesktopSender()
	require.False(t, sink.Enabled())
}

// TestDesktopSinkUsesEventNameWhenTitleEmpty 验证空标题回退事件名。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDesktopSinkUsesEventNameWhenTitleEmpty(t *testing.T) {
	sender := &stubDesktopSender{}
	sink := NewDesktopSink(config.NotifyDesktopConfig{Enabled: true}, DesktopSinkOptions{Sender: sender})
	event := testEvent("no-title")
	event.Title = ""
	require.NoError(t, sink.Deliver(context.Background(), event))
	require.Equal(t, string(NameTaskRunSucceeded), sender.calls[0].title)
}
