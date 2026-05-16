package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
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

// TestFrontendFallbackServesIndex 验证启用前端静态资源后，根路径和 SPA 路径返回 index.html。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestFrontendFallbackServesIndex(t *testing.T) {
	router := NewRouter(Options{FrontendFS: testFrontendFS()})
	for _, target := range []string{"/", "/tasks/7"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "<html>taskdaemon</html>" {
			t.Fatalf("%s body = %q, want index html", target, rec.Body.String())
		}
	}
}

// TestFrontendStaticAssetServed 验证前端静态资源路径会返回真实资源。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestFrontendStaticAssetServed(t *testing.T) {
	router := NewRouter(Options{FrontendFS: testFrontendFS()})
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "console.log('ok')" {
		t.Fatalf("body = %q, want asset", rec.Body.String())
	}
}

// TestFrontendFallbackDoesNotHandleReservedBackendPaths 验证 API 和 Swagger 路径不会落到前端 fallback。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestFrontendFallbackDoesNotHandleReservedBackendPaths(t *testing.T) {
	router := NewRouter(Options{FrontendFS: testFrontendFS()})
	for _, target := range []string{"/api/missing", "/swagger/missing"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusNotFound)
		}
		var body apiResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode body: %v", target, err)
		}
		if body.Code != responseEnvelopeErrorCode {
			t.Fatalf("%s code = %d, want error", target, body.Code)
		}
	}
}

// testFrontendFS 返回 router 测试用的前端静态资源文件系统。
//
// 参数:
//   - 无。
//
// 返回值:
//   - fstest.MapFS: 包含 index 和一个静态资源的测试文件系统。
func testFrontendFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>taskdaemon</html>")},
		"assets/app.js": {Data: []byte("console.log('ok')")},
	}
}
