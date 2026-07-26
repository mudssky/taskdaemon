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
	FileOnly        []string `json:"fileOnly"`
}

// audioConfigWriteRequest audio section 部分更新请求。
// 仅提交需要修改的字段；省略字段保持不变。
type audioConfigWriteRequest struct {
	Autoplay *audioAutoplayWriteRequest `json:"autoplay"`
	Playback *audioPlaybackWriteRequest `json:"playback"`
	Inbound  *audioInboundWriteRequest  `json:"inbound"`
	History  *audioHistoryWriteRequest  `json:"history"`
	FFmpeg   *audioFFmpegWriteRequest   `json:"ffmpeg"`
}

type audioAutoplayWriteRequest struct {
	Enabled *bool   `json:"enabled"`
	Target  *string `json:"target"`
}

type audioPlaybackWriteRequest struct {
	QueueLimit *int `json:"queueLimit"`
}

type audioInboundWriteRequest struct {
	// Token 明文 Bearer；落盘为 SHA-256 hash，响应永不回显。
	Token    *string                      `json:"token"`
	MaxBytes *int64                       `json:"maxBytes"`
	URL      *audioInboundURLWriteRequest `json:"url"`
	// TokenHash 禁止 API 直写。
	TokenHash *string `json:"tokenHash"`
}

type audioInboundURLWriteRequest struct {
	AllowedSchemes         *[]string `json:"allowedSchemes"`
	AllowPrivateNetworks   *bool     `json:"allowPrivateNetworks"`
	AllowedHosts           *[]string `json:"allowedHosts"`
	DownloadTimeoutSeconds *int      `json:"downloadTimeoutSeconds"`
	MaxRedirects           *int      `json:"maxRedirects"`
}

type audioHistoryWriteRequest struct {
	Limit *int `json:"limit"`
}

type audioFFmpegWriteRequest struct {
	Path                    *string `json:"path"`
	ProbePath               *string `json:"probePath"`
	TranscodeTimeoutSeconds *int    `json:"transcodeTimeoutSeconds"`
}

// ConfigSectionWriteResponse PUT /api/config/:section 成功响应。
type ConfigSectionWriteResponse struct {
	Config          any                         `json:"config"`
	Applied         []string                    `json:"applied"`
	RestartRequired []string                    `json:"restartRequired"`
	Reload          ConfigSectionReloadResponse `json:"reload"`
}

// ConfigSectionReloadResponse 写入后的 reload 观测。
type ConfigSectionReloadResponse struct {
	Applied         []string                        `json:"applied"`
	RestartRequired []string                        `json:"restartRequired"`
	Subsystems      []ConfigSubsystemStatusResponse `json:"subsystems"`
}

// ConfigSubsystemStatusResponse 子系统应用状态。
type ConfigSubsystemStatusResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// AudioConfigResponseFromConfig 导出安全音频配置 DTO，供 app 装配写入响应。
//
// 参数:
//   - cfg: 音频配置。
//
// 返回值:
//   - any: 可 JSON 序列化的安全 DTO。
func AudioConfigResponseFromConfig(cfg config.AudioConfig) any {
	return audioConfigResponseFromConfig(cfg)
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
			RuntimeEditable: config.AudioRuntimeEditablePaths(),
			RestartRequired: config.AudioRestartRequiredPaths(),
			FileOnly:        config.AudioFileOnlyPaths(),
		},
	}
}

// audioPatchFromRequest 将请求 DTO 转为配置补丁。
//
// 参数:
//   - req: 写入请求。
//
// 返回值:
//   - config.AudioSectionPatch: 配置层补丁。
//   - []config.FieldError: 请求层不可写字段错误（如 tokenHash）。
func audioPatchFromRequest(req audioConfigWriteRequest) (config.AudioSectionPatch, []config.FieldError) {
	var fields []config.FieldError
	patch := config.AudioSectionPatch{}
	if req.Autoplay != nil {
		patch.Autoplay = &config.AudioAutoplayPatch{
			Enabled: req.Autoplay.Enabled,
			Target:  req.Autoplay.Target,
		}
	}
	if req.Playback != nil {
		patch.Playback = &config.AudioPlaybackPatch{QueueLimit: req.Playback.QueueLimit}
	}
	if req.Inbound != nil {
		if req.Inbound.TokenHash != nil {
			fields = append(fields, config.FieldError{
				Path:   "audio.inbound.tokenHash",
				Reason: "file-only field; submit inbound.token instead",
				Code:   config.CodeFieldNotWritable,
			})
		}
		inbound := &config.AudioInboundPatch{
			Token:    req.Inbound.Token,
			MaxBytes: req.Inbound.MaxBytes,
		}
		if req.Inbound.URL != nil {
			inbound.URL = &config.AudioInboundURLPatch{
				AllowedSchemes:         req.Inbound.URL.AllowedSchemes,
				AllowPrivateNetworks:   req.Inbound.URL.AllowPrivateNetworks,
				AllowedHosts:           req.Inbound.URL.AllowedHosts,
				DownloadTimeoutSeconds: req.Inbound.URL.DownloadTimeoutSeconds,
				MaxRedirects:           req.Inbound.URL.MaxRedirects,
			}
		}
		patch.Inbound = inbound
	}
	if req.History != nil {
		patch.History = &config.AudioHistoryPatch{Limit: req.History.Limit}
	}
	if req.FFmpeg != nil {
		patch.FFmpeg = &config.AudioFFmpegPatch{
			Path:                    req.FFmpeg.Path,
			ProbePath:               req.FFmpeg.ProbePath,
			TranscodeTimeoutSeconds: req.FFmpeg.TranscodeTimeoutSeconds,
		}
	}
	return patch, fields
}
