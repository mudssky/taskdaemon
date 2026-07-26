package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"taskdaemon/internal/config"
)

// DesktopSinkName 是 Desktop 原生通知 sink 的稳定名称。
const DesktopSinkName = "desktop"

// DesktopSender 抽象系统通知发送，便于测试与跨包注入。
// desktop 包在 Wails 壳启动时 Bind，退出时 Unbind。
type DesktopSender interface {
	// SendNotification 弹出系统原生通知。
	//
	// 参数:
	//   - ctx: 请求上下文。
	//   - title: 通知标题。
	//   - body: 通知正文。
	//   - id: 可选通知标识（用于平台去重/更新）。
	//
	// 返回值:
	//   - error: 权限不足或发送失败时返回错误。
	SendNotification(ctx context.Context, title, body, id string) error
}

// desktopSenderRegistry 保存进程内 Desktop 发送器（仅 Desktop 壳绑定）。
var desktopSenderRegistry struct {
	mu     sync.RWMutex
	sender DesktopSender
}

// BindDesktopSender 注册 Desktop 运行时发送器。
//
// 参数:
//   - sender: 发送实现；nil 等价于 Unbind。
//
// 返回值:
//   - 无。
func BindDesktopSender(sender DesktopSender) {
	desktopSenderRegistry.mu.Lock()
	desktopSenderRegistry.sender = sender
	desktopSenderRegistry.mu.Unlock()
}

// UnbindDesktopSender 清除 Desktop 发送器（纯 server / 壳退出时调用）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func UnbindDesktopSender() {
	desktopSenderRegistry.mu.Lock()
	desktopSenderRegistry.sender = nil
	desktopSenderRegistry.mu.Unlock()
}

// currentDesktopSender 返回当前绑定的发送器。
//
// 参数:
//   - 无。
//
// 返回值:
//   - DesktopSender: 已绑定发送器；未绑定返回 nil。
func currentDesktopSender() DesktopSender {
	desktopSenderRegistry.mu.RLock()
	defer desktopSenderRegistry.mu.RUnlock()
	return desktopSenderRegistry.sender
}

// DesktopSink 将 C-2 事件投递为系统原生通知。
// 仅在 Desktop 壳绑定 sender 且配置启用时生效；纯 server 自动禁用。
type DesktopSink struct {
	logger *slog.Logger
	// localSender 仅测试注入；生产路径走 BindDesktopSender。
	localSender DesktopSender

	mu  sync.RWMutex
	cfg config.NotifyDesktopConfig
}

// DesktopSinkOptions 控制 Desktop sink 可选依赖。
type DesktopSinkOptions struct {
	Logger *slog.Logger
	// Sender 覆盖进程级绑定，仅测试使用。
	Sender DesktopSender
}

// NewDesktopSink 创建 Desktop 原生通知 sink。
//
// 参数:
//   - cfg: Desktop sink 配置。
//   - opts: 可选 logger / 测试 sender。
//
// 返回值:
//   - *DesktopSink: sink 实例。
func NewDesktopSink(cfg config.NotifyDesktopConfig, opts DesktopSinkOptions) *DesktopSink {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &DesktopSink{
		logger:      logger,
		localSender: opts.Sender,
		cfg:         cfg,
	}
}

// Name 返回 sink 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 固定为 "desktop"。
func (sink *DesktopSink) Name() string {
	return DesktopSinkName
}

// Enabled 返回 Desktop sink 是否可投递。
// 配置关闭或未绑定 Desktop sender（纯 server）时返回 false。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 启用且存在 sender 时 true。
func (sink *DesktopSink) Enabled() bool {
	if sink == nil {
		return false
	}
	sink.mu.RLock()
	enabled := sink.cfg.Enabled
	sink.mu.RUnlock()
	if !enabled {
		return false
	}
	return sink.resolveSender() != nil
}

// UpdateConfig 热更新 Desktop sink 配置段。
//
// 参数:
//   - cfg: 完整通知配置。
//
// 返回值:
//   - 无。
func (sink *DesktopSink) UpdateConfig(cfg config.NotifyConfig) {
	if sink == nil {
		return
	}
	sink.mu.Lock()
	sink.cfg = cfg.Desktop
	sink.mu.Unlock()
}

// desktopConfig 返回当前配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.NotifyDesktopConfig: 当前配置。
func (sink *DesktopSink) desktopConfig() config.NotifyDesktopConfig {
	if sink == nil {
		return config.Default().Notify.Desktop
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg
}

// resolveSender 优先使用测试注入的 localSender，否则读进程绑定。
//
// 参数:
//   - 无。
//
// 返回值:
//   - DesktopSender: 可用发送器；无则 nil。
func (sink *DesktopSink) resolveSender() DesktopSender {
	if sink != nil && sink.localSender != nil {
		return sink.localSender
	}
	return currentDesktopSender()
}

// Deliver 按严重级别过滤后弹出系统通知。
//
// 参数:
//   - ctx: 请求上下文。
//   - event: C-2 事件。
//
// 返回值:
//   - error: 无 sender 或发送失败时返回错误；级别过滤跳过返回 nil。
func (sink *DesktopSink) Deliver(ctx context.Context, event Event) error {
	if sink == nil {
		return fmt.Errorf("desktop sink is nil")
	}
	cfg := sink.desktopConfig()
	if !cfg.Enabled {
		return nil
	}
	if !severityMeetsMinimum(event.Severity, cfg.MinSeverity) {
		return nil
	}
	sender := sink.resolveSender()
	if sender == nil {
		// 纯 server 或壳未就绪：不视为硬错误，避免污染失败计数。
		return nil
	}
	title := event.Title
	if title == "" {
		title = string(event.Name)
	}
	if err := sender.SendNotification(ctx, title, event.Body, event.ID); err != nil {
		return fmt.Errorf("desktop notify deliver failed: %w", err)
	}
	return nil
}
