package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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
		Audio: AudioConfig{
			Autoplay: AudioAutoplayConfig{
				Enabled: k.Bool("audio.autoplay.enabled"),
				Target:  k.String("audio.autoplay.target"),
			},
			Playback: AudioPlaybackConfig{
				QueueLimit: k.Int("audio.playback.queueLimit"),
			},
			Inbound: AudioInboundConfig{
				TokenHash: k.String("audio.inbound.tokenHash"),
				MaxBytes:  int64Value(k.Get("audio.inbound.maxBytes")),
				URL: AudioInboundURLConfig{
					AllowedSchemes:         stringSliceValue(k.Get("audio.inbound.url.allowedSchemes")),
					AllowPrivateNetworks:   k.Bool("audio.inbound.url.allowPrivateNetworks"),
					AllowedHosts:           stringSliceValue(k.Get("audio.inbound.url.allowedHosts")),
					DownloadTimeoutSeconds: k.Int("audio.inbound.url.downloadTimeoutSeconds"),
					MaxRedirects:           k.Int("audio.inbound.url.maxRedirects"),
				},
			},
			History: AudioHistoryConfig{
				Limit: k.Int("audio.history.limit"),
			},
			FFmpeg: AudioFFmpegConfig{
				Path:                    k.String("audio.ffmpeg.path"),
				ProbePath:               k.String("audio.ffmpeg.probePath"),
				TranscodeTimeoutSeconds: k.Int("audio.ffmpeg.transcodeTimeoutSeconds"),
			},
		},
		// T2a
		Notify: NotifyConfig{
			BufferSize: k.Int("notify.bufferSize"),
			Store: NotifyStoreConfig{
				Enabled:     k.Bool("notify.store.enabled"),
				MaxRecords:  k.Int("notify.store.maxRecords"),
				RetainDays:  k.Int("notify.store.retainDays"),
				MinSeverity: k.String("notify.store.minSeverity"),
			},
			// T4
			Webhook: NotifyWebhookConfig{
				Enabled:              k.Bool("notify.webhook.enabled"),
				DefaultTimeoutSec:    k.Int("notify.webhook.defaultTimeoutSec"),
				DefaultMaxRetries:    k.Int("notify.webhook.defaultMaxRetries"),
				DefaultBackoffMs:     k.Int("notify.webhook.defaultBackoffMs"),
				AllowedSchemes:       stringSliceValue(k.Get("notify.webhook.allowedSchemes")),
				AllowPrivateNetworks: k.Bool("notify.webhook.allowPrivateNetworks"),
				AllowedHosts:         stringSliceValue(k.Get("notify.webhook.allowedHosts")),
				MaxRedirects:         k.Int("notify.webhook.maxRedirects"),
				Targets:              webhookTargetsValue(k.Get("notify.webhook.targets")),
			},
			Email: NotifyEmailConfig{
				Enabled:        k.Bool("notify.email.enabled"),
				MinSeverity:    k.String("notify.email.minSeverity"),
				From:           k.String("notify.email.from"),
				To:             stringSliceValue(k.Get("notify.email.to")),
				TimeoutSeconds: k.Int("notify.email.timeoutSeconds"),
				MaxRetries:     k.Int("notify.email.maxRetries"),
				BackoffMs:      k.Int("notify.email.backoffMs"),
				SMTP: NotifySMTPConfig{
					Host:       k.String("notify.email.smtp.host"),
					Port:       k.Int("notify.email.smtp.port"),
					Username:   k.String("notify.email.smtp.username"),
					Password:   k.String("notify.email.smtp.password"),
					Encryption: k.String("notify.email.smtp.encryption"),
				},
			},
		},
		// T6
		RunLog: RunLogConfig{
			Enabled:       k.Bool("runlog.enabled"),
			RetainDays:    k.Int("runlog.retainDays"),
			MaxTotalBytes: int64Value(k.Get("runlog.maxTotalBytes")),
		},
		// G6
		AgentBridge: AgentBridgeConfig{
			Enabled:               k.Bool("agentBridge.enabled"),
			GatewayBaseURL:        k.String("agentBridge.gatewayBaseUrl"),
			GatewaySubject:        k.String("agentBridge.gatewaySubject"),
			GatewayTenantID:       k.String("agentBridge.gatewayTenantId"),
			InboundTokenHash:      k.String("agentBridge.inboundTokenHash"),
			AllowedTaskIDs:        intSliceValue(k.Get("agentBridge.allowedTaskIds")),
			MaxLoopDepth:          k.Int("agentBridge.maxLoopDepth"),
			RequestTimeoutSeconds: k.Int("agentBridge.requestTimeoutSeconds"),
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

// int64Value 将配置值转换为 int64。
//
// 参数:
//   - value: koanf 读取到的配置值。
//
// 返回值:
//   - int64: 转换后的整数，不支持的类型返回 0。
func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

// intSliceValue 将任意配置值转为 int 切片。
//
// 参数:
//   - value: koanf 读取到的值。
//
// 返回值:
//   - []int: 整数切片；无法解析时返回空切片。
func intSliceValue(value any) []int {
	switch typed := value.(type) {
	case []int:
		return append([]int(nil), typed...)
	case []any:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			out = append(out, int(int64Value(item)))
		}
		return out
	case []string:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			if n, err := strconv.Atoi(strings.TrimSpace(item)); err == nil {
				out = append(out, n)
			}
		}
		return out
	case string:
		parts := strings.Split(typed, ",")
		out := make([]int, 0, len(parts))
		for _, part := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				out = append(out, n)
			}
		}
		return out
	default:
		return []int{}
	}
}

// webhookTargetsValue 解析 Webhook 目标列表配置。
//
// 参数:
//   - value: koanf 读取到的 notify.webhook.targets 值。
//
// 返回值:
//   - []NotifyWebhookTargetConfig: 目标列表；无法解析时返回空切片。
func webhookTargetsValue(value any) []NotifyWebhookTargetConfig {
	items, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]NotifyWebhookTargetConfig); ok {
			return typed
		}
		return []NotifyWebhookTargetConfig{}
	}
	targets := make([]NotifyWebhookTargetConfig, 0, len(items))
	for _, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		target := NotifyWebhookTargetConfig{
			Name:           stringValue(raw["name"]),
			URL:            stringValue(raw["url"]),
			Enabled:        boolValue(raw["enabled"]),
			Method:         stringValue(raw["method"]),
			Headers:        stringMapValue(raw["headers"]),
			SigningSecret:  stringValue(raw["signingSecret"]),
			SigningHeader:  stringValue(raw["signingHeader"]),
			TimeoutSeconds: intValue(raw["timeoutSeconds"]),
			MaxRetries:     intValue(raw["maxRetries"]),
			BackoffMs:      intValue(raw["backoffMs"]),
		}
		targets = append(targets, target)
	}
	return targets
}

// stringValue 将任意配置值转为字符串。
//
// 参数:
//   - value: 原始值。
//
// 返回值:
//   - string: 字符串；不支持类型返回空串。
func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// boolValue 将任意配置值转为 bool。
//
// 参数:
//   - value: 原始值。
//
// 返回值:
//   - bool: 布尔值；不支持类型返回 false。
func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	default:
		return false
	}
}

// intValue 将任意配置值转为 int。
//
// 参数:
//   - value: 原始值。
//
// 返回值:
//   - int: 整数值；不支持类型返回 0。
func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// stringMapValue 将配置值转为 map[string]string。
//
// 参数:
//   - value: 原始 map 值。
//
// 返回值:
//   - map[string]string: 字符串 map；空或非法时返回 nil。
func stringMapValue(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		if typed, ok := value.(map[string]string); ok {
			return typed
		}
		return nil
	}
	out := make(map[string]string, len(raw))
	for key, nested := range raw {
		out[key] = stringValue(nested)
	}
	return out
}
