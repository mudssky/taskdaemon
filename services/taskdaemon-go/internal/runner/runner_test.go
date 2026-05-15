package runner

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBuildCommandUsesTSXForTypeScript 验证 TypeScript runner 默认使用 tsx 执行器。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestBuildCommandUsesTSXForTypeScript(t *testing.T) {
	cmd, err := BuildCommand(Config{
		Type:       TypeTypeScript,
		ScriptPath: "scripts/backup.ts",
		Args:       []string{"--dry-run"},
		Timeout:    time.Minute,
	})

	require.NoError(t, err)
	require.Equal(t, "tsx", cmd.Name)
	require.Equal(t, []string{"scripts/backup.ts", "--dry-run"}, cmd.Args)
}

// TestExecuteShellCapturesTruncatedOutput 验证 runner 会截断保存 stdout/stderr。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestExecuteShellCapturesTruncatedOutput(t *testing.T) {
	cfg := Config{
		Type:             TypeShell,
		Inline:           shellPrintCommand("abcdef", "ghijkl"),
		Timeout:          time.Second,
		OutputLimitBytes: 4,
	}

	result, err := NewExecutor().Execute(context.Background(), cfg)

	require.NoError(t, err)
	require.Equal(t, StatusSuccess, result.Status)
	require.Equal(t, "abcd", result.Stdout)
	require.Equal(t, "ghij", result.Stderr)
	require.True(t, result.StdoutTruncated)
	require.True(t, result.StderrTruncated)
}

// TestExecuteTimeoutRecordsTimeout 验证超时会终止进程并记录 timeout 状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestExecuteTimeoutRecordsTimeout(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Config{
		Type:    TypeShell,
		Inline:  shellSleepCommand(time.Second),
		Timeout: 20 * time.Millisecond,
	})

	require.NoError(t, err)
	require.Equal(t, StatusTimeout, result.Status)
	require.Contains(t, result.ErrorSummary, "timeout")
}

// TestExecuteCancelRecordsCancelled 验证调用方取消 context 会记录 cancelled 状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestExecuteCancelRecordsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewExecutor().Execute(ctx, Config{
		Type:    TypeShell,
		Inline:  shellSleepCommand(time.Second),
		Timeout: time.Second,
	})

	require.NoError(t, err)
	require.Equal(t, StatusCancelled, result.Status)
	require.Contains(t, result.ErrorSummary, "cancelled")
}

// TestValidateRejectsUnsupportedRunner 验证结构化校验会拒绝未知 runner 类型。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestValidateRejectsUnsupportedRunner(t *testing.T) {
	err := Validate(Config{
		Type:    Type("raw-command"),
		Inline:  "echo hello",
		Timeout: time.Second,
	})

	require.ErrorIs(t, err, ErrUnsupportedType)
}

// shellPrintCommand 返回跨平台 stdout/stderr 输出命令。
//
// 参数:
//   - stdout: 要写入 stdout 的文本。
//   - stderr: 要写入 stderr 的文本。
//
// 返回值:
//   - string: 可交给 shell runner 执行的命令片段。
func shellPrintCommand(stdout string, stderr string) string {
	if runtime.GOOS == "windows" {
		return "echo " + stdout + " & echo " + stderr + " 1>&2"
	}
	return "printf '" + stdout + "'; printf '" + stderr + "' 1>&2"
}

// shellSleepCommand 返回跨平台等待命令。
//
// 参数:
//   - duration: 希望命令等待的时长。
//
// 返回值:
//   - string: 可交给 shell runner 执行的命令片段。
func shellSleepCommand(duration time.Duration) string {
	if runtime.GOOS == "windows" {
		return "for /L %i in (1,1,100000000) do @rem"
	}
	return "sleep " + duration.Truncate(time.Second).String()
}
