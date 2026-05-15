package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"

	"taskdaemon/internal/config"
)

// Options 控制 logger 构造时使用的外部 writer。
type Options struct {
	ConsoleWriter io.Writer
}

// Logger 保存应用 logger 及其需要关闭的输出资源。
type Logger struct {
	*slog.Logger
	closers []io.Closer
}

// New 根据配置创建应用 logger。
//
// 参数:
//   - cfg: 日志配置。
//   - opts: logger 构造选项；ConsoleWriter 为空时写入 os.Stderr。
//
// 返回值:
//   - *Logger: 配置后的结构化 logger。
//   - error: 配置非法或文件日志不可写时返回错误。
func New(cfg config.LoggingConfig, opts Options) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	handlerOptions := &slog.HandlerOptions{Level: level}

	handlers := make([]slog.Handler, 0, 2)
	closers := make([]io.Closer, 0, 1)
	switch strings.ToLower(strings.TrimSpace(cfg.Output)) {
	case "console":
		handlers = append(handlers, newHandler(writerOrDefault(opts.ConsoleWriter, os.Stderr), cfg.Console.Format, handlerOptions))
	case "file":
		handler, closer, err := newFileHandler(cfg.File, handlerOptions)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, handler)
		closers = append(closers, closer)
	case "both":
		handlers = append(handlers, newHandler(writerOrDefault(opts.ConsoleWriter, os.Stderr), cfg.Console.Format, handlerOptions))
		handler, closer, err := newFileHandler(cfg.File, handlerOptions)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, handler)
		closers = append(closers, closer)
	default:
		return nil, fmt.Errorf("unsupported logging output %q", cfg.Output)
	}

	return &Logger{
		Logger:  slog.New(newFanoutHandler(handlers...)),
		closers: closers,
	}, nil
}

// Close 关闭 logger 持有的输出资源。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 任一输出资源关闭失败时返回错误。
func (logger *Logger) Close() error {
	if logger == nil {
		return nil
	}
	var closeErr error
	for _, closer := range logger.closers {
		if err := closer.Close(); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

// parseLevel 将配置中的日志级别转换为 slog.Leveler。
//
// 参数:
//   - value: 日志级别字符串。
//
// 返回值:
//   - slog.Leveler: 可传入 slog handler 的级别。
//   - error: 级别不支持时返回错误。
func parseLevel(value string) (slog.Leveler, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, fmt.Errorf("unsupported logging level %q", value)
	}
}

// newFileHandler 创建带 lumberjack 轮转 writer 的 slog handler。
//
// 参数:
//   - cfg: 文件日志配置。
//   - opts: slog handler 选项。
//
// 返回值:
//   - slog.Handler: 文件日志 handler。
//   - error: 文件路径为空、目录创建失败或文件不可写时返回错误。
func newFileHandler(cfg config.LoggingFileConfig, opts *slog.HandlerOptions) (slog.Handler, io.Closer, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("logging file path is required when file output is enabled")
	}
	if err := ensureWritableLogFile(path); err != nil {
		return nil, nil, err
	}
	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	}
	return newHandler(writer, cfg.Format, opts), writer, nil
}

// ensureWritableLogFile 确认日志文件目录存在并且目标文件可写。
//
// 参数:
//   - path: 日志文件路径。
//
// 返回值:
//   - error: 创建目录或打开文件失败时返回错误。
func ensureWritableLogFile(path string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create log directory %s: %w", dir, err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file %s: %w", path, err)
	}
	return nil
}

// newHandler 按格式创建 slog handler。
//
// 参数:
//   - writer: 日志输出 writer。
//   - format: 日志格式，支持 text 或 json。
//   - opts: slog handler 选项。
//
// 返回值:
//   - slog.Handler: 对应格式的 handler。
func newHandler(writer io.Writer, format string, opts *slog.HandlerOptions) slog.Handler {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.NewJSONHandler(writer, opts)
	case "", "text":
		return slog.NewTextHandler(writer, opts)
	default:
		return slog.NewTextHandler(writer, opts)
	}
}

// writerOrDefault 返回 writer 或默认 writer。
//
// 参数:
//   - writer: 调用方提供的 writer。
//   - fallback: writer 为空时使用的默认 writer。
//
// 返回值:
//   - io.Writer: 可用于日志输出的 writer。
func writerOrDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return fallback
}

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
