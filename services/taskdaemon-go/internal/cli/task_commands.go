package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
)

// newTaskCommand 创建 task 命令组。
//
// 参数:
//   - ctx: 控制 CLI 命令生命周期的 context。
//   - opts: CLI 参数、输出、配置加载器与入口 hooks。
//   - load: 当前命令树共享的配置加载函数。
//   - sessionToken: 根级 --session-token flag 值指针。
//
// 返回值:
//   - *cobra.Command: 已配置 trigger/cancel/runs 子命令的 task 命令。
func newTaskCommand(ctx context.Context, opts Options, load func() (config.Config, error), sessionToken *string) *cobra.Command {
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
			return opts.Hooks.TaskTrigger(withSessionToken(ctx, *sessionToken), cfg, taskID)
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
			return opts.Hooks.TaskCancel(withSessionToken(ctx, *sessionToken), cfg, taskID)
		},
	})
	task.AddCommand(&cobra.Command{
		Use:   "runs <task-id>",
		Short: "List recent task run history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			taskID, err := parsePositiveTaskID(args[0])
			if err != nil {
				return err
			}
			if opts.Hooks.TaskRuns == nil {
				return fmt.Errorf("task runs hook is not configured")
			}
			runs, err := opts.Hooks.TaskRuns(withSessionToken(ctx, *sessionToken), cfg, taskID)
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), runs)
		},
	})
	return task
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
