package runlog

import "errors"

// 稳定错误，供 API 层映射 RUNLOG_* 错误码。
var (
	// ErrRunNotFound 表示执行历史不存在。
	ErrRunNotFound = errors.New("runlog: run not found")
	// ErrArchiveNotFound 表示无可用归档（absent / pruned / 文件缺失）。
	ErrArchiveNotFound = errors.New("runlog: archive not found")
	// ErrInvalidRange 表示范围或尾部参数非法。
	ErrInvalidRange = errors.New("runlog: invalid range")
	// ErrReadFailed 表示打开或读取归档失败。
	ErrReadFailed = errors.New("runlog: read failed")
	// ErrInvalidRunID 表示 runID 非法。
	ErrInvalidRunID = errors.New("runlog: invalid run id")
	// ErrPathEscape 表示推导路径逃出归档根目录。
	ErrPathEscape = errors.New("runlog: path escapes archive root")
	// ErrDisabled 表示 runlog 功能已关闭。
	ErrDisabled = errors.New("runlog: disabled")
)
