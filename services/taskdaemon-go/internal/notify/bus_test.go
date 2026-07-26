package notify

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

type stubSink struct {
	name      string
	enabled   atomic.Bool
	delay     time.Duration
	err       error
	panicOn   bool
	delivered atomic.Int64
	mu        sync.Mutex
	events    []Event
}

func newStubSink(name string) *stubSink {
	sink := &stubSink{name: name}
	sink.enabled.Store(true)
	return sink
}

func (sink *stubSink) Name() string { return sink.name }

func (sink *stubSink) Enabled() bool { return sink.enabled.Load() }

func (sink *stubSink) Deliver(ctx context.Context, event Event) error {
	if sink.delay > 0 {
		select {
		case <-time.After(sink.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if sink.panicOn {
		panic("boom")
	}
	if sink.err != nil {
		return sink.err
	}
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
	sink.delivered.Add(1)
	return nil
}

func testEvent(id string) Event {
	return Event{
		ID:         id,
		Name:       NameTaskRunSucceeded,
		Severity:   SeverityInfo,
		OccurredAt: time.Now().UTC(),
		Title:      "t",
		Body:       "b",
	}
}

// TestBusRegisterEmptySinkWithoutCodeChange 验证空 sink 注册无需改 Bus 代码。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusRegisterEmptySinkWithoutCodeChange(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	sink := newStubSink("empty")
	bus.Register(sink)
	bus.Publish(context.Background(), testEvent("e1"))
	require.Eventually(t, func() bool { return sink.delivered.Load() == 1 }, time.Second, 10*time.Millisecond)
}

// TestBusSingleSinkErrorDoesNotAffectOthers 验证单 sink 失败不影响其他 sink。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusSingleSinkErrorDoesNotAffectOthers(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	failing := newStubSink("fail")
	failing.err = errors.New("deliver failed")
	ok := newStubSink("ok")
	bus.Register(failing)
	bus.Register(ok)

	bus.Publish(context.Background(), testEvent("e2"))
	require.Eventually(t, func() bool { return ok.delivered.Load() == 1 }, time.Second, 10*time.Millisecond)

	statuses := bus.SinkStatuses()
	require.Len(t, statuses, 2)
	var failStatus, okStatus SinkStatus
	for _, status := range statuses {
		switch status.Name {
		case "fail":
			failStatus = status
		case "ok":
			okStatus = status
		}
	}
	require.Equal(t, int64(1), failStatus.FailureCount)
	require.NotEmpty(t, failStatus.LastError)
	require.Equal(t, int64(1), okStatus.SuccessCount)
}

// TestBusSingleSinkPanicDoesNotCrash 验证单 sink panic 不影响其他 sink。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusSingleSinkPanicDoesNotCrash(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	panicking := newStubSink("panic")
	panicking.panicOn = true
	ok := newStubSink("ok")
	bus.Register(panicking)
	bus.Register(ok)

	bus.Publish(context.Background(), testEvent("e3"))
	require.Eventually(t, func() bool { return ok.delivered.Load() == 1 }, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		for _, status := range bus.SinkStatuses() {
			if status.Name == "panic" {
				return status.FailureCount >= 1
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

// TestBusPublishDoesNotBlock 验证慢 sink 时 Publish 不阻塞调用方。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusPublishDoesNotBlock(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	slow := newStubSink("slow")
	slow.delay = 300 * time.Millisecond
	bus.Register(slow)

	started := time.Now()
	bus.Publish(context.Background(), testEvent("e4"))
	require.Less(t, time.Since(started), 100*time.Millisecond)
	require.Eventually(t, func() bool { return slow.delivered.Load() == 1 }, time.Second, 20*time.Millisecond)
}

// TestBusDropsWhenBufferFull 验证缓冲满时丢弃并计数。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusDropsWhenBufferFull(t *testing.T) {
	cfg := config.Default().Notify
	cfg.BufferSize = 1
	// 不 Start，让 channel 无法被消费，从而快速填满。
	bus := NewBus(cfg, BusOptions{})

	bus.Publish(context.Background(), testEvent("keep"))
	bus.Publish(context.Background(), testEvent("drop-1"))
	bus.Publish(context.Background(), testEvent("drop-2"))
	require.Equal(t, int64(2), bus.DroppedCount())
}

// TestBusSkipsDisabledSink 验证 Enabled()==false 的 sink 不被调用。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusSkipsDisabledSink(t *testing.T) {
	bus := NewBus(config.Default().Notify, BusOptions{})
	bus.Start()
	t.Cleanup(bus.Shutdown)

	disabled := newStubSink("disabled")
	disabled.enabled.Store(false)
	enabled := newStubSink("enabled")
	bus.Register(disabled)
	bus.Register(enabled)

	bus.Publish(context.Background(), testEvent("e5"))
	require.Eventually(t, func() bool { return enabled.delivered.Load() == 1 }, time.Second, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, int64(0), disabled.delivered.Load())
}
