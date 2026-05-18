package logging

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

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
