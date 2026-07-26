package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// platformDriver 封装单平台的单元路径、渲染与系统注册操作。
type platformDriver interface {
	name() string
	defaultScope() Scope
	validateScope(scope Scope) error
	unitPath(scope Scope) (string, error)
	render(resolved ResolvedSpec) (Unit, error)
	checkPermission(scope Scope) error
	isInstalled(scope Scope) (bool, string, error)
	writeUnit(unit Unit) error
	removeUnit(unitPath string) error
	register(ctx context.Context, unit Unit, scope Scope) error
	unregister(ctx context.Context, scope Scope, unitPath string) error
	start(ctx context.Context, scope Scope) error
	stop(ctx context.Context, scope Scope) error
	isRunning(ctx context.Context, scope Scope) (bool, string, error)
	installHints(scope Scope) []string
	manualCleanupHint(scope Scope, unitPath string) string
}

// driverForGOOS 按 GOOS 选择平台驱动。
//
// 参数:
//   - goos: 目标操作系统，通常为 runtime.GOOS。
//
// 返回值:
//   - platformDriver: 平台实现。
func driverForGOOS(goos string) platformDriver {
	switch goos {
	case "darwin":
		return darwinDriver{}
	case "linux":
		return linuxDriver{}
	case "windows":
		return windowsDriver{}
	default:
		return unsupportedDriver{goos: goos}
	}
}

// NewManager 创建当前操作系统的服务管理器。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Manager: 平台实现的管理器。
func NewManager() Manager {
	return newManagerWithDriver(driverForGOOS(runtime.GOOS))
}

// manager 是带可注入 driver 的默认 Manager 实现。
type manager struct {
	driver platformDriver
}

// newManagerWithDriver 使用指定 driver 构造管理器（测试可注入）。
//
// 参数:
//   - driver: 平台驱动。
//
// 返回值:
//   - *manager: 管理器实例。
func newManagerWithDriver(driver platformDriver) *manager {
	return &manager{driver: driver}
}

// Platform 返回平台名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 平台名。
func (m *manager) Platform() string {
	return m.driver.name()
}

// DefaultScope 返回平台默认作用域。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Scope: 默认作用域。
func (m *manager) DefaultScope() Scope {
	return m.driver.defaultScope()
}

// Render 纯函数生成单元内容。
//
// 参数:
//   - spec: 安装规格。
//
// 返回值:
//   - Unit: 单元描述。
//   - error: 解析或渲染失败时返回错误。
func (m *manager) Render(spec Spec) (Unit, error) {
	resolved, err := resolveSpec(spec, m.driver.defaultScope())
	if err != nil {
		return Unit{}, err
	}
	if err := m.driver.validateScope(resolved.Scope); err != nil {
		return Unit{}, err
	}
	return m.driver.render(resolved)
}

// Install 分步安装并在失败时逆序回滚。
//
// 参数:
//   - ctx: 控制生命周期的 context。
//   - spec: 安装规格。
//
// 返回值:
//   - Result: 安装结果。
//   - error: 安装或回滚失败时返回错误。
func (m *manager) Install(ctx context.Context, spec Spec) (Result, error) {
	resolved, err := resolveSpec(spec, m.driver.defaultScope())
	if err != nil {
		return Result{}, err
	}
	if err := m.driver.validateScope(resolved.Scope); err != nil {
		return Result{}, err
	}
	if err := m.driver.checkPermission(resolved.Scope); err != nil {
		return Result{}, err
	}

	installed, existingPath, err := m.driver.isInstalled(resolved.Scope)
	if err != nil {
		return Result{}, newError(CodeInstallFailed, "check existing installation", "", err)
	}
	if installed && !resolved.Force {
		path := existingPath
		if path == "" {
			path, _ = m.driver.unitPath(resolved.Scope)
		}
		return Result{}, newError(
			CodeAlreadyInstalled,
			"service unit already exists",
			fmt.Sprintf("unit path: %s\nre-run with --force to replace, or: taskdaemon service uninstall --scope %s", path, resolved.Scope),
			nil,
		)
	}

	unit, err := m.driver.render(resolved)
	if err != nil {
		return Result{}, err
	}

	type step struct {
		name string
		do   func() error
		undo func() error
	}

	var undos []func() error
	run := func(s step) error {
		if err := s.do(); err != nil {
			return err
		}
		if s.undo != nil {
			undos = append(undos, s.undo)
		}
		return nil
	}

	// force 时先尝试停止/注销旧注册，再覆盖写文件。
	if installed && resolved.Force {
		_ = m.driver.stop(ctx, resolved.Scope)
		_ = m.driver.unregister(ctx, resolved.Scope, unit.Path)
	}

	steps := []step{
		{
			name: "write unit",
			do: func() error {
				if err := m.driver.writeUnit(unit); err != nil {
					return newError(CodeUnitWriteFailed, "write service unit", m.driver.manualCleanupHint(resolved.Scope, unit.Path), err)
				}
				return nil
			},
			undo: func() error {
				return m.driver.removeUnit(unit.Path)
			},
		},
		{
			name: "register",
			do: func() error {
				if err := m.driver.register(ctx, unit, resolved.Scope); err != nil {
					return newError(CodeRegisterFailed, "register service with OS", m.driver.manualCleanupHint(resolved.Scope, unit.Path), err)
				}
				return nil
			},
			undo: func() error {
				return m.driver.unregister(ctx, resolved.Scope, unit.Path)
			},
		},
		{
			name: "start",
			do: func() error {
				if err := m.driver.start(ctx, resolved.Scope); err != nil {
					return newError(CodeStartFailed, "start service", m.driver.manualCleanupHint(resolved.Scope, unit.Path), err)
				}
				return nil
			},
			undo: func() error {
				return m.driver.stop(ctx, resolved.Scope)
			},
		},
	}

	for _, s := range steps {
		if err := run(s); err != nil {
			rollbackErrs := rollbackNamed(undos)
			if len(rollbackErrs) > 0 {
				return Result{}, newError(
					CodeRollbackFailed,
					fmt.Sprintf("install failed at %s and rollback was incomplete", s.name),
					fmt.Sprintf("%s\nrollback errors: %s", m.driver.manualCleanupHint(resolved.Scope, unit.Path), strings.Join(rollbackErrs, "; ")),
					err,
				)
			}
			return Result{}, err
		}
	}

	return Result{
		Action:     "install",
		Scope:      resolved.Scope,
		UnitPath:   unit.Path,
		ConfigPath: resolved.ConfigPath,
		Executable: resolved.Executable,
		Messages:   m.driver.installHints(resolved.Scope),
	}, nil
}

// Uninstall 注销服务并清理单元；未安装时返回明确提示的成功结果。
//
// 参数:
//   - ctx: 控制生命周期的 context。
//   - scope: 作用域；空则默认。
//
// 返回值:
//   - Result: 卸载结果。
//   - error: 卸载失败时返回错误。
func (m *manager) Uninstall(ctx context.Context, scope Scope) (Result, error) {
	if scope == "" {
		scope = m.driver.defaultScope()
	}
	if err := m.driver.validateScope(scope); err != nil {
		return Result{}, err
	}
	if err := m.driver.checkPermission(scope); err != nil {
		return Result{}, err
	}

	unitPath, err := m.driver.unitPath(scope)
	if err != nil {
		return Result{}, err
	}

	installed, existingPath, err := m.driver.isInstalled(scope)
	if err != nil {
		return Result{}, newError(CodeUninstallFailed, "check existing installation", "", err)
	}
	if existingPath != "" {
		unitPath = existingPath
	}

	if !installed {
		return Result{
			Action:   "uninstall",
			Scope:    scope,
			UnitPath: unitPath,
			Messages: []string{"service is not installed; nothing to do"},
		}, nil
	}

	// 尽力停止 → 注销 → 删文件；任一步失败汇总。
	var failures []string
	if err := m.driver.stop(ctx, scope); err != nil {
		failures = append(failures, "stop: "+err.Error())
	}
	if err := m.driver.unregister(ctx, scope, unitPath); err != nil {
		failures = append(failures, "unregister: "+err.Error())
	}
	if err := m.driver.removeUnit(unitPath); err != nil {
		failures = append(failures, "remove unit: "+err.Error())
	}
	if len(failures) > 0 {
		return Result{}, newError(
			CodeUninstallFailed,
			"uninstall incomplete",
			fmt.Sprintf("%s\nerrors: %s", m.driver.manualCleanupHint(scope, unitPath), strings.Join(failures, "; ")),
			nil,
		)
	}

	return Result{
		Action:   "uninstall",
		Scope:    scope,
		UnitPath: unitPath,
		Messages: []string{"service uninstalled"},
	}, nil
}

// Status 查询注册与运行状态。
//
// 参数:
//   - ctx: 控制生命周期的 context。
//   - scope: 作用域；空则默认。
//
// 返回值:
//   - Status: 状态快照。
//   - error: 查询失败时返回错误。
func (m *manager) Status(ctx context.Context, scope Scope) (Status, error) {
	if scope == "" {
		scope = m.driver.defaultScope()
	}
	if err := m.driver.validateScope(scope); err != nil {
		return Status{}, err
	}

	unitPath, err := m.driver.unitPath(scope)
	if err != nil {
		return Status{}, err
	}
	installed, existingPath, err := m.driver.isInstalled(scope)
	if err != nil {
		return Status{}, newError(CodeStatusFailed, "check installation", "", err)
	}
	if existingPath != "" {
		unitPath = existingPath
	}

	running := false
	detail := ""
	if installed {
		running, detail, err = m.driver.isRunning(ctx, scope)
		if err != nil {
			detail = err.Error()
		}
	}

	label := ""
	if unit, renderErr := m.driver.render(ResolvedSpec{
		Scope:      scope,
		Executable: "taskdaemon",
		ConfigPath: "config.yaml",
		WorkingDir: "/",
	}); renderErr == nil {
		label = unit.Label
	}

	return Status{
		Platform:  m.driver.name(),
		Scope:     scope,
		Installed: installed,
		Running:   running,
		UnitPath:  unitPath,
		Label:     label,
		Detail:    detail,
	}, nil
}

// rollbackNamed 逆序回滚已完成步骤。
//
// 参数:
//   - undos: 已完成步骤的 undo 闭包（正序）。
//
// 返回值:
//   - []string: 失败信息。
func rollbackNamed(undos []func() error) []string {
	var errs []string
	for i := len(undos) - 1; i >= 0; i-- {
		if undos[i] == nil {
			continue
		}
		if err := undos[i](); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return errs
}

// writeFileAtomic 以指定权限写入文件，必要时创建父目录。
//
// 参数:
//   - path: 目标路径。
//   - content: 文件内容。
//   - mode: 文件权限。
//
// 返回值:
//   - error: 写入失败时返回错误。
func writeFileAtomic(path, content string, mode os.FileMode) error {
	if mode == 0 {
		mode = defaultUnitMode()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, mode)
}

// removeFileIfExists 删除文件；不存在则忽略。
//
// 参数:
//   - path: 文件路径。
//
// 返回值:
//   - error: 删除失败时返回错误。
func removeFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

// fileExists 判断路径是否为已存在文件。
//
// 参数:
//   - path: 路径。
//
// 返回值:
//   - bool: 存在且非目录。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// runCommand 执行外部命令并返回合并输出。
//
// 参数:
//   - ctx: context。
//   - name: 命令。
//   - args: 参数。
//
// 返回值:
//   - string: stdout+stderr。
//   - error: 非零退出时返回错误。
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// currentUID 返回当前用户 UID 字符串。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: UID。
func currentUID() string {
	return strconv.Itoa(os.Getuid())
}

// isRoot 判断当前进程是否为 root（uid 0）。Windows 上 Geteuid 为 -1，返回 false。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 是 root 时为 true。
func isRoot() bool {
	return os.Geteuid() == 0
}
