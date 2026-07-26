package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/transfer"
)

// newDBCommand 创建 db 命令组（migrate + export/import/transfer）。
//
// 参数:
//   - ctx: 控制 CLI 命令生命周期的 context。
//   - opts: CLI 参数、输出、配置加载器与入口 hooks。
//   - load: 当前命令树共享的配置加载函数。
//
// 返回值:
//   - *cobra.Command: 数据库维护命令组。
func newDBCommand(ctx context.Context, opts Options, load func() (config.Config, error)) *cobra.Command {
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

	var (
		exportOut    string
		importIn     string
		batchSize    int
		force        bool
		dryRun       bool
		checkpoint   string
		srcDriver    string
		srcDSN       string
		targetDriver string
		targetDSN    string
	)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export business tables from the configured database to a transfer file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return runDBExport(ctx, cmd.OutOrStdout(), cfg.Database, exportOut, transfer.Options{
				BatchSize: batchSize,
				DryRun:    dryRun,
				Writer:    cmd.OutOrStdout(),
			})
		},
	}
	exportCmd.Flags().StringVar(&exportOut, "output", "", "Output .tdxfer path")
	exportCmd.Flags().IntVar(&batchSize, "batch-size", transfer.DefaultBatchSize, "Rows per batch")
	exportCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print table counts without writing")

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import a transfer file into the configured database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return runDBImport(ctx, cmd.OutOrStdout(), cfg.Database, importIn, transfer.Options{
				BatchSize:      batchSize,
				Force:          force,
				DryRun:         dryRun,
				CheckpointPath: checkpoint,
				Writer:         cmd.OutOrStdout(),
			})
		},
	}
	importCmd.Flags().StringVar(&importIn, "input", "", "Input .tdxfer path")
	importCmd.Flags().IntVar(&batchSize, "batch-size", transfer.DefaultBatchSize, "Rows per batch")
	importCmd.Flags().BoolVar(&force, "force", false, "Allow import into a non-empty target")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print table counts without writing")
	importCmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint file for batched resume")

	transferCmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer business tables from source to target database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			source := cfg.Database
			if strings.TrimSpace(srcDriver) != "" {
				source.Driver = srcDriver
			}
			if strings.TrimSpace(srcDSN) != "" {
				source.DSN = srcDSN
			}
			if strings.TrimSpace(targetDriver) == "" || strings.TrimSpace(targetDSN) == "" {
				return fmt.Errorf("%s: --target-driver and --target-dsn are required", transfer.CodeInvalidInput)
			}
			// 进度输出不打印明文 DSN
			fmt.Fprintf(cmd.OutOrStdout(), "transfer source driver=%s dsn=%s\n", source.Driver, redactConfigValue(source.DSN))
			fmt.Fprintf(cmd.OutOrStdout(), "transfer target driver=%s dsn=%s\n", targetDriver, redactConfigValue(targetDSN))
			report, err := transfer.Transfer(ctx, source, config.DatabaseConfig{
				Driver: targetDriver,
				DSN:    targetDSN,
			}, transfer.Options{
				BatchSize:      batchSize,
				Force:          force,
				DryRun:         dryRun,
				CheckpointPath: checkpoint,
				Writer:         cmd.OutOrStdout(),
			})
			if err != nil {
				return formatTransferError(err)
			}
			return printYAML(cmd.OutOrStdout(), report)
		},
	}
	transferCmd.Flags().StringVar(&srcDriver, "source-driver", "", "Source driver (default: configured database.driver)")
	transferCmd.Flags().StringVar(&srcDSN, "source-dsn", "", "Source DSN (default: configured database.dsn)")
	transferCmd.Flags().StringVar(&targetDriver, "target-driver", "", "Target driver (sqlite|postgres)")
	transferCmd.Flags().StringVar(&targetDSN, "target-dsn", "", "Target DSN")
	transferCmd.Flags().IntVar(&batchSize, "batch-size", transfer.DefaultBatchSize, "Rows per batch")
	transferCmd.Flags().BoolVar(&force, "force", false, "Allow transfer into a non-empty target")
	transferCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print table counts without writing")
	transferCmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint file for batched resume")

	db.AddCommand(exportCmd, importCmd, transferCmd)
	return db
}

// runDBExport 打开配置库并导出。
//
// 参数:
//   - ctx: 上下文。
//   - out: 输出。
//   - dbCfg: 源库配置。
//   - outputPath: 输出文件。
//   - opts: 传输选项。
//
// 返回值:
//   - error: 失败。
func runDBExport(ctx context.Context, out io.Writer, dbCfg config.DatabaseConfig, outputPath string, opts transfer.Options) error {
	fmt.Fprintf(out, "export driver=%s dsn=%s\n", dbCfg.Driver, redactConfigValue(dbCfg.DSN))
	store, err := data.Open(ctx, dbCfg)
	if err != nil {
		return formatTransferError(transfer.AsErrorOrWrap(transfer.CodeConnectFailed, "open source", err))
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return formatTransferError(transfer.AsErrorOrWrap(transfer.CodePermission, "migrate source", err))
	}
	meta, err := transfer.Export(ctx, store, outputPath, opts)
	if err != nil {
		return formatTransferError(err)
	}
	return printYAML(out, meta)
}

// runDBImport 打开配置库并导入。
//
// 参数:
//   - ctx: 上下文。
//   - out: 输出。
//   - dbCfg: 目标库配置。
//   - inputPath: 输入文件。
//   - opts: 传输选项。
//
// 返回值:
//   - error: 失败。
func runDBImport(ctx context.Context, out io.Writer, dbCfg config.DatabaseConfig, inputPath string, opts transfer.Options) error {
	fmt.Fprintf(out, "import driver=%s dsn=%s\n", dbCfg.Driver, redactConfigValue(dbCfg.DSN))
	store, err := data.Open(ctx, dbCfg)
	if err != nil {
		return formatTransferError(transfer.AsErrorOrWrap(transfer.CodeConnectFailed, "open target", err))
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return formatTransferError(transfer.AsErrorOrWrap(transfer.CodePermission, "migrate target", err))
	}
	report, err := transfer.Import(ctx, store, inputPath, opts)
	if err != nil {
		return formatTransferError(err)
	}
	return printYAML(out, report)
}

// formatTransferError 将迁移错误格式化为 CLI 可读错误。
//
// 参数:
//   - err: 原始错误。
//
// 返回值:
//   - error: 带稳定 code 的错误。
func formatTransferError(err error) error {
	if err == nil {
		return nil
	}
	if te, ok := transfer.AsError(err); ok {
		// 避免把底层 DSN 再展开到最终消息：只保留 code + message
		if te.Err != nil {
			return fmt.Errorf("%s: %s", te.Code, te.Message)
		}
		return fmt.Errorf("%s: %s", te.Code, te.Message)
	}
	return err
}
