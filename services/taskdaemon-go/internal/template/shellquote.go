package template

import (
	"strings"
	"unicode"
)

// shellQuote 将 s 变为 POSIX 单引号字面量，防止 shell 元字符逃逸。
//
// 参数:
//   - s: 原始字符串。
//
// 返回值:
//   - string: 已引用字面量（含两侧单引号）。
//   - error: 含 NUL 时返回错误。
func shellQuote(s string) (string, error) {
	if strings.ContainsRune(s, 0) {
		return "", errNUL
	}
	// 'foo'bar' → 'foo'\''bar'
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'", nil
}

var errNUL = &FieldError{
	Path:   "params",
	Reason: "value must not contain NUL",
	Code:   CodeFieldInvalid,
}

// validatePathValue 校验路径参数：非空、无 NUL、无 ASCII 控制字符。
//
// 参数:
//   - path: 参数点分路径。
//   - value: 路径字符串。
//
// 返回值:
//   - *FieldError: 非法时返回字段错误。
func validatePathValue(path, value string) *FieldError {
	if value == "" {
		return &FieldError{Path: path, Reason: "path must not be empty", Code: CodeFieldInvalid}
	}
	if strings.ContainsRune(value, 0) {
		return &FieldError{Path: path, Reason: "path must not contain NUL", Code: CodeFieldInvalid}
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return &FieldError{Path: path, Reason: "path must not contain control characters", Code: CodeFieldInvalid}
		}
	}
	return nil
}

// validateSecretRef 校验凭据引用名为合法环境变量标识。
//
// 参数:
//   - path: 参数路径。
//   - value: 引用名。
//
// 返回值:
//   - *FieldError: 非法时返回字段错误。
func validateSecretRef(path, value string) *FieldError {
	if value == "" {
		return &FieldError{Path: path, Reason: "secret reference must not be empty", Code: CodeFieldInvalid}
	}
	for i, r := range value {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return &FieldError{Path: path, Reason: "secret reference must be a valid env name", Code: CodeFieldInvalid}
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return &FieldError{Path: path, Reason: "secret reference must be a valid env name", Code: CodeFieldInvalid}
		}
	}
	return nil
}

// joinCommand 用空格拼接已 quote 的 argv 片段。
//
// 参数:
//   - parts: 命令片段（应已安全处理）。
//
// 返回值:
//   - string: inline 命令。
func joinCommand(parts ...string) string {
	return strings.Join(parts, " ")
}
