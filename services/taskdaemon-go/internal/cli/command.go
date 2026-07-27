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
	Serve        func(context.Context, config.Config, config.LoadOptions) error
	Desktop      func(context.Context, config.Config) error
	DBMigrate    func(context.Context, config.Config) error
	TaskTrigger  func(context.Context, config.Config, int) error
	TaskCancel   func(context.Context, config.Config, int) error
	TaskRuns     func(context.Context, config.Config, int) ([]TaskRunSummary, error)
	ConfigReload func(context.Context, config.Config) (config.ReloadResult, error)
}

// TaskRunSummary 是 CLI 展示执行历史所需的稳定摘要。
type TaskRunSummary struct {
	ID           int    `yaml:"id" json:"id"`
	Trigger      string `yaml:"trigger" json:"trigger"`
	Status       string `yaml:"status" json:"status"`
	ExitCode     *int   `yaml:"exitCode,omitempty" json:"exitCode,omitempty"`
	StartedAt    string `yaml:"startedAt,omitempty" json:"startedAt,omitempty"`
	FinishedAt   string `yaml:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	DurationMs   int64  `yaml:"durationMs" json:"durationMs"`
	ErrorSummary string `yaml:"errorSummary,omitempty" json:"errorSummary,omitempty"`
	Stdout       string `yaml:"stdout,omitempty" json:"stdout,omitempty"`
	Stderr       string `yaml:"stderr,omitempty" json:"stderr,omitempty"`
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
	var sessionToken string
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
	root.PersistentFlags().StringVar(&sessionToken, "session-token", "", "Admin session token for daemon API commands")

	loadOptions := func() config.LoadOptions {
		return config.LoadOptions{
			ConfigPath: configPath,
			Optional:   configPath == "",
		}
	}
	load := func() (config.Config, error) {
		return loader(loadOptions())
	}

	root.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if opts.Hooks.Serve == nil {
				return fmt.Errorf("serve hook is not configured")
			}
			return opts.Hooks.Serve(ctx, cfg, loadOptions())
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

	root.AddCommand(newDBCommand(ctx, opts, load))
	root.AddCommand(newConfigCommand(ctx, opts, load, loadOptions, &sessionToken))
	root.AddCommand(newTaskCommand(ctx, opts, load, &sessionToken))
	root.AddCommand(newServiceCommand(ctx, opts, loadOptions))

	return root
}
