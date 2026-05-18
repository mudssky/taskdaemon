package config

// Config 保存 taskdaemon 启动阶段需要的基础配置。
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Logging       LoggingConfig
	Observability ObservabilityConfig
}

// ResolvedPaths 保存本次配置加载会使用的文件路径。
type ResolvedPaths struct {
	ConfigPath         string
	ConfigExists       bool
	ConfigOptional     bool
	LocalConfigPath    string
	LocalConfigExists  bool
	ExplicitConfigPath bool
}

// ReloadResult 描述一次运行时配置重载的结果。
type ReloadResult struct {
	Applied         []string `json:"applied" yaml:"applied"`
	RestartRequired []string `json:"restartRequired" yaml:"restartRequired"`
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
