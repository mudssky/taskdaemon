package transfer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
)

// Transfer 将源库直接迁移到目标库（内部经临时 .tdxfer，不落用户路径）。
//
// 参数:
//   - ctx: 上下文。
//   - source: 源库配置。
//   - target: 目标库配置。
//   - opts: 迁移选项。
//
// 返回值:
//   - *VerifyReport: 校验报告。
//   - error: 失败时返回错误。
func Transfer(ctx context.Context, source, target config.DatabaseConfig, opts Options) (*VerifyReport, error) {
	printBackupHint(opts.Writer)

	src, err := openMigrated(ctx, source)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	if opts.DryRun {
		// dry-run 只扫源侧
		meta, err := Export(ctx, src, "", opts)
		if err != nil {
			return nil, err
		}
		printDryRun(opts.Writer, "transfer", meta.Tables)
		return &VerifyReport{OK: true}, nil
	}

	tmpDir, err := os.MkdirTemp("", "taskdaemon-dbxfer-*")
	if err != nil {
		return nil, newError(CodeImportFailed, "create temp dir", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "stream.tdxfer")

	// 导出到临时文件（流式写）
	exportOpts := opts
	exportOpts.DryRun = false
	if _, err := Export(ctx, src, tmpFile, exportOpts); err != nil {
		return nil, err
	}

	dst, err := openMigrated(ctx, target)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	importOpts := opts
	importOpts.DryRun = false
	// 备份提示已打印，避免重复刷屏
	importOpts.Writer = opts.Writer
	return Import(ctx, dst, tmpFile, importOpts)
}

// openMigrated 打开数据库并执行 schema migration。
//
// 参数:
//   - ctx: 上下文。
//   - cfg: 数据库配置。
//
// 返回值:
//   - *data.Store: 已 migrate 的 store。
//   - error: 失败。
func openMigrated(ctx context.Context, cfg config.DatabaseConfig) (*data.Store, error) {
	if strings.TrimSpace(cfg.Driver) == "" || strings.TrimSpace(cfg.DSN) == "" {
		return nil, newError(CodeInvalidInput, "database driver and dsn are required", nil)
	}
	store, err := data.Open(ctx, cfg)
	if err != nil {
		return nil, newError(CodeConnectFailed, fmt.Sprintf("open %s database", cfg.Driver), err)
	}
	if err := store.Migrate(ctx); err != nil {
		_ = store.Close()
		return nil, newError(CodePermission, "migrate schema", err)
	}
	return store, nil
}
