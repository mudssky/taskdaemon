// Package transfer 提供 taskdaemon 业务表在 SQLite 与 PostgreSQL 之间的导出/导入/直连迁移。
package transfer

import (
	"errors"
	"fmt"
)

// 稳定错误码（CLI/日志检索用）。
const (
	CodeSchemaMismatch    = "DBXFER_SCHEMA_MISMATCH"
	CodeTargetNotEmpty    = "DBXFER_TARGET_NOT_EMPTY"
	CodeConnectFailed     = "DBXFER_CONNECT_FAILED"
	CodePermission        = "DBXFER_PERMISSION"
	CodeInvalidInput      = "DBXFER_INVALID_INPUT"
	CodeImportFailed      = "DBXFER_IMPORT_FAILED"
	CodeVerifyFailed      = "DBXFER_VERIFY_FAILED"
	CodeCheckpointInvalid = "DBXFER_CHECKPOINT_INVALID"
)

// Error 是带稳定 code 的迁移错误。
type Error struct {
	Code    string
	Message string
	Err     error
	Details map[string]any
}

// Error 实现 error。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 带 code 前缀的错误文案。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 返回底层错误。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 包装的底层错误。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// newError 构造迁移错误。
//
// 参数:
//   - code: 稳定错误码。
//   - message: 简短说明。
//   - err: 可选底层错误。
//
// 返回值:
//   - *Error: 迁移错误。
func newError(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// withDetails 附加结构化 details。
//
// 参数:
//   - details: 可安全展示的键值。
//
// 返回值:
//   - *Error: 同一错误实例。
func (e *Error) withDetails(details map[string]any) *Error {
	if e == nil {
		return nil
	}
	e.Details = details
	return e
}

// AsError 尝试将 error 转为 *Error。
//
// 参数:
//   - err: 任意错误。
//
// 返回值:
//   - *Error: 转换成功时的值。
//   - bool: 是否为迁移错误。
func AsError(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// AsErrorOrWrap 若已是 *Error 则原样返回，否则包装为指定 code。
//
// 参数:
//   - code: 稳定错误码。
//   - message: 简短说明。
//   - err: 底层错误。
//
// 返回值:
//   - error: 迁移错误。
func AsErrorOrWrap(code, message string, err error) error {
	if err == nil {
		return nil
	}
	if te, ok := AsError(err); ok {
		return te
	}
	return newError(code, message, err)
}
