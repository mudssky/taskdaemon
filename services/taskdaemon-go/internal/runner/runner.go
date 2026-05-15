package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	defaultTimeout          = time.Hour
	defaultOutputLimitBytes = 64 * 1024
)

var (
	// ErrUnsupportedType 表示 runner 类型不在内置白名单中。
	ErrUnsupportedType = errors.New("unsupported runner type")
	// ErrMissingCommand 表示 runner 配置缺少脚本路径或命令片段。
	ErrMissingCommand = errors.New("missing runner command")
	// ErrInvalidTimeout 表示 runner timeout 不是正数。
	ErrInvalidTimeout = errors.New("invalid runner timeout")
)

// Type 表示内置结构化 runner 类型。
type Type string

const (
	// TypeShell 使用当前系统 shell 执行命令片段。
	TypeShell Type = "shell"
	// TypeBash 使用 bash 执行脚本路径或命令片段。
	TypeBash Type = "bash"
	// TypePwsh 使用 PowerShell Core 执行脚本路径或命令片段。
	TypePwsh Type = "pwsh"
	// TypePython 使用 python 执行脚本路径或命令片段。
	TypePython Type = "python"
	// TypeNode 使用 node 执行脚本路径或命令片段。
	TypeNode Type = "node"
	// TypeTypeScript 使用 tsx 执行 TypeScript 脚本。
	TypeTypeScript Type = "typescript"
)

// Status 表示 runner 执行后的业务状态。
type Status string

const (
	// StatusSuccess 表示进程退出码为 0。
	StatusSuccess Status = "success"
	// StatusFailed 表示进程启动或执行失败，且不是 timeout/cancel。
	StatusFailed Status = "failed"
	// StatusTimeout 表示任务超过 timeout 后被终止。
	StatusTimeout Status = "timeout"
	// StatusCancelled 表示调用方 context 取消导致任务终止。
	StatusCancelled Status = "cancelled"
)

// Config 保存一次 runner 执行需要的结构化配置。
type Config struct {
	Type             Type
	Inline           string
	ScriptPath       string
	Args             []string
	WorkDir          string
	Env              map[string]string
	Timeout          time.Duration
	OutputLimitBytes int
}

// Command 是从结构化 runner 配置构造出的进程命令摘要。
type Command struct {
	Name string
	Args []string
}

// Result 保存一次 runner 执行结果。
type Result struct {
	Status          Status
	ExitCode        *int
	Duration        time.Duration
	ErrorSummary    string
	Stdout          string
	Stderr          string
	StdoutTruncated bool
	StderrTruncated bool
}

// Executor 执行结构化 runner 配置。
type Executor struct{}

// NewExecutor 创建 runner 执行器。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Executor: 可执行结构化 runner 配置的执行器。
func NewExecutor() Executor {
	return Executor{}
}

// Validate 校验 runner 配置是否可保存和执行。
//
// 参数:
//   - cfg: 待校验的 runner 配置。
//
// 返回值:
//   - error: 类型不支持、命令缺失或 timeout 非法时返回错误。
func Validate(cfg Config) error {
	if !slices.Contains(supportedTypes(), cfg.Type) {
		return fmt.Errorf("%w: %s", ErrUnsupportedType, cfg.Type)
	}
	if normalizeTimeout(cfg.Timeout) <= 0 {
		return ErrInvalidTimeout
	}
	if strings.TrimSpace(cfg.Inline) == "" && strings.TrimSpace(cfg.ScriptPath) == "" {
		return ErrMissingCommand
	}
	return nil
}

// BuildCommand 将结构化 runner 配置转换为进程命令摘要。
//
// 参数:
//   - cfg: 已通过结构化校验的 runner 配置。
//
// 返回值:
//   - Command: 进程名称和参数列表。
//   - error: runner 配置非法时返回错误。
func BuildCommand(cfg Config) (Command, error) {
	if err := Validate(cfg); err != nil {
		return Command{}, err
	}

	inline := strings.TrimSpace(cfg.Inline)
	scriptPath := strings.TrimSpace(cfg.ScriptPath)
	switch cfg.Type {
	case TypeShell:
		return shellCommand(inline)
	case TypeBash:
		return interpreterCommand("bash", "-lc", inline, scriptPath, cfg.Args), nil
	case TypePwsh:
		if inline != "" {
			return Command{Name: "pwsh", Args: append([]string{"-NoProfile", "-Command", inline}, cfg.Args...)}, nil
		}
		return Command{Name: "pwsh", Args: append([]string{"-NoProfile", "-File", scriptPath}, cfg.Args...)}, nil
	case TypePython:
		return interpreterCommand("python", "-c", inline, scriptPath, cfg.Args), nil
	case TypeNode:
		return interpreterCommand("node", "-e", inline, scriptPath, cfg.Args), nil
	case TypeTypeScript:
		return interpreterCommand("tsx", "-e", inline, scriptPath, cfg.Args), nil
	default:
		return Command{}, fmt.Errorf("%w: %s", ErrUnsupportedType, cfg.Type)
	}
}

// Execute 执行 runner 配置并返回业务结果。
//
// 参数:
//   - executor: runner 执行器。
//   - ctx: 调用方 context，用于取消运行中的进程。
//   - cfg: runner 结构化配置。
//
// 返回值:
//   - Result: 执行状态、退出码、耗时、错误摘要和截断输出。
//   - error: 配置非法时返回错误；进程非零退出映射到 Result，不作为 error 返回。
func (executor Executor) Execute(ctx context.Context, cfg Config) (Result, error) {
	command, err := BuildCommand(cfg)
	if err != nil {
		return Result{}, err
	}

	runCtx, cancel := executionContext(ctx, normalizeTimeout(cfg.Timeout))
	defer cancel()

	cmd := exec.CommandContext(runCtx, command.Name, command.Args...)
	cmd.Dir = strings.TrimSpace(cfg.WorkDir)
	cmd.Env = mergeEnv(os.Environ(), cfg.Env)

	var stdout, stderr limitedBuffer
	limit := normalizeOutputLimit(cfg.OutputLimitBytes)
	stdout.limit = limit
	stderr.limit = limit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	err = cmd.Run()
	duration := time.Since(started)

	result := Result{
		Duration:        duration,
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}

	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		result.Status = StatusCancelled
		result.ErrorSummary = "cancelled"
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		result.Status = StatusTimeout
		result.ErrorSummary = "timeout"
	case err == nil:
		code := 0
		result.Status = StatusSuccess
		result.ExitCode = &code
	case errors.As(err, new(*exec.ExitError)):
		exitCode := cmd.ProcessState.ExitCode()
		result.Status = StatusFailed
		result.ExitCode = &exitCode
		result.ErrorSummary = summarizeExecutionError(err, result.Stderr)
	default:
		result.Status = StatusFailed
		result.ErrorSummary = summarizeExecutionError(err, result.Stderr)
	}

	return result, nil
}

// supportedTypes 返回第一版允许保存和执行的内置 runner 类型。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []Type: 内置 runner 类型列表。
func supportedTypes() []Type {
	return []Type{TypeShell, TypeBash, TypePwsh, TypePython, TypeNode, TypeTypeScript}
}

// normalizeTimeout 返回 runner timeout，未配置时使用默认 1 小时。
//
// 参数:
//   - timeout: 调用方配置的 timeout。
//
// 返回值:
//   - time.Duration: 实际 timeout。
func normalizeTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return defaultTimeout
	}
	return timeout
}

// normalizeOutputLimit 返回输出截断字节数，未配置时使用默认限制。
//
// 参数:
//   - limit: 调用方配置的输出限制字节数。
//
// 返回值:
//   - int: 实际输出限制字节数。
func normalizeOutputLimit(limit int) int {
	if limit <= 0 {
		return defaultOutputLimitBytes
	}
	return limit
}

// shellCommand 构造当前平台的 shell 命令。
//
// 参数:
//   - inline: 交给系统 shell 执行的命令片段。
//
// 返回值:
//   - Command: 进程名称和参数列表。
//   - error: shell runner 缺少 inline 命令时返回错误。
func shellCommand(inline string) (Command, error) {
	if inline == "" {
		return Command{}, ErrMissingCommand
	}
	if runtime.GOOS == "windows" {
		return Command{Name: "cmd", Args: []string{"/C", inline}}, nil
	}
	return Command{Name: "sh", Args: []string{"-c", inline}}, nil
}

// interpreterCommand 构造脚本解释器命令。
//
// 参数:
//   - name: 解释器进程名称。
//   - evalFlag: 执行 inline 片段时使用的参数，例如 -c 或 -e。
//   - inline: inline 命令片段。
//   - scriptPath: 脚本路径。
//   - args: 透传给脚本或片段的参数。
//
// 返回值:
//   - Command: 进程名称和参数列表。
func interpreterCommand(name string, evalFlag string, inline string, scriptPath string, args []string) Command {
	if inline != "" {
		return Command{Name: name, Args: append([]string{evalFlag, inline}, args...)}
	}
	return Command{Name: name, Args: append([]string{scriptPath}, args...)}
}

// executionContext 合并调用方取消与 runner timeout。
//
// 参数:
//   - parent: 调用方 context。
//   - timeout: runner timeout。
//
// 返回值:
//   - context.Context: 执行进程使用的 context。
//   - context.CancelFunc: 释放 context 资源的函数。
func executionContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, timeout)
}

// mergeEnv 合并基础环境变量和 runner 自定义环境变量。
//
// 参数:
//   - base: 继承的环境变量列表。
//   - extra: runner 自定义环境变量 map。
//
// 返回值:
//   - []string: 可传给 exec.Cmd.Env 的环境变量列表。
func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	env := slices.Clone(base)
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

// summarizeExecutionError 生成可持久化的稳定错误摘要。
//
// 参数:
//   - err: 进程启动或退出错误。
//   - stderr: 已截断的 stderr 内容。
//
// 返回值:
//   - string: 错误摘要。
func summarizeExecutionError(err error, stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		return truncateString(stderr, 512)
	}
	if err == nil {
		return ""
	}
	return truncateString(err.Error(), 512)
}

// truncateString 按字节长度截断字符串。
//
// 参数:
//   - value: 待截断字符串。
//   - limit: 最大字节数。
//
// 返回值:
//   - string: 截断后的字符串。
func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

// limitedBuffer 是带最大字节数的进程输出缓冲区。
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write 写入进程输出，超过 limit 的内容会被丢弃并标记截断。
//
// 参数:
//   - buffer: 输出缓冲区。
//   - p: 本次写入的字节片段。
//
// 返回值:
//   - int: 按 io.Writer 约定返回调用方写入的原始字节数。
//   - error: 当前实现不会返回错误。
func (buffer *limitedBuffer) Write(p []byte) (int, error) {
	if buffer.limit <= 0 {
		buffer.truncated = true
		return len(p), nil
	}
	remaining := buffer.limit - buffer.buf.Len()
	if remaining <= 0 {
		buffer.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		buffer.truncated = true
		_, _ = buffer.buf.Write(p[:remaining])
		return len(p), nil
	}
	_, _ = buffer.buf.Write(p)
	return len(p), nil
}

// String 返回已保留的输出内容。
//
// 参数:
//   - buffer: 输出缓冲区。
//
// 返回值:
//   - string: 未超过限制部分的输出内容。
func (buffer *limitedBuffer) String() string {
	return buffer.buf.String()
}
