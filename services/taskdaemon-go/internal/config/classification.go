package config

// FieldClass 配置字段三级分类。
// 新增字段必须显式登记；禁止默认视为热生效。
type FieldClass string

const (
	// FieldClassHot 写入后立即热生效。
	FieldClassHot FieldClass = "hot"
	// FieldClassRestart 写入落盘，进程重启后生效。
	FieldClassRestart FieldClass = "restart"
	// FieldClassFileOnly 仅允许配置文件/环境变量修改，API 拒绝写入。
	FieldClassFileOnly FieldClass = "file_only"
)

// FieldMeta 描述单个配置字段的契约元数据。
type FieldMeta struct {
	// Path 使用 camelCase 点分路径，与 YAML/API 一致。
	Path string
	// Class 三级分类。
	Class FieldClass
	// Sensitive 为 true 时响应不明文回显。
	Sensitive bool
}

// writableSections 当前开放 PUT 的 section 名。
var writableSections = map[string]struct{}{
	"audio": {},
}

// fieldRegistry 全量配置字段分类表（C-1 冻结源）。
// 键为完整点分路径。
var fieldRegistry = map[string]FieldMeta{
	// server — 部署/监听，仅配置文件
	"server.host":            {Path: "server.host", Class: FieldClassFileOnly},
	"server.port":            {Path: "server.port", Class: FieldClassFileOnly},
	"server.swagger.enabled": {Path: "server.swagger.enabled", Class: FieldClassFileOnly},

	// database — 凭证与连接，仅配置文件
	"database.driver": {Path: "database.driver", Class: FieldClassFileOnly},
	"database.dsn":    {Path: "database.dsn", Class: FieldClassFileOnly, Sensitive: true},

	// logging
	"logging.level":                    {Path: "logging.level", Class: FieldClassRestart},
	"logging.output":                   {Path: "logging.output", Class: FieldClassRestart},
	"logging.console.format":           {Path: "logging.console.format", Class: FieldClassRestart},
	"logging.file.path":                {Path: "logging.file.path", Class: FieldClassFileOnly},
	"logging.file.format":              {Path: "logging.file.format", Class: FieldClassRestart},
	"logging.file.maxSizeMB":           {Path: "logging.file.maxSizeMB", Class: FieldClassRestart},
	"logging.file.maxBackups":          {Path: "logging.file.maxBackups", Class: FieldClassRestart},
	"logging.file.maxAgeDays":          {Path: "logging.file.maxAgeDays", Class: FieldClassRestart},
	"logging.file.compress":            {Path: "logging.file.compress", Class: FieldClassRestart},
	"logging.http.includeRequestBody":  {Path: "logging.http.includeRequestBody", Class: FieldClassHot},
	"logging.http.includeResponseBody": {Path: "logging.http.includeResponseBody", Class: FieldClassHot},
	"logging.http.maxBodyBytes":        {Path: "logging.http.maxBodyBytes", Class: FieldClassHot},
	"logging.http.redactFields":        {Path: "logging.http.redactFields", Class: FieldClassHot},

	// observability
	"observability.traceId.includeInResponse": {Path: "observability.traceId.includeInResponse", Class: FieldClassHot},

	// audio
	"audio.autoplay.enabled":                   {Path: "audio.autoplay.enabled", Class: FieldClassHot},
	"audio.autoplay.target":                    {Path: "audio.autoplay.target", Class: FieldClassHot},
	"audio.playback.queueLimit":                {Path: "audio.playback.queueLimit", Class: FieldClassHot},
	"audio.inbound.tokenHash":                  {Path: "audio.inbound.tokenHash", Class: FieldClassFileOnly, Sensitive: true},
	"audio.inbound.token":                      {Path: "audio.inbound.token", Class: FieldClassHot, Sensitive: true},
	"audio.inbound.maxBytes":                   {Path: "audio.inbound.maxBytes", Class: FieldClassHot},
	"audio.inbound.url.allowedSchemes":         {Path: "audio.inbound.url.allowedSchemes", Class: FieldClassHot},
	"audio.inbound.url.allowPrivateNetworks":   {Path: "audio.inbound.url.allowPrivateNetworks", Class: FieldClassHot},
	"audio.inbound.url.allowedHosts":           {Path: "audio.inbound.url.allowedHosts", Class: FieldClassHot},
	"audio.inbound.url.downloadTimeoutSeconds": {Path: "audio.inbound.url.downloadTimeoutSeconds", Class: FieldClassHot},
	"audio.inbound.url.maxRedirects":           {Path: "audio.inbound.url.maxRedirects", Class: FieldClassHot},
	"audio.history.limit":                      {Path: "audio.history.limit", Class: FieldClassHot},
	"audio.ffmpeg.path":                        {Path: "audio.ffmpeg.path", Class: FieldClassFileOnly},
	"audio.ffmpeg.probePath":                   {Path: "audio.ffmpeg.probePath", Class: FieldClassFileOnly},
	"audio.ffmpeg.transcodeTimeoutSeconds":     {Path: "audio.ffmpeg.transcodeTimeoutSeconds", Class: FieldClassRestart},

	// notify
	"notify.bufferSize":                   {Path: "notify.bufferSize", Class: FieldClassRestart},
	"notify.store.enabled":                {Path: "notify.store.enabled", Class: FieldClassHot},
	"notify.store.maxRecords":             {Path: "notify.store.maxRecords", Class: FieldClassHot},
	"notify.store.retainDays":             {Path: "notify.store.retainDays", Class: FieldClassHot},
	"notify.store.minSeverity":            {Path: "notify.store.minSeverity", Class: FieldClassHot},
	"notify.webhook.enabled":              {Path: "notify.webhook.enabled", Class: FieldClassHot},
	"notify.webhook.defaultTimeoutSec":    {Path: "notify.webhook.defaultTimeoutSec", Class: FieldClassHot},
	"notify.webhook.defaultMaxRetries":    {Path: "notify.webhook.defaultMaxRetries", Class: FieldClassHot},
	"notify.webhook.defaultBackoffMs":     {Path: "notify.webhook.defaultBackoffMs", Class: FieldClassHot},
	"notify.webhook.allowedSchemes":       {Path: "notify.webhook.allowedSchemes", Class: FieldClassHot},
	"notify.webhook.allowPrivateNetworks": {Path: "notify.webhook.allowPrivateNetworks", Class: FieldClassHot},
	"notify.webhook.allowedHosts":         {Path: "notify.webhook.allowedHosts", Class: FieldClassHot},
	"notify.webhook.maxRedirects":         {Path: "notify.webhook.maxRedirects", Class: FieldClassHot},
	"notify.webhook.targets":              {Path: "notify.webhook.targets", Class: FieldClassHot, Sensitive: true},
	"notify.email.enabled":                {Path: "notify.email.enabled", Class: FieldClassHot},
	"notify.email.minSeverity":            {Path: "notify.email.minSeverity", Class: FieldClassHot},
	"notify.email.from":                   {Path: "notify.email.from", Class: FieldClassHot},
	"notify.email.to":                     {Path: "notify.email.to", Class: FieldClassHot},
	"notify.email.timeoutSeconds":         {Path: "notify.email.timeoutSeconds", Class: FieldClassHot},
	"notify.email.maxRetries":             {Path: "notify.email.maxRetries", Class: FieldClassHot},
	"notify.email.backoffMs":              {Path: "notify.email.backoffMs", Class: FieldClassHot},
	"notify.email.smtp.host":              {Path: "notify.email.smtp.host", Class: FieldClassHot},
	"notify.email.smtp.port":              {Path: "notify.email.smtp.port", Class: FieldClassHot},
	"notify.email.smtp.username":          {Path: "notify.email.smtp.username", Class: FieldClassHot},
	"notify.email.smtp.password":          {Path: "notify.email.smtp.password", Class: FieldClassHot, Sensitive: true},
	"notify.email.smtp.encryption":        {Path: "notify.email.smtp.encryption", Class: FieldClassHot},
	"notify.desktop.enabled":              {Path: "notify.desktop.enabled", Class: FieldClassHot},
	"notify.desktop.minSeverity":          {Path: "notify.desktop.minSeverity", Class: FieldClassHot},

	// runlog
	"runlog.enabled":       {Path: "runlog.enabled", Class: FieldClassRestart},
	"runlog.retainDays":    {Path: "runlog.retainDays", Class: FieldClassRestart},
	"runlog.maxTotalBytes": {Path: "runlog.maxTotalBytes", Class: FieldClassRestart},

	// agentBridge
	"agentBridge.enabled":               {Path: "agentBridge.enabled", Class: FieldClassRestart},
	"agentBridge.gatewayBaseUrl":        {Path: "agentBridge.gatewayBaseUrl", Class: FieldClassRestart},
	"agentBridge.gatewaySubject":        {Path: "agentBridge.gatewaySubject", Class: FieldClassRestart},
	"agentBridge.gatewayTenantId":       {Path: "agentBridge.gatewayTenantId", Class: FieldClassRestart},
	"agentBridge.inboundTokenHash":      {Path: "agentBridge.inboundTokenHash", Class: FieldClassFileOnly, Sensitive: true},
	"agentBridge.inboundToken":          {Path: "agentBridge.inboundToken", Class: FieldClassRestart, Sensitive: true},
	"agentBridge.allowedTaskIds":        {Path: "agentBridge.allowedTaskIds", Class: FieldClassHot},
	"agentBridge.maxLoopDepth":          {Path: "agentBridge.maxLoopDepth", Class: FieldClassHot},
	"agentBridge.requestTimeoutSeconds": {Path: "agentBridge.requestTimeoutSeconds", Class: FieldClassRestart},

	// desktop
	"desktop.trayEnabled":        {Path: "desktop.trayEnabled", Class: FieldClassRestart},
	"desktop.minimizeToTray":     {Path: "desktop.minimizeToTray", Class: FieldClassHot},
	"desktop.autostartEnabled":   {Path: "desktop.autostartEnabled", Class: FieldClassRestart},
	"desktop.singleInstance":     {Path: "desktop.singleInstance", Class: FieldClassRestart},
	"desktop.windowStateEnabled": {Path: "desktop.windowStateEnabled", Class: FieldClassRestart},
}

// FieldClassOf 返回字段分类；未知字段返回 false。
//
// 参数:
//   - path: 点分配置路径。
//
// 返回值:
//   - FieldMeta: 字段元数据。
//   - bool: 是否已登记。
func FieldClassOf(path string) (FieldMeta, bool) {
	meta, ok := fieldRegistry[path]
	return meta, ok
}

// IsSectionWritable 判断 section 是否开放 PUT。
//
// 参数:
//   - section: section 名，如 audio。
//
// 返回值:
//   - bool: true 表示可写。
func IsSectionWritable(section string) bool {
	_, ok := writableSections[section]
	return ok
}

// AllFieldMeta 返回分类表快照（测试与文档生成用）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []FieldMeta: 全部字段元数据（无序）。
func AllFieldMeta() []FieldMeta {
	out := make([]FieldMeta, 0, len(fieldRegistry))
	for _, meta := range fieldRegistry {
		out = append(out, meta)
	}
	return out
}

// AudioRuntimeEditablePaths 返回 audio 热生效字段组（API 展示）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []string: 组级路径。
func AudioRuntimeEditablePaths() []string {
	return []string{"autoplay", "playback", "inbound", "history"}
}

// AudioRestartRequiredPaths 返回 audio 需重启且可 API 写的字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []string: 字段路径。
func AudioRestartRequiredPaths() []string {
	return []string{"ffmpeg.transcodeTimeoutSeconds"}
}

// AudioFileOnlyPaths 返回 audio 仅配置文件字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []string: 字段路径。
func AudioFileOnlyPaths() []string {
	return []string{"ffmpeg.path", "ffmpeg.probePath", "inbound.tokenHash"}
}
