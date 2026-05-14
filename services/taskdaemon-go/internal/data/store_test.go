package data

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

// TestOpenMigrateSQLite 验证默认 SQLite 方言可以打开并执行 Ent schema migration。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestOpenMigrateSQLite(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "taskdaemon.db")

	store, err := Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + dbPath + "?_fk=1",
	})
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Migrate(ctx))

	count, err := store.Client().Admin.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

// TestOpenRejectsUnsupportedDriver 验证数据层会拒绝未声明支持的数据库方言。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	store, err := Open(context.Background(), config.DatabaseConfig{
		Driver: "mysql",
		DSN:    "example",
	})

	require.ErrorIs(t, err, ErrUnsupportedDriver)
	require.Nil(t, store)
}
