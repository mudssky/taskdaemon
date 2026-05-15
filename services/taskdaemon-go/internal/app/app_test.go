package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

// TestServeMigratesEmptySQLiteBeforeRegisteringTasks 验证空 SQLite 首次启动会先迁移 schema，再注册任务并提供认证状态 API。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestServeMigratesEmptySQLiteBeforeRegisteringTasks(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = freeTCPPort(t)
	cfg.Database = config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.Join(t.TempDir(), "taskdaemon.db") + "?_fk=1",
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- New(cfg, discardLogger()).Serve(ctx)
	}()

	status := waitForAuthStatus(t, fmt.Sprintf("http://%s/api/auth/status", cfg.Server.Address()), errCh)
	if status.Initialized {
		t.Fatal("auth status initialized = true, want false for empty database")
	}
	if status.Authenticated {
		t.Fatal("auth status authenticated = true, want false for empty database")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("serve shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not stop after context cancellation")
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

type authStatus struct {
	Initialized   bool `json:"initialized"`
	Authenticated bool `json:"authenticated"`
}

// waitForAuthStatus 等待测试 HTTP server 返回认证状态。
//
// 参数:
//   - t: Go 测试上下文。
//   - endpoint: 认证状态 API 地址。
//   - errCh: Serve 返回错误的 channel，用于提前发现启动失败。
//
// 返回值:
//   - authStatus: API 返回的认证状态。
func waitForAuthStatus(t *testing.T, endpoint string, errCh <-chan error) authStatus {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			t.Fatalf("serve exited before auth status was available: %v", err)
		case <-deadline:
			t.Fatal("auth status API did not become available")
		case <-ticker.C:
			status, ok := requestAuthStatus(client, endpoint)
			if ok {
				return status
			}
		}
	}
}

// requestAuthStatus 请求认证状态 API。
//
// 参数:
//   - client: HTTP client。
//   - endpoint: 认证状态 API 地址。
//
// 返回值:
//   - authStatus: API 返回的认证状态。
//   - bool: true 表示请求成功且响应可解析。
func requestAuthStatus(client *http.Client, endpoint string) (authStatus, bool) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return authStatus{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return authStatus{}, false
	}
	var status authStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return authStatus{}, false
	}
	return status, true
}

// freeTCPPort 返回本机当前可用的 TCP 端口。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - int: 当前测试可尝试监听的端口。
func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
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
