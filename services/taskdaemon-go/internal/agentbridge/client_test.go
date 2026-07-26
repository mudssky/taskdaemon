package agentbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

// TestHashAndAuth 验证 token 哈希与入站鉴权（非 cookie）。
//
// 参数:
//   - t: 测试上下文。
func TestHashAndAuth(t *testing.T) {
	token := "machine-secret"
	cfg := config.AgentBridgeConfig{
		Enabled:          true,
		InboundTokenHash: HashToken(token),
	}
	require.NoError(t, AuthenticateInbound(cfg, token))
	require.ErrorIs(t, AuthenticateInbound(cfg, "wrong"), ErrUnauthorized)
	require.ErrorIs(t, AuthenticateInbound(config.AgentBridgeConfig{Enabled: false}, token), ErrDisabled)
}

// TestTaskAllowlistDefaultDeny 验证默认拒绝。
//
// 参数:
//   - t: 测试上下文。
func TestTaskAllowlistDefaultDeny(t *testing.T) {
	cfg := config.AgentBridgeConfig{Enabled: true, AllowedTaskIDs: nil}
	require.ErrorIs(t, AssertTaskAllowed(cfg, 1), ErrTaskNotAllowed)
	cfg.AllowedTaskIDs = []int{2, 3}
	require.NoError(t, AssertTaskAllowed(cfg, 2))
	require.ErrorIs(t, AssertTaskAllowed(cfg, 9), ErrTaskNotAllowed)
}

// TestLoopDepth 验证环路深度限制。
//
// 参数:
//   - t: 测试上下文。
func TestLoopDepth(t *testing.T) {
	cfg := config.AgentBridgeConfig{MaxLoopDepth: 1}
	require.NoError(t, AssertLoopDepth(cfg, 0))
	require.ErrorIs(t, AssertLoopDepth(cfg, 1), ErrLoopDetected)
	require.ErrorIs(t, AssertLoopDepth(cfg, 2), ErrLoopDetected)
}

// TestCreateAndWaitRunPassesTraceID 验证 traceId 透传且不重新生成。
//
// 参数:
//   - t: 测试上下文。
func TestCreateAndWaitRunPassesTraceID(t *testing.T) {
	var seenTrace []string
	var usedCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			usedCookie = true
		}
		seenTrace = append(seenTrace, r.Header.Get("x-trace-id"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v1/threads"):
			_ = json.NewEncoder(w).Encode(map[string]any{"threadId": "th-1", "status": "idle"})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/runs"):
			_ = json.NewEncoder(w).Encode(map[string]any{"runId": "run-1", "status": "running"})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"runId": "run-1", "status": "success"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(config.AgentBridgeConfig{
		Enabled:               true,
		GatewayBaseURL:        server.URL,
		GatewaySubject:        "svc",
		GatewayTenantID:       "sys",
		RequestTimeoutSeconds: 5,
	})
	result, err := client.CreateAndWaitRun(context.Background(), RunRequest{
		TraceID:   "trace-from-taskdaemon",
		LoopDepth: 0,
		InputText: "hello",
	})
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.False(t, usedCookie, "must not use admin cookie")
	for _, tr := range seenTrace {
		require.Equal(t, "trace-from-taskdaemon", tr)
	}
}

// TestGatewayUnavailable 验证网关失败路径。
//
// 参数:
//   - t: 测试上下文。
func TestGatewayUnavailable(t *testing.T) {
	client := NewClient(config.AgentBridgeConfig{
		Enabled:               true,
		GatewayBaseURL:        "http://127.0.0.1:1",
		RequestTimeoutSeconds: 1,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.CreateAndWaitRun(ctx, RunRequest{TraceID: "t1", InputText: "x"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrGatewayUnavailable) || strings.Contains(err.Error(), "agent gateway"))
	_ = fmt.Sprintf("%v", err)
}
