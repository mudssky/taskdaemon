package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderLaunchdContainsAbsolutePaths 验证 launchd plist 含绝对路径与 KeepAlive。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderLaunchdContainsAbsolutePaths(t *testing.T) {
	resolved := ResolvedSpec{
		Scope:      ScopeUser,
		Executable: "/usr/local/bin/taskdaemon",
		ConfigPath: "/Users/demo/.config/taskdaemon/config.yaml",
		WorkingDir: "/Users/demo",
	}
	unit, err := renderLaunchd(resolved, "/Users/demo/Library/LaunchAgents/com.taskdaemon.daemon.plist")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if unit.Label != launchdLabel {
		t.Fatalf("label = %s", unit.Label)
	}
	if !strings.Contains(unit.Content, "/usr/local/bin/taskdaemon") {
		t.Fatal("missing executable")
	}
	if !strings.Contains(unit.Content, "/Users/demo/.config/taskdaemon/config.yaml") {
		t.Fatal("missing config path")
	}
	if !strings.Contains(unit.Content, "<key>KeepAlive</key>") {
		t.Fatal("missing KeepAlive")
	}
	if strings.Contains(unit.Content, "token") || strings.Contains(unit.Content, "password") {
		t.Fatal("unit must not contain secrets")
	}
	if unit.Mode != 0 && unit.Mode != 0o644 {
		t.Fatalf("mode = %o", unit.Mode)
	}
}

// TestRenderSystemdContainsRestart 验证 systemd unit 含 Restart 与绝对路径。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderSystemdContainsRestart(t *testing.T) {
	resolved := ResolvedSpec{
		Scope:      ScopeUser,
		Executable: "/opt/taskdaemon/taskdaemon",
		ConfigPath: "/home/demo/.config/taskdaemon/config.yaml",
		WorkingDir: "/home/demo",
	}
	unit, err := renderSystemd(resolved, "/home/demo/.config/systemd/user/taskdaemon.service")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(unit.Content, "Restart=on-failure") {
		t.Fatal("missing Restart=on-failure")
	}
	if !strings.Contains(unit.Content, "ExecStart=/opt/taskdaemon/taskdaemon serve --config /home/demo/.config/taskdaemon/config.yaml") {
		t.Fatalf("unexpected ExecStart in:\n%s", unit.Content)
	}
	if strings.Contains(strings.ToLower(unit.Content), "token=") {
		t.Fatal("unit must not contain tokens")
	}
}

// TestRenderWindowsBinPath 验证 Windows 逻辑单元含 BinPath。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderWindowsBinPath(t *testing.T) {
	resolved := ResolvedSpec{
		Scope:      ScopeSystem,
		Executable: `C:\Program Files\taskdaemon\taskdaemon.exe`,
		ConfigPath: `C:\Users\demo\AppData\Roaming\taskdaemon\config.yaml`,
		WorkingDir: `C:\Users\demo`,
	}
	unit, err := renderWindows(resolved)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if unit.Path != windowsUnitPath {
		t.Fatalf("path = %s", unit.Path)
	}
	if !strings.Contains(unit.Content, windowsServiceName) {
		t.Fatal("missing service name")
	}
	if !strings.Contains(unit.Content, `C:\Program Files\taskdaemon\taskdaemon.exe`) {
		t.Fatal("missing executable in content")
	}
}

// TestResolveConfigPathExplicitRelative 验证相对 --config 转为绝对路径。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestResolveConfigPathExplicitRelative(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(rel, []byte("server:\n  port: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 使用相对路径（从 temp 的 base 名构造不可靠）；直接测 Abs 行为：传入相对片段。
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 在 cwd 下创建临时相对文件名
	name := "taskdaemon-service-test-config.yaml"
	path := filepath.Join(cwd, name)
	if err := os.WriteFile(path, []byte("server:\n  port: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got, err := resolveConfigPath(name)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want, _ := filepath.Abs(name)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatal("expected absolute path")
	}
}

// TestResolveConfigPathExplicitAbsolute 验证绝对 --config 原样保留。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestResolveConfigPathExplicitAbsolute(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "config.yaml")
	got, err := resolveConfigPath(abs)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != abs {
		t.Fatalf("got %s want %s", got, abs)
	}
}

// TestManagerRenderUsesDriver 验证 Manager.Render 走注入 driver。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestManagerRenderUsesDriver(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	exe := filepath.Join(dir, "taskdaemon")
	drv := &fakeDriver{
		scope:    ScopeUser,
		unitFile: filepath.Join(dir, "unit.plist"),
		home:     dir,
	}
	m := newManagerWithDriver(drv)
	unit, err := m.Render(Spec{
		ConfigPath: cfg,
		Executable: exe,
		WorkingDir: dir,
		Scope:      ScopeUser,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if unit.Path != drv.unitFile {
		t.Fatalf("path = %s", unit.Path)
	}
	if !strings.Contains(unit.Content, cfg) {
		t.Fatal("config path missing from unit")
	}
}

// TestInstallRejectsDuplicateWithoutForce 验证重复 install 默认拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestInstallRejectsDuplicateWithoutForce(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{
		scope:     ScopeUser,
		unitFile:  filepath.Join(dir, "unit"),
		installed: true,
	}
	m := newManagerWithDriver(drv)
	_, err := m.Install(context.Background(), Spec{
		ConfigPath: filepath.Join(dir, "c.yaml"),
		Executable: filepath.Join(dir, "bin"),
		WorkingDir: dir,
		Scope:      ScopeUser,
	})
	if err == nil {
		t.Fatal("expected already installed error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeAlreadyInstalled {
		t.Fatalf("error = %v", err)
	}
}

// TestInstallForceOverrides 验证 --force 可覆盖。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestInstallForceOverrides(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{
		scope:     ScopeUser,
		unitFile:  filepath.Join(dir, "unit"),
		installed: true,
	}
	m := newManagerWithDriver(drv)
	result, err := m.Install(context.Background(), Spec{
		ConfigPath: filepath.Join(dir, "c.yaml"),
		Executable: filepath.Join(dir, "bin"),
		WorkingDir: dir,
		Scope:      ScopeUser,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if result.Action != "install" {
		t.Fatalf("action = %s", result.Action)
	}
	if !drv.wrote {
		t.Fatal("expected write")
	}
}

// TestInstallRollbackOnRegisterFailure 验证注册失败时删除已写单元。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestInstallRollbackOnRegisterFailure(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{
		scope:        ScopeUser,
		unitFile:     filepath.Join(dir, "unit"),
		failRegister: true,
	}
	m := newManagerWithDriver(drv)
	_, err := m.Install(context.Background(), Spec{
		ConfigPath: filepath.Join(dir, "c.yaml"),
		Executable: filepath.Join(dir, "bin"),
		WorkingDir: dir,
		Scope:      ScopeUser,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeRegisterFailed {
		t.Fatalf("error = %v", err)
	}
	if !drv.removed {
		t.Fatal("expected unit rollback remove")
	}
}

// TestInstallRollbackOnStartFailure 验证启动失败时注销并删除单元。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestInstallRollbackOnStartFailure(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{
		scope:     ScopeUser,
		unitFile:  filepath.Join(dir, "unit"),
		failStart: true,
	}
	m := newManagerWithDriver(drv)
	_, err := m.Install(context.Background(), Spec{
		ConfigPath: filepath.Join(dir, "c.yaml"),
		Executable: filepath.Join(dir, "bin"),
		WorkingDir: dir,
		Scope:      ScopeUser,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeStartFailed {
		t.Fatalf("error = %v", err)
	}
	if !drv.unregistered {
		t.Fatal("expected unregister rollback")
	}
	if !drv.removed {
		t.Fatal("expected remove rollback")
	}
}

// TestUninstallIdempotentWhenMissing 验证未安装时 uninstall 安全。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestUninstallIdempotentWhenMissing(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{scope: ScopeUser, unitFile: filepath.Join(dir, "unit")}
	m := newManagerWithDriver(drv)
	result, err := m.Uninstall(context.Background(), ScopeUser)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected noop message")
	}
}

// TestPermissionDenied 验证权限不足返回稳定错误码与补救命令。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	drv := &fakeDriver{
		scope:            ScopeSystem,
		unitFile:         filepath.Join(dir, "unit"),
		permissionDenied: true,
	}
	m := newManagerWithDriver(drv)
	_, err := m.Install(context.Background(), Spec{
		ConfigPath: filepath.Join(dir, "c.yaml"),
		Executable: filepath.Join(dir, "bin"),
		WorkingDir: dir,
		Scope:      ScopeSystem,
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodePermissionDenied {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(se.Hint, "taskdaemon service install") {
		t.Fatalf("hint should include remediation, got %q", se.Hint)
	}
}

// TestUnsupportedPlatform 验证未知平台明确报错。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestUnsupportedPlatform(t *testing.T) {
	m := newManagerWithDriver(unsupportedDriver{goos: "plan9"})
	_, err := m.Render(Spec{ConfigPath: "/tmp/c.yaml", Executable: "/bin/x", WorkingDir: "/tmp"})
	if err == nil {
		t.Fatal("expected unsupported")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeUnsupportedPlatform {
		t.Fatalf("error = %v", err)
	}
}

// TestWindowsUserScopeRejected 验证 Windows 拒绝 user 作用域。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWindowsUserScopeRejected(t *testing.T) {
	m := newManagerWithDriver(windowsDriver{})
	_, err := m.Render(Spec{
		Scope:      ScopeUser,
		ConfigPath: `C:\cfg.yaml`,
		Executable: `C:\taskdaemon.exe`,
		WorkingDir: `C:\`,
	})
	if err == nil {
		t.Fatal("expected invalid scope")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeInvalidScope {
		t.Fatalf("error = %v", err)
	}
}

// TestWriteFileAtomicMode 验证单元文件权限为 0644。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWriteFileAtomicMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "unit")
	if err := writeFileAtomic(path, "hello", 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %o", info.Mode().Perm())
	}
}

// fakeDriver 是可注入失败点的测试驱动。
type fakeDriver struct {
	scope            Scope
	unitFile         string
	home             string
	installed        bool
	permissionDenied bool
	failRegister     bool
	failStart        bool
	wrote            bool
	removed          bool
	unregistered     bool
	started          bool
}

func (f *fakeDriver) name() string        { return "fake" }
func (f *fakeDriver) defaultScope() Scope { return f.scope }

func (f *fakeDriver) validateScope(scope Scope) error {
	if scope != ScopeUser && scope != ScopeSystem {
		return newError(CodeInvalidScope, "bad scope", "", nil)
	}
	return nil
}

func (f *fakeDriver) unitPath(Scope) (string, error) { return f.unitFile, nil }

func (f *fakeDriver) render(resolved ResolvedSpec) (Unit, error) {
	content := "unit\nexecutable=" + resolved.Executable + "\nconfig=" + resolved.ConfigPath + "\n"
	return Unit{
		Path:             f.unitFile,
		Content:          content,
		Mode:             0o644,
		Label:            "fake.service",
		ProgramArguments: programArguments(resolved.Executable, resolved.ConfigPath),
	}, nil
}

func (f *fakeDriver) checkPermission(Scope) error {
	if f.permissionDenied {
		return newError(CodePermissionDenied, "denied", "re-run: sudo taskdaemon service install --scope system", nil)
	}
	return nil
}

func (f *fakeDriver) isInstalled(Scope) (bool, string, error) {
	return f.installed, f.unitFile, nil
}

func (f *fakeDriver) writeUnit(unit Unit) error {
	f.wrote = true
	f.installed = true
	return os.WriteFile(unit.Path, []byte(unit.Content), 0o644)
}

func (f *fakeDriver) removeUnit(string) error {
	f.removed = true
	f.installed = false
	return removeFileIfExists(f.unitFile)
}

func (f *fakeDriver) register(context.Context, Unit, Scope) error {
	if f.failRegister {
		return errors.New("register boom")
	}
	return nil
}

func (f *fakeDriver) unregister(context.Context, Scope, string) error {
	f.unregistered = true
	return nil
}

func (f *fakeDriver) start(context.Context, Scope) error {
	if f.failStart {
		return errors.New("start boom")
	}
	f.started = true
	return nil
}

func (f *fakeDriver) stop(context.Context, Scope) error { return nil }

func (f *fakeDriver) isRunning(context.Context, Scope) (bool, string, error) {
	return f.started, "fake-running", nil
}

func (f *fakeDriver) installHints(Scope) []string { return []string{"hint"} }

func (f *fakeDriver) manualCleanupHint(_ Scope, path string) string {
	return "cleanup " + path
}
