package config

// Config 保存 taskdaemon 启动阶段需要的基础配置。
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Logging       LoggingConfig
	Observability ObservabilityConfig
	Audio         AudioConfig
	// T2a: 通知事件总线配置（append-only）
	Notify NotifyConfig
	// T6: run 完整日志归档配置（append-only）
	RunLog RunLogConfig
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

// AudioConfig 保存外部音频播放请求的配置。
type AudioConfig struct {
	Autoplay AudioAutoplayConfig
	Playback AudioPlaybackConfig
	Inbound  AudioInboundConfig
	History  AudioHistoryConfig
	FFmpeg   AudioFFmpegConfig
}

// AudioAutoplayConfig 控制收到音频播放请求后的自动播放行为。
type AudioAutoplayConfig struct {
	Enabled bool
	Target  string
}

// AudioPlaybackConfig 保存后端播放队列配置。
type AudioPlaybackConfig struct {
	QueueLimit int
}

// AudioInboundConfig 保存外部入站播放请求的认证和下载限制。
type AudioInboundConfig struct {
	TokenHash string
	MaxBytes  int64
	URL       AudioInboundURLConfig
}

// AudioInboundURLConfig 保存 URL 来源音频的下载安全策略。
type AudioInboundURLConfig struct {
	AllowedSchemes         []string
	AllowPrivateNetworks   bool
	AllowedHosts           []string
	DownloadTimeoutSeconds int
	MaxRedirects           int
}

// AudioHistoryConfig 保存最近音频记录保留策略。
type AudioHistoryConfig struct {
	Limit int
}

// AudioFFmpegConfig 保存 FFmpeg/ffprobe 定位与转码限制。
type AudioFFmpegConfig struct {
	Path                    string
	ProbePath               string
	TranscodeTimeoutSeconds int
}

// NotifyConfig 保存通知事件总线配置。
// T2a
type NotifyConfig struct {
	BufferSize int               // 事件缓冲区大小；需重启生效
	Store      NotifyStoreConfig // 站内 sink
	// T4 追加 Webhook / Email
	// D2 追加 Desktop
}

// NotifyStoreConfig 保存站内通知 sink 配置。
// T2a
type NotifyStoreConfig struct {
	Enabled     bool
	MaxRecords  int    // 0 = 不限
	RetainDays  int    // 0 = 不限
	MinSeverity string // 最低投递级别；空表示不限
}

// RunLogConfig 保存 run 完整日志归档配置。
// T6
type RunLogConfig struct {
	Enabled       bool  // 是否启用落盘
	RetainDays    int   // 保留天数；0 = 不限
	MaxTotalBytes int64 // 总体积上限；0 = 不限
}

// LoadOptions 控制配置加载来源和覆盖值。
type LoadOptions struct {
	ConfigPath string
	Optional   bool
	Overrides  map[string]any
}
