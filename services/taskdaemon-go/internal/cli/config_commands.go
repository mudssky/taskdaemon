package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
)

// newConfigCommand 创建 config 命令组。
//
// 参数:
//   - ctx: 控制 CLI 命令生命周期的 context。
//   - opts: CLI 参数、输出、配置加载器与入口 hooks。
//   - load: 当前命令树共享的配置加载函数。
//   - loadOptions: 当前命令树共享的配置加载参数函数。
//   - sessionToken: 根级 --session-token flag 值指针。
//
// 返回值:
//   - *cobra.Command: 已配置 path/show/validate/reload 子命令的 config 命令。
func newConfigCommand(ctx context.Context, opts Options, load func() (config.Config, error), loadOptions func() config.LoadOptions, sessionToken *string) *cobra.Command {
	configCommand := &cobra.Command{
		Use:   "config",
		Short: "Configuration inspection and runtime reload commands",
	}
	configCommand.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config file paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := config.ResolvePaths(loadOptions())
			if err != nil {
				return err
			}
			printConfigPaths(cmd.OutOrStdout(), paths)
			return nil
		},
	})
	configCommand.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print merged config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), sanitizedConfig(cfg))
		},
	})
	configCommand.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate config can be loaded",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := load(); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "config ok")
			return err
		},
	})
	configCommand.AddCommand(&cobra.Command{
		Use:   "reload",
		Short: "Reload runtime config in the running daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if opts.Hooks.ConfigReload == nil {
				return fmt.Errorf("config reload hook is not configured")
			}
			result, err := opts.Hooks.ConfigReload(withSessionToken(ctx, *sessionToken), cfg)
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), result)
		},
	})
	return configCommand
}
