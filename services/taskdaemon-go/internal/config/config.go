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

// Config 保存 taskdaemon 启动阶段需要的基础配置。
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
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
			Port: 8080,
			Swagger: SwaggerConfig{
				Enabled: false,
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    defaultDatabaseDSN(),
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

// Load 按 defaults < file < env < overrides 的顺序加载配置。
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
	if configPath == "" {
		defaultPath, err := DefaultPath()
		if err != nil {
			return Config{}, err
		}
		configPath = defaultPath
		opts.Optional = true
	}

	if err := loadFile(k, configPath, opts.Optional); err != nil {
		return Config{}, err
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
	}, nil
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
		"server.host":            defaults.Server.Host,
		"server.port":            defaults.Server.Port,
		"server.swagger.enabled": defaults.Server.Swagger.Enabled,
		"database.driver":        defaults.Database.Driver,
		"database.dsn":           defaults.Database.DSN,
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
	return values
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
