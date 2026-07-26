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

	t.Setenv("TASKDAEMON_NOTIFY_BUFFER_SIZE", "64")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_ENABLED", "false")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_MAX_RECORDS", "10")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_RETAIN_DAYS", "7")
	t.Setenv("TASKDAEMON_NOTIFY_STORE_MIN_SEVERITY", "warning")

	cfg, err := Load(LoadOptions{Optional: true})
	require.NoError(t, err)
	require.Equal(t, 64, cfg.Notify.BufferSize)
	require.False(t, cfg.Notify.Store.Enabled)
	require.Equal(t, 10, cfg.Notify.Store.MaxRecords)
	require.Equal(t, 7, cfg.Notify.Store.RetainDays)
	require.Equal(t, "warning", cfg.Notify.Store.MinSeverity)

	// 清理，避免影响同进程其他测试的 env 读取顺序（t.Setenv 已自动还原）
	_ = os.Getenv("TASKDAEMON_NOTIFY_BUFFER_SIZE")
}
