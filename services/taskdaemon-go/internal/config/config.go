package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
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

// Config 保存 taskdaemon 启动阶段需要的基础配置。
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Logging       LoggingConfig
	Observability ObservabilityConfig
}

// ServerConfig 保存 HTTP API 与文档路由配置。
type ServerConfig struct {
	Host    string
	Port    int
	Swagger SwaggerConfig
}

// SwaggerConfig 控制 Swagger 文档路由是否注册。
type SwaggerConfig struct {
	Enabled bool
}

// DatabaseConfig 保存当前数据库连接配置。
type DatabaseConfig struct {
	Driver string
	DSN    string
}

// LoggingConfig 保存应用日志输出配置。
type LoggingConfig struct {
	Level   string
	Output  string
	Console LoggingConsoleConfig
	File    LoggingFileConfig
	HTTP    LoggingHTTPConfig
}

// LoggingConsoleConfig 保存控制台日志配置。
type LoggingConsoleConfig struct {
	Format string
}

// LoggingFileConfig 保存文件日志与轮转配置。
type LoggingFileConfig struct {
	Path       string
	Format     string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// LoggingHTTPConfig 保存 HTTP 请求日志的可选 body 采集配置。
type LoggingHTTPConfig struct {
	IncludeRequestBody  bool
	IncludeResponseBody bool
	MaxBodyBytes        int
	RedactFields        []string
}

// ObservabilityConfig 保存可观测性相关配置。
type ObservabilityConfig struct {
	TraceID TraceIDConfig
}

// TraceIDConfig 控制 trace id 的响应暴露行为。
type TraceIDConfig struct {
	IncludeInResponse bool
}

// LoadOptions 控制配置加载来源和覆盖值。
type LoadOptions struct {
	ConfigPath string
	Optional   bool
	Overrides  map[string]any
}

// Default 返回项目默认配置。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Config: 默认配置值。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 39245,
			Swagger: SwaggerConfig{
				Enabled: false,
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    defaultDatabaseDSN(),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Output: "console",
			Console: LoggingConsoleConfig{
				Format: "text",
			},
			File: LoggingFileConfig{
				Path:       "./logs/taskdaemon.log",
				Format:     "json",
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
			HTTP: LoggingHTTPConfig{
				IncludeRequestBody:  false,
				IncludeResponseBody: false,
				MaxBodyBytes:        4096,
				RedactFields: []string{
					"password",
					"token",
					"access_token",
					"refresh_token",
					"session",
					"session_token",
					"cookie",
					"authorization",
					"csrf",
					"csrf_token",
				},
			},
		},
		Observability: ObservabilityConfig{
			TraceID: TraceIDConfig{
				IncludeInResponse: true,
			},
		},
	}
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

// Load 按 defaults < file < env < overrides 的顺序加载配置。
//
// 未显式指定 ConfigPath 时，仅在开发工作区里自动查找当前目录的项目配置文件；
// 若存在项目本地配置，则按 defaults < file < local < env < overrides 叠加；
// 发布版不会读取当前目录里的同名配置文件，避免误吃部署目录下的临时文件。
//
// 参数:
//   - opts: 配置文件路径、是否允许缺失和最终覆盖值。
//
// 返回值:
//   - Config: 合并后的配置。
//   - error: 配置文件读取或解析失败时返回错误。
func Load(opts LoadOptions) (Config, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaultMap(), "."), nil); err != nil {
		return Config{}, fmt.Errorf("load defaults: %w", err)
	}

	configPath := opts.ConfigPath
	localConfigPath := ""
	if configPath == "" {
		discoveredPath, found, err := ProjectPath()
		if err != nil {
			return Config{}, err
		}
		if found {
			configPath = discoveredPath
		} else {
			defaultPath, err := DefaultPath()
			if err != nil {
				return Config{}, err
			}
			configPath = defaultPath
		}
		discoveredLocalPath, localFound, err := ProjectLocalPath()
		if err != nil {
			return Config{}, err
		}
		if localFound {
			localConfigPath = discoveredLocalPath
		}
		opts.Optional = true
	}

	if err := loadFile(k, configPath, opts.Optional); err != nil {
		return Config{}, err
	}
	if localConfigPath != "" {
		if err := loadFile(k, localConfigPath, true); err != nil {
			return Config{}, err
		}
	}

	if err := k.Load(confmap.Provider(envMap("TASKDAEMON_"), "."), nil); err != nil {
		return Config{}, fmt.Errorf("load env: %w", err)
	}

	if len(opts.Overrides) > 0 {
		if err := k.Load(confmap.Provider(opts.Overrides, "."), nil); err != nil {
			return Config{}, fmt.Errorf("load overrides: %w", err)
		}
	}

	return Config{
		Server: ServerConfig{
			Host: k.String("server.host"),
			Port: k.Int("server.port"),
			Swagger: SwaggerConfig{
				Enabled: k.Bool("server.swagger.enabled"),
			},
		},
		Database: DatabaseConfig{
			Driver: k.String("database.driver"),
			DSN:    k.String("database.dsn"),
		},
		Logging: LoggingConfig{
			Level:  k.String("logging.level"),
			Output: k.String("logging.output"),
			Console: LoggingConsoleConfig{
				Format: k.String("logging.console.format"),
			},
			File: LoggingFileConfig{
				Path:       k.String("logging.file.path"),
				Format:     k.String("logging.file.format"),
				MaxSizeMB:  k.Int("logging.file.maxSizeMB"),
				MaxBackups: k.Int("logging.file.maxBackups"),
				MaxAgeDays: k.Int("logging.file.maxAgeDays"),
				Compress:   k.Bool("logging.file.compress"),
			},
			HTTP: LoggingHTTPConfig{
				IncludeRequestBody:  k.Bool("logging.http.includeRequestBody"),
				IncludeResponseBody: k.Bool("logging.http.includeResponseBody"),
				MaxBodyBytes:        k.Int("logging.http.maxBodyBytes"),
				RedactFields:        stringSliceValue(k.Get("logging.http.redactFields")),
			},
		},
		Observability: ObservabilityConfig{
			TraceID: TraceIDConfig{
				IncludeInResponse: k.Bool("observability.traceId.includeInResponse"),
			},
		},
	}, nil
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

// defaultMap 返回 koanf 使用的默认配置 map。
//
// 参数:
//   - 无。
//
// 返回值:
//   - map[string]any: 默认配置键值。
func defaultMap() map[string]any {
	defaults := Default()
	return map[string]any{
		"server.host":                             defaults.Server.Host,
		"server.port":                             defaults.Server.Port,
		"server.swagger.enabled":                  defaults.Server.Swagger.Enabled,
		"database.driver":                         defaults.Database.Driver,
		"database.dsn":                            defaults.Database.DSN,
		"logging.level":                           defaults.Logging.Level,
		"logging.output":                          defaults.Logging.Output,
		"logging.console.format":                  defaults.Logging.Console.Format,
		"logging.file.path":                       defaults.Logging.File.Path,
		"logging.file.format":                     defaults.Logging.File.Format,
		"logging.file.maxSizeMB":                  defaults.Logging.File.MaxSizeMB,
		"logging.file.maxBackups":                 defaults.Logging.File.MaxBackups,
		"logging.file.maxAgeDays":                 defaults.Logging.File.MaxAgeDays,
		"logging.file.compress":                   defaults.Logging.File.Compress,
		"logging.http.includeRequestBody":         defaults.Logging.HTTP.IncludeRequestBody,
		"logging.http.includeResponseBody":        defaults.Logging.HTTP.IncludeResponseBody,
		"logging.http.maxBodyBytes":               defaults.Logging.HTTP.MaxBodyBytes,
		"logging.http.redactFields":               defaults.Logging.HTTP.RedactFields,
		"observability.traceId.includeInResponse": defaults.Observability.TraceID.IncludeInResponse,
	}
}

// loadFile 在存在配置文件时将其加载进 koanf。
//
// 参数:
//   - k: 待写入的 koanf 实例。
//   - configPath: 配置文件路径。
//   - optional: 为 true 时允许文件不存在。
//
// 返回值:
//   - error: 文件缺失且不允许缺失，或解析失败时返回错误。
func loadFile(k *koanf.Koanf, configPath string, optional bool) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file %s: %w", configPath, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return fmt.Errorf("parse config file %s: %w", configPath, err)
	}
	if err := k.Load(confmap.Provider(flattenMap(raw), "."), nil); err != nil {
		return fmt.Errorf("load config file %s: %w", configPath, err)
	}
	return nil
}

// envMap 读取指定前缀的环境变量，并转换为 koanf 点分键。
//
// 参数:
//   - prefix: 环境变量前缀，例如 TASKDAEMON_。
//
// 返回值:
//   - map[string]any: 可通过 confmap.Provider 加载的配置键值。
func envMap(prefix string) map[string]any {
	values := make(map[string]any)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(strings.ToUpper(key), prefix) {
			continue
		}
		normalized := strings.TrimPrefix(strings.ToUpper(key), prefix)
		normalized = strings.ToLower(strings.ReplaceAll(normalized, "_", "."))
		values[normalized] = parseScalar(value)
	}
	applyEnvAlias(values, prefix, "LOGGING_FILE_MAX_SIZE_MB", "logging.file.maxSizeMB")
	applyEnvAlias(values, prefix, "LOGGING_FILE_MAX_BACKUPS", "logging.file.maxBackups")
	applyEnvAlias(values, prefix, "LOGGING_FILE_MAX_AGE_DAYS", "logging.file.maxAgeDays")
	applyEnvAlias(values, prefix, "LOGGING_HTTP_INCLUDE_REQUEST_BODY", "logging.http.includeRequestBody")
	applyEnvAlias(values, prefix, "LOGGING_HTTP_INCLUDE_RESPONSE_BODY", "logging.http.includeResponseBody")
	applyEnvAlias(values, prefix, "LOGGING_HTTP_MAX_BODY_BYTES", "logging.http.maxBodyBytes")
	applyEnvAlias(values, prefix, "LOGGING_HTTP_REDACT_FIELDS", "logging.http.redactFields")
	applyEnvAlias(values, prefix, "OBSERVABILITY_TRACE_ID_INCLUDE_IN_RESPONSE", "observability.traceId.includeInResponse")
	return values
}

// applyEnvAlias 为 camelCase 配置键提供自然的环境变量别名。
//
// 参数:
//   - values: 已解析的环境变量配置 map。
//   - prefix: 环境变量前缀，例如 TASKDAEMON_。
//   - suffix: 不含前缀的环境变量名。
//   - configKey: 对应的 koanf 点分配置键。
//
// 返回值:
//   - 无。
func applyEnvAlias(values map[string]any, prefix string, suffix string, configKey string) {
	value, ok := os.LookupEnv(prefix + suffix)
	if !ok {
		return
	}
	values[configKey] = parseScalar(value)
}

// flattenMap 将嵌套 YAML map 转为点分键 map。
//
// 参数:
//   - values: YAML 解析后的嵌套 map。
//
// 返回值:
//   - map[string]any: 点分键配置 map。
func flattenMap(values map[string]any) map[string]any {
	flattened := make(map[string]any)
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		if nested, ok := value.(map[string]any); ok {
			for key, nestedValue := range nested {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				walk(next, nestedValue)
			}
			return
		}
		flattened[prefix] = value
	}
	for key, value := range values {
		walk(key, value)
	}
	return flattened
}

// stringSliceValue 将配置值转换为字符串切片。
//
// 参数:
//   - value: koanf 读取到的配置值。
//
// 返回值:
//   - []string: 去空白后的字符串切片。
func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return cleanStringSlice(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return cleanStringSlice(values)
	case string:
		if typed == "" {
			return nil
		}
		return cleanStringSlice(strings.Split(typed, ","))
	default:
		return nil
	}
}

// cleanStringSlice 去除字符串切片中的空白项。
//
// 参数:
//   - values: 待清理的字符串切片。
//
// 返回值:
//   - []string: 清理后的字符串切片。
func cleanStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

// parseScalar 将环境变量字符串转换为基础标量类型。
//
// 参数:
//   - value: 环境变量字符串值。
//
// 返回值:
//   - any: bool、int 或原始字符串。
func parseScalar(value string) any {
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return value
}

// defaultDatabaseDSN 返回默认 SQLite 数据库路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: SQLite DSN。用户配置目录不可用时退回相对路径。
func defaultDatabaseDSN() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "file:taskdaemon.db?_fk=1"
	}
	return "file:" + filepath.Join(configDir, "taskdaemon", "taskdaemon.db") + "?_fk=1"
}
