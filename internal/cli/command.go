package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
)

// Hooks 保存不同入口模式的执行函数。
type Hooks struct {
	Serve     func(context.Context, config.Config) error
	Desktop   func(context.Context, config.Config) error
	DBMigrate func(context.Context, config.Config) error
}

// Options 控制 CLI 执行时的输入、输出、配置加载器和入口 hooks。
type Options struct {
	Args       []string
	Stdout     io.Writer
	Stderr     io.Writer
	LoadConfig func(config.LoadOptions) (config.Config, error)
	Hooks      Hooks
}

// Execute 构建并执行 taskdaemon CLI。
//
// 参数:
//   - ctx: 控制 CLI 执行生命周期的 context。
//   - opts: CLI 参数、输出、配置加载器与入口 hooks。
//
// 返回值:
//   - error: 命令执行失败时返回错误。
func Execute(ctx context.Context, opts Options) error {
	root := NewRootCommand(ctx, opts)
	if err := root.Execute(); err != nil {
		return err
	}
	return nil
}

// NewRootCommand 创建 taskdaemon 根命令。
//
// 参数:
//   - ctx: 控制命令执行生命周期的 context。
//   - opts: CLI 参数、输出、配置加载器与入口 hooks。
//
// 返回值:
//   - *cobra.Command: 已配置子命令与 flag 的根命令。
func NewRootCommand(ctx context.Context, opts Options) *cobra.Command {
	var configPath string
	loader := opts.LoadConfig
	if loader == nil {
		loader = config.Load
	}

	root := &cobra.Command{
		Use:           "taskdaemon",
		Short:         "Cross-platform task scheduling daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(opts.Args)
	root.SetOut(writerOrDefault(opts.Stdout, os.Stdout))
	root.SetErr(writerOrDefault(opts.Stderr, os.Stderr))
	root.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")

	load := func() (config.Config, error) {
		return loader(config.LoadOptions{
			ConfigPath: configPath,
			Optional:   configPath == "",
		})
	}

	root.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(*cobra.Command, []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if opts.Hooks.Serve == nil {
				return fmt.Errorf("serve hook is not configured")
			}
			return opts.Hooks.Serve(ctx, cfg)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "desktop",
		Short: "Start the Wails desktop shell",
		RunE: func(*cobra.Command, []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if opts.Hooks.Desktop == nil {
				return fmt.Errorf("desktop hook is not configured")
			}
			return opts.Hooks.Desktop(ctx, cfg)
		},
	})

	db := &cobra.Command{
		Use:   "db",
		Short: "Database maintenance commands",
	}
	db.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Run schema migrations for the configured database",
		RunE: func(*cobra.Command, []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if opts.Hooks.DBMigrate == nil {
				return fmt.Errorf("db migrate hook is not configured")
			}
			return opts.Hooks.DBMigrate(ctx, cfg)
		},
	})
	root.AddCommand(db)

	return root
}

// writerOrDefault 返回 writer 或默认 writer。
//
// 参数:
//   - writer: 调用方提供的 writer。
//   - fallback: writer 为空时使用的默认 writer。
//
// 返回值:
//   - io.Writer: 可用于命令输出的 writer。
func writerOrDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return fallback
}
