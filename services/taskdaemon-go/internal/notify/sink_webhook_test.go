package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

// TestWebhookSinkDeliversSignedJSON 验证 Webhook 投递 C-2 JSON、自定义 header 与签名可验。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkDeliversSignedJSON(t *testing.T) {
	var gotBody []byte
	var gotAuth string
	var gotSignature string
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotMethod = request.Method
		gotAuth = request.Header.Get("Authorization")
		gotSignature = request.Header.Get(defaultSigningHeader)
		body, _ := io.ReadAll(request.Body)
		gotBody = body
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		DefaultTimeoutSec:    5,
		DefaultMaxRetries:    0,
		DefaultBackoffMs:     1,
		AllowedSchemes:       []string{"http", "https"},
		AllowPrivateNetworks: true,
		MaxRedirects:         1,
		Targets: []config.NotifyWebhookTargetConfig{
			{
				Name:          "primary",
				URL:           server.URL + "/hook",
				Enabled:       true,
				Method:        http.MethodPost,
				Headers:       map[string]string{"Authorization": "Bearer secret-token"},
				SigningSecret: "super-secret",
			},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{HTTPClient: server.Client()})
	event := testEvent("evt-1")
	event.Title = "Task failed"
	event.Body = "run ended with error"
	event.Detail = map[string]any{"taskId": 7, "password": "should-drop"}

	err := sink.Deliver(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "Bearer secret-token", gotAuth)
	require.True(t, VerifyWebhookSignature("super-secret", gotBody, gotSignature))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &payload))
	require.Equal(t, "evt-1", payload["id"])
	require.Equal(t, string(NameTaskRunSucceeded), payload["name"])
	detail, ok := payload["detail"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 7, detail["taskId"])
	_, hasPassword := detail["password"]
	require.False(t, hasPassword)

	statuses := sink.TargetStatuses()
	require.Len(t, statuses, 1)
	require.Equal(t, int64(1), statuses[0].SuccessCount)
	require.Equal(t, int64(0), statuses[0].FailureCount)
}

// TestWebhookSinkRejectsPrivateURLByDefault 验证默认拒绝内网地址。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkRejectsPrivateURLByDefault(t *testing.T) {
	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http", "https"},
		AllowPrivateNetworks: false,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "local", URL: "http://127.0.0.1:9/hook", Enabled: true},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{})
	err := sink.Deliver(context.Background(), testEvent("evt-private"))
	require.Error(t, err)
	require.Contains(t, err.Error(), CodeWebhookURLRejected)
}

// TestWebhookSinkAllowsPrivateWhenConfigured 验证显式放行内网。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkAllowsPrivateWhenConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: true,
		DefaultMaxRetries:    0,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "local", URL: server.URL, Enabled: true},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{HTTPClient: server.Client()})
	require.NoError(t, sink.Deliver(context.Background(), testEvent("evt-ok")))
}

// TestWebhookSinkRetriesWithBackoff 验证失败重试与退避注入。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkRetriesWithBackoff(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	var sleeps atomic.Int32
	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: true,
		DefaultMaxRetries:    2,
		DefaultBackoffMs:     5,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "retry", URL: server.URL, Enabled: true},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{
		HTTPClient: server.Client(),
		Sleep: func(d time.Duration) {
			require.Greater(t, d, time.Duration(0))
			sleeps.Add(1)
		},
	})
	require.NoError(t, sink.Deliver(context.Background(), testEvent("evt-retry")))
	require.Equal(t, int32(3), attempts.Load())
	require.Equal(t, int32(2), sleeps.Load())
}

// TestWebhookSinkLimitsRedirects 验证重定向次数上限。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkLimitsRedirects(t *testing.T) {
	var redirects atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirects.Add(1)
		http.Redirect(writer, request, "/next", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: true,
		MaxRedirects:         1,
		DefaultMaxRetries:    0,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "redir", URL: server.URL, Enabled: true},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{HTTPClient: server.Client()})
	err := sink.Deliver(context.Background(), testEvent("evt-redir"))
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), CodeWebhookURLRejected) || strings.Contains(err.Error(), CodeWebhookDeliverFailed),
	)
	require.GreaterOrEqual(t, redirects.Load(), int32(2))
}

// TestWebhookSinkSkipsDisabledTarget 验证目标独立开关。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkSkipsDisabledTarget(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: true,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "off", URL: server.URL, Enabled: false},
			{Name: "on", URL: server.URL, Enabled: true},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{HTTPClient: server.Client()})
	require.NoError(t, sink.Deliver(context.Background(), testEvent("evt-switch")))
	require.Equal(t, int32(1), hits.Load())
}

// TestWebhookSinkDoesNotLogSecrets 验证失败路径错误不含签名密钥与 header 凭据。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestWebhookSinkDoesNotLogSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	secret := "signing-secret-value-xyz"
	token := "Bearer top-secret-token"
	cfg := config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: true,
		DefaultMaxRetries:    0,
		Targets: []config.NotifyWebhookTargetConfig{
			{
				Name:          "sec",
				URL:           server.URL,
				Enabled:       true,
				Headers:       map[string]string{"Authorization": token},
				SigningSecret: secret,
			},
		},
	}
	sink := NewWebhookSink(cfg, WebhookSinkOptions{HTTPClient: server.Client()})
	err := sink.Deliver(context.Background(), testEvent("evt-sec"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), token)
	statuses := sink.TargetStatuses()
	require.NotEmpty(t, statuses)
	require.NotContains(t, statuses[0].LastError, secret)
	require.NotContains(t, statuses[0].LastError, token)
}
