package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
)

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

	paths, err := ResolvePaths(opts)
	if err != nil {
		return Config{}, err
	}
	if err := loadFile(k, paths.ConfigPath, paths.ConfigOptional); err != nil {
		return Config{}, err
	}
	if paths.LocalConfigPath != "" {
		if err := loadFile(k, paths.LocalConfigPath, true); err != nil {
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
