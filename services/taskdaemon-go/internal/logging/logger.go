package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// formatHTTPRequestLog 格式化 HTTP 请求日志。
//
// 参数:
//   - level: 日志级别文本。
//   - timestamp: 日志时间。
//   - attrs: 请求日志属性。
//
// 返回值:
//   - string: 单行 HTTP 请求日志。
func formatHTTPRequestLog(level string, timestamp time.Time, attrs []slog.Attr) string {
	fields := attrMap(attrs)
	parts := []string{
		level,
		timestamp.Format("15:04:05"),
		stringValue(fields["method"]),
		stringValue(fields["path"]),
		fmt.Sprintf("%d", intValue(fields["status"])),
		fmt.Sprintf("%dms", intValue(fields["duration_ms"])),
	}
	if traceID := stringValue(fields["trace_id"]); traceID != "" {
		parts = append(parts, "trace="+traceID)
	}
	if body, ok := fields["request_body"]; ok {
		parts = append(parts, "req="+prettyValue(body))
	}
	if truncated, ok := fields["request_body_truncated"]; ok && boolValue(truncated) {
		parts = append(parts, "req_truncated=true")
	}
	if body, ok := fields["response_body"]; ok {
		parts = append(parts, "res="+prettyValue(body))
	}
	if truncated, ok := fields["response_body_truncated"]; ok && boolValue(truncated) {
		parts = append(parts, "res_truncated=true")
	}
	if errValue := stringValue(fields["error"]); errValue != "" {
		parts = append(parts, "error="+errValue)
	}
	return strings.Join(parts, " ")
}

// attrMap 将属性列表转为 map。
//
// 参数:
//   - attrs: 日志属性。
//
// 返回值:
//   - map[string]slog.Value: 属性 map。
func attrMap(attrs []slog.Attr) map[string]slog.Value {
	values := make(map[string]slog.Value, len(attrs))
	for _, attr := range attrs {
		attr.Value = attr.Value.Resolve()
		values[attr.Key] = attr.Value
	}
	return values
}

// prettyLevel 返回固定宽度日志级别。
//
// 参数:
//   - level: slog 级别。
//
// 返回值:
//   - string: 固定宽度级别文本。
func prettyLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN "
	case level <= slog.LevelDebug:
		return "DEBUG"
	default:
		return "INFO "
	}
}

// formatAttr 格式化普通属性。
//
// 参数:
//   - group: 属性分组。
//   - attr: 日志属性。
//
// 返回值:
//   - string: key=value 文本。
func formatAttr(group string, attr slog.Attr) string {
	key := attr.Key
	if group != "" {
		key = group + "." + key
	}
	return key + "=" + prettyValue(attr.Value.Resolve())
}

// prettyValue 格式化日志属性值。
//
// 参数:
//   - value: slog 属性值。
//
// 返回值:
//   - string: 适合控制台展示的字符串。
func prettyValue(value slog.Value) string {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return quoteIfNeeded(value.String())
	case slog.KindInt64:
		return fmt.Sprintf("%d", value.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", value.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%g", value.Float64())
	case slog.KindBool:
		return fmt.Sprintf("%t", value.Bool())
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339)
	case slog.KindAny:
		return prettyAny(value.Any())
	case slog.KindGroup:
		return prettyAny(groupValue(value.Group()))
	default:
		return quoteIfNeeded(value.String())
	}
}

// prettyAny 格式化任意值。
//
// 参数:
//   - value: 任意日志值。
//
// 返回值:
//   - string: JSON 或字符串形式。
func prettyAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case error:
		return quoteIfNeeded(typed.Error())
	case string:
		return quoteIfNeeded(typed)
	default:
		content, err := json.Marshal(typed)
		if err != nil {
			return quoteIfNeeded(fmt.Sprint(typed))
		}
		return string(content)
	}
}

// groupValue 将 slog group 转为普通 map。
//
// 参数:
//   - attrs: group 属性。
//
// 返回值:
//   - map[string]any: 普通 map。
func groupValue(attrs []slog.Attr) map[string]any {
	values := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		values[attr.Key] = attr.Value.Resolve().Any()
	}
	return values
}

// stringValue 从 slog.Value 中读取字符串。
//
// 参数:
//   - value: slog 属性值。
//
// 返回值:
//   - string: 字符串值。
func stringValue(value slog.Value) string {
	value = value.Resolve()
	if value.Kind() == slog.KindString {
		return value.String()
	}
	if value.Any() == nil {
		return ""
	}
	return fmt.Sprint(value.Any())
}

// intValue 从 slog.Value 中读取整数。
//
// 参数:
//   - value: slog 属性值。
//
// 返回值:
//   - int64: 整数值，无法解析时为 0。
func intValue(value slog.Value) int64 {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return int64(value.Uint64())
	default:
		return 0
	}
}

// boolValue 从 slog.Value 中读取 bool。
//
// 参数:
//   - value: slog 属性值。
//
// 返回值:
//   - bool: bool 值，无法解析时为 false。
func boolValue(value slog.Value) bool {
	value = value.Resolve()
	return value.Kind() == slog.KindBool && value.Bool()
}

// quoteIfNeeded 在包含空白字符时为值加引号。
//
// 参数:
//   - value: 原始字符串。
//
// 返回值:
//   - string: 格式化后的字符串。
func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\r") {
		content, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%q", value)
		}
		return string(content)
	}
	return value
}
