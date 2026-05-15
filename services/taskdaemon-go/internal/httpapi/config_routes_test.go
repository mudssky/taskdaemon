package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
}
