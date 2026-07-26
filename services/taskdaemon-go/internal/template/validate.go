package template

import (
	"fmt"
	"math"
)

// applyDefaults 合并默认值与用户输入（用户优先）。
//
// 参数:
//   - def: 模板定义。
//   - input: 用户 params。
//
// 返回值:
//   - map[string]any: 合并后的值表。
func applyDefaults(def Definition, input map[string]any) map[string]any {
	out := make(map[string]any, len(def.Params))
	for _, p := range def.Params {
		if p.Default != nil {
			out[p.Name] = p.Default
		}
	}
	for k, v := range input {
		out[k] = v
	}
	return out
}

// validateParams 校验参数类型、必填、范围与安全约束。
//
// 参数:
//   - def: 模板定义。
//   - values: 已合并默认值的参数表。
//
// 返回值:
//   - map[string]any: 规范化后的参数（number 转为 float64 等）。
//   - error: *ValidationError 或 nil。
func validateParams(def Definition, values map[string]any) (map[string]any, error) {
	known := make(map[string]ParamDef, len(def.Params))
	for _, p := range def.Params {
		known[p.Name] = p
	}

	var fields []FieldError
	for key := range values {
		if _, ok := known[key]; !ok {
			fields = append(fields, FieldError{
				Path:   "params." + key,
				Reason: "unknown parameter",
				Code:   CodeFieldInvalid,
			})
		}
	}

	normalized := make(map[string]any, len(def.Params))
	for _, p := range def.Params {
		path := "params." + p.Name
		raw, ok := values[p.Name]
		if !ok || raw == nil {
			if p.Required {
				fields = append(fields, FieldError{
					Path:   path,
					Reason: "required",
					Code:   CodeFieldRequired,
				})
			}
			continue
		}

		switch p.Type {
		case ParamString:
			s, err := asString(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a string", Code: CodeFieldInvalid})
				continue
			}
			if s == "" && p.Required {
				fields = append(fields, FieldError{Path: path, Reason: "required", Code: CodeFieldRequired})
				continue
			}
			normalized[p.Name] = s
		case ParamNumber:
			n, err := asNumber(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a number", Code: CodeFieldInvalid})
				continue
			}
			if p.Min != nil && n < *p.Min {
				fields = append(fields, FieldError{Path: path, Reason: fmt.Sprintf("must be >= %v", *p.Min), Code: CodeFieldOutOfRange})
				continue
			}
			if p.Max != nil && n > *p.Max {
				fields = append(fields, FieldError{Path: path, Reason: fmt.Sprintf("must be <= %v", *p.Max), Code: CodeFieldOutOfRange})
				continue
			}
			normalized[p.Name] = n
		case ParamBoolean:
			b, err := asBool(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a boolean", Code: CodeFieldInvalid})
				continue
			}
			normalized[p.Name] = b
		case ParamEnum:
			s, err := asString(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a string enum value", Code: CodeFieldInvalid})
				continue
			}
			if !containsString(p.EnumOptions, s) {
				fields = append(fields, FieldError{Path: path, Reason: "value is not an allowed option", Code: CodeFieldInvalid})
				continue
			}
			normalized[p.Name] = s
		case ParamPath:
			s, err := asString(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a string path", Code: CodeFieldInvalid})
				continue
			}
			if fe := validatePathValue(path, s); fe != nil {
				fields = append(fields, *fe)
				continue
			}
			normalized[p.Name] = s
		case ParamSecretRef:
			s, err := asString(raw)
			if err != nil {
				fields = append(fields, FieldError{Path: path, Reason: "must be a string secret reference", Code: CodeFieldInvalid})
				continue
			}
			if fe := validateSecretRef(path, s); fe != nil {
				fields = append(fields, *fe)
				continue
			}
			normalized[p.Name] = s
		default:
			fields = append(fields, FieldError{Path: path, Reason: "unsupported param type in definition", Code: CodeFieldInvalid})
		}
	}

	if len(fields) > 0 {
		return nil, validationErrorFromFields(fields)
	}
	return normalized, nil
}

// asString 将 JSON 标量转为 string。
//
// 参数:
//   - raw: 任意值。
//
// 返回值:
//   - string: 字符串。
//   - error: 类型不匹配。
func asString(raw any) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("not a string")
	}
	return s, nil
}

// asNumber 将 JSON number 转为 float64。
//
// 参数:
//   - raw: 任意值。
//
// 返回值:
//   - float64: 数值。
//   - error: 类型不匹配。
func asNumber(raw any) (float64, error) {
	switch v := raw.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("invalid number")
		}
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case jsonNumber:
		return v.Float64()
	default:
		return 0, fmt.Errorf("not a number")
	}
}

// jsonNumber 兼容 encoding/json.Number（测试中较少出现）。
type jsonNumber interface {
	Float64() (float64, error)
}

// asBool 将 JSON bool 转为 bool。
//
// 参数:
//   - raw: 任意值。
//
// 返回值:
//   - bool: 布尔。
//   - error: 类型不匹配。
func asBool(raw any) (bool, error) {
	b, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("not a bool")
	}
	return b, nil
}

// containsString 判断切片是否包含 s。
//
// 参数:
//   - items: 字符串切片。
//   - s: 目标。
//
// 返回值:
//   - bool: 是否包含。
func containsString(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}

// stringValue 从规范化 map 取 string。
//
// 参数:
//   - values: 参数表。
//   - key: 键。
//
// 返回值:
//   - string: 值或空串。
func stringValue(values map[string]any, key string) string {
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	s, _ := raw.(string)
	return s
}

// boolValue 从规范化 map 取 bool。
//
// 参数:
//   - values: 参数表。
//   - key: 键。
//
// 返回值:
//   - bool: 值或 false。
func boolValue(values map[string]any, key string) bool {
	raw, ok := values[key]
	if !ok || raw == nil {
		return false
	}
	b, _ := raw.(bool)
	return b
}

// intValue 从规范化 map 取整数 int（截断）。
//
// 参数:
//   - values: 参数表。
//   - key: 键。
//   - fallback: 缺省。
//
// 返回值:
//   - int: 整数值。
func intValue(values map[string]any, key string, fallback int) int {
	raw, ok := values[key]
	if !ok || raw == nil {
		return fallback
	}
	n, ok := raw.(float64)
	if !ok {
		return fallback
	}
	return int(n)
}
