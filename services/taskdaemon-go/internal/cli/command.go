package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"taskdaemon/internal/config"
)

// Hooks 保存不同入口模式的执行函数。
type Hooks struct {
	Serve        func(context.Context, config.Config, config.LoadOptions) error
	Desktop      func(context.Context, config.Config) error
	DBMigrate    func(context.Context, config.Config) error
	TaskTrigger  func(context.Context, config.Config, int) error
	TaskCancel   func(context.Context, config.Config, int) error
	ConfigReload func(context.Context, config.Config) (config.ReloadResult, error)
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
			result, err := opts.Hooks.ConfigReload(withSessionToken(ctx, sessionToken), cfg)
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), result)
		},
	})
	root.AddCommand(configCommand)

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

// printConfigPaths 输出配置文件解析结果。
//
// 参数:
//   - writer: 输出 writer。
//   - paths: 配置路径解析结果。
//
// 返回值:
//   - 无。
func printConfigPaths(writer io.Writer, paths config.ResolvedPaths) {
	fmt.Fprintf(writer, "config: %s\n", paths.ConfigPath)
	fmt.Fprintf(writer, "configExists: %t\n", paths.ConfigExists)
	fmt.Fprintf(writer, "localConfig: %s\n", paths.LocalConfigPath)
	fmt.Fprintf(writer, "localConfigExists: %t\n", paths.LocalConfigExists)
	fmt.Fprintf(writer, "explicitConfig: %t\n", paths.ExplicitConfigPath)
}

// printYAML 将结构化值输出为 YAML。
//
// 参数:
//   - writer: 输出 writer。
//   - value: 待输出值。
//
// 返回值:
//   - error: YAML 编码失败或写入失败时返回错误。
func printYAML(writer io.Writer, value any) error {
	content, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = writer.Write(content)
	return err
}

// sanitizedConfig 返回适合 CLI 展示的脱敏配置。
//
// 参数:
//   - cfg: 应用配置。
//
// 返回值:
//   - map[string]any: 可序列化的脱敏配置。
func sanitizedConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"server": map[string]any{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
			"swagger": map[string]any{
				"enabled": cfg.Server.Swagger.Enabled,
			},
		},
		"database": map[string]any{
			"driver": cfg.Database.Driver,
			"dsn":    redactConfigValue(cfg.Database.DSN),
		},
		"logging": map[string]any{
			"level":  cfg.Logging.Level,
			"output": cfg.Logging.Output,
			"console": map[string]any{
				"format": cfg.Logging.Console.Format,
			},
			"file": map[string]any{
				"path":       cfg.Logging.File.Path,
				"format":     cfg.Logging.File.Format,
				"maxSizeMB":  cfg.Logging.File.MaxSizeMB,
				"maxBackups": cfg.Logging.File.MaxBackups,
				"maxAgeDays": cfg.Logging.File.MaxAgeDays,
				"compress":   cfg.Logging.File.Compress,
			},
			"http": map[string]any{
				"includeRequestBody":  cfg.Logging.HTTP.IncludeRequestBody,
				"includeResponseBody": cfg.Logging.HTTP.IncludeResponseBody,
				"maxBodyBytes":        cfg.Logging.HTTP.MaxBodyBytes,
				"redactFields":        cfg.Logging.HTTP.RedactFields,
			},
		},
		"observability": map[string]any{
			"traceId": map[string]any{
				"includeInResponse": cfg.Observability.TraceID.IncludeInResponse,
			},
		},
	}
}

// redactConfigValue 脱敏配置展示中的敏感字符串。
//
// 参数:
//   - value: 原始配置值。
//
// 返回值:
//   - string: 脱敏后的配置值。
func redactConfigValue(value string) string {
	if value == "" {
		return value
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "password") || strings.Contains(lower, "://") && strings.Contains(value, "@") {
		return "[REDACTED]"
	}
	return value
}

// marshalConfigJSON 将配置转为 JSON 字符串，供测试断言使用。
//
// 参数:
//   - value: 待序列化值。
//
// 返回值:
//   - string: JSON 字符串；序列化失败时为空。
func marshalConfigJSON(value any) string {
	content, _ := json.Marshal(value)
	return string(content)
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
