package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBuildTaskRunEventExcludesSensitiveRunnerOutput 验证含敏感值的 run 结果不会进入通知。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBuildTaskRunEventExcludesSensitiveRunnerOutput(t *testing.T) {
	// 模拟 runner 结果中的敏感字段：构造时根本不接受这些字段进入 Event
	exitCode := 1
	event, err := BuildTaskRunEvent(
		NameTaskRunFailed,
		3,
		12,
		time.Now().UTC(),
		&exitCode,
		1500,
		"manual",
	)
	require.NoError(t, err)
	require.Equal(t, "Task run failed", event.Title)
	require.Equal(t, "Task 3 run 12 failed", event.Body)
	require.NotContains(t, event.Title, "token")
	require.NotContains(t, event.Body, "password")
	require.Equal(t, map[string]any{
		"taskId":     3,
		"runId":      12,
		"status":     string(NameTaskRunFailed),
		"exitCode":   1,
		"durationMs": int64(1500),
		"trigger":    "manual",
	}, event.Detail)

	// 即便调用方试图塞敏感 detail，也会被白名单过滤
	raw, err := NewEvent(
		NameTaskRunFailed,
		SeverityError,
		time.Now().UTC(),
		Subject{Kind: "run", ID: "12"},
		"Task run failed",
		"Task 3 run 12 failed",
		map[string]any{
			"taskId":      3,
			"commandLine": "curl -H 'Authorization: Bearer secret-token' http://x",
			"env":         "TOKEN=abc",
		},
	)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"taskId": 3}, raw.Detail)
	require.NotContains(t, raw.Title+raw.Body, "Bearer")
}
