package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// darwinDriver 实现 macOS launchd 用户/系统级服务。
type darwinDriver struct{}

func (darwinDriver) name() string        { return "darwin" }
func (darwinDriver) defaultScope() Scope { return ScopeUser }

func (darwinDriver) validateScope(scope Scope) error {
	if scope == ScopeUser || scope == ScopeSystem {
		return nil
	}
	return newError(CodeInvalidScope, fmt.Sprintf("unsupported scope %q", scope), "use --scope user or --scope system", nil)
}

func (d darwinDriver) unitPath(scope Scope) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil && scope == ScopeUser {
		return "", newError(CodeInvalidSpec, "resolve user home", "", err)
	}
	return launchdUnitPath(scope, home)
}

func (d darwinDriver) render(resolved ResolvedSpec) (Unit, error) {
	path, err := d.unitPath(resolved.Scope)
	if err != nil {
		return Unit{}, err
	}
	return renderLaunchd(resolved, path)
}

func (d darwinDriver) checkPermission(scope Scope) error {
	if scope == ScopeSystem && !isRoot() {
		return newError(
			CodePermissionDenied,
			"system LaunchDaemon requires root",
			"re-run: sudo taskdaemon service install --scope system",
			nil,
		)
	}
	return nil
}

func (d darwinDriver) isInstalled(scope Scope) (bool, string, error) {
	path, err := d.unitPath(scope)
	if err != nil {
		return false, "", err
	}
	return fileExists(path), path, nil
}

func (darwinDriver) writeUnit(unit Unit) error {
	return writeFileAtomic(unit.Path, unit.Content, unit.Mode)
}

func (darwinDriver) removeUnit(unitPath string) error {
	return removeFileIfExists(unitPath)
}

func (d darwinDriver) register(ctx context.Context, unit Unit, scope Scope) error {
	// 确保日志目录存在。
	_ = os.MkdirAll(filepath.Join(filepath.Dir(filepath.Dir(unit.Path)), "Logs", "taskdaemon"), 0o755)
	if scope == ScopeUser {
		home, _ := os.UserHomeDir()
		_ = os.MkdirAll(filepath.Join(home, "Library", "Logs", "taskdaemon"), 0o755)
	} else {
		_ = os.MkdirAll("/Library/Logs/taskdaemon", 0o755)
	}

	domain := d.launchDomain(scope)
	// 先 bootout 残留，忽略失败。
	_, _ = runCommand(ctx, "launchctl", "bootout", domain+"/"+launchdLabel)
	out, err := runCommand(ctx, "launchctl", "bootstrap", domain, unit.Path)
	if err != nil {
		// 回退旧 API。
		out2, err2 := runCommand(ctx, "launchctl", "load", "-w", unit.Path)
		if err2 != nil {
			return fmt.Errorf("launchctl bootstrap: %w (%s); load fallback: %v (%s)", err, out, err2, out2)
		}
	}
	_, _ = runCommand(ctx, "launchctl", "enable", domain+"/"+launchdLabel)
	return nil
}

func (d darwinDriver) unregister(ctx context.Context, scope Scope, unitPath string) error {
	domain := d.launchDomain(scope)
	out, err := runCommand(ctx, "launchctl", "bootout", domain+"/"+launchdLabel)
	if err != nil {
		out2, err2 := runCommand(ctx, "launchctl", "unload", "-w", unitPath)
		if err2 != nil && !strings.Contains(out+out2, "No such process") && !strings.Contains(out+out2, "Could not find") {
			return fmt.Errorf("launchctl bootout: %w (%s); unload fallback: %v (%s)", err, out, err2, out2)
		}
	}
	return nil
}

func (d darwinDriver) start(ctx context.Context, scope Scope) error {
	domain := d.launchDomain(scope)
	out, err := runCommand(ctx, "launchctl", "kickstart", "-k", domain+"/"+launchdLabel)
	if err != nil {
		out2, err2 := runCommand(ctx, "launchctl", "start", launchdLabel)
		if err2 != nil {
			return fmt.Errorf("launchctl kickstart: %w (%s); start fallback: %v (%s)", err, out, err2, out2)
		}
	}
	return nil
}

func (d darwinDriver) stop(ctx context.Context, scope Scope) error {
	domain := d.launchDomain(scope)
	out, err := runCommand(ctx, "launchctl", "kill", "SIGTERM", domain+"/"+launchdLabel)
	if err != nil {
		_, _ = runCommand(ctx, "launchctl", "stop", launchdLabel)
		// stop 失败在未运行时常见，忽略。
		_ = out
	}
	return nil
}

func (d darwinDriver) isRunning(ctx context.Context, scope Scope) (bool, string, error) {
	out, err := runCommand(ctx, "launchctl", "print", d.launchDomain(scope)+"/"+launchdLabel)
	if err != nil {
		// list 回退
		out2, err2 := runCommand(ctx, "launchctl", "list", launchdLabel)
		if err2 != nil {
			return false, strings.TrimSpace(out + "\n" + out2), nil
		}
		return true, out2, nil
	}
	running := strings.Contains(out, "state = running") || strings.Contains(out, "pid = ")
	return running, out, nil
}

func (darwinDriver) installHints(scope Scope) []string {
	if scope == ScopeUser {
		return []string{
			"macOS user LaunchAgent: loads for the installing user session (RunAtLoad + KeepAlive).",
			"Log out/in or re-login is usually not required; kickstart already attempted.",
			"Inspect: launchctl print gui/$(id -u)/com.taskdaemon.daemon",
		}
	}
	return []string{
		"macOS system LaunchDaemon requires root and runs at boot independent of GUI login.",
		"Inspect: sudo launchctl print system/com.taskdaemon.daemon",
	}
}

func (darwinDriver) manualCleanupHint(scope Scope, unitPath string) string {
	if scope == ScopeSystem {
		return fmt.Sprintf("manual cleanup: sudo launchctl bootout system/%s; sudo rm -f %s", launchdLabel, unitPath)
	}
	return fmt.Sprintf("manual cleanup: launchctl bootout gui/$(id -u)/%s; rm -f %s", launchdLabel, unitPath)
}

func (darwinDriver) launchDomain(scope Scope) string {
	if scope == ScopeSystem {
		return "system"
	}
	return "gui/" + currentUID()
}
