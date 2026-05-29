package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"

	"taskdaemon/internal/config"
)

const (
	playbackSampleRate   = 44100
	playbackChannelCount = 2
)

// Player 定义后端音频播放适配器。
type Player interface {
	Play(context.Context, string, config.AudioConfig) error
}

// FFmpegOtoPlayer 使用 FFmpeg 转码为 WAV PCM，再交给 Oto 播放。
type FFmpegOtoPlayer struct {
	context *oto.Context
}

// NewFFmpegOtoPlayer 创建后端 Oto 播放器。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *FFmpegOtoPlayer: 后端播放适配器。
func NewFFmpegOtoPlayer() *FFmpegOtoPlayer {
	return &FFmpegOtoPlayer{}
}

// Play 将本地音频文件转码并阻塞播放完成。
//
// 参数:
//   - ctx: 控制转码和等待播放的上下文。
//   - path: 本地音频文件绝对路径。
//   - cfg: 音频配置快照。
//
// 返回值:
//   - error: FFmpeg 不可用、转码失败或 Oto 播放失败时返回错误。
func (player *FFmpegOtoPlayer) Play(ctx context.Context, path string, cfg config.AudioConfig) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: empty path", ErrPlaybackFailed)
	}
	pcm, err := transcodeToPCM(ctx, path, cfg)
	if err != nil {
		return err
	}
	return player.playPCM(ctx, pcm)
}

// playPCM 使用 Oto 播放 PCM 字节流。
//
// 参数:
//   - ctx: 控制等待播放完成的上下文。
//   - pcm: signed 16-bit little-endian stereo PCM 数据。
//
// 返回值:
//   - error: 声卡初始化或播放失败时返回错误。
func (player *FFmpegOtoPlayer) playPCM(ctx context.Context, pcm []byte) error {
	otoCtx, err := player.otoContext()
	if err != nil {
		return err
	}
	otoPlayer := otoCtx.NewPlayer(bytes.NewReader(pcm))
	otoPlayer.Play()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for otoPlayer.IsPlaying() {
		select {
		case <-ctx.Done():
			otoPlayer.Pause()
			return ctx.Err()
		case <-ticker.C:
		}
	}
	if err := otoPlayer.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrPlaybackFailed, err)
	}
	return nil
}

// otoContext 懒初始化 Oto context，并复用单进程唯一 context。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *oto.Context: 可创建播放器的 Oto context。
//   - error: 初始化失败时返回错误。
func (player *FFmpegOtoPlayer) otoContext() (*oto.Context, error) {
	if player.context != nil {
		return player.context, nil
	}
	options := &oto.NewContextOptions{
		SampleRate:   playbackSampleRate,
		ChannelCount: playbackChannelCount,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   100 * time.Millisecond,
	}
	ctx, ready, err := oto.NewContext(options)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlaybackFailed, err)
	}
	<-ready
	player.context = ctx
	return ctx, nil
}

// transcodeToPCM 使用 FFmpeg 把输入文件转换为 Oto 可播放的 PCM 字节。
//
// 参数:
//   - ctx: 控制 FFmpeg 进程生命周期的上下文。
//   - path: 本地音频文件绝对路径。
//   - cfg: 音频配置快照。
//
// 返回值:
//   - []byte: signed 16-bit little-endian stereo PCM 数据。
//   - error: FFmpeg 不可用、进程失败或输出为空时返回错误。
func transcodeToPCM(ctx context.Context, path string, cfg config.AudioConfig) ([]byte, error) {
	ffmpegPath, err := resolveFFmpegPath(cfg.FFmpeg.Path)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(cfg.FFmpeg.TranscodeTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-i", path,
		"-vn",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ar", "44100",
		"-ac", "2",
		"pipe:1",
	}
	cmd := exec.CommandContext(runCtx, ffmpegPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: ffmpeg timeout", ErrPlaybackFailed)
		}
		return nil, fmt.Errorf("%w: %s", ErrPlaybackFailed, summarizeProcessError(err, stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("%w: ffmpeg produced empty pcm", ErrPlaybackFailed)
	}
	return stdout.Bytes(), nil
}

// resolveFFmpegPath 按配置路径、内置二进制、PATH 的顺序定位 FFmpeg。
//
// 参数:
//   - configuredPath: 配置文件中的 FFmpeg 路径。
//
// 返回值:
//   - string: 可执行 FFmpeg 路径或命令名。
//   - error: 未找到 FFmpeg 时返回 ErrFFmpegUnavailable。
func resolveFFmpegPath(configuredPath string) (string, error) {
	if strings.TrimSpace(configuredPath) != "" {
		return configuredPath, nil
	}
	if bundled := bundledFFmpegPath(); bundled != "" {
		return bundled, nil
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("%w: ffmpeg", ErrFFmpegUnavailable)
	}
	return path, nil
}

// bundledFFmpegPath 返回应用内置 FFmpeg 的候选路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 存在的内置 FFmpeg 路径；不存在时返回空字符串。
func bundledFFmpegPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidates := []string{
		filepath.Join(filepath.Dir(execPath), "bin", name),
		filepath.Join(filepath.Dir(execPath), name),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// summarizeProcessError 生成不包含路径参数细节的进程错误摘要。
//
// 参数:
//   - err: 进程错误。
//   - stderr: FFmpeg stderr。
//
// 返回值:
//   - string: 截断后的错误摘要。
func summarizeProcessError(err error, stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		return truncate(stderr, 512)
	}
	if err == nil {
		return "unknown"
	}
	return truncate(err.Error(), 512)
}

// wavPCM 从 WAV 文件中读取 PCM 数据，用于测试或后续扩展。
//
// 参数:
//   - reader: WAV 文件 reader。
//
// 返回值:
//   - []byte: data chunk 中的 PCM 字节。
//   - error: WAV 格式非法时返回错误。
func wavPCM(reader io.Reader) ([]byte, error) {
	header := make([]byte, 12)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if string(header[:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("%w: invalid wav", ErrPlaybackFailed)
	}
	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(reader, chunkHeader); err != nil {
			return nil, err
		}
		size := binary.LittleEndian.Uint32(chunkHeader[4:8])
		data := make([]byte, size)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		if string(chunkHeader[:4]) == "data" {
			return data, nil
		}
		if size%2 == 1 {
			if _, err := io.CopyN(io.Discard, reader, 1); err != nil {
				return nil, err
			}
		}
	}
}

// truncate 按字节长度截断字符串。
//
// 参数:
//   - value: 待截断字符串。
//   - limit: 最大字节数。
//
// 返回值:
//   - string: 截断后的字符串。
func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
