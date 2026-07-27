package service

import (
	"errors"
	"fmt"
)

// 稳定 SERVICE_* 错误码（路线图 §9.1）。
const (
	CodePermissionDenied    = "SERVICE_INSTALL_PERMISSION_DENIED"
	CodeAlreadyInstalled    = "SERVICE_ALREADY_INSTALLED"
	CodeNotInstalled        = "SERVICE_NOT_INSTALLED"
	CodeUnsupportedPlatform = "SERVICE_UNSUPPORTED_PLATFORM"
	CodeInvalidScope        = "SERVICE_INVALID_SCOPE"
	CodeInvalidSpec         = "SERVICE_INVALID_SPEC"
	CodeInstallFailed       = "SERVICE_INSTALL_FAILED"
	CodeUninstallFailed     = "SERVICE_UNINSTALL_FAILED"
	CodeStatusFailed        = "SERVICE_STATUS_FAILED"
	CodeUnitWriteFailed     = "SERVICE_UNIT_WRITE_FAILED"
	CodeRegisterFailed      = "SERVICE_REGISTER_FAILED"
	CodeStartFailed         = "SERVICE_START_FAILED"
	CodeRollbackFailed      = "SERVICE_ROLLBACK_FAILED"
)

// Error 是服务安装器的稳定 typed error。
type Error struct {
	Code    string
	Message string
	Hint    string
	Err     error
}

// Error 实现 error 接口。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 含错误码与可选补救提示的消息。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := fmt.Sprintf("%s: %s", e.Code, e.Message)
	if e.Hint != "" {
		msg = msg + "\n" + e.Hint
	}
	if e.Err != nil {
		msg = msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap 返回被包装的底层错误。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 底层错误；无则 nil。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is 支持按错误码比较 SERVICE_* 错误。
//
// 参数:
//   - target: 目标错误。
//
// 返回值:
//   - bool: 当 target 为同 Code 的 *Error 时为 true。
func (e *Error) Is(target error) bool {
	var other *Error
	if !errors.As(target, &other) {
		return false
	}
	return e != nil && other != nil && e.Code == other.Code
}

// newError 构造带稳定错误码的 Error。
//
// 参数:
//   - code: SERVICE_* 错误码。
//   - message: 简短技术说明。
//   - hint: 可执行补救指引；可为空。
//   - err: 可选底层错误。
//
// 返回值:
//   - *Error: 新错误实例。
func newError(code, message, hint string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Hint:    hint,
		Err:     err,
	}
}
