package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNotifyDefaultsAndEnv 验证通知默认值与环境变量覆盖（T2a）。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotifyDefaultsAndEnv(t *testing.T) {
	defaults := Default()
	require.Equal(t, 256, defaults.Notify.BufferSize)
	require.True(t, defaults.Notify.Store.Enabled)
	require.Equal(t, 200, defaults.Notify.Store.MaxRecords)
	require.Equal(t, 30, defaults.Notify.Store.RetainDays)
	// T4 defaults
	require.False(t, defaults.Notify.Webhook.Enabled)
	require.Equal(t, []string{"https"}, defaults.Notify.Webhook.AllowedSchemes)
	require.False(t, defaults.Notify.Webhook.AllowPrivateNetworks)
	require.Equal(t, 3, defaults.Notify.Webhook.MaxRedirects)
	require.False(t, defaults.Notify.Email.Enabled)
	require.Equal(t, 587, defaults.Notify.Email.SMTP.Port)
	require.Equal(t, "starttls", defaults.Notify.Email.SMTP.Encryption)
	// D2 / T3 defaults
	require.True(t, defaults.Notify.Desktop.Enabled)
	require.Equal(t, "", defaults.Notify.Desktop.MinSeverity)

	t.Setenv("TASKDAEMON_NOTIFY_BUFFER_SIZE", "64")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_ENABLED", "false")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_MAX_RECORDS", "10")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_RETAIN_DAYS", "7")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_MIN_SEVERITY", "warning")
	t.Setenv("TASKDAEMON_NOTIFY_WEBHOOK_ENABLED", "true")
	t.Setenv("TASKDAEMON_NOTIFY_WEBHOOK_ALLOW_PRIVATE_NETWORKS", "true")
	t.Setenv("TASKDAEMON_NOTIFY_EMAIL_ENABLED", "true")
	t.Setenv("TASKDAEMON_NOTIFY_EMAIL_MIN_SEVERITY", "error")
	t.Setenv("TASKDAEMON_NOTIFY_EMAIL_SMTP_HOST", "smtp.example.com")
	t.Setenv("TASKDAEMON_NOTIFY_EMAIL_SMTP_PASSWORD", "should-load-but-not-log")
	t.Setenv("TASKDAEMON_NOTIFY_DESKTOP_ENABLED", "false")
	t.Setenv("TASKDAEMON_NOTIFY_DESKTOP_MIN_SEVERITY", "critical")

	cfg, err := Load(LoadOptions{Optional: true})
	require.NoError(t, err)
	require.Equal(t, 64, cfg.Notify.BufferSize)
	require.False(t, cfg.Notify.Store.Enabled)
	require.Equal(t, 10, cfg.Notify.Store.MaxRecords)
	require.Equal(t, 7, cfg.Notify.Store.RetainDays)
	require.Equal(t, "warning", cfg.Notify.Store.MinSeverity)
	require.True(t, cfg.Notify.Webhook.Enabled)
	require.True(t, cfg.Notify.Webhook.AllowPrivateNetworks)
	require.True(t, cfg.Notify.Email.Enabled)
	require.Equal(t, "error", cfg.Notify.Email.MinSeverity)
	require.Equal(t, "smtp.example.com", cfg.Notify.Email.SMTP.Host)
	require.Equal(t, "should-load-but-not-log", cfg.Notify.Email.SMTP.Password)
	require.False(t, cfg.Notify.Desktop.Enabled)
	require.Equal(t, "critical", cfg.Notify.Desktop.MinSeverity)

	// 清理，避免影响同进程其他测试的 env 读取顺序（t.Setenv 已自动还原）
	_ = os.Getenv("TASKDAEMON_NOTIFY_BUFFER_SIZE")
}

// TestNotifyWebhookTargetsFromFile 验证 YAML 目标列表加载（T4）。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNotifyWebhookTargetsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/taskdaemon.yaml"
	content := `
notify:
  webhook:
    enabled: true
    allowPrivateNetworks: true
    allowedSchemes:
      - https
      - http
    targets:
      - name: ops
        url: https://hooks.example.com/a
        enabled: true
        method: POST
        headers:
          X-Token: secret
        signingSecret: hmac-key
        timeoutSeconds: 8
        maxRetries: 1
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	cfg, err := Load(LoadOptions{ConfigPath: path, Optional: false})
	require.NoError(t, err)
	require.True(t, cfg.Notify.Webhook.Enabled)
	require.True(t, cfg.Notify.Webhook.AllowPrivateNetworks)
	require.Len(t, cfg.Notify.Webhook.Targets, 1)
	target := cfg.Notify.Webhook.Targets[0]
	require.Equal(t, "ops", target.Name)
	require.Equal(t, "https://hooks.example.com/a", target.URL)
	require.True(t, target.Enabled)
	require.Equal(t, "POST", target.Method)
	require.Equal(t, "secret", target.Headers["X-Token"])
	require.Equal(t, "hmac-key", target.SigningSecret)
	require.Equal(t, 8, target.TimeoutSeconds)
	require.Equal(t, 1, target.MaxRetries)
}
