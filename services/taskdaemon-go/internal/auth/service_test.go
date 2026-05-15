package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
)

// TestInitializeAdminAndLogin 验证单管理员初始化、登录和 session 鉴权闭环。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminAndLogin(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	service := New(store, Options{
		BcryptCost: bcryptMinCostForTest,
		SessionTTL: time.Hour,
		Now:        func() time.Time { return time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC) },
	})

	admin, err := service.InitializeAdmin(ctx, "admin", "correct horse battery staple")
	require.NoError(t, err)
	require.Equal(t, "admin", admin.Username)

	_, err = service.InitializeAdmin(ctx, "other", "password")
	require.ErrorIs(t, err, ErrAdminAlreadyInitialized)

	login, err := service.Login(ctx, "admin", "correct horse battery staple", LoginMetadata{
		UserAgent: "test-agent",
		IP:        "127.0.0.1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, login.SessionToken)
	require.NotEmpty(t, login.CSRFToken)

	principal, err := service.AuthenticateSession(ctx, login.SessionToken)
	require.NoError(t, err)
	require.Equal(t, admin.ID, principal.AdminID)
	require.Equal(t, "admin", principal.Username)
}

// TestAdminInitializationStatus 验证认证服务能区分首次启动和已初始化状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAdminInitializationStatus(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	service := New(store, Options{BcryptCost: bcryptMinCostForTest})

	initialized, err := service.IsAdminInitialized(ctx)
	require.NoError(t, err)
	require.False(t, initialized)

	_, err = service.InitializeAdmin(ctx, "admin", "secret")
	require.NoError(t, err)

	initialized, err = service.IsAdminInitialized(ctx)
	require.NoError(t, err)
	require.True(t, initialized)
}

// TestInitializeAdminWithSession 验证首次创建管理员后会立即建立可用 session。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminWithSession(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	service := New(store, Options{
		BcryptCost: bcryptMinCostForTest,
		SessionTTL: time.Hour,
		Now:        func() time.Time { return time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC) },
	})

	result, err := service.InitializeAdminWithSession(ctx, "admin", "secret", LoginMetadata{
		UserAgent: "test-agent",
		IP:        "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, "admin", result.Principal.Username)
	require.NotEmpty(t, result.SessionToken)
	require.NotEmpty(t, result.CSRFToken)

	principal, err := service.AuthenticateSession(ctx, result.SessionToken)
	require.NoError(t, err)
	require.Equal(t, result.Principal.AdminID, principal.AdminID)
	require.Equal(t, "admin", principal.Username)

	_, err = service.InitializeAdminWithSession(ctx, "other", "secret", LoginMetadata{})
	require.ErrorIs(t, err, ErrAdminAlreadyInitialized)
}

// TestLogoutInvalidatesSession 验证退出登录会删除当前 session。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestLogoutInvalidatesSession(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	service := New(store, Options{BcryptCost: bcryptMinCostForTest})
	_, err := service.InitializeAdmin(ctx, "admin", "secret")
	require.NoError(t, err)
	login, err := service.Login(ctx, "admin", "secret", LoginMetadata{})
	require.NoError(t, err)

	require.NoError(t, service.Logout(ctx, login.SessionToken))

	_, err = service.AuthenticateSession(ctx, login.SessionToken)
	require.ErrorIs(t, err, ErrInvalidSession)
}

// TestLoginRejectsInvalidPassword 验证登录失败不会创建 session，也不会泄漏具体原因。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestLoginRejectsInvalidPassword(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	service := New(store, Options{BcryptCost: bcryptMinCostForTest})
	_, err := service.InitializeAdmin(ctx, "admin", "secret")
	require.NoError(t, err)

	_, err = service.Login(ctx, "admin", "wrong", LoginMetadata{})

	require.True(t, errors.Is(err, ErrInvalidCredentials))
}

// newTestStore 创建已完成 migration 的 SQLite 测试数据层。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - *data.Store: 测试数据层实例，测试结束时自动关闭。
func newTestStore(t *testing.T) *data.Store {
	t.Helper()
	ctx := context.Background()
	store, err := data.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.Join(t.TempDir(), "taskdaemon.db") + "?_fk=1",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})
	require.NoError(t, store.Migrate(ctx))
	return store
}
