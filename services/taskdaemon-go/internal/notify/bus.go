package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"taskdaemon/internal/config"
)

// Bus 是通知事件总线：异步分发、按 sink 隔离失败与 panic。
// Publish 刻意不返回 error —— 通知失败不得改变调度器业务行为。
type Bus struct {
	mu           sync.RWMutex
	sinks        []*sinkRuntime
	cfg          config.NotifyConfig
	logger       *slog.Logger
	events       chan Event
	droppedCount int64
	wg           sync.WaitGroup
	stopOnce     sync.Once
	stopCh       chan struct{}
	started      bool
}

// BusOptions 控制 Bus 可选依赖。
type BusOptions struct {
	Logger *slog.Logger
}

// NewBus 创建通知事件总线。
//
// 参数:
//   - cfg: 通知配置；BufferSize 仅启动时生效。
//   - opts: 可选 logger。
//
// 返回值:
//   - *Bus: 未启动的总线实例。
func NewBus(cfg config.NotifyConfig, opts BusOptions) *Bus {
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = 256
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Bus{
		cfg:    cfg,
		logger: logger,
		events: make(chan Event, bufferSize),
		stopCh: make(chan struct{}),
	}
}

// Start 启动异步分发 goroutine。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (bus *Bus) Start() {
	if bus == nil {
		return
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if bus.started {
		return
	}
	bus.started = true
	bus.wg.Add(1)
	go bus.loop()
}

// Shutdown 关闭总线并尽量排空缓冲中的事件。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (bus *Bus) Shutdown() {
	if bus == nil {
		return
	}
	bus.stopOnce.Do(func() {
		close(bus.stopCh)
	})
	bus.wg.Wait()
}

// Register 注册 sink；同一总线可注册多个 sink。
//
// 参数:
//   - sink: 待注册的 sink；nil 会被忽略。
//
// 返回值:
//   - 无。
func (bus *Bus) Register(sink Sink) {
	if bus == nil || sink == nil {
		return
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.sinks = append(bus.sinks, &sinkRuntime{
		sink: sink,
		status: SinkStatus{
			Name:    sink.Name(),
			Enabled: sink.Enabled(),
		},
	})
}

// Publish 将事件投入缓冲通道；缓冲满时丢弃并计数打日志，不阻塞调用方。
//
// 参数:
//   - ctx: 调用方 context（当前仅用于保留 API 形态，分发使用独立生命周期）。
//   - event: 待投递事件。
//
// 返回值:
//   - 无。
func (bus *Bus) Publish(ctx context.Context, event Event) {
	if bus == nil {
		return
	}
	_ = ctx
	select {
	case bus.events <- event:
	default:
		bus.mu.Lock()
		bus.droppedCount++
		dropped := bus.droppedCount
		bus.mu.Unlock()
		bus.logger.Warn("notify bus buffer full, event dropped",
			"event_id", event.ID,
			"event_name", string(event.Name),
			"dropped_count", dropped,
		)
	}
}

// SinkStatuses 返回当前全部 sink 状态快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []SinkStatus: sink 状态列表。
func (bus *Bus) SinkStatuses() []SinkStatus {
	if bus == nil {
		return nil
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	out := make([]SinkStatus, 0, len(bus.sinks))
	for _, runtime := range bus.sinks {
		status := runtime.status
		status.Enabled = runtime.sink.Enabled()
		status.Name = runtime.sink.Name()
		out = append(out, status)
	}
	return out
}

// DroppedCount 返回因缓冲满而丢弃的事件数。
//
// 参数:
//   - 无。
//
// 返回值:
//   - int64: 丢弃计数。
func (bus *Bus) DroppedCount() int64 {
	if bus == nil {
		return 0
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return bus.droppedCount
}

// UpdateConfig 热更新可运行时变更的通知配置。
// BufferSize 需重启生效，本方法不会重建 channel。
//
// 参数:
//   - cfg: 新的通知配置快照。
//
// 返回值:
//   - 无。
func (bus *Bus) UpdateConfig(cfg config.NotifyConfig) {
	if bus == nil {
		return
	}
	bus.mu.Lock()
	// 保留启动时的 BufferSize 语义：热更新只覆盖 Store 等可热生效字段。
	cfg.BufferSize = bus.cfg.BufferSize
	bus.cfg = cfg
	bus.mu.Unlock()

	// 将 store 配置下推到已注册的可热更新 sink。
	bus.mu.RLock()
	sinks := append([]*sinkRuntime(nil), bus.sinks...)
	bus.mu.RUnlock()
	for _, runtime := range sinks {
		if updater, ok := runtime.sink.(configUpdater); ok {
			updater.UpdateConfig(cfg)
		}
	}
}

// Config 返回当前通知配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.NotifyConfig: 当前配置。
func (bus *Bus) Config() config.NotifyConfig {
	if bus == nil {
		return config.Default().Notify
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return bus.cfg
}

// configUpdater 是支持热更新的 sink 可选接口。
type configUpdater interface {
	UpdateConfig(cfg config.NotifyConfig)
}

// loop 消费事件并分发到各 sink。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (bus *Bus) loop() {
	defer bus.wg.Done()
	for {
		select {
		case <-bus.stopCh:
			bus.drain()
			return
		case event := <-bus.events:
			bus.dispatch(event)
		}
	}
}

// drain 在关闭时排空缓冲中的剩余事件。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (bus *Bus) drain() {
	for {
		select {
		case event := <-bus.events:
			bus.dispatch(event)
		default:
			return
		}
	}
}

// dispatch 将事件并发投递给所有 sink。
//
// 参数:
//   - event: 待分发事件。
//
// 返回值:
//   - 无。
func (bus *Bus) dispatch(event Event) {
	bus.mu.RLock()
	sinks := append([]*sinkRuntime(nil), bus.sinks...)
	bus.mu.RUnlock()

	var wg sync.WaitGroup
	for _, runtime := range sinks {
		if !runtime.sink.Enabled() {
			continue
		}
		wg.Add(1)
		go func(runtime *sinkRuntime) {
			defer wg.Done()
			bus.deliverOne(runtime, event)
		}(runtime)
	}
	wg.Wait()
}

// deliverOne 调用单个 sink，并 recover panic、更新状态。
//
// 参数:
//   - runtime: sink 运行态。
//   - event: 事件。
//
// 返回值:
//   - 无。
func (bus *Bus) deliverOne(runtime *sinkRuntime, event Event) {
	defer func() {
		if recovered := recover(); recovered != nil {
			bus.logger.Error("notify sink panicked",
				"sink", runtime.sink.Name(),
				"event_id", event.ID,
				"event_name", string(event.Name),
				"panic", recovered,
			)
			bus.recordFailure(runtime, "panic recovered")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := runtime.sink.Deliver(ctx, event); err != nil {
		bus.logger.Error("notify sink deliver failed",
			"sink", runtime.sink.Name(),
			"event_id", event.ID,
			"event_name", string(event.Name),
			"error", err,
		)
		bus.recordFailure(runtime, err.Error())
		return
	}
	bus.recordSuccess(runtime)
}

// recordSuccess 更新 sink 成功状态。
//
// 参数:
//   - runtime: sink 运行态。
//
// 返回值:
//   - 无。
func (bus *Bus) recordSuccess(runtime *sinkRuntime) {
	now := time.Now().UTC()
	bus.mu.Lock()
	defer bus.mu.Unlock()
	runtime.status.SuccessCount++
	runtime.status.LastSuccessAt = &now
	runtime.status.LastDeliveredAt = &now
	runtime.status.LastError = ""
	runtime.status.Enabled = runtime.sink.Enabled()
	runtime.status.Name = runtime.sink.Name()
}

// recordFailure 更新 sink 失败状态。
//
// 参数:
//   - runtime: sink 运行态。
//   - message: 失败原因摘要。
//
// 返回值:
//   - 无。
func (bus *Bus) recordFailure(runtime *sinkRuntime, message string) {
	now := time.Now().UTC()
	bus.mu.Lock()
	defer bus.mu.Unlock()
	runtime.status.FailureCount++
	runtime.status.LastFailureAt = &now
	runtime.status.LastDeliveredAt = &now
	runtime.status.LastError = message
	runtime.status.Enabled = runtime.sink.Enabled()
	runtime.status.Name = runtime.sink.Name()
}
