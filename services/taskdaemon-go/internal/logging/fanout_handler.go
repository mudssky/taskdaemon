package logging

import (
	"context"
	"log/slog"
)

type fanoutHandler struct {
	handlers []slog.Handler
}

// newFanoutHandler 创建把日志记录分发到多个 handler 的 handler。
//
// 参数:
//   - handlers: 目标 slog handler 列表。
//
// 返回值:
//   - slog.Handler: 分发 handler。
func newFanoutHandler(handlers ...slog.Handler) slog.Handler {
	return fanoutHandler{handlers: handlers}
}

// Enabled 判断至少一个子 handler 是否会处理该级别日志。
//
// 参数:
//   - ctx: 日志上下文。
//   - level: 日志级别。
//
// 返回值:
//   - bool: true 表示至少一个子 handler 启用该级别。
func (handler fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, child := range handler.handlers {
		if child.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle 将日志记录写入所有启用该级别的子 handler。
//
// 参数:
//   - ctx: 日志上下文。
//   - record: 日志记录。
//
// 返回值:
//   - error: 任一子 handler 写入失败时返回错误。
func (handler fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, child := range handler.handlers {
		if !child.Enabled(ctx, record.Level) {
			continue
		}
		if err := child.Handle(ctx, record.Clone()); err != nil {
			return err
		}
	}
	return nil
}

// WithAttrs 返回追加属性后的分发 handler。
//
// 参数:
//   - attrs: 需要附加到后续日志的属性。
//
// 返回值:
//   - slog.Handler: 带属性的分发 handler。
func (handler fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	children := make([]slog.Handler, 0, len(handler.handlers))
	for _, child := range handler.handlers {
		children = append(children, child.WithAttrs(attrs))
	}
	return fanoutHandler{handlers: children}
}

// WithGroup 返回追加分组后的分发 handler。
//
// 参数:
//   - name: 日志属性分组名称。
//
// 返回值:
//   - slog.Handler: 带分组的分发 handler。
func (handler fanoutHandler) WithGroup(name string) slog.Handler {
	children := make([]slog.Handler, 0, len(handler.handlers))
	for _, child := range handler.handlers {
		children = append(children, child.WithGroup(name))
	}
	return fanoutHandler{handlers: children}
}
