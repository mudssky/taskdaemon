package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	launchdLabel      = "com.taskdaemon.daemon"
	launchdPlistName  = "com.taskdaemon.daemon.plist"
	launchdSystemDir  = "/Library/LaunchDaemons"
	launchdUserSubdir = "Library/LaunchAgents"
)

// renderLaunchd 生成 macOS launchd plist 单元。
//
// 参数:
//   - resolved: 已解析的绝对路径规格。
//   - unitPath: 目标 plist 路径。
//
// 返回值:
//   - Unit: 含路径与完整 plist 内容。
//   - error: 生成失败时返回错误。
func renderLaunchd(resolved ResolvedSpec, unitPath string) (Unit, error) {
	args := programArguments(resolved.Executable, resolved.ConfigPath)
	var argLines []string
	for _, arg := range args {
		argLines = append(argLines, fmt.Sprintf("\t\t<string>%s</string>", xmlEscape(arg)))
	}

	// 日志落到用户/系统 Library/Logs，单元本身不含任何 token。
	logDir := filepath.Join(filepath.Dir(filepath.Dir(unitPath)), "Logs", "taskdaemon")
	if resolved.Scope == ScopeSystem {
		logDir = "/Library/Logs/taskdaemon"
	} else {
		home, err := homeDirFromUnitPath(unitPath)
		if err == nil {
			logDir = filepath.Join(home, "Library", "Logs", "taskdaemon")
		}
	}

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, launchdLabel, strings.Join(argLines, "\n"), xmlEscape(resolved.WorkingDir),
		xmlEscape(filepath.Join(logDir, "stdout.log")),
		xmlEscape(filepath.Join(logDir, "stderr.log")),
	)

	return Unit{
		Path:             unitPath,
		Content:          content,
		Mode:             defaultUnitMode(),
		Label:            launchdLabel,
		ProgramArguments: args,
	}, nil
}

// launchdUnitPath 计算 LaunchAgent/Daemon plist 路径。
//
// 参数:
//   - scope: 作用域。
//   - homeDir: 用户主目录（user 作用域需要）。
//
// 返回值:
//   - string: plist 绝对路径。
//   - error: 路径无效时返回错误。
func launchdUnitPath(scope Scope, homeDir string) (string, error) {
	switch scope {
	case ScopeUser:
		if homeDir == "" {
			return "", newError(CodeInvalidSpec, "user home is required for LaunchAgent", "", nil)
		}
		return filepath.Join(homeDir, launchdUserSubdir, launchdPlistName), nil
	case ScopeSystem:
		return filepath.Join(launchdSystemDir, launchdPlistName), nil
	default:
		return "", newError(CodeInvalidScope, fmt.Sprintf("unsupported scope %q", scope), "", nil)
	}
}

// homeDirFromUnitPath 从 LaunchAgent 路径反推 home。
//
// 参数:
//   - unitPath: plist 路径。
//
// 返回值:
//   - string: home 目录。
//   - error: 无法解析时返回错误。
func homeDirFromUnitPath(unitPath string) (string, error) {
	// ~/Library/LaunchAgents/com.taskdaemon.daemon.plist
	dir := filepath.Dir(unitPath) // LaunchAgents
	library := filepath.Dir(dir)  // Library
	home := filepath.Dir(library) // home
	if home == "" || home == string(filepath.Separator) {
		return "", fmt.Errorf("cannot derive home from %s", unitPath)
	}
	return home, nil
}

// xmlEscape 转义 plist XML 特殊字符。
//
// 参数:
//   - value: 原始字符串。
//
// 返回值:
//   - string: 转义后的字符串。
func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
