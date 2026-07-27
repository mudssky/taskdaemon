package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	systemdUnitName   = "taskdaemon.service"
	systemdSystemDir  = "/etc/systemd/system"
	systemdUserSubdir = ".config/systemd/user"
)

// renderSystemd 生成 Linux systemd unit 文件内容。
//
// 参数:
//   - resolved: 已解析的绝对路径规格。
//   - unitPath: 目标 unit 路径。
//
// 返回值:
//   - Unit: 含路径与完整 unit 内容。
//   - error: 生成失败时返回错误。
func renderSystemd(resolved ResolvedSpec, unitPath string) (Unit, error) {
	args := programArguments(resolved.Executable, resolved.ConfigPath)
	// ExecStart 需要可执行文件 + 参数；路径含空格时加引号。
	execParts := make([]string, 0, len(args))
	for _, arg := range args {
		execParts = append(execParts, systemdQuote(arg))
	}
	execStart := strings.Join(execParts, " ")

	content := fmt.Sprintf(`[Unit]
Description=taskdaemon scheduling service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
WorkingDirectory=%s
Restart=on-failure
RestartSec=5
KillMode=mixed

[Install]
WantedBy=default.target
`, execStart, systemdQuote(resolved.WorkingDir))

	return Unit{
		Path:             unitPath,
		Content:          content,
		Mode:             defaultUnitMode(),
		Label:            systemdUnitName,
		ProgramArguments: args,
	}, nil
}

// systemdUnitPath 计算 user/system unit 路径。
//
// 参数:
//   - scope: 作用域。
//   - homeDir: 用户主目录（user 作用域需要）。
//
// 返回值:
//   - string: unit 绝对路径。
//   - error: 路径无效时返回错误。
func systemdUnitPath(scope Scope, homeDir string) (string, error) {
	switch scope {
	case ScopeUser:
		if homeDir == "" {
			return "", newError(CodeInvalidSpec, "user home is required for systemd user unit", "", nil)
		}
		return filepath.Join(homeDir, systemdUserSubdir, systemdUnitName), nil
	case ScopeSystem:
		return filepath.Join(systemdSystemDir, systemdUnitName), nil
	default:
		return "", newError(CodeInvalidScope, fmt.Sprintf("unsupported scope %q", scope), "", nil)
	}
}

// systemdQuote 在参数含空格或特殊字符时加双引号。
//
// 参数:
//   - value: 原始参数。
//
// 返回值:
//   - string: 适合写入 unit 的参数文本。
func systemdQuote(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"'\\") {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
