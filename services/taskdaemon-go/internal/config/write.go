package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// configWriteMu 串行化配置文件写入，避免并发互相覆盖。
var configWriteMu sync.Mutex

// AudioSectionPatch 描述 audio section 的部分更新（指针 nil = 未提交）。
type AudioSectionPatch struct {
	Autoplay *AudioAutoplayPatch
	Playback *AudioPlaybackPatch
	Inbound  *AudioInboundPatch
	History  *AudioHistoryPatch
	FFmpeg   *AudioFFmpegPatch
}

// AudioAutoplayPatch 自动播放部分更新。
type AudioAutoplayPatch struct {
	Enabled *bool
	Target  *string
}

// AudioPlaybackPatch 播放队列部分更新。
type AudioPlaybackPatch struct {
	QueueLimit *int
}

// AudioInboundPatch 入站部分更新。
// Token 为明文；落盘时转为 TokenHash。
type AudioInboundPatch struct {
	Token    *string
	MaxBytes *int64
	URL      *AudioInboundURLPatch
}

// AudioInboundURLPatch URL 策略部分更新。
type AudioInboundURLPatch struct {
	AllowedSchemes         *[]string
	AllowPrivateNetworks   *bool
	AllowedHosts           *[]string
	DownloadTimeoutSeconds *int
	MaxRedirects           *int
}

// AudioHistoryPatch 历史保留部分更新。
type AudioHistoryPatch struct {
	Limit *int
}

// AudioFFmpegPatch FFmpeg 部分更新。
// Path / ProbePath 为 file_only，提交即拒绝。
type AudioFFmpegPatch struct {
	Path                    *string
	ProbePath               *string
	TranscodeTimeoutSeconds *int
}

// SectionWritePlan 校验通过后的写入计划。
type SectionWritePlan struct {
	// Section section 名。
	Section string
	// Applied 将热生效的字段路径。
	Applied []string
	// RestartRequired 需重启字段路径。
	RestartRequired []string
	// MergedAudio 合并后的 audio 配置（仅 audio section）。
	MergedAudio AudioConfig
	// FileAudioMap 写入 YAML 的 audio 子树（map 形式）。
	FileAudioMap map[string]any
}

// ResolveWritePath 解析 API 写入目标路径（与 Load 文件源对齐）。
//
// 规则：显式 --config 优先；否则若存在 local 配置则写 local（最高优先级文件源）；
// 否则写 base ConfigPath（不存在时创建父目录后写入）。
//
// 参数:
//   - opts: 与 daemon Load 相同的加载选项。
//
// 返回值:
//   - string: 可写配置文件绝对或逻辑路径。
//   - error: 路径解析失败。
func ResolveWritePath(opts LoadOptions) (string, error) {
	paths, err := ResolvePaths(opts)
	if err != nil {
		return "", &WriteError{Code: CodePathUnavailable, Message: "resolve config write path failed", Err: err}
	}
	if paths.ExplicitConfigPath {
		if paths.ConfigPath == "" {
			return "", &WriteError{Code: CodePathUnavailable, Message: "explicit config path is empty"}
		}
		return paths.ConfigPath, nil
	}
	if paths.LocalConfigExists && paths.LocalConfigPath != "" {
		return paths.LocalConfigPath, nil
	}
	if paths.ConfigPath == "" {
		return "", &WriteError{Code: CodePathUnavailable, Message: "config path is empty"}
	}
	return paths.ConfigPath, nil
}

// HashSecret 返回敏感明文的 SHA-256 hex（token 落盘）。
//
// 参数:
//   - plain: 明文。
//
// 返回值:
//   - string: hex 摘要；空输入返回空串。
func HashSecret(plain string) string {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// BuildAudioWritePlan 校验 audio 补丁并生成写入计划（不落盘）。
//
// 参数:
//   - current: 当前有效 AudioConfig。
//   - patch: 部分更新补丁。
//
// 返回值:
//   - SectionWritePlan: 写入计划。
//   - error: 校验失败返回 *ValidationError。
func BuildAudioWritePlan(current AudioConfig, patch AudioSectionPatch) (SectionWritePlan, error) {
	fields := collectAudioFieldErrors(patch)
	if len(fields) > 0 {
		return SectionWritePlan{}, validationErrorFromFields(fields)
	}

	merged := current
	var applied, restart []string

	if patch.Autoplay != nil {
		if patch.Autoplay.Enabled != nil {
			merged.Autoplay.Enabled = *patch.Autoplay.Enabled
			applied = appendUnique(applied, "audio.autoplay.enabled")
		}
		if patch.Autoplay.Target != nil {
			merged.Autoplay.Target = strings.TrimSpace(*patch.Autoplay.Target)
			applied = appendUnique(applied, "audio.autoplay.target")
		}
	}
	if patch.Playback != nil && patch.Playback.QueueLimit != nil {
		merged.Playback.QueueLimit = *patch.Playback.QueueLimit
		applied = appendUnique(applied, "audio.playback.queueLimit")
	}
	if patch.Inbound != nil {
		if patch.Inbound.Token != nil {
			merged.Inbound.TokenHash = HashSecret(*patch.Inbound.Token)
			applied = appendUnique(applied, "audio.inbound.token")
		}
		if patch.Inbound.MaxBytes != nil {
			merged.Inbound.MaxBytes = *patch.Inbound.MaxBytes
			applied = appendUnique(applied, "audio.inbound.maxBytes")
		}
		if patch.Inbound.URL != nil {
			urlPatch := patch.Inbound.URL
			if urlPatch.AllowedSchemes != nil {
				merged.Inbound.URL.AllowedSchemes = cleanStringSlice(*urlPatch.AllowedSchemes)
				applied = appendUnique(applied, "audio.inbound.url.allowedSchemes")
			}
			if urlPatch.AllowPrivateNetworks != nil {
				merged.Inbound.URL.AllowPrivateNetworks = *urlPatch.AllowPrivateNetworks
				applied = appendUnique(applied, "audio.inbound.url.allowPrivateNetworks")
			}
			if urlPatch.AllowedHosts != nil {
				merged.Inbound.URL.AllowedHosts = cleanStringSlice(*urlPatch.AllowedHosts)
				applied = appendUnique(applied, "audio.inbound.url.allowedHosts")
			}
			if urlPatch.DownloadTimeoutSeconds != nil {
				merged.Inbound.URL.DownloadTimeoutSeconds = *urlPatch.DownloadTimeoutSeconds
				applied = appendUnique(applied, "audio.inbound.url.downloadTimeoutSeconds")
			}
			if urlPatch.MaxRedirects != nil {
				merged.Inbound.URL.MaxRedirects = *urlPatch.MaxRedirects
				applied = appendUnique(applied, "audio.inbound.url.maxRedirects")
			}
		}
	}
	if patch.History != nil && patch.History.Limit != nil {
		merged.History.Limit = *patch.History.Limit
		applied = appendUnique(applied, "audio.history.limit")
	}
	if patch.FFmpeg != nil && patch.FFmpeg.TranscodeTimeoutSeconds != nil {
		merged.FFmpeg.TranscodeTimeoutSeconds = *patch.FFmpeg.TranscodeTimeoutSeconds
		restart = appendUnique(restart, "audio.ffmpeg.transcodeTimeoutSeconds")
	}

	// 合并后二次冲突校验
	if conflict := validateMergedAudio(merged); conflict != nil {
		return SectionWritePlan{}, validationErrorFromFields([]FieldError{*conflict})
	}

	return SectionWritePlan{
		Section:         "audio",
		Applied:         applied,
		RestartRequired: restart,
		MergedAudio:     merged,
		FileAudioMap:    audioConfigToFileMap(merged),
	}, nil
}

// PersistSectionMap 将 section map 原子写入配置文件（保留其他顶层 key）。
//
// 注意：目标文件会整体重新序列化，**不保留注释与原有格式**（已在 C-1 文档说明）。
//
// 参数:
//   - path: 目标配置文件路径。
//   - section: 顶层 section 名。
//   - sectionMap: section 内容 map。
//
// 返回值:
//   - []byte: 写入前备份内容；文件不存在时为 nil。
//   - error: 读写失败。
func PersistSectionMap(path string, section string, sectionMap map[string]any) ([]byte, error) {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	return persistSectionMapLocked(path, section, sectionMap)
}

// RestoreFileContent 用备份内容恢复配置文件（回滚）。
//
// 参数:
//   - path: 配置文件路径。
//   - backup: 备份字节；nil 表示删除文件。
//
// 返回值:
//   - error: 恢复失败。
func RestoreFileContent(path string, backup []byte) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	if backup == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return &WriteError{Code: CodeWriteFailed, Message: "remove config after failed write", Err: err}
		}
		return nil
	}
	return atomicWriteFile(path, backup)
}

func persistSectionMapLocked(path string, section string, sectionMap map[string]any) ([]byte, error) {
	var backup []byte
	root := map[string]any{}
	if content, err := os.ReadFile(path); err == nil {
		backup = append([]byte(nil), content...)
		if len(strings.TrimSpace(string(content))) > 0 {
			if err := yaml.Unmarshal(content, &root); err != nil {
				return nil, &WriteError{Code: CodeWriteFailed, Message: "parse existing config failed", Err: err}
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, &WriteError{Code: CodeWriteFailed, Message: "read existing config failed", Err: err}
	}
	if root == nil {
		root = map[string]any{}
	}
	root[section] = sectionMap
	encoded, err := yaml.Marshal(root)
	if err != nil {
		return nil, &WriteError{Code: CodeWriteFailed, Message: "marshal config failed", Err: err}
	}
	if err := atomicWriteFile(path, encoded); err != nil {
		return nil, err
	}
	return backup, nil
}

// atomicWriteFile 先写临时文件再 rename，避免写一半损坏。
//
// 参数:
//   - path: 目标路径。
//   - content: 完整文件内容。
//
// 返回值:
//   - error: 写入失败。
func atomicWriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &WriteError{Code: CodeWriteFailed, Message: "create config directory failed", Err: err}
	}
	tmp, err := os.CreateTemp(dir, ".taskdaemon-config-*.tmp")
	if err != nil {
		return &WriteError{Code: CodeWriteFailed, Message: "create temp config failed", Err: err}
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return &WriteError{Code: CodeWriteFailed, Message: "write temp config failed", Err: err}
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return &WriteError{Code: CodeWriteFailed, Message: "chmod temp config failed", Err: err}
	}
	if err := tmp.Close(); err != nil {
		return &WriteError{Code: CodeWriteFailed, Message: "close temp config failed", Err: err}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return &WriteError{Code: CodeWriteFailed, Message: "replace config failed", Err: err}
	}
	cleanup = false
	return nil
}

func collectAudioFieldErrors(patch AudioSectionPatch) []FieldError {
	var fields []FieldError
	if patch.Autoplay != nil && patch.Autoplay.Target != nil {
		target := strings.TrimSpace(*patch.Autoplay.Target)
		if target != "backend" && target != "frontend" {
			fields = append(fields, FieldError{
				Path:   "audio.autoplay.target",
				Reason: "must be backend or frontend",
				Code:   CodeFieldInvalid,
			})
		}
	}
	if patch.Playback != nil && patch.Playback.QueueLimit != nil && *patch.Playback.QueueLimit < 0 {
		fields = append(fields, FieldError{
			Path:   "audio.playback.queueLimit",
			Reason: "must be >= 0",
			Code:   CodeFieldOutOfRange,
		})
	}
	if patch.Inbound != nil {
		if patch.Inbound.MaxBytes != nil && *patch.Inbound.MaxBytes <= 0 {
			fields = append(fields, FieldError{
				Path:   "audio.inbound.maxBytes",
				Reason: "must be > 0",
				Code:   CodeFieldOutOfRange,
			})
		}
		if patch.Inbound.URL != nil {
			urlPatch := patch.Inbound.URL
			if urlPatch.AllowedSchemes != nil {
				for _, scheme := range *urlPatch.AllowedSchemes {
					scheme = strings.ToLower(strings.TrimSpace(scheme))
					if scheme != "http" && scheme != "https" {
						fields = append(fields, FieldError{
							Path:   "audio.inbound.url.allowedSchemes",
							Reason: "only http and https are allowed",
							Code:   CodeFieldInvalid,
						})
						break
					}
				}
			}
			if urlPatch.DownloadTimeoutSeconds != nil && *urlPatch.DownloadTimeoutSeconds <= 0 {
				fields = append(fields, FieldError{
					Path:   "audio.inbound.url.downloadTimeoutSeconds",
					Reason: "must be > 0",
					Code:   CodeFieldOutOfRange,
				})
			}
			if urlPatch.MaxRedirects != nil && *urlPatch.MaxRedirects < 0 {
				fields = append(fields, FieldError{
					Path:   "audio.inbound.url.maxRedirects",
					Reason: "must be >= 0",
					Code:   CodeFieldOutOfRange,
				})
			}
		}
	}
	if patch.History != nil && patch.History.Limit != nil && *patch.History.Limit < 0 {
		fields = append(fields, FieldError{
			Path:   "audio.history.limit",
			Reason: "must be >= 0",
			Code:   CodeFieldOutOfRange,
		})
	}
	if patch.FFmpeg != nil {
		if patch.FFmpeg.Path != nil {
			fields = append(fields, FieldError{
				Path:   "audio.ffmpeg.path",
				Reason: "file-only field; edit config file instead",
				Code:   CodeFieldNotWritable,
			})
		}
		if patch.FFmpeg.ProbePath != nil {
			fields = append(fields, FieldError{
				Path:   "audio.ffmpeg.probePath",
				Reason: "file-only field; edit config file instead",
				Code:   CodeFieldNotWritable,
			})
		}
		if patch.FFmpeg.TranscodeTimeoutSeconds != nil && *patch.FFmpeg.TranscodeTimeoutSeconds <= 0 {
			fields = append(fields, FieldError{
				Path:   "audio.ffmpeg.transcodeTimeoutSeconds",
				Reason: "must be > 0",
				Code:   CodeFieldOutOfRange,
			})
		}
	}
	return fields
}

func validateMergedAudio(cfg AudioConfig) *FieldError {
	if len(cfg.Inbound.URL.AllowedSchemes) == 0 {
		return &FieldError{
			Path:   "audio.inbound.url.allowedSchemes",
			Reason: "must not be empty after update",
			Code:   CodeFieldConflict,
		}
	}
	return nil
}

func validationErrorFromFields(fields []FieldError) *ValidationError {
	if len(fields) == 0 {
		return &ValidationError{Code: CodeValidationFailed, Message: "validation failed"}
	}
	code := fields[0].Code
	for _, field := range fields[1:] {
		if field.Code != code {
			code = CodeValidationFailed
			break
		}
	}
	return &ValidationError{
		Code:    code,
		Message: "config validation failed",
		Fields:  fields,
	}
}

func audioConfigToFileMap(cfg AudioConfig) map[string]any {
	return map[string]any{
		"autoplay": map[string]any{
			"enabled": cfg.Autoplay.Enabled,
			"target":  cfg.Autoplay.Target,
		},
		"playback": map[string]any{
			"queueLimit": cfg.Playback.QueueLimit,
		},
		"inbound": map[string]any{
			"tokenHash": cfg.Inbound.TokenHash,
			"maxBytes":  cfg.Inbound.MaxBytes,
			"url": map[string]any{
				"allowedSchemes":         cfg.Inbound.URL.AllowedSchemes,
				"allowPrivateNetworks":   cfg.Inbound.URL.AllowPrivateNetworks,
				"allowedHosts":           cfg.Inbound.URL.AllowedHosts,
				"downloadTimeoutSeconds": cfg.Inbound.URL.DownloadTimeoutSeconds,
				"maxRedirects":           cfg.Inbound.URL.MaxRedirects,
			},
		},
		"history": map[string]any{
			"limit": cfg.History.Limit,
		},
		"ffmpeg": map[string]any{
			"path":                    cfg.FFmpeg.Path,
			"probePath":               cfg.FFmpeg.ProbePath,
			"transcodeTimeoutSeconds": cfg.FFmpeg.TranscodeTimeoutSeconds,
		},
	}
}

func appendUnique(values []string, item string) []string {
	for _, value := range values {
		if value == item {
			return values
		}
	}
	return append(values, item)
}

// EnsureWritePathParent 确保写入路径父目录存在（测试辅助导出语义在 atomicWriteFile 内已处理）。
//
// 参数:
//   - path: 文件路径。
//
// 返回值:
//   - error: 创建失败。
func EnsureWritePathParent(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
