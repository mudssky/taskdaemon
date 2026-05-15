package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthEndpoint 验证 API health endpoint 可用于后端单独启动探活。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(Options{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != responseEnvelopeSuccessCode {
		t.Fatalf("code = %d, want success", body.Code)
	}
	data, ok := body.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want map", body.Data)
	}
	if data["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", data["status"])
	}
}

// TestSwaggerRouteCanBeDisabled 验证 swagger route 默认关闭，避免生产默认暴露文档入口。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestSwaggerRouteCanBeDisabled(t *testing.T) {
	router := NewRouter(Options{EnableSwagger: false})
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestSwaggerRouteCanBeEnabled 验证配置打开后 swagger route 会被注册。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestSwaggerRouteCanBeEnabled(t *testing.T) {
	router := NewRouter(Options{EnableSwagger: true})
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
