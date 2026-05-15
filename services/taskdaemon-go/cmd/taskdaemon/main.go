package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"taskdaemon/internal/app"
	"taskdaemon/internal/cli"
	"taskdaemon/internal/config"
	"taskdaemon/internal/desktop"
	"taskdaemon/internal/logging"
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

	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "taskdaemon: %v\n", err)
		os.Exit(1)
	}
}

// run 执行 CLI 根命令。
//
// 参数:
//   - ctx: 控制命令生命周期的 context。
//   - args: 命令行参数，不包含程序名。
//
// 返回值:
//   - error: 命令执行失败时返回错误。
func run(ctx context.Context, args []string) error {
	return cli.Execute(ctx, cli.Options{
		Args: args,
		Hooks: cli.Hooks{
			Serve: func(ctx context.Context, cfg config.Config) error {
				logger, err := logging.New(cfg.Logging, logging.Options{})
				if err != nil {
					return err
				}
				defer logger.Close()
				return app.New(cfg, logger.Logger).Serve(ctx)
			},
			Desktop: func(ctx context.Context, cfg config.Config) error {
				logger, err := logging.New(cfg.Logging, logging.Options{})
				if err != nil {
					return err
				}
				defer logger.Close()
				return desktop.Run(ctx, cfg, logger.Logger)
			},
			DBMigrate: func(ctx context.Context, cfg config.Config) error {
				logger, err := logging.New(cfg.Logging, logging.Options{})
				if err != nil {
					return err
				}
				defer logger.Close()
				return app.New(cfg, logger.Logger).MigrateSchema(ctx)
			},
			TaskTrigger: func(ctx context.Context, cfg config.Config, taskID int) error {
				logger, err := logging.New(cfg.Logging, logging.Options{})
				if err != nil {
					return err
				}
				defer logger.Close()
				return app.New(cfg, logger.Logger).TriggerTask(ctx, taskID, cli.SessionTokenFromContext(ctx))
			},
			TaskCancel: func(ctx context.Context, cfg config.Config, taskID int) error {
				logger, err := logging.New(cfg.Logging, logging.Options{})
				if err != nil {
					return err
				}
				defer logger.Close()
				return app.New(cfg, logger.Logger).CancelTask(ctx, taskID, cli.SessionTokenFromContext(ctx))
			},
		},
	})
}
