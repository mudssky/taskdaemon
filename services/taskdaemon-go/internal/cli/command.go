package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
)

// Hooks 保存不同入口模式的执行函数。
type Hooks struct {
	Serve       func(context.Context, config.Config) error
	Desktop     func(context.Context, config.Config) error
	DBMigrate   func(context.Context, config.Config) error
	TaskTrigger func(context.Context, config.Config, int) error
	TaskCancel  func(context.Context, config.Config, int) error
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

	load := func() (config.Config, error) {
		return loader(config.LoadOptions{
			ConfigPath: configPath,
			Optional:   configPath == "",
		})
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

	task := &cobra.Command{
		Use:   "task",
		Short: "Task operations",
	}
	task.AddCommand(&cobra.Command{
		Use:   "trigger <task-id>",
		Short: "Trigger a task manually",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			taskID, err := parsePositiveTaskID(args[0])
			if err != nil {
				return err
			}
			if opts.Hooks.TaskTrigger == nil {
				return fmt.Errorf("task trigger hook is not configured")
			}
			return opts.Hooks.TaskTrigger(withSessionToken(ctx, sessionToken), cfg, taskID)
		},
	})
	task.AddCommand(&cobra.Command{
		Use:   "cancel <task-id>",
		Short: "Cancel a running task",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			taskID, err := parsePositiveTaskID(args[0])
			if err != nil {
				return err
			}
			if opts.Hooks.TaskCancel == nil {
				return fmt.Errorf("task cancel hook is not configured")
			}
			return opts.Hooks.TaskCancel(withSessionToken(ctx, sessionToken), cfg, taskID)
		},
	})
	root.AddCommand(task)

	return root
}

type sessionTokenContextKey struct{}

// SessionTokenFromContext 读取 CLI 传入的管理员 session token。
//
// 参数:
//   - ctx: CLI 命令 context。
//
// 返回值:
//   - string: 管理员 session token，未配置时为空。
func SessionTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(sessionTokenContextKey{}).(string)
	if token != "" {
		return token
	}
	return os.Getenv("TASKDAEMON_SESSION_TOKEN")
}

// withSessionToken 将命令行 session token 放入 context。
//
// 参数:
//   - ctx: CLI 命令 context。
//   - token: flag 传入的 session token。
//
// 返回值:
//   - context.Context: 携带 token 的 context。
func withSessionToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionTokenContextKey{}, token)
}

// parsePositiveTaskID 解析正整数任务 ID。
//
// 参数:
//   - value: 命令行传入的任务 ID 字符串。
//
// 返回值:
//   - int: 任务 ID。
//   - error: 不是正整数时返回错误。
func parsePositiveTaskID(value string) (int, error) {
	taskID, err := strconv.Atoi(value)
	if err != nil || taskID <= 0 {
		return 0, fmt.Errorf("task id must be a positive integer")
	}
	return taskID, nil
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
