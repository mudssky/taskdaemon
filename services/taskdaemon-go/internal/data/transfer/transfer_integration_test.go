//go:build integration

package transfer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/transfer"
)

// TestTransferSQLiteToPostgresIntegration 验证 SQLite → PostgreSQL 端到端迁移与序列复位。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。Docker 不可用时跳过。
func TestTransferSQLiteToPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	srcPath := t.TempDir() + "/src-fixed.db"
	// 导出到文件再导入 PG，避免复用临时 store 路径语义
	out := t.TempDir() + "/dump.tdxfer"
	_, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("taskdaemon"),
		postgres.WithUsername("taskdaemon"),
		postgres.WithPassword("taskdaemon"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	if err != nil {
		t.Skipf("postgres testcontainer unavailable: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	dst, err := data.Open(ctx, config.DatabaseConfig{Driver: "postgres", DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = dst.Close() })
	require.NoError(t, dst.Migrate(ctx))

	report, err := transfer.Import(ctx, dst, out, transfer.Options{})
	require.NoError(t, err)
	require.True(t, report.OK)

	v, err := transfer.VerifyStores(ctx, src, dst, transfer.Options{})
	require.NoError(t, err)
	require.True(t, v.OK, "%+v", v)

	// 序列复位：后续插入不冲突
	_, err = dst.SQL().ExecContext(ctx, `
INSERT INTO admins (singleton_key, username, password_hash, active, created_at, updated_at)
VALUES ('singleton2', 'admin2', 'hash2', true, NOW(), NOW())`)
	require.NoError(t, err)
	_ = srcPath
}
