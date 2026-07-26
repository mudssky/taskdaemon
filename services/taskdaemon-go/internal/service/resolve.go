package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"taskdaemon/internal/config"
)

// resolveSpec 将用户输入解析为绝对路径规格。
//
// 参数:
//   - spec: 原始安装规格。
//   - defaultScope: 平台默认作用域。
//
// 返回值:
//   - ResolvedSpec: 全部为绝对路径的规格。
//   - error: 路径解析失败时返回 SERVICE_INVALID_SPEC。
func resolveSpec(spec Spec, defaultScope Scope) (ResolvedSpec, error) {
	scope := spec.Scope
	if scope == "" {
		scope = defaultScope
	}
	if scope != ScopeUser && scope != ScopeSystem {
		return ResolvedSpec{}, newError(
			CodeInvalidScope,
			fmt.Sprintf("unsupported scope %q", scope),
			"use --scope user or --scope system",
			nil,
		)
	}

	executable := strings.TrimSpace(spec.Executable)
	if executable == "" {
		path, err := os.Executable()
		if err != nil {
			return ResolvedSpec{}, newError(CodeInvalidSpec, "resolve executable path", "", err)
		}
		executable = path
	}
	executable, err := filepath.Abs(executable)
	if err != nil {
		return ResolvedSpec{}, newError(CodeInvalidSpec, "absolute executable path", "", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	configPath, err := resolveConfigPath(spec.ConfigPath)
	if err != nil {
		return ResolvedSpec{}, err
	}

	workingDir := strings.TrimSpace(spec.WorkingDir)
	if workingDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			workingDir = filepath.Dir(executable)
		} else {
			workingDir = home
		}
	}
	workingDir, err = filepath.Abs(workingDir)
	if err != nil {
		return ResolvedSpec{}, newError(CodeInvalidSpec, "absolute working directory", "", err)
	}

	return ResolvedSpec{
		Scope:      scope,
		Executable: executable,
		ConfigPath: configPath,
		WorkingDir: workingDir,
		Force:      spec.Force,
	}, nil
}

// resolveConfigPath 解析服务单元应写入的配置绝对路径。
//
// 规则与 CLI 一致：显式 --config 只使用该文件；否则复用 config.ResolvePaths。
//
// 参数:
//   - explicit: 用户传入的 --config；可为空。
//
// 返回值:
//   - string: 绝对配置路径。
//   - error: 解析失败时返回错误。
func resolveConfigPath(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", newError(CodeInvalidSpec, "absolute config path", "", err)
		}
		return abs, nil
	}

	paths, err := config.ResolvePaths(config.LoadOptions{Optional: true})
	if err != nil {
		return "", newError(CodeInvalidSpec, "resolve config paths", "", err)
	}
	if paths.ConfigPath == "" {
		return "", newError(CodeInvalidSpec, "config path is empty after resolve", "", nil)
	}
	abs, err := filepath.Abs(paths.ConfigPath)
	if err != nil {
		return "", newError(CodeInvalidSpec, "absolute resolved config path", "", err)
	}
	return abs, nil
}

// programArguments 构造服务启动参数：serve --config <abs>。
//
// 参数:
//   - executable: 可执行文件绝对路径。
//   - configPath: 配置文件绝对路径。
//
// 返回值:
//   - []string: ProgramArguments 列表。
func programArguments(executable, configPath string) []string {
	return []string{executable, "serve", "--config", configPath}
}

// defaultUnitMode 返回单元文件默认权限 0644。
//
// 参数:
//   - 无。
//
// 返回值:
//   - os.FileMode: 0644。
func defaultUnitMode() os.FileMode {
	return 0o644
}
