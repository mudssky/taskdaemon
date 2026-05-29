package httpapi

import "taskdaemon/internal/config"

type audioConfigResponse struct {
	Autoplay      audioAutoplayConfigResponse `json:"autoplay"`
	Playback      audioPlaybackConfigResponse `json:"playback"`
	Inbound       audioInboundConfigResponse  `json:"inbound"`
	History       audioHistoryConfigResponse  `json:"history"`
	FFmpeg        audioFFmpegConfigResponse   `json:"ffmpeg"`
	Configuration audioConfigurationResponse  `json:"configuration"`
}

type audioAutoplayConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Target  string `json:"target"`
}

type audioPlaybackConfigResponse struct {
	QueueLimit int `json:"queueLimit"`
}

type audioInboundConfigResponse struct {
	TokenConfigured bool                          `json:"tokenConfigured"`
	MaxBytes        int64                         `json:"maxBytes"`
	URL             audioInboundURLConfigResponse `json:"url"`
}

type audioInboundURLConfigResponse struct {
	AllowedSchemes         []string `json:"allowedSchemes"`
	AllowPrivateNetworks   bool     `json:"allowPrivateNetworks"`
	AllowedHosts           []string `json:"allowedHosts"`
	DownloadTimeoutSeconds int      `json:"downloadTimeoutSeconds"`
	MaxRedirects           int      `json:"maxRedirects"`
}

type audioHistoryConfigResponse struct {
	Limit int `json:"limit"`
}

type audioFFmpegConfigResponse struct {
	PathConfigured      bool `json:"pathConfigured"`
	ProbePathConfigured bool `json:"probePathConfigured"`
	TranscodeTimeoutSec int  `json:"transcodeTimeoutSeconds"`
}

type audioConfigurationResponse struct {
	RuntimeEditable []string `json:"runtimeEditable"`
	RestartRequired []string `json:"restartRequired"`
}

// audioConfigResponseFromConfig 将音频配置转换为安全 API DTO。
//
// 参数:
//   - cfg: 音频配置。
//
// 返回值:
//   - audioConfigResponse: 不包含 token hash 或本地路径明文的响应 DTO。
func audioConfigResponseFromConfig(cfg config.AudioConfig) audioConfigResponse {
	return audioConfigResponse{
		Autoplay: audioAutoplayConfigResponse{
			Enabled: cfg.Autoplay.Enabled,
			Target:  cfg.Autoplay.Target,
		},
		Playback: audioPlaybackConfigResponse{
			QueueLimit: cfg.Playback.QueueLimit,
		},
		Inbound: audioInboundConfigResponse{
			TokenConfigured: cfg.Inbound.TokenHash != "",
			MaxBytes:        cfg.Inbound.MaxBytes,
			URL: audioInboundURLConfigResponse{
				AllowedSchemes:         cfg.Inbound.URL.AllowedSchemes,
				AllowPrivateNetworks:   cfg.Inbound.URL.AllowPrivateNetworks,
				AllowedHosts:           cfg.Inbound.URL.AllowedHosts,
				DownloadTimeoutSeconds: cfg.Inbound.URL.DownloadTimeoutSeconds,
				MaxRedirects:           cfg.Inbound.URL.MaxRedirects,
			},
		},
		History: audioHistoryConfigResponse{
			Limit: cfg.History.Limit,
		},
		FFmpeg: audioFFmpegConfigResponse{
			PathConfigured:      cfg.FFmpeg.Path != "",
			ProbePathConfigured: cfg.FFmpeg.ProbePath != "",
			TranscodeTimeoutSec: cfg.FFmpeg.TranscodeTimeoutSeconds,
		},
		Configuration: audioConfigurationResponse{
			RuntimeEditable: []string{"autoplay", "playback", "inbound", "history"},
			RestartRequired: []string{
				"ffmpeg.path",
				"ffmpeg.probePath",
				"ffmpeg.transcodeTimeoutSeconds",
			},
		},
	}
}
