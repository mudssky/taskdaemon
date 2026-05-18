package config

import (
	"os"
	"path/filepath"
)

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
