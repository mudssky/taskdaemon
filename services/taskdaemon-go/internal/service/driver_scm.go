package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// windowsDriver 通过 sc.exe 管理 Windows SCM 服务。
type windowsDriver struct{}

func (windowsDriver) name() string        { return "windows" }
func (windowsDriver) defaultScope() Scope { return ScopeSystem }

func (windowsDriver) validateScope(scope Scope) error {
	return windowsValidateScope(scope)
}

func (windowsDriver) unitPath(Scope) (string, error) {
	return windowsUnitPath, nil
}

func (windowsDriver) render(resolved ResolvedSpec) (Unit, error) {
	return renderWindows(resolved)
}

func (windowsDriver) checkPermission(Scope) error {
	if runtime.GOOS != "windows" {
		// 非 Windows 主机上的交叉逻辑测试不强制提权。
		return nil
	}
	if !isWindowsElevated() {
		return newError(
			CodePermissionDenied,
			"windows SCM install requires administrator privileges",
			"re-open an elevated shell (Run as administrator), then:\n  taskdaemon service install --scope system",
			nil,
		)
	}
	return nil
}

func (w windowsDriver) isInstalled(ctxScope Scope) (bool, string, error) {
	_ = ctxScope
	if runtime.GOOS != "windows" {
		return false, windowsUnitPath, nil
	}
	out, err := runCommand(context.Background(), "sc", "query", windowsServiceName)
	if err != nil {
		if strings.Contains(strings.ToLower(out), "specified service does not exist") ||
			strings.Contains(strings.ToLower(out), "1060") {
			return false, windowsUnitPath, nil
		}
		// sc 不可用时保守返回未安装。
		return false, windowsUnitPath, nil
	}
	return true, windowsUnitPath, nil
}

func (windowsDriver) writeUnit(Unit) error {
	// Windows SCM 不落盘 unit 文件；注册步骤创建服务。
	return nil
}

func (windowsDriver) removeUnit(string) error {
	return nil
}

func (windowsDriver) register(ctx context.Context, unit Unit, _ Scope) error {
	binPath := windowsBinPath(unit.ProgramArguments)
	// sc create 语法：binPath= 后必须有空格。
	out, err := runCommand(ctx, "sc", "create", windowsServiceName,
		"binPath=", binPath,
		"start=", "auto",
		"DisplayName=", windowsServiceDisplayName,
	)
	if err != nil {
		// 已存在时尝试 update config。
		if strings.Contains(strings.ToLower(out), "already exists") || strings.Contains(out, "1073") {
			out2, err2 := runCommand(ctx, "sc", "config", windowsServiceName, "binPath=", binPath, "start=", "auto")
			if err2 != nil {
				return fmt.Errorf("sc create: %w (%s); sc config: %v (%s)", err, out, err2, out2)
			}
		} else {
			return fmt.Errorf("sc create: %w (%s)", err, out)
		}
	}
	// 失败重启：reset= 86400 actions= restart/5000
	_, _ = runCommand(ctx, "sc", "failure", windowsServiceName, "reset=", "86400", "actions=", "restart/5000/restart/5000/restart/5000")
	return nil
}

func (windowsDriver) unregister(ctx context.Context, _ Scope, _ string) error {
	_, _ = runCommand(ctx, "sc", "stop", windowsServiceName)
	out, err := runCommand(ctx, "sc", "delete", windowsServiceName)
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "does not exist") || strings.Contains(lower, "1060") {
			return nil
		}
		return fmt.Errorf("sc delete: %w (%s)", err, out)
	}
	return nil
}

func (windowsDriver) start(ctx context.Context, _ Scope) error {
	out, err := runCommand(ctx, "sc", "start", windowsServiceName)
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "already been started") || strings.Contains(lower, "1056") {
			return nil
		}
		return fmt.Errorf("sc start: %w (%s)", err, out)
	}
	return nil
}

func (windowsDriver) stop(ctx context.Context, _ Scope) error {
	out, err := runCommand(ctx, "sc", "stop", windowsServiceName)
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "not started") || strings.Contains(lower, "1062") || strings.Contains(lower, "does not exist") {
			return nil
		}
		return fmt.Errorf("sc stop: %w (%s)", err, out)
	}
	return nil
}

func (windowsDriver) isRunning(ctx context.Context, _ Scope) (bool, string, error) {
	out, err := runCommand(ctx, "sc", "query", windowsServiceName)
	if err != nil {
		return false, out, nil
	}
	running := strings.Contains(out, "RUNNING")
	return running, out, nil
}

func (windowsDriver) installHints(Scope) []string {
	return []string{
		"Windows SCM service requires administrator privileges.",
		"Plain console binaries may need a service wrapper for full SCM control signals.",
		"Inspect: sc query TaskDaemon  or services.msc",
	}
}

func (windowsDriver) manualCleanupHint(_ Scope, _ string) string {
	return "manual cleanup: sc stop TaskDaemon & sc delete TaskDaemon"
}

// isWindowsElevated 检测当前进程是否以管理员运行。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: elevated 时为 true。
func isWindowsElevated() bool {
	// 尝试打开需要管理员权限的系统目录写入探测过于危险；
	// 使用 net session 作为常见启发式（elevated 时成功）。
	out, err := runCommand(context.Background(), "net", "session")
	if err == nil {
		return true
	}
	_ = out
	// 回退：检查是否能写 %SYSTEMROOT%\System32\drivers\etc —— 仍可能误判。
	testPath := os.Getenv("SYSTEMROOT")
	if testPath == "" {
		testPath = `C:\Windows`
	}
	f, err := os.OpenFile(testPath+`\System32\taskdaemon_elev_probe.tmp`, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(testPath + `\System32\taskdaemon_elev_probe.tmp`)
	return true
}
