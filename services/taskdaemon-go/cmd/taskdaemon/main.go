package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"taskdaemon/internal/app"
	"taskdaemon/internal/cli"
	"taskdaemon/internal/config"
	"taskdaemon/internal/desktop"
)

// main 是 taskdaemon 单二进制入口，负责分发 serve、desktop 与 CLI-only 子命令。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。失败时写 stderr 并以非零状态退出。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(ctx, logger, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "taskdaemon: %v\n", err)
		os.Exit(1)
	}
}

// run 执行 CLI 根命令。
//
// 参数:
//   - ctx: 控制命令生命周期的 context。
//   - logger: 结构化 logger。
//   - args: 命令行参数，不包含程序名。
//
// 返回值:
//   - error: 命令执行失败时返回错误。
func run(ctx context.Context, logger *slog.Logger, args []string) error {
	return cli.Execute(ctx, cli.Options{
		Args: args,
		Hooks: cli.Hooks{
			Serve: func(ctx context.Context, cfg config.Config) error {
				return app.New(cfg, logger).Serve(ctx)
			},
			Desktop: func(ctx context.Context, cfg config.Config) error {
				return desktop.Run(ctx, cfg)
			},
			DBMigrate: func(ctx context.Context, cfg config.Config) error {
				return app.New(cfg, logger).MigrateSchema(ctx)
			},
			TaskTrigger: func(ctx context.Context, cfg config.Config, taskID int) error {
				return app.New(cfg, logger).TriggerTask(ctx, taskID, cli.SessionTokenFromContext(ctx))
			},
			TaskCancel: func(ctx context.Context, cfg config.Config, taskID int) error {
				return app.New(cfg, logger).CancelTask(ctx, taskID, cli.SessionTokenFromContext(ctx))
			},
		},
	})
}
