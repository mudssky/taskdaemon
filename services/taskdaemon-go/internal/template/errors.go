package template

import (
	"fmt"
	"strings"
)

// TEMPLATE_* 稳定错误码（T7a / 路线图 §9.1）。
const (
	CodeNotFound          = "TEMPLATE_NOT_FOUND"
	CodeInvalidJSON       = "TEMPLATE_INVALID_JSON"
	CodeFieldInvalid      = "TEMPLATE_FIELD_INVALID"
	CodeFieldRequired     = "TEMPLATE_FIELD_REQUIRED"
	CodeFieldOutOfRange   = "TEMPLATE_FIELD_OUT_OF_RANGE"
	CodeValidationFailed  = "TEMPLATE_VALIDATION_FAILED"
	CodeRunnerUnsupported = "TEMPLATE_RUNNER_UNSUPPORTED"
	CodeRenderFailed      = "TEMPLATE_RENDER_FAILED"
	CodeUnavailable       = "TEMPLATE_UNAVAILABLE"
	CodeRegisterRejected  = "TEMPLATE_REGISTER_REJECTED"
)

// FieldError 描述单个字段级校验失败；形状与 C-1 config.FieldError 一致。
type FieldError struct {
	// Path 字段点分路径，例如 params.host。
	Path string `json:"path"`
	// Reason 人类可读原因（英文技术短语）。
	Reason string `json:"reason"`
	// Code 字段级稳定码（TEMPLATE_FIELD_*）。
	Code string `json:"code"`
}

// Error 实现 error。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 错误文本。
func (err FieldError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", err.Path, err.Reason, err.Code)
}

// ValidationError 聚合多个字段错误。
type ValidationError struct {
	// Code 顶层稳定码。
	Code string
	// Message 简短说明。
	Message string
	// Fields 字段错误列表。
	Fields []FieldError
}

// Error 实现 error。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 错误文本。
func (err *ValidationError) Error() string {
	if err == nil {
		return "template validation failed"
	}
	if len(err.Fields) == 0 {
		return err.Message
	}
	parts := make([]string, 0, len(err.Fields))
	for _, field := range err.Fields {
		parts = append(parts, field.Error())
	}
	if err.Message == "" {
		return strings.Join(parts, "; ")
	}
	return err.Message + ": " + strings.Join(parts, "; ")
}

// AsValidationError 提取 ValidationError。
//
// 参数:
//   - err: 任意错误。
//
// 返回值:
//   - *ValidationError: 校验错误。
//   - bool: 是否匹配。
func AsValidationError(err error) (*ValidationError, bool) {
	if err == nil {
		return nil, false
	}
	ve, ok := err.(*ValidationError)
	return ve, ok
}

// DomainError 非字段聚合的领域错误（未找到、runner 不支持等）。
type DomainError struct {
	// Code 稳定码。
	Code string
	// Message 说明。
	Message string
	// Details 安全结构化细节。
	Details map[string]any
}

// Error 实现 error。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 错误文本。
func (err *DomainError) Error() string {
	if err == nil {
		return "template domain error"
	}
	if err.Message != "" {
		return err.Message
	}
	return err.Code
}

// AsDomainError 提取 DomainError。
//
// 参数:
//   - err: 任意错误。
//
// 返回值:
//   - *DomainError: 领域错误。
//   - bool: 是否匹配。
func AsDomainError(err error) (*DomainError, bool) {
	if err == nil {
		return nil, false
	}
	de, ok := err.(*DomainError)
	return de, ok
}

// validationErrorFromFields 由字段错误构造 ValidationError。
//
// 参数:
//   - fields: 字段错误列表。
//
// 返回值:
//   - *ValidationError: 聚合错误。
func validationErrorFromFields(fields []FieldError) *ValidationError {
	if len(fields) == 0 {
		return &ValidationError{Code: CodeValidationFailed, Message: "validation failed"}
	}
	code := fields[0].Code
	for _, field := range fields[1:] {
		if field.Code != code {
			code = CodeValidationFailed
			break
		}
	}
	return &ValidationError{
		Code:    code,
		Message: "Template parameter validation failed",
		Fields:  fields,
	}
}
