package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewEventRejectsInvalidSeverity 验证非法 Severity 在构造期被拒绝。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNewEventRejectsInvalidSeverity(t *testing.T) {
	_, err := NewEvent(NameTaskRunFailed, Severity("fatal"), time.Now(), Subject{}, "t", "b", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid notify severity")
}

// TestNewEventFiltersDetailWhitelist 验证 Detail 白名单过滤生效。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNewEventFiltersDetailWhitelist(t *testing.T) {
	event, err := NewEvent(
		NameTaskRunSucceeded,
		SeverityInfo,
		time.Unix(1700000000, 0).UTC(),
		Subject{Kind: string(SubjectKindRun), ID: "12"},
		"ok",
		"body",
		map[string]any{
			"taskId":      3,
			"runId":       12,
			"commandLine": "curl -H 'Authorization: Bearer secret-token' https://example.com",
			"password":    "super-secret",
			"env":         map[string]string{"TOKEN": "abc"},
			"exitCode":    0,
		},
	)
	require.NoError(t, err)
	require.Equal(t, NameTaskRunSucceeded, event.Name)
	require.Equal(t, SeverityInfo, event.Severity)
	require.NotEmpty(t, event.ID)
	require.Equal(t, map[string]any{
		"taskId":   3,
		"runId":    12,
		"exitCode": 0,
	}, event.Detail)
}

// TestNewEventDropsSensitiveDetailValues 验证疑似敏感字符串值不会进入 Detail。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNewEventDropsSensitiveDetailValues(t *testing.T) {
	event, err := NewEvent(
		NameTaskRunFailed,
		SeverityError,
		time.Time{},
		Subject{Kind: string(SubjectKindRun), ID: "9"},
		"failed",
		"body",
		map[string]any{
			"status":    "failed",
			"errorKind": "password=leaked-value",
		},
	)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"status": "failed"}, event.Detail)
}

// TestNewEventWithIDUsesStableID 验证指定 event ID 可复用做幂等键。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestNewEventWithIDUsesStableID(t *testing.T) {
	event, err := NewEventWithID(
		"task.run.succeeded:run:42",
		NameTaskRunSucceeded,
		SeverityInfo,
		time.Now().UTC(),
		Subject{Kind: string(SubjectKindRun), ID: "42"},
		"ok",
		"body",
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, "task.run.succeeded:run:42", event.ID)
}

// TestSeverityValid 验证 Severity.Valid 覆盖全部合法值与非法值。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestSeverityValid(t *testing.T) {
	require.True(t, SeverityInfo.Valid())
	require.True(t, SeverityWarning.Valid())
	require.True(t, SeverityError.Valid())
	require.True(t, SeverityCritical.Valid())
	require.False(t, Severity("debug").Valid())
}
