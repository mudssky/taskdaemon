package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
)

// TestConfigReloadRequiresSession 验证配置重载接口需要管理员登录态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestConfigReloadRequiresSession(t *testing.T) {
	router := NewRouter(Options{
		Auth: &fakeAuthService{authErr: auth.ErrInvalidSession},
		ReloadConfig: func(context.Context) (config.ReloadResult, error) {
			t.Fatal("reload should not be called without session")
			return config.ReloadResult{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestAudioConfigReturnsSafeRuntimeSnapshot 验证音频配置接口只返回安全运行时状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestAudioConfigReturnsSafeRuntimeSnapshot(t *testing.T) {
	cfg := config.Default()
	cfg.Audio.Autoplay.Enabled = true
	cfg.Audio.Inbound.TokenHash = "secret-hash"
	cfg.Audio.FFmpeg.Path = "C:/secret/ffmpeg.exe"
	router := NewRouter(Options{
		Auth:          loggedInAuthService(),
		RuntimeConfig: NewRuntimeConfig(cfg),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/config/audio", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response audioConfigResponse
	decodeAPIData(t, rec.Body.Bytes(), &response)
	if !response.Autoplay.Enabled || !response.Inbound.TokenConfigured {
		t.Fatalf("audio config response = %#v, want enabled and token configured", response)
	}
	body := rec.Body.String()
	if body == "" || strings.Contains(body, "secret-hash") {
		t.Fatalf("response leaked token hash: %s", rec.Body.String())
	}
	if strings.Contains(body, "C:/secret/ffmpeg.exe") {
		t.Fatalf("response leaked ffmpeg path: %s", rec.Body.String())
	}
}

// TestConfigReloadReturnsResult 验证配置重载接口返回统一 envelope 和重载结果。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestConfigReloadReturnsResult(t *testing.T) {
	router := NewRouter(Options{
		Auth: loggedInAuthService(),
		ReloadConfig: func(context.Context) (config.ReloadResult, error) {
			return config.ReloadResult{
				Applied:         []string{"logging.http"},
				RestartRequired: []string{"server"},
			}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result config.ReloadResult
	decodeAPIData(t, rec.Body.Bytes(), &result)
	if len(result.Applied) != 1 || result.Applied[0] != "logging.http" {
		t.Fatalf("applied = %#v, want logging.http", result.Applied)
	}
	if len(result.RestartRequired) != 1 || result.RestartRequired[0] != "server" {
		t.Fatalf("restart required = %#v, want server", result.RestartRequired)
	}
}

// TestRuntimeConfigApplyUpdatesRequestLogger 验证运行时配置更新后请求日志会读取新配置。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRuntimeConfigApplyUpdatesRequestLogger(t *testing.T) {
	cfg := config.Default()
	runtimeConfig := NewRuntimeConfig(cfg)

	updated := config.Default()
	updated.Logging.HTTP.IncludeRequestBody = true
	updated.Logging.HTTP.IncludeResponseBody = true
	updated.Logging.HTTP.MaxBodyBytes = 512
	result := runtimeConfig.Apply(updated)

	if len(result.Applied) == 0 {
		t.Fatal("reload result should include applied groups")
	}
	if !runtimeConfig.HTTPLog().IncludeRequestBody || !runtimeConfig.HTTPLog().IncludeResponseBody {
		content, _ := json.Marshal(runtimeConfig.HTTPLog())
		t.Fatalf("runtime http logging not updated: %s", content)
	}
	updated.Audio.Autoplay.Enabled = true
	updated.Audio.Playback.QueueLimit = 7
	result = runtimeConfig.Apply(updated)
	if !containsString(result.Applied, "audio.autoplay") {
		t.Fatalf("applied = %#v, want audio.autoplay", result.Applied)
	}
	if !runtimeConfig.Audio().Autoplay.Enabled || runtimeConfig.Audio().Playback.QueueLimit != 7 {
		content, _ := json.Marshal(runtimeConfig.Audio())
		t.Fatalf("runtime audio config not updated: %s", content)
	}
}

// containsString 判断字符串切片是否包含目标值。
//
// 参数:
//   - values: 候选字符串。
//   - target: 目标字符串。
//
// 返回值:
//   - bool: 包含时返回 true。
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// TestPutAudioConfigRequiresSession 验证配置写入需要管理员 session。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigRequiresSession(t *testing.T) {
	router := NewRouter(Options{
		Auth: &fakeAuthService{authErr: auth.ErrInvalidSession},
		WriteConfig: DefaultConfigSectionWriter(func(context.Context, config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			t.Fatal("write should not run without session")
			return ConfigSectionWriteResponse{}, nil
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{"autoplay":{"enabled":true}}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestPutAudioConfigPartialUpdate 验证部分更新与响应形状。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigPartialUpdate(t *testing.T) {
	cfg := config.Default()
	runtime := NewRuntimeConfig(cfg)
	var gotPatch config.AudioSectionPatch
	router := NewRouter(Options{
		Auth:          loggedInAuthService(),
		RuntimeConfig: runtime,
		WriteConfig: DefaultConfigSectionWriter(func(_ context.Context, patch config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			gotPatch = patch
			cfg.Audio.Autoplay.Enabled = true
			runtime.Apply(cfg)
			return ConfigSectionWriteResponse{
				Config:  audioConfigResponseFromConfig(cfg.Audio),
				Applied: []string{"audio.autoplay.enabled"},
				Reload: ConfigSectionReloadResponse{
					Applied: []string{"audio.autoplay.enabled"},
					Subsystems: []ConfigSubsystemStatusResponse{
						{Name: "runtime", Status: "ok"},
						{Name: "audio", Status: "ok"},
					},
				},
			}, nil
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{"autoplay":{"enabled":true}}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotPatch.Autoplay == nil || gotPatch.Autoplay.Enabled == nil || !*gotPatch.Autoplay.Enabled {
		t.Fatalf("patch = %#v", gotPatch)
	}
	if gotPatch.Playback != nil {
		t.Fatal("unsubmitted playback should stay nil")
	}
	var response ConfigSectionWriteResponse
	decodeAPIData(t, rec.Body.Bytes(), &response)
	if len(response.Applied) != 1 || response.Applied[0] != "audio.autoplay.enabled" {
		t.Fatalf("applied = %#v", response.Applied)
	}
	if len(response.Reload.Subsystems) == 0 {
		t.Fatal("subsystems required")
	}
	if !runtime.Audio().Autoplay.Enabled {
		t.Fatal("runtime should hot-apply")
	}
}

// TestPutAudioConfigRejectsFileOnlyField 验证 file_only 字段被拒绝。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigRejectsFileOnlyField(t *testing.T) {
	router := NewRouter(Options{
		Auth: loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(func(context.Context, config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			t.Fatal("write should not run")
			return ConfigSectionWriteResponse{}, nil
		}),
	})
	// 走真实校验：使用 config.Build 路径 — writer 收到 patch 后由业务校验；
	// 这里直接在 writer 前由 audioPatch + Build 模拟：Default writer 只转 patch，
	// file_only 在 BuildAudioWritePlan。注入真实 plan 校验：
	router = NewRouter(Options{
		Auth: loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(func(_ context.Context, patch config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			_, err := config.BuildAudioWritePlan(config.Default().Audio, patch)
			return ConfigSectionWriteResponse{}, err
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{"ffmpeg":{"path":"/secret/ffmpeg"}}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	envelope := decodeAPIError(t, rec.Body.Bytes())
	if envelope.Error.Code != config.CodeFieldNotWritable {
		t.Fatalf("code = %s", envelope.Error.Code)
	}
	details, _ := json.Marshal(envelope.Error.Details)
	if !strings.Contains(string(details), "audio.ffmpeg.path") {
		t.Fatalf("details = %s", details)
	}
}

// TestPutAudioConfigRejectsTokenHashDirectWrite 验证不可直接写 tokenHash。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigRejectsTokenHashDirectWrite(t *testing.T) {
	router := NewRouter(Options{
		Auth: loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(func(context.Context, config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			t.Fatal("should fail before write")
			return ConfigSectionWriteResponse{}, nil
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{"inbound":{"tokenHash":"abc"}}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	envelope := decodeAPIError(t, rec.Body.Bytes())
	if envelope.Error.Code != config.CodeFieldNotWritable {
		t.Fatalf("code = %s", envelope.Error.Code)
	}
}

// TestPutAudioConfigUnknownSection 验证未知 section。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigUnknownSection(t *testing.T) {
	router := NewRouter(Options{
		Auth: loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(func(context.Context, config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			t.Fatal("should not write")
			return ConfigSectionWriteResponse{}, nil
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/server", strings.NewReader(`{"host":"0.0.0.0"}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	envelope := decodeAPIError(t, rec.Body.Bytes())
	if envelope.Error.Code != config.CodeSectionUnknown {
		t.Fatalf("code = %s", envelope.Error.Code)
	}
}

// TestPutAudioConfigInvalidJSON 验证非法 JSON。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigInvalidJSON(t *testing.T) {
	router := NewRouter(Options{
		Auth:        loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(nil),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	envelope := decodeAPIError(t, rec.Body.Bytes())
	if envelope.Error.Code != config.CodeInvalidJSON {
		t.Fatalf("code = %s", envelope.Error.Code)
	}
}

// TestPutAudioConfigTokenNotLeaked 验证 token 不明文回显。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestPutAudioConfigTokenNotLeaked(t *testing.T) {
	router := NewRouter(Options{
		Auth: loggedInAuthService(),
		WriteConfig: DefaultConfigSectionWriter(func(_ context.Context, patch config.AudioSectionPatch) (ConfigSectionWriteResponse, error) {
			cfg := config.Default().Audio
			if patch.Inbound != nil && patch.Inbound.Token != nil {
				cfg.Inbound.TokenHash = config.HashSecret(*patch.Inbound.Token)
			}
			return ConfigSectionWriteResponse{
				Config:  audioConfigResponseFromConfig(cfg),
				Applied: []string{"audio.inbound.token"},
				Reload: ConfigSectionReloadResponse{
					Applied:    []string{"audio.inbound.token"},
					Subsystems: []ConfigSubsystemStatusResponse{{Name: "runtime", Status: "ok"}},
				},
			}, nil
		}),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/audio", strings.NewReader(`{"inbound":{"token":"super-secret-token"}}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "super-secret-token") {
		t.Fatalf("leaked token: %s", body)
	}
	if !strings.Contains(body, `"tokenConfigured":true`) {
		t.Fatalf("want tokenConfigured true: %s", body)
	}
}
