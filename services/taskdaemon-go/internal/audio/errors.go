package audio

import "errors"

var (
	// ErrUnauthorized 表示外部播放请求缺失或携带了错误的 Bearer Token。
	ErrUnauthorized = errors.New("audio inbound token is invalid")
	// ErrInvalidInput 表示外部播放请求缺失必要字段或字段非法。
	ErrInvalidInput = errors.New("audio play request is invalid")
	// ErrFileTooLarge 表示音频文件超过配置的大小上限。
	ErrFileTooLarge = errors.New("audio file is too large")
	// ErrURLRejected 表示 URL 下载策略拒绝该地址。
	ErrURLRejected = errors.New("audio url is rejected")
	// ErrMIMERejected 表示音频 MIME 类型不在第一版允许范围内。
	ErrMIMERejected = errors.New("audio mime type is rejected")
	// ErrRecordNotFound 表示音频历史记录不存在。
	ErrRecordNotFound = errors.New("audio record not found")
	// ErrQueueFull 表示播放队列已满。
	ErrQueueFull = errors.New("audio playback queue is full")
	// ErrFFmpegUnavailable 表示后端播放所需 FFmpeg 不可用。
	ErrFFmpegUnavailable = errors.New("ffmpeg is unavailable")
	// ErrPlaybackFailed 表示后端播放过程失败。
	ErrPlaybackFailed = errors.New("audio playback failed")
)
