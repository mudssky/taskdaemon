package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
)

// TestTraceIDIsReusedInHeaderBodyAndLog 验证 trace id 会复用请求头并进入响应和日志。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTraceIDIsReusedInHeaderBodyAndLog(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := NewRouter(Options{
		Logger: logger,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(traceIDHeader, "trace-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Header().Get(traceIDHeader) != "trace-123" {
		t.Fatalf("trace header = %q, want trace-123", rec.Header().Get(traceIDHeader))
	}
	var body struct {
		TraceID string `json:"traceId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TraceID != "trace-123" {
		t.Fatalf("body traceId = %q, want trace-123", body.TraceID)
	}
	if strings.Contains(logs.String(), "trace-123") {
		t.Fatalf("health success should not be logged, got %q", logs.String())
	}
}

// TestTraceIDCanBeHiddenFromResponseBody 验证配置关闭时响应体不包含 traceId。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTraceIDCanBeHiddenFromResponseBody(t *testing.T) {
	includeTrace := false
	router := NewRouter(Options{IncludeTraceInResponse: &includeTrace})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Header().Get(traceIDHeader) == "" {
		t.Fatal("trace header should be written")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["traceId"]; ok {
		t.Fatalf("body should not include traceId: %s", rec.Body.String())
	}
}

// TestRequestLoggerLogsFailuresWithTraceID 验证失败请求会记录方法、路径、状态、耗时和 trace_id。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRequestLoggerLogsFailuresWithTraceID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := NewRouter(Options{
		Logger: logger,
		Auth:   &fakeAuthService{authErr: auth.ErrInvalidSession},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set(traceIDHeader, "trace-unauthorized")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	text := logs.String()
	if !strings.Contains(text, "method=GET") ||
		!strings.Contains(text, "path=/api/auth/me") ||
		!strings.Contains(text, "status=401") ||
		!strings.Contains(text, "duration_ms=") ||
		!strings.Contains(text, "trace_id=trace-unauthorized") {
		t.Fatalf("request log missing fields: %q", text)
	}
	if strings.Contains(text, "taskdaemon_session") || strings.Contains(text, "Cookie") {
		t.Fatalf("request log leaked cookie data: %q", text)
	}
}

// TestRecoveryMiddlewareUsesAPIEnvelope 验证 panic 会被转换为统一错误响应并记录 trace_id。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRecoveryMiddlewareUsesAPIEnvelope(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := gin.New()
	router.Use(traceIDMiddleware())
	router.Use(responseOptionsMiddleware(NewRuntimeConfig(config.Default())))
	router.Use(requestLoggerMiddleware(logger, NewRuntimeConfig(config.Default())))
	router.Use(recoveryMiddleware())
	router.GET("/panic", func(*gin.Context) {
		panic("secret panic value")
	})
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(traceIDHeader, "trace-panic")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	envelope := decodeAPIError(t, rec.Body.Bytes())
	if envelope.Error.Code != "internal_error" {
		t.Fatalf("error code = %s, want internal_error", envelope.Error.Code)
	}
	text := logs.String()
	if !strings.Contains(text, "trace_id=trace-panic") || !strings.Contains(text, "status=500") {
		t.Fatalf("panic request log missing fields: %q", text)
	}
	if strings.Contains(text, "secret panic value") || strings.Contains(rec.Body.String(), "secret panic value") {
		t.Fatalf("panic value leaked: log=%q body=%q", text, rec.Body.String())
	}
}

// TestRequestLoggerDoesNotLogBodiesByDefault 验证默认请求日志不会记录请求体或响应体。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRequestLoggerDoesNotLogBodiesByDefault(t *testing.T) {
	var logs bytes.Buffer
	router := bodyLoggingTestRouter(t, &logs, config.LoggingHTTPConfig{})
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"password":"secret","name":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	text := logs.String()
	if strings.Contains(text, "request_body") || strings.Contains(text, "response_body") || strings.Contains(text, "secret") {
		t.Fatalf("body fields should not be logged by default: %q", text)
	}
}

// TestRequestLoggerCanLogRedactedBodies 验证开启后请求体与响应体会脱敏后进入日志。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRequestLoggerCanLogRedactedBodies(t *testing.T) {
	var logs bytes.Buffer
	router := bodyLoggingTestRouter(t, &logs, config.LoggingHTTPConfig{
		IncludeRequestBody:  true,
		IncludeResponseBody: true,
		MaxBodyBytes:        1024,
		RedactFields:        []string{"password", "token"},
	})
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"username":"admin","password":"secret","nested":{"token":"abc"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	text := logs.String()
	if !strings.Contains(text, "request_body=") || !strings.Contains(text, "response_body=") {
		t.Fatalf("body fields missing: %q", text)
	}
	if !strings.Contains(text, redactedLogValue) {
		t.Fatalf("redacted marker missing: %q", text)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "abc") {
		t.Fatalf("sensitive values leaked: %q", text)
	}
	if !strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("handler should still receive original request body, response=%q", rec.Body.String())
	}
}

// TestRequestLoggerMarksTruncatedBodies 验证 body 超过限制时记录截断标记，且 handler 仍收到完整请求。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestRequestLoggerMarksTruncatedBodies(t *testing.T) {
	var logs bytes.Buffer
	router := bodyLoggingTestRouter(t, &logs, config.LoggingHTTPConfig{
		IncludeRequestBody:  true,
		IncludeResponseBody: true,
		MaxBodyBytes:        8,
	})
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"message":"hello world"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	text := logs.String()
	if !strings.Contains(text, "request_body_truncated=true") || !strings.Contains(text, "response_body_truncated=true") {
		t.Fatalf("truncated markers missing: %q", text)
	}
	if !strings.Contains(rec.Body.String(), "hello world") {
		t.Fatalf("handler should receive full request body, response=%q", rec.Body.String())
	}
}

// bodyLoggingTestRouter 创建用于验证 body 日志的 router。
//
// 参数:
//   - t: Go 测试上下文。
//   - logs: 日志输出缓冲区。
//   - cfg: HTTP 日志配置。
//
// 返回值:
//   - http.Handler: 测试 router。
func bodyLoggingTestRouter(t *testing.T, logs *bytes.Buffer, cfg config.LoggingHTTPConfig) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(logs, nil))
	appConfig := config.Default()
	appConfig.Logging.HTTP = cfg
	runtimeConfig := NewRuntimeConfig(appConfig)
	router := gin.New()
	router.Use(traceIDMiddleware())
	router.Use(responseOptionsMiddleware(runtimeConfig))
	router.Use(requestLoggerMiddleware(logger, runtimeConfig))
	router.POST("/echo", func(ctx *gin.Context) {
		body, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		ctx.Data(http.StatusOK, "application/json", body)
	})
	return router
}
