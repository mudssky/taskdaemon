//go:build integration

package data

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"taskdaemon/internal/config"
)

// TestPostgresMigrateWithTestcontainers 验证 PostgreSQL 方言可以通过 testcontainers 执行 migration。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止；Docker 不可用时跳过。
func TestPostgresMigrateWithTestcontainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	defer func() {
		require.NoError(t, testcontainers.TerminateContainer(container))
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	store, err := Open(ctx, config.DatabaseConfig{Driver: "postgres", DSN: dsn})
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Migrate(ctx))
}
