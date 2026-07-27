package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
	"taskdaemon/internal/service"
)

// newServiceCommand 创建 service 命令组（install / uninstall / status）。
//
// 参数:
//   - ctx: 控制 CLI 命令生命周期的 context。
//   - opts: CLI 参数与输出。
//   - loadOptions: 共享的配置加载参数函数（读取根级 --config）。
//
// 返回值:
//   - *cobra.Command: service 命令。
//
// 说明:
//   - 本文件禁止出现 runtime.GOOS 分支；平台差异全部在 internal/service。
func newServiceCommand(ctx context.Context, opts Options, loadOptions func() config.LoadOptions) *cobra.Command {
	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Install, uninstall, or inspect the OS service unit",
	}

	serviceCmd.AddCommand(newServiceInstallCommand(ctx, opts, loadOptions))
	serviceCmd.AddCommand(newServiceUninstallCommand(ctx, opts))
	serviceCmd.AddCommand(newServiceStatusCommand(ctx, opts))
	return serviceCmd
}

// newServiceInstallCommand 创建 service install 子命令。
//
// 参数:
//   - ctx: context。
//   - opts: CLI 选项。
//   - loadOptions: 配置加载参数。
//
// 返回值:
//   - *cobra.Command: install 命令。
func newServiceInstallCommand(ctx context.Context, opts Options, loadOptions func() config.LoadOptions) *cobra.Command {
	var (
		scope  string
		force  bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate and register the OS service unit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mgr := service.NewManager()
			spec := service.Spec{
				ConfigPath: loadOptions().ConfigPath,
				Scope:      service.Scope(scope),
				Force:      force,
			}
			if dryRun {
				unit, err := mgr.Render(spec)
				if err != nil {
					return err
				}
				return printServiceDryRun(cmd.OutOrStdout(), mgr, spec, unit)
			}
			result, err := mgr.Install(ctx, spec)
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "", "Service scope: user or system (platform default if empty)")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing service unit")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print unit content and paths without changes")
	return cmd
}

// newServiceUninstallCommand 创建 service uninstall 子命令。
//
// 参数:
//   - ctx: context。
//   - opts: CLI 选项。
//
// 返回值:
//   - *cobra.Command: uninstall 命令。
func newServiceUninstallCommand(ctx context.Context, opts Options) *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Unregister the OS service and remove its unit file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = opts
			mgr := service.NewManager()
			result, err := mgr.Uninstall(ctx, service.Scope(scope))
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "", "Service scope: user or system (platform default if empty)")
	return cmd
}

// newServiceStatusCommand 创建 service status 子命令。
//
// 参数:
//   - ctx: context。
//   - opts: CLI 选项。
//
// 返回值:
//   - *cobra.Command: status 命令。
func newServiceStatusCommand(ctx context.Context, opts Options) *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show service registration and running state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = opts
			mgr := service.NewManager()
			status, err := mgr.Status(ctx, service.Scope(scope))
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), status)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "", "Service scope: user or system (platform default if empty)")
	return cmd
}

// printServiceDryRun 按 design §8 打印 dry-run 信息。
//
// 参数:
//   - writer: 输出目标。
//   - mgr: 服务管理器。
//   - spec: 原始规格。
//   - unit: 渲染结果。
//
// 返回值:
//   - error: 写入失败时返回错误。
func printServiceDryRun(writer io.Writer, mgr service.Manager, spec service.Spec, unit service.Unit) error {
	configPath := ""
	executable := ""
	if len(unit.ProgramArguments) >= 4 {
		executable = unit.ProgramArguments[0]
		configPath = unit.ProgramArguments[3]
	} else if len(unit.ProgramArguments) > 0 {
		executable = unit.ProgramArguments[0]
	}
	scope := spec.Scope
	if scope == "" {
		scope = mgr.DefaultScope()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "将写入: %s\n", unit.Path)
	fmt.Fprintf(&b, "配置路径: %s\n", configPath)
	fmt.Fprintf(&b, "可执行文件: %s\n", executable)
	fmt.Fprintf(&b, "作用域: %s\n", scope)
	fmt.Fprintf(&b, "平台: %s\n", mgr.Platform())
	fmt.Fprintf(&b, "--- 单元内容 ---\n%s", unit.Content)
	if !strings.HasSuffix(unit.Content, "\n") {
		b.WriteString("\n")
	}
	_, err := io.WriteString(writer, b.String())
	return err
}
