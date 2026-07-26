package transfer

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// normalizeRow 将数据库扫描值转为可 JSON 序列化的 map。
//
// 参数:
//   - columns: 列名顺序。
//   - values: 与列对应的原始扫描值（*interface{} 解引用后）。
//
// 返回值:
//   - map[string]any: 规范化行。
//   - error: 转换失败。
func normalizeRow(columns []string, values []any) (map[string]any, error) {
	out := make(map[string]any, len(columns))
	for i, col := range columns {
		v, err := normalizeValue(values[i])
		if err != nil {
			return nil, fmt.Errorf("column %s: %w", col, err)
		}
		out[col] = v
	}
	return out, nil
}

// normalizeValue 规范化单个扫描值。
//
// 参数:
//   - v: 原始值。
//
// 返回值:
//   - any: JSON 友好值。
//   - error: 转换失败。
func normalizeValue(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano), nil
	case []byte:
		// 可能是 JSON 或文本
		if len(t) == 0 {
			return "", nil
		}
		if json.Valid(t) && (t[0] == '{' || t[0] == '[') {
			var decoded any
			if err := json.Unmarshal(t, &decoded); err == nil {
				return decoded, nil
			}
		}
		return string(t), nil
	case string:
		// 尝试 JSON 对象/数组
		if (strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}")) ||
			(strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")) {
			var decoded any
			if err := json.Unmarshal([]byte(t), &decoded); err == nil {
				return decoded, nil
			}
		}
		return t, nil
	case bool:
		return t, nil
	case int64:
		return t, nil
	case int32:
		return int64(t), nil
	case int:
		return int64(t), nil
	case float64:
		// JSON number
		return t, nil
	case float32:
		return float64(t), nil
	default:
		// 处理 sql.RawBytes 等
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil, nil
			}
			return normalizeValue(rv.Elem().Interface())
		}
		return v, nil
	}
}

// bindValue 将中间格式值转为可插入参数。
//
// 参数:
//   - driver: 方言。
//   - column: 列名。
//   - v: 中间格式值。
//
// 返回值:
//   - any: 绑定参数。
//   - error: 转换失败。
func bindValue(driver, column string, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case string:
		// 时间列尝试解析
		if isTimeColumn(column) {
			if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
				return parsed.UTC(), nil
			}
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				return parsed.UTC(), nil
			}
		}
		return t, nil
	case bool:
		if driver == "sqlite" {
			if t {
				return 1, nil
			}
			return 0, nil
		}
		return t, nil
	case float64:
		// JSON 数字默认 float64；整数列尽量转 int64
		if isIntColumn(column) {
			return int64(t), nil
		}
		return t, nil
	case json.Number:
		if isIntColumn(column) {
			i, err := t.Int64()
			if err != nil {
				return nil, err
			}
			return i, nil
		}
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return f, nil
	case map[string]any, []any:
		raw, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		// Postgres jsonb 可直接传 string/[]byte；SQLite 存 TEXT
		return string(raw), nil
	default:
		return t, nil
	}
}

// isTimeColumn 判断是否时间列。
//
// 参数:
//   - column: 列名。
//
// 返回值:
//   - bool: 是否时间列。
func isTimeColumn(column string) bool {
	switch column {
	case "created_at", "updated_at", "started_at", "finished_at", "expires_at", "played_at", "read_at", "occurred_at":
		return true
	default:
		return strings.HasSuffix(column, "_at")
	}
}

// isIntColumn 判断是否整型列。
//
// 参数:
//   - column: 列名。
//
// 返回值:
//   - bool: 是否整型。
func isIntColumn(column string) bool {
	switch column {
	case "id", "exit_code", "timeout_seconds", "admin_sessions", "task_runs":
		return true
	default:
		return strings.HasSuffix(column, "_id") ||
			strings.HasSuffix(column, "_ms") ||
			strings.HasSuffix(column, "_bytes") ||
			strings.HasSuffix(column, "_seconds")
	}
}

// scanDest 构造 Scan 目标切片。
//
// 参数:
//   - n: 列数。
//
// 返回值:
//   - []any: Scan 目标（均为 *any）。
//   - []any: 底层 any 容器。
func scanDest(n int) ([]any, []any) {
	raw := make([]any, n)
	dest := make([]any, n)
	for i := range raw {
		dest[i] = &raw[i]
	}
	return dest, raw
}

// rowEqual 比较两行规范化后的关键内容。
//
// 参数:
//   - a: 行 A。
//   - b: 行 B。
//
// 返回值:
//   - bool: 是否相等。
func rowEqual(a, b map[string]any) bool {
	ab, _ := json.Marshal(canonicalize(a))
	bb, _ := json.Marshal(canonicalize(b))
	return bytes.Equal(ab, bb)
}

// canonicalize 规范化 map 以便比较（时间字符串统一、数字统一）。
//
// 参数:
//   - m: 行 map。
//
// 返回值:
//   - map[string]any: 规范化副本。
func canonicalize(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = canonicalizeValue(v)
	}
	return out
}

// canonicalizeValue 规范化比较值。
//
// 参数:
//   - v: 任意值。
//
// 返回值:
//   - any: 规范化值。
func canonicalizeValue(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case float64:
		if t == float64(int64(t)) {
			return int64(t)
		}
		return t
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		return canonicalize(t)
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = canonicalizeValue(t[i])
		}
		return out
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
		return t
	default:
		return t
	}
}

// formatID 将 id 值格式化为稳定字符串。
//
// 参数:
//   - v: id 值。
//
// 返回值:
//   - string: 字符串形式。
func formatID(v any) string {
	switch t := v.(type) {
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// nullString 辅助。
type nullString = sql.NullString
