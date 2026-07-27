package service

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// linuxDriver 实现 Linux systemd 用户/系统级服务。
type linuxDriver struct{}

func (linuxDriver) name() string        { return "linux" }
func (linuxDriver) defaultScope() Scope { return ScopeUser }

func (linuxDriver) validateScope(scope Scope) error {
	if scope == ScopeUser || scope == ScopeSystem {
		return nil
	}
	return newError(CodeInvalidScope, fmt.Sprintf("unsupported scope %q", scope), "use --scope user or --scope system", nil)
}

func (d linuxDriver) unitPath(scope Scope) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil && scope == ScopeUser {
		return "", newError(CodeInvalidSpec, "resolve user home", "", err)
	}
	return systemdUnitPath(scope, home)
}

func (d linuxDriver) render(resolved ResolvedSpec) (Unit, error) {
	path, err := d.unitPath(resolved.Scope)
	if err != nil {
		return Unit{}, err
	}
	return renderSystemd(resolved, path)
}

func (d linuxDriver) checkPermission(scope Scope) error {
	if scope == ScopeSystem && !isRoot() {
		return newError(
			CodePermissionDenied,
			"system systemd unit requires root",
			"re-run: sudo taskdaemon service install --scope system",
			nil,
		)
	}
	return nil
}

func (d linuxDriver) isInstalled(scope Scope) (bool, string, error) {
	path, err := d.unitPath(scope)
	if err != nil {
		return false, "", err
	}
	return fileExists(path), path, nil
}

func (linuxDriver) writeUnit(unit Unit) error {
	return writeFileAtomic(unit.Path, unit.Content, unit.Mode)
}

func (linuxDriver) removeUnit(unitPath string) error {
	return removeFileIfExists(unitPath)
}

func (d linuxDriver) register(ctx context.Context, unit Unit, scope Scope) error {
	args := d.systemctlPrefix(scope)
	reloadArgs := append(append([]string{}, args...), "daemon-reload")
	if out, err := runCommand(ctx, "systemctl", reloadArgs...); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w (%s)", err, out)
	}
	enableArgs := append(append([]string{}, args...), "enable", systemdUnitName)
	if out, err := runCommand(ctx, "systemctl", enableArgs...); err != nil {
		return fmt.Errorf("systemctl enable: %w (%s)", err, out)
	}
	return nil
}

func (d linuxDriver) unregister(ctx context.Context, scope Scope, _ string) error {
	args := d.systemctlPrefix(scope)
	disableArgs := append(append([]string{}, args...), "disable", "--now", systemdUnitName)
	out, err := runCommand(ctx, "systemctl", disableArgs...)
	if err != nil && !strings.Contains(out, "not loaded") && !strings.Contains(out, "No such file") {
		// 继续 daemon-reload
		_ = err
	}
	reloadArgs := append(append([]string{}, args...), "daemon-reload")
	_, _ = runCommand(ctx, "systemctl", reloadArgs...)
	return nil
}

func (d linuxDriver) start(ctx context.Context, scope Scope) error {
	args := append(d.systemctlPrefix(scope), "start", systemdUnitName)
	out, err := runCommand(ctx, "systemctl", args...)
	if err != nil {
		return fmt.Errorf("systemctl start: %w (%s)", err, out)
	}
	return nil
}

func (d linuxDriver) stop(ctx context.Context, scope Scope) error {
	args := append(d.systemctlPrefix(scope), "stop", systemdUnitName)
	out, err := runCommand(ctx, "systemctl", args...)
	if err != nil && !strings.Contains(out, "not loaded") && !strings.Contains(strings.ToLower(out), "not active") {
		return fmt.Errorf("systemctl stop: %w (%s)", err, out)
	}
	return nil
}

func (d linuxDriver) isRunning(ctx context.Context, scope Scope) (bool, string, error) {
	args := append(d.systemctlPrefix(scope), "is-active", systemdUnitName)
	out, err := runCommand(ctx, "systemctl", args...)
	active := strings.TrimSpace(out) == "active"
	if err != nil && !active {
		return false, out, nil
	}
	statusArgs := append(d.systemctlPrefix(scope), "status", systemdUnitName, "--no-pager")
	detail, _ := runCommand(ctx, "systemctl", statusArgs...)
	if detail == "" {
		detail = out
	}
	return active, detail, nil
}

func (linuxDriver) installHints(scope Scope) []string {
	if scope == ScopeUser {
		return []string{
			"Linux user systemd unit installed (systemctl --user).",
			"User services stop on logout unless lingering is enabled.",
			"Enable lingering: loginctl enable-linger $USER",
			"Inspect: systemctl --user status taskdaemon.service",
		}
	}
	return []string{
		"Linux system unit installed under /etc/systemd/system.",
		"Inspect: systemctl status taskdaemon.service",
	}
}

func (linuxDriver) manualCleanupHint(scope Scope, unitPath string) string {
	if scope == ScopeSystem {
		return fmt.Sprintf("manual cleanup: sudo systemctl disable --now %s; sudo rm -f %s; sudo systemctl daemon-reload", systemdUnitName, unitPath)
	}
	return fmt.Sprintf("manual cleanup: systemctl --user disable --now %s; rm -f %s; systemctl --user daemon-reload", systemdUnitName, unitPath)
}

func (linuxDriver) systemctlPrefix(scope Scope) []string {
	if scope == ScopeUser {
		return []string{"--user"}
	}
	return nil
}
