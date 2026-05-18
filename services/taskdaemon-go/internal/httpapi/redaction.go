package httpapi

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

const redactedLogValue = "[REDACTED]"

// appendBodyLogAttrs 追加 body 日志字段。
//
// 参数:
//   - attrs: 已有日志属性。
//   - field: body 字段名前缀。
//   - body: body 片段与截断状态。
//   - redactFields: 需要脱敏的 JSON 字段名。
//
// 返回值:
//   - []any: 追加后的日志属性。
func appendBodyLogAttrs(attrs []any, field string, body loggedBody, redactFields []string) []any {
	attrs = append(attrs, field, sanitizedLogBody(body.content, redactFields))
	if body.truncated {
		attrs = append(attrs, field+"_truncated", true)
	}
	return attrs
}

// sanitizedLogBody 将 body 转为适合日志输出的脱敏值。
//
// 参数:
//   - body: body 字节片段。
//   - redactFields: 需要脱敏的 JSON 字段名。
//
// 返回值:
//   - any: JSON body 返回结构化对象，非 JSON body 返回字符串。
func sanitizedLogBody(body []byte, redactFields []string) any {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return redactedRawBody(string(body), redactFields)
	}
	redactJSONValue(decoded, redactFieldSet(redactFields))
	return decoded
}

// redactJSONValue 递归脱敏 JSON 对象中的敏感字段。
//
// 参数:
//   - value: JSON 解码后的值。
//   - fields: 需要脱敏的字段名集合。
//
// 返回值:
//   - 无。
func redactJSONValue(value any, fields map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if _, ok := fields[strings.ToLower(key)]; ok {
				typed[key] = redactedLogValue
				continue
			}
			redactJSONValue(nested, fields)
		}
	case []any:
		for _, item := range typed {
			redactJSONValue(item, fields)
		}
	}
}

// redactFieldSet 将脱敏字段配置转为大小写不敏感集合。
//
// 参数:
//   - fields: 脱敏字段名列表。
//
// 返回值:
//   - map[string]struct{}: 字段名集合。
func redactFieldSet(fields []string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field != "" {
			set[field] = struct{}{}
		}
	}
	return set
}

// redactedRawBody 对无法解析为 JSON 的 body 文本做保守脱敏。
//
// 参数:
//   - body: body 文本。
//   - fields: 需要脱敏的字段名列表。
//
// 返回值:
//   - string: 脱敏后的 body 文本。
func redactedRawBody(body string, fields []string) string {
	redacted := body
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		fieldPattern := regexp.QuoteMeta(field)
		jsonLikePattern := regexp.MustCompile(`(?i)(["']?` + fieldPattern + `["']?\s*:\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^,\s}\]]+)`)
		formLikePattern := regexp.MustCompile(`(?i)(` + fieldPattern + `\s*=\s*)[^&\s]+`)
		redacted = jsonLikePattern.ReplaceAllString(redacted, `${1}"`+redactedLogValue+`"`)
		redacted = formLikePattern.ReplaceAllString(redacted, `${1}`+redactedLogValue)
	}
	return redacted
}
