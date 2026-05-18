package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type prettyHandler struct {
	writer io.Writer
	opts   *slog.HandlerOptions
	attrs  []slog.Attr
	group  string
	mu     *sync.Mutex
}

// newPrettyHandler 创建面向控制台阅读的 slog handler。
//
// 参数:
//   - writer: 日志输出 writer。
//   - opts: slog handler 选项。
//
// 返回值:
//   - slog.Handler: 控制台友好 handler。
func newPrettyHandler(writer io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return prettyHandler{
		writer: writer,
		opts:   opts,
		mu:     &sync.Mutex{},
	}
}

// Enabled 判断该级别日志是否会输出。
//
// 参数:
//   - ctx: 日志上下文。
//   - level: 日志级别。
//
// 返回值:
//   - bool: true 表示该级别已启用。
func (handler prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if handler.opts == nil || handler.opts.Level == nil {
		return level >= slog.LevelInfo
	}
	return level >= handler.opts.Level.Level()
}

// Handle 输出控制台友好日志。
//
// 参数:
//   - ctx: 日志上下文。
//   - record: 日志记录。
//
// 返回值:
//   - error: 写入失败时返回错误。
func (handler prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs := make([]slog.Attr, 0, len(handler.attrs)+record.NumAttrs())
	attrs = append(attrs, handler.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	line := handler.formatRecord(record, attrs)
	handler.mu.Lock()
	defer handler.mu.Unlock()
	_, err := fmt.Fprintln(handler.writer, line)
	return err
}

// WithAttrs 返回追加属性后的 pretty handler。
//
// 参数:
//   - attrs: 需要追加的属性。
//
// 返回值:
//   - slog.Handler: 带属性的 handler。
func (handler prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := handler
	next.attrs = append(append([]slog.Attr{}, handler.attrs...), attrs...)
	return next
}

// WithGroup 返回追加分组后的 pretty handler。
//
// 参数:
//   - name: 分组名称。
//
// 返回值:
//   - slog.Handler: 带分组的 handler。
func (handler prettyHandler) WithGroup(name string) slog.Handler {
	next := handler
	if next.group == "" {
		next.group = name
	} else {
		next.group += "." + name
	}
	return next
}

// formatRecord 格式化单条日志。
//
// 参数:
//   - record: 日志记录。
//   - attrs: 日志属性。
//
// 返回值:
//   - string: 单行日志文本。
func (handler prettyHandler) formatRecord(record slog.Record, attrs []slog.Attr) string {
	level := prettyLevel(record.Level)
	timestamp := record.Time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	if record.Message == "http request" {
		return formatHTTPRequestLog(level, timestamp, attrs)
	}
	parts := []string{level, timestamp.Format("15:04:05"), record.Message}
	for _, attr := range attrs {
		parts = append(parts, formatAttr(handler.group, attr))
	}
	return strings.Join(parts, " ")
}
