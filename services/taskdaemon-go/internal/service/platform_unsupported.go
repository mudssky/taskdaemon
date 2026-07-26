package service

import (
	"context"
	"fmt"
)

// unsupportedDriver 用于非 darwin/linux/windows 平台。
type unsupportedDriver struct {
	goos string
}

func (u unsupportedDriver) name() string { return u.goos }
func (unsupportedDriver) defaultScope() Scope {
	return ScopeUser
}

func (u unsupportedDriver) validateScope(Scope) error {
	return newError(
		CodeUnsupportedPlatform,
		fmt.Sprintf("service install is not supported on %s", u.goos),
		"supported platforms: darwin (launchd), linux (systemd), windows (SCM)",
		nil,
	)
}

func (u unsupportedDriver) unitPath(Scope) (string, error) {
	return "", newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) render(ResolvedSpec) (Unit, error) {
	return Unit{}, newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) checkPermission(Scope) error {
	return newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) isInstalled(Scope) (bool, string, error) {
	return false, "", newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (unsupportedDriver) writeUnit(Unit) error    { return nil }
func (unsupportedDriver) removeUnit(string) error { return nil }

func (u unsupportedDriver) register(context.Context, Unit, Scope) error {
	return newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) unregister(context.Context, Scope, string) error {
	return newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) start(context.Context, Scope) error {
	return newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) stop(context.Context, Scope) error {
	return newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (u unsupportedDriver) isRunning(context.Context, Scope) (bool, string, error) {
	return false, "", newError(CodeUnsupportedPlatform, fmt.Sprintf("unsupported platform %s", u.goos), "", nil)
}

func (unsupportedDriver) installHints(Scope) []string { return nil }

func (u unsupportedDriver) manualCleanupHint(Scope, string) string {
	return fmt.Sprintf("platform %s is not supported", u.goos)
}
