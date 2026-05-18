package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var projectConfigFilenames = []string{
	"taskdaemon.yaml",
	"taskdaemon.yml",
	"config.yaml",
	"config.yml",
}

var projectLocalConfigFilenames = []string{
	"taskdaemon.local.yaml",
	"taskdaemon.local.yml",
	"config.local.yaml",
	"config.local.yml",
}

// DefaultPath 返回跨平台用户配置目录中的默认配置文件路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 默认配置文件路径。
//   - error: 读取用户配置目录失败时返回错误。
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "taskdaemon", "config.yaml"), nil
}

// ProjectPath 返回开发工作区当前工作目录下第一个存在的项目内配置文件路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 项目内配置文件路径；未找到时为空。
//   - bool: true 表示找到了项目内配置文件。
//   - error: 读取当前工作目录或检查文件状态失败时返回错误。
func ProjectPath() (string, bool, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("resolve working directory: %w", err)
	}
	enabled, err := projectConfigSearchEnabled(workingDir)
	if err != nil {
		return "", false, err
	}
	if !enabled {
		return "", false, nil
	}
	return projectPathInDir(workingDir)
}

// ResolvePaths 解析本次配置加载涉及的基础配置和本地覆盖配置路径。
//
// 参数:
//   - opts: 配置加载选项。
//
// 返回值:
//   - ResolvedPaths: 解析后的配置路径信息。
//   - error: 读取默认路径、工作目录或检查文件状态失败时返回错误。
func ResolvePaths(opts LoadOptions) (ResolvedPaths, error) {
	result := ResolvedPaths{
		ConfigPath:         opts.ConfigPath,
		ConfigOptional:     opts.Optional,
		ExplicitConfigPath: opts.ConfigPath != "",
	}
	if result.ConfigPath == "" {
		discoveredPath, found, err := ProjectPath()
		if err != nil {
			return ResolvedPaths{}, err
		}
		if found {
			result.ConfigPath = discoveredPath
		} else {
			defaultPath, err := DefaultPath()
			if err != nil {
				return ResolvedPaths{}, err
			}
			result.ConfigPath = defaultPath
		}
		result.ConfigOptional = true

		discoveredLocalPath, localFound, err := ProjectLocalPath()
		if err != nil {
			return ResolvedPaths{}, err
		}
		if localFound {
			result.LocalConfigPath = discoveredLocalPath
			result.LocalConfigExists = true
		}
	}

	exists, err := fileExists(result.ConfigPath)
	if err != nil {
		return ResolvedPaths{}, err
	}
	result.ConfigExists = exists
	return result, nil
}

// ProjectLocalPath 返回开发工作区当前工作目录下第一个存在的项目本地配置文件路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 项目本地配置文件路径；未找到时为空。
//   - bool: true 表示找到了项目本地配置文件。
//   - error: 读取当前工作目录或检查文件状态失败时返回错误。
func ProjectLocalPath() (string, bool, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("resolve working directory: %w", err)
	}
	enabled, err := projectConfigSearchEnabled(workingDir)
	if err != nil {
		return "", false, err
	}
	if !enabled {
		return "", false, nil
	}
	return projectLocalPathInDir(workingDir)
}

// Address 返回 HTTP server 使用的 host:port 地址。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: host:port 格式监听地址。
func (cfg ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

// projectPathInDir 返回指定目录中的第一个项目内配置文件路径。
//
// 参数:
//   - dir: 待搜索的目录。
//
// 返回值:
//   - string: 项目内配置文件路径；未找到时为空。
//   - bool: true 表示找到了项目内配置文件。
//   - error: 检查文件状态失败时返回错误。
func projectPathInDir(dir string) (string, bool, error) {
	for _, filename := range projectConfigFilenames {
		candidate := filepath.Join(dir, filename)
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, fmt.Errorf("stat project config file %s: %w", candidate, err)
		}
		if !info.IsDir() {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// projectLocalPathInDir 返回指定目录中的第一个项目本地配置文件路径。
//
// 参数:
//   - dir: 待搜索的目录。
//
// 返回值:
//   - string: 项目本地配置文件路径；未找到时为空。
//   - bool: true 表示找到了项目本地配置文件。
//   - error: 检查文件状态失败时返回错误。
func projectLocalPathInDir(dir string) (string, bool, error) {
	for _, filename := range projectLocalConfigFilenames {
		candidate := filepath.Join(dir, filename)
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, fmt.Errorf("stat project local config file %s: %w", candidate, err)
		}
		if !info.IsDir() {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// projectConfigSearchEnabled 判断当前工作目录是否属于开发工作区。
//
// 参数:
//   - dir: 当前工作目录。
//
// 返回值:
//   - bool: true 表示允许自动查找项目内配置文件。
//   - error: 检查工作区标记失败时返回错误。
func projectConfigSearchEnabled(dir string) (bool, error) {
	current := dir
	for {
		if existsFileOrDir(filepath.Join(current, "pnpm-workspace.yaml")) {
			return true, nil
		}
		if existsFileOrDir(filepath.Join(current, ".git")) {
			return true, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

// existsFileOrDir 判断路径是否存在。
//
// 参数:
//   - path: 待检查的文件或目录路径。
//
// 返回值:
//   - bool: true 表示路径存在。
func existsFileOrDir(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fileExists 判断文件路径是否存在。
//
// 参数:
//   - path: 待检查路径。
//
// 返回值:
//   - bool: true 表示路径存在且不是目录。
//   - error: 检查文件状态失败时返回错误。
func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat config file %s: %w", path, err)
	}
	return !info.IsDir(), nil
}
