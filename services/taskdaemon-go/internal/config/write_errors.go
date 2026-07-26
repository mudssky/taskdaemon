package config

import (
	"errors"
	"fmt"
)

// 配置写入稳定错误码（C-1 / CONFIG_*）。
const (
	CodeSectionUnknown   = "CONFIG_SECTION_UNKNOWN"
	CodeInvalidJSON      = "CONFIG_INVALID_JSON"
	CodeFieldInvalid     = "CONFIG_FIELD_INVALID"
	CodeFieldOutOfRange  = "CONFIG_FIELD_OUT_OF_RANGE"
	CodeFieldConflict    = "CONFIG_FIELD_CONFLICT"
	CodeFieldNotWritable = "CONFIG_FIELD_NOT_WRITABLE"
	CodeValidationFailed = "CONFIG_VALIDATION_FAILED"
	CodePathUnavailable  = "CONFIG_PATH_UNAVAILABLE"
	CodeWriteFailed      = "CONFIG_WRITE_FAILED"
	CodeReloadFailed     = "CONFIG_RELOAD_FAILED"
)

// FieldError 描述单个字段级校验失败。
type FieldError struct {
	// Path 字段点分路径。
	Path string `json:"path"`
	// Reason 人类可读原因（英文技术短语）。
	Reason string `json:"reason"`
	// Code 字段级稳定码（CONFIG_FIELD_*）。
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

// ValidationError 聚合多个字段错误；校验失败时零写入。
type ValidationError struct {
	// Code 顶层稳定码。
	Code string
	// Message 默认说明。
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
		return "config validation failed"
	}
	if len(err.Fields) == 1 {
		return err.Fields[0].Error()
	}
	return fmt.Sprintf("%s: %d field errors", err.Code, len(err.Fields))
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
	var target *ValidationError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// WriteError 落盘/路径/重载类错误。
type WriteError struct {
	Code    string
	Message string
	Err     error
}

// Error 实现 error。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 错误文本。
func (err *WriteError) Error() string {
	if err == nil {
		return "config write failed"
	}
	if err.Err != nil {
		return fmt.Sprintf("%s: %v", err.Message, err.Err)
	}
	return err.Message
}

// Unwrap 返回底层错误。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 底层错误。
func (err *WriteError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

// AsWriteError 提取 WriteError。
//
// 参数:
//   - err: 任意错误。
//
// 返回值:
//   - *WriteError: 写入错误。
//   - bool: 是否匹配。
func AsWriteError(err error) (*WriteError, bool) {
	var target *WriteError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
