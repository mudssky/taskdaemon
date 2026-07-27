package service

import (
	"fmt"
	"strings"
)

const (
	windowsServiceName        = "TaskDaemon"
	windowsServiceDisplayName = "taskdaemon scheduling service"
	// windowsUnitPath 是逻辑描述路径，Windows SCM 不落盘 unit 文件。
	windowsUnitPath = `SCM:\Services\TaskDaemon`
)

// renderWindows 生成 Windows SCM 服务描述（用于 dry-run 与文档）。
//
// 参数:
//   - resolved: 已解析的绝对路径规格。
//
// 返回值:
//   - Unit: 含逻辑路径与完整描述内容。
//   - error: 生成失败时返回错误。
func renderWindows(resolved ResolvedSpec) (Unit, error) {
	args := programArguments(resolved.Executable, resolved.ConfigPath)
	binPath := windowsBinPath(args)

	content := fmt.Sprintf(`# Windows Service Control Manager unit (logical)
ServiceName: %s
DisplayName: %s
Start: auto
BinPath: %s
WorkingDirectory: %s
# Recovery: restart on failure (configured at install time)
# Note: unit file is not written to disk; registration lives in SCM.
`, windowsServiceName, windowsServiceDisplayName, binPath, resolved.WorkingDir)

	return Unit{
		Path:             windowsUnitPath,
		Content:          content,
		Mode:             defaultUnitMode(),
		Label:            windowsServiceName,
		ProgramArguments: args,
	}, nil
}

// windowsBinPath 构造 sc.exe binPath= 所需字符串。
//
// 参数:
//   - args: ProgramArguments。
//
// 返回值:
//   - string: 带引号的二进制与参数串。
func windowsBinPath(args []string) string {
	if len(args) == 0 {
		return `""`
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, windowsQuote(arg))
	}
	return strings.Join(parts, " ")
}

// windowsQuote 为 Windows 命令行参数加引号。
//
// 参数:
//   - value: 原始参数。
//
// 返回值:
//   - string: 引号包裹后的参数。
func windowsQuote(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	escaped := strings.ReplaceAll(value, `"`, `\"`)
	return `"` + escaped + `"`
}

// windowsValidateScope Windows 仅支持系统级 SCM 服务。
//
// 参数:
//   - scope: 请求的作用域。
//
// 返回值:
//   - error: 非 system 时返回 SERVICE_INVALID_SCOPE。
func windowsValidateScope(scope Scope) error {
	if scope == ScopeSystem {
		return nil
	}
	return newError(
		CodeInvalidScope,
		"windows only supports system scope via SCM",
		"re-run as: taskdaemon service install --scope system\nadministrator privileges are required",
		nil,
	)
}
