package config

import (
	"os"
	"strconv"
	"strings"
)

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
	applyEnvAlias(values, prefix, "AUDIO_PLAYBACK_QUEUE_LIMIT", "audio.playback.queueLimit")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_TOKEN_HASH", "audio.inbound.tokenHash")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_MAX_BYTES", "audio.inbound.maxBytes")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_URL_ALLOWED_SCHEMES", "audio.inbound.url.allowedSchemes")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_URL_ALLOW_PRIVATE_NETWORKS", "audio.inbound.url.allowPrivateNetworks")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_URL_ALLOWED_HOSTS", "audio.inbound.url.allowedHosts")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_URL_DOWNLOAD_TIMEOUT_SECONDS", "audio.inbound.url.downloadTimeoutSeconds")
	applyEnvAlias(values, prefix, "AUDIO_INBOUND_URL_MAX_REDIRECTS", "audio.inbound.url.maxRedirects")
	applyEnvAlias(values, prefix, "AUDIO_HISTORY_LIMIT", "audio.history.limit")
	applyEnvAlias(values, prefix, "AUDIO_FFMPEG_PROBE_PATH", "audio.ffmpeg.probePath")
	applyEnvAlias(values, prefix, "AUDIO_FFMPEG_TRANSCODE_TIMEOUT_SECONDS", "audio.ffmpeg.transcodeTimeoutSeconds")
	// T2a
	applyEnvAlias(values, prefix, "NOTIFY_BUFFER_SIZE", "notify.bufferSize")
	applyEnvAlias(values, prefix, "NOTIFY_STORE_ENABLED", "notify.store.enabled")
	applyEnvAlias(values, prefix, "NOTIFY_STORE_MAX_RECORDS", "notify.store.maxRecords")
	applyEnvAlias(values, prefix, "NOTIFY_STORE_RETAIN_DAYS", "notify.store.retainDays")
	applyEnvAlias(values, prefix, "NOTIFY_STORE_MIN_SEVERITY", "notify.store.minSeverity")
	// T6
	applyEnvAlias(values, prefix, "RUNLOG_ENABLED", "runlog.enabled")
	applyEnvAlias(values, prefix, "RUNLOG_RETAIN_DAYS", "runlog.retainDays")
	applyEnvAlias(values, prefix, "RUNLOG_MAX_TOTAL_BYTES", "runlog.maxTotalBytes")
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
