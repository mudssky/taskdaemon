package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildAudioWritePlanPartialKeepsUnsubmitted(t *testing.T) {
	current := Default().Audio
	current.Autoplay.Enabled = false
	current.Playback.QueueLimit = 20
	enabled := true
	plan, err := BuildAudioWritePlan(current, AudioSectionPatch{
		Autoplay: &AudioAutoplayPatch{Enabled: &enabled},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.MergedAudio.Autoplay.Enabled {
		t.Fatal("enabled should update")
	}
	if plan.MergedAudio.Playback.QueueLimit != 20 {
		t.Fatalf("queueLimit = %d, want unchanged 20", plan.MergedAudio.Playback.QueueLimit)
	}
	if len(plan.Applied) != 1 || plan.Applied[0] != "audio.autoplay.enabled" {
		t.Fatalf("applied = %#v", plan.Applied)
	}
}

func TestBuildAudioWritePlanRejectsFileOnlyPath(t *testing.T) {
	path := "/secret/ffmpeg"
	_, err := BuildAudioWritePlan(Default().Audio, AudioSectionPatch{
		FFmpeg: &AudioFFmpegPatch{Path: &path},
	})
	ve, ok := AsValidationError(err)
	if !ok {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if ve.Code != CodeFieldNotWritable {
		t.Fatalf("code = %s, want %s", ve.Code, CodeFieldNotWritable)
	}
	if len(ve.Fields) == 0 || ve.Fields[0].Path != "audio.ffmpeg.path" {
		t.Fatalf("fields = %#v", ve.Fields)
	}
}

func TestBuildAudioWritePlanOutOfRange(t *testing.T) {
	limit := -1
	_, err := BuildAudioWritePlan(Default().Audio, AudioSectionPatch{
		Playback: &AudioPlaybackPatch{QueueLimit: &limit},
	})
	ve, ok := AsValidationError(err)
	if !ok || ve.Code != CodeFieldOutOfRange {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildAudioWritePlanTokenHashes(t *testing.T) {
	token := "plain-secret"
	plan, err := BuildAudioWritePlan(Default().Audio, AudioSectionPatch{
		Inbound: &AudioInboundPatch{Token: &token},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.MergedAudio.Inbound.TokenHash != HashSecret(token) {
		t.Fatalf("hash = %s", plan.MergedAudio.Inbound.TokenHash)
	}
	if plan.MergedAudio.Inbound.TokenHash == token {
		t.Fatal("token must not stay plaintext")
	}
}

func TestPersistSectionMapAtomicAndPartialRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	initial := []byte("server:\n  port: 1\naudio:\n  autoplay:\n    enabled: false\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	current := Default().Audio
	enabled := true
	plan, err := BuildAudioWritePlan(current, AudioSectionPatch{
		Autoplay: &AudioAutoplayPatch{Enabled: &enabled},
	})
	if err != nil {
		t.Fatal(err)
	}
	backup, err := PersistSectionMap(path, "audio", plan.FileAudioMap)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if string(backup) != string(initial) {
		t.Fatalf("backup mismatch")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	if root["server"] == nil {
		t.Fatal("server section must be preserved")
	}
	audioMap, _ := root["audio"].(map[string]any)
	autoplay, _ := audioMap["autoplay"].(map[string]any)
	if autoplay["enabled"] != true {
		t.Fatalf("audio.autoplay.enabled = %#v", autoplay["enabled"])
	}
}

func TestFieldRegistryCoversKnownSections(t *testing.T) {
	meta, ok := FieldClassOf("audio.ffmpeg.path")
	if !ok || meta.Class != FieldClassFileOnly {
		t.Fatalf("ffmpeg.path class = %#v ok=%v", meta, ok)
	}
	meta, ok = FieldClassOf("logging.http.maxBodyBytes")
	if !ok || meta.Class != FieldClassHot {
		t.Fatalf("logging.http class = %#v", meta)
	}
	if !IsSectionWritable("audio") {
		t.Fatal("audio should be writable")
	}
	if IsSectionWritable("server") {
		t.Fatal("server should not be writable section")
	}
}

func TestResolveWritePathPrefersLocal(t *testing.T) {
	project := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	// mark as workspace so project discovery works
	if err := os.WriteFile(filepath.Join(project, "pnpm-workspace.yaml"), []byte("packages: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "taskdaemon.yaml"), []byte("server:\n  port: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "taskdaemon.local.yaml"), []byte("server:\n  port: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := ResolveWritePath(LoadOptions{Optional: true})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "taskdaemon.local.yaml" {
		t.Fatalf("write path = %s, want local", path)
	}
}
