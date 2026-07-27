package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskdaemon/internal/config"
	"taskdaemon/internal/httpapi"
)

// TestWriteAudioConfigHotApplyAndPersist 验证 audio 写入落盘且热生效。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWriteAudioConfigHotApplyAndPersist(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("audio:\n  autoplay:\n    enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := config.LoadOptions{ConfigPath: configPath}
	cfg, err := config.Load(opts)
	if err != nil {
		t.Fatal(err)
	}
	runtime := httpapi.NewRuntimeConfig(cfg)
	application := New(cfg, nil).WithConfigReload(config.Load, opts)
	application.runtimeConfig = runtime

	enabled := true
	result, err := application.WriteAudioConfig(context.Background(), config.AudioSectionPatch{
		Autoplay: &config.AudioAutoplayPatch{Enabled: &enabled},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Config.Audio.Autoplay.Enabled {
		t.Fatal("effective config not updated")
	}
	if !runtime.Audio().Autoplay.Enabled {
		t.Fatal("runtime not hot applied")
	}
	if len(result.Applied) == 0 {
		t.Fatal("applied empty")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "enabled: true") {
		t.Fatalf("file content = %s", raw)
	}
}

// TestWriteAudioConfigValidationNoWrite 验证校验失败零写入。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWriteAudioConfigValidationNoWrite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	initial := []byte("audio:\n  playback:\n    queueLimit: 9\n")
	if err := os.WriteFile(configPath, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	opts := config.LoadOptions{ConfigPath: configPath}
	cfg, err := config.Load(opts)
	if err != nil {
		t.Fatal(err)
	}
	application := New(cfg, nil).WithConfigReload(config.Load, opts)
	application.runtimeConfig = httpapi.NewRuntimeConfig(cfg)

	limit := -3
	_, err = application.WriteAudioConfig(context.Background(), config.AudioSectionPatch{
		Playback: &config.AudioPlaybackPatch{QueueLimit: &limit},
	})
	if err == nil {
		t.Fatal("want validation error")
	}
	if _, ok := config.AsValidationError(err); !ok {
		t.Fatalf("err = %v", err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(initial) {
		t.Fatalf("file mutated: %s", raw)
	}
}
