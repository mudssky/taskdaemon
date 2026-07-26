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
		Audio: AudioConfig{
			Autoplay: AudioAutoplayConfig{
				Enabled: false,
				Target:  "backend",
			},
			Playback: AudioPlaybackConfig{
				QueueLimit: 20,
			},
			Inbound: AudioInboundConfig{
				MaxBytes: 209715200,
				URL: AudioInboundURLConfig{
					AllowedSchemes:         []string{"https"},
					AllowPrivateNetworks:   false,
					AllowedHosts:           []string{},
					DownloadTimeoutSeconds: 60,
					MaxRedirects:           3,
				},
			},
			History: AudioHistoryConfig{
				Limit: 50,
			},
			FFmpeg: AudioFFmpegConfig{
				TranscodeTimeoutSeconds: 120,
			},
		},
		// T2a
		Notify: NotifyConfig{
			BufferSize: 256,
			Store: NotifyStoreConfig{
				Enabled:     true,
				MaxRecords:  200,
				RetainDays:  30,
				MinSeverity: "",
			},
			// T4
			Webhook: NotifyWebhookConfig{
				Enabled:              false,
				DefaultTimeoutSec:    10,
				DefaultMaxRetries:    2,
				DefaultBackoffMs:     200,
				AllowedSchemes:       []string{"https"},
				AllowPrivateNetworks: false,
				AllowedHosts:         []string{},
				MaxRedirects:         3,
				Targets:              []NotifyWebhookTargetConfig{},
			},
			Email: NotifyEmailConfig{
				Enabled:        false,
				MinSeverity:    "",
				From:           "",
				To:             []string{},
				TimeoutSeconds: 15,
				MaxRetries:     2,
				BackoffMs:      200,
				SMTP: NotifySMTPConfig{
					Host:       "",
					Port:       587,
					Username:   "",
					Password:   "",
					Encryption: "starttls",
				},
			},
		},
		// T6
		RunLog: RunLogConfig{
			Enabled:       true,
			RetainDays:    30,
			MaxTotalBytes: 2 * 1024 * 1024 * 1024, // 2 GiB
		},
		// G6
		AgentBridge: AgentBridgeConfig{
			Enabled:               false,
			GatewayBaseURL:        "http://127.0.0.1:8787",
			GatewaySubject:        "taskdaemon-service",
			GatewayTenantID:       "system",
			InboundTokenHash:      "",
			AllowedTaskIDs:        []int{},
			MaxLoopDepth:          1,
			RequestTimeoutSeconds: 120,
		},
		// TD2/D3
		Desktop: DesktopConfig{
			TrayEnabled:        true,
			MinimizeToTray:     true,
			AutostartEnabled:   false,
			SingleInstance:     true,
			WindowStateEnabled: true,
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
		"server.host":                              defaults.Server.Host,
		"server.port":                              defaults.Server.Port,
		"server.swagger.enabled":                   defaults.Server.Swagger.Enabled,
		"database.driver":                          defaults.Database.Driver,
		"database.dsn":                             defaults.Database.DSN,
		"logging.level":                            defaults.Logging.Level,
		"logging.output":                           defaults.Logging.Output,
		"logging.console.format":                   defaults.Logging.Console.Format,
		"logging.file.path":                        defaults.Logging.File.Path,
		"logging.file.format":                      defaults.Logging.File.Format,
		"logging.file.maxSizeMB":                   defaults.Logging.File.MaxSizeMB,
		"logging.file.maxBackups":                  defaults.Logging.File.MaxBackups,
		"logging.file.maxAgeDays":                  defaults.Logging.File.MaxAgeDays,
		"logging.file.compress":                    defaults.Logging.File.Compress,
		"logging.http.includeRequestBody":          defaults.Logging.HTTP.IncludeRequestBody,
		"logging.http.includeResponseBody":         defaults.Logging.HTTP.IncludeResponseBody,
		"logging.http.maxBodyBytes":                defaults.Logging.HTTP.MaxBodyBytes,
		"logging.http.redactFields":                defaults.Logging.HTTP.RedactFields,
		"observability.traceId.includeInResponse":  defaults.Observability.TraceID.IncludeInResponse,
		"audio.autoplay.enabled":                   defaults.Audio.Autoplay.Enabled,
		"audio.autoplay.target":                    defaults.Audio.Autoplay.Target,
		"audio.playback.queueLimit":                defaults.Audio.Playback.QueueLimit,
		"audio.inbound.tokenHash":                  defaults.Audio.Inbound.TokenHash,
		"audio.inbound.maxBytes":                   defaults.Audio.Inbound.MaxBytes,
		"audio.inbound.url.allowedSchemes":         defaults.Audio.Inbound.URL.AllowedSchemes,
		"audio.inbound.url.allowPrivateNetworks":   defaults.Audio.Inbound.URL.AllowPrivateNetworks,
		"audio.inbound.url.allowedHosts":           defaults.Audio.Inbound.URL.AllowedHosts,
		"audio.inbound.url.downloadTimeoutSeconds": defaults.Audio.Inbound.URL.DownloadTimeoutSeconds,
		"audio.inbound.url.maxRedirects":           defaults.Audio.Inbound.URL.MaxRedirects,
		"audio.history.limit":                      defaults.Audio.History.Limit,
		"audio.ffmpeg.path":                        defaults.Audio.FFmpeg.Path,
		"audio.ffmpeg.probePath":                   defaults.Audio.FFmpeg.ProbePath,
		"audio.ffmpeg.transcodeTimeoutSeconds":     defaults.Audio.FFmpeg.TranscodeTimeoutSeconds,
		// T2a
		"notify.bufferSize":        defaults.Notify.BufferSize,
		"notify.store.enabled":     defaults.Notify.Store.Enabled,
		"notify.store.maxRecords":  defaults.Notify.Store.MaxRecords,
		"notify.store.retainDays":  defaults.Notify.Store.RetainDays,
		"notify.store.minSeverity": defaults.Notify.Store.MinSeverity,
		// T6
		"runlog.enabled":       defaults.RunLog.Enabled,
		"runlog.retainDays":    defaults.RunLog.RetainDays,
		"runlog.maxTotalBytes": defaults.RunLog.MaxTotalBytes,
		// T4
		"notify.webhook.enabled":              defaults.Notify.Webhook.Enabled,
		"notify.webhook.defaultTimeoutSec":    defaults.Notify.Webhook.DefaultTimeoutSec,
		"notify.webhook.defaultMaxRetries":    defaults.Notify.Webhook.DefaultMaxRetries,
		"notify.webhook.defaultBackoffMs":     defaults.Notify.Webhook.DefaultBackoffMs,
		"notify.webhook.allowedSchemes":       defaults.Notify.Webhook.AllowedSchemes,
		"notify.webhook.allowPrivateNetworks": defaults.Notify.Webhook.AllowPrivateNetworks,
		"notify.webhook.allowedHosts":         defaults.Notify.Webhook.AllowedHosts,
		"notify.webhook.maxRedirects":         defaults.Notify.Webhook.MaxRedirects,
		"notify.webhook.targets":              defaults.Notify.Webhook.Targets,
		"notify.email.enabled":                defaults.Notify.Email.Enabled,
		"notify.email.minSeverity":            defaults.Notify.Email.MinSeverity,
		"notify.email.from":                   defaults.Notify.Email.From,
		"notify.email.to":                     defaults.Notify.Email.To,
		"notify.email.timeoutSeconds":         defaults.Notify.Email.TimeoutSeconds,
		"notify.email.smtp.host":              defaults.Notify.Email.SMTP.Host,
		"notify.email.smtp.port":              defaults.Notify.Email.SMTP.Port,
		"notify.email.smtp.username":          defaults.Notify.Email.SMTP.Username,
		"notify.email.smtp.password":          defaults.Notify.Email.SMTP.Password,
		"notify.email.smtp.encryption":        defaults.Notify.Email.SMTP.Encryption,
		// G6
		"agentBridge.enabled":               defaults.AgentBridge.Enabled,
		"agentBridge.gatewayBaseUrl":        defaults.AgentBridge.GatewayBaseURL,
		"agentBridge.gatewaySubject":        defaults.AgentBridge.GatewaySubject,
		"agentBridge.gatewayTenantId":       defaults.AgentBridge.GatewayTenantID,
		"agentBridge.inboundTokenHash":      defaults.AgentBridge.InboundTokenHash,
		"agentBridge.allowedTaskIds":        defaults.AgentBridge.AllowedTaskIDs,
		"agentBridge.maxLoopDepth":          defaults.AgentBridge.MaxLoopDepth,
		"agentBridge.requestTimeoutSeconds": defaults.AgentBridge.RequestTimeoutSeconds,
		// TD2/D3
		"desktop.trayEnabled":        defaults.Desktop.TrayEnabled,
		"desktop.minimizeToTray":     defaults.Desktop.MinimizeToTray,
		"desktop.autostartEnabled":   defaults.Desktop.AutostartEnabled,
		"desktop.singleInstance":     defaults.Desktop.SingleInstance,
		"desktop.windowStateEnabled": defaults.Desktop.WindowStateEnabled,
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
