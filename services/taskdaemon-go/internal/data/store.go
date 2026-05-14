package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/data/ent/migrate"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// ErrUnsupportedDriver 表示配置中的数据库方言不是当前版本支持的 sqlite 或 postgres。
var ErrUnsupportedDriver = errors.New("unsupported database driver")

// Store 封装 Ent client 和数据库方言细节。
type Store struct {
	client *ent.Client
}

// Open 根据配置打开数据层连接。
//
// 参数:
//   - ctx: 控制数据库连通性探测的 context。
//   - cfg: 数据库方言和连接串配置。
//
// 返回值:
//   - *Store: 可用于 migration 和 repository 查询的数据层实例。
//   - error: 方言不支持、数据库打开失败或连通性探测失败时返回错误。
func Open(ctx context.Context, cfg config.DatabaseConfig) (*Store, error) {
	driverName, entDialect, err := resolveDriver(cfg.Driver)
	if err != nil {
		return nil, err
	}
	if driverName == sqliteDriverName {
		if err := ensureSQLiteParentDir(cfg.DSN); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if driverName == sqliteDriverName {
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if driverName == sqliteDriverName {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
		}
	}

	driver := entsql.OpenDB(entDialect, db)
	return &Store{client: ent.NewClient(ent.Driver(driver))}, nil
}

// Client 返回当前 Store 持有的 Ent client。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *ent.Client: 数据库访问 client。
func (store *Store) Client() *ent.Client {
	return store.client
}

// Migrate 对当前连接的数据库执行 Ent schema migration。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//
// 返回值:
//   - error: schema migration 失败时返回错误。
func (store *Store) Migrate(ctx context.Context) error {
	if err := store.client.Schema.Create(ctx, migrate.WithForeignKeys(true)); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}

// Close 关闭底层数据库连接。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 关闭数据库连接失败时返回错误。
func (store *Store) Close() error {
	if store == nil || store.client == nil {
		return nil
	}
	return store.client.Close()
}

const (
	sqliteDriverName   = "sqlite"
	postgresDriverName = "postgres"
)

// resolveDriver 将项目配置方言映射到 database/sql driver 和 Ent 方言。
//
// 参数:
//   - driver: 配置中的数据库方言名称。
//
// 返回值:
//   - string: database/sql driver 名称。
//   - string: Ent 方言名称。
//   - error: 方言不支持时返回 ErrUnsupportedDriver。
func resolveDriver(driver string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case sqliteDriverName:
		return sqliteDriverName, dialect.SQLite, nil
	case postgresDriverName:
		return postgresDriverName, dialect.Postgres, nil
	default:
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}
}

// ensureSQLiteParentDir 确保文件型 SQLite DSN 的父目录存在。
//
// 参数:
//   - dsn: SQLite 连接串。
//
// 返回值:
//   - error: 创建父目录失败时返回错误。
func ensureSQLiteParentDir(dsn string) error {
	dbPath := sqliteFilePathFromDSN(dsn)
	if dbPath == "" {
		return nil
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sqlite parent dir: %w", err)
	}
	return nil
}

// sqliteFilePathFromDSN 从 SQLite DSN 中提取需要落盘的数据库文件路径。
//
// 参数:
//   - dsn: SQLite 连接串。
//
// 返回值:
//   - string: 需要创建父目录的数据库文件路径；内存库或无法判断时返回空字符串。
func sqliteFilePathFromDSN(dsn string) string {
	if dsn == "" || strings.Contains(dsn, "mode=memory") {
		return ""
	}
	base, _, _ := strings.Cut(dsn, "?")
	if base == ":memory:" || strings.HasPrefix(base, "file::memory:") {
		return ""
	}
	if !strings.HasPrefix(base, "file:") {
		return base
	}

	raw := strings.TrimPrefix(base, "file:")
	if strings.HasPrefix(raw, "//") {
		parsed, err := url.Parse(base)
		if err == nil {
			raw = parsed.Path
		}
	}
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}
	if runtime.GOOS == "windows" && len(raw) >= 3 && raw[0] == '/' && raw[2] == ':' {
		raw = raw[1:]
	}
	if raw == "" || raw == ":memory:" || strings.HasPrefix(raw, ":memory:") {
		return ""
	}
	return raw
}
