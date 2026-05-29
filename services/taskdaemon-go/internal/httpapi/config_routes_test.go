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
