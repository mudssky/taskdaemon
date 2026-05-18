package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

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
		return newPrettyHandler(writer, opts)
	default:
		return newPrettyHandler(writer, opts)
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
