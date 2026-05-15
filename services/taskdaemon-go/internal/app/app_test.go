package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"taskdaemon/internal/config"
	"taskdaemon/internal/httpapi"
)

// TestTriggerTaskCallsDaemonAPIWithSessionCookie 验证 CLI trigger hook 会调用 daemon HTTP API 并携带 session cookie。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTriggerTaskCallsDaemonAPIWithSessionCookie(t *testing.T) {
	var requestedPath string
	var sessionToken string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		requestedPath = req.URL.Path
		cookie, err := req.Cookie(httpapi.SessionCookieName)
		if err != nil {
			t.Fatalf("session cookie missing: %v", err)
		}
		sessionToken = cookie.Value
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":11,"status":"success"}`))
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Server.Host, cfg.Server.Port = serverHostPort(t, server.URL)

	err := New(cfg, discardLogger()).TriggerTask(context.Background(), 42, "session-token")

	if err != nil {
		t.Fatalf("trigger task: %v", err)
	}
	if requestedPath != "/api/tasks/42/trigger" {
		t.Fatalf("request path = %s, want /api/tasks/42/trigger", requestedPath)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
}

// TestTriggerTaskRequiresSessionToken 验证手动触发任务时必须提供管理员 session token。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTriggerTaskRequiresSessionToken(t *testing.T) {
	err := New(config.Default(), discardLogger()).TriggerTask(context.Background(), 42, "")

	if err == nil {
		t.Fatal("trigger task should require session token")
	}
}

// TestCancelTaskCallsDaemonAPIWithSessionCookie 验证 CLI cancel hook 会调用 daemon HTTP API 并携带 session cookie。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestCancelTaskCallsDaemonAPIWithSessionCookie(t *testing.T) {
	var requestedPath string
	var sessionToken string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		requestedPath = req.URL.Path
		cookie, err := req.Cookie(httpapi.SessionCookieName)
		if err != nil {
			t.Fatalf("session cookie missing: %v", err)
		}
		sessionToken = cookie.Value
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Server.Host, cfg.Server.Port = serverHostPort(t, server.URL)

	err := New(cfg, discardLogger()).CancelTask(context.Background(), 42, "session-token")

	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if requestedPath != "/api/tasks/42/cancel" {
		t.Fatalf("request path = %s, want /api/tasks/42/cancel", requestedPath)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
}

// TestCancelTaskRequiresSessionToken 验证取消任务时必须提供管理员 session token。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestCancelTaskRequiresSessionToken(t *testing.T) {
	err := New(config.Default(), discardLogger()).CancelTask(context.Background(), 42, "")

	if err == nil {
		t.Fatal("cancel task should require session token")
	}
}

// TestDaemonClientAddressUsesLoopbackForWildcardHost 验证 wildcard 监听地址会转换为 CLI 可访问的 loopback 地址。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestDaemonClientAddressUsesLoopbackForWildcardHost(t *testing.T) {
	address := daemonClientAddress(config.ServerConfig{Host: "0.0.0.0", Port: 8080})

	if address != "127.0.0.1:8080" {
		t.Fatalf("address = %s, want 127.0.0.1:8080", address)
	}
}

// serverHostPort 从 httptest server URL 中提取 host 和 port。
//
// 参数:
//   - t: Go 测试上下文。
//   - serverURL: httptest server URL。
//
// 返回值:
//   - string: server host。
//   - int: server port。
func serverHostPort(t *testing.T, serverURL string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(serverURL, "http://"))
	if err != nil {
		t.Fatalf("split server URL: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	return host, port
}

// discardLogger 返回测试使用的静默 logger。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *slog.Logger: 输出到 io.Discard 的 logger。
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
