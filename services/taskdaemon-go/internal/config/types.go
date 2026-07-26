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
	// G6: agent-gateway 双向桥接配置（append-only）
	AgentBridge AgentBridgeConfig
	// TD2/D3: Desktop 壳增量能力配置（append-only）
	Desktop DesktopConfig
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
	// T4
	Webhook NotifyWebhookConfig // Webhook 出站 sink
	Email   NotifyEmailConfig   // 邮件出站 sink
	// D2 / T3
	Desktop NotifyDesktopConfig // Desktop 原生通知 sink
}

// NotifyDesktopConfig 保存 Desktop 原生通知 sink 配置。
// D2 / T3
type NotifyDesktopConfig struct {
	Enabled     bool
	MinSeverity string // 最低投递级别；空表示不限
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

// NotifyWebhookConfig 保存 Webhook 出站 sink 配置。
// T4
type NotifyWebhookConfig struct {
	Enabled              bool
	DefaultTimeoutSec    int
	DefaultMaxRetries    int
	DefaultBackoffMs     int
	AllowedSchemes       []string
	AllowPrivateNetworks bool
	AllowedHosts         []string
	MaxRedirects         int
	Targets              []NotifyWebhookTargetConfig
}

// NotifyWebhookTargetConfig 描述单个 Webhook 目标。
// T4
type NotifyWebhookTargetConfig struct {
	Name           string
	URL            string
	Enabled        bool
	Method         string
	Headers        map[string]string
	SigningSecret  string
	SigningHeader  string
	TimeoutSeconds int
	MaxRetries     int
	BackoffMs      int
}

// NotifyEmailConfig 保存邮件出站 sink 配置。
// T4
type NotifyEmailConfig struct {
	Enabled        bool
	MinSeverity    string
	From           string
	To             []string
	TimeoutSeconds int
	MaxRetries     int
	BackoffMs      int
	SMTP           NotifySMTPConfig
}

// NotifySMTPConfig 保存 SMTP 连接配置。
// T4
type NotifySMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	Encryption string // none | starttls | tls
}

// LoadOptions 控制配置加载来源和覆盖值。
type LoadOptions struct {
	ConfigPath string
	Optional   bool
	Overrides  map[string]any
}

// AgentBridgeConfig 保存 G6 双引擎互调配置。
// 分级：outbound 连接需重启；allowedTaskIDs / maxLoopDepth 热更新（经 RuntimeConfig 若扩展）。
// 默认保守：未启用、无允许任务、环路深度 1。
type AgentBridgeConfig struct {
	// Enabled 总开关；false 时 agent runner 与 inbound 均拒绝。
	Enabled bool
	// GatewayBaseURL agent-gateway 根地址，如 http://127.0.0.1:8787。
	GatewayBaseURL string
	// GatewaySubject 调用 gateway 时注入的 x-auth-subject（机器身份，非管理员 cookie）。
	GatewaySubject string
	// GatewayTenantID 调用 gateway 时注入的 x-tenant-id。
	GatewayTenantID string
	// InboundTokenHash agent→taskdaemon Bearer Token 的 SHA-256 hex；空 = 拒绝一切入站。
	InboundTokenHash string
	// AllowedTaskIDs agent 可触发的任务 ID 白名单；空 = 全部拒绝。
	AllowedTaskIDs []int
	// MaxLoopDepth 环路深度上限；请求 depth >= 此值则拒绝。默认 1。
	MaxLoopDepth int
	// RequestTimeoutSeconds 出站 HTTP 超时秒数。
	RequestTimeoutSeconds int
}

// DesktopConfig 保存 Desktop 壳原生增量能力配置。
// TD2/D3
//
// 分级：
//   - trayEnabled / singleInstance / windowStateEnabled：进程启动时读取，改后需重启
//   - minimizeToTray：运行时 host 可即时生效
//   - autostartEnabled：期望状态；实际登录项经 desktop.autostart capability 写系统
//
// 与 T5 系统服务安装无关：本结构只控制 Desktop GUI 应用行为。
type DesktopConfig struct {
	// TrayEnabled 是否创建系统托盘。
	TrayEnabled bool
	// MinimizeToTray 关闭主窗口时隐藏到托盘而非退出。
	MinimizeToTray bool
	// AutostartEnabled 是否期望登录时启动 Desktop 应用（非系统服务）。
	AutostartEnabled bool
	// SingleInstance 是否启用单实例（第二进程激活已有窗口）。
	SingleInstance bool
	// WindowStateEnabled 是否持久化窗口位置/大小/最大化。
	WindowStateEnabled bool
}
