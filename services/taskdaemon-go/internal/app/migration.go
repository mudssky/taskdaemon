package app

import (
	"context"
	"fmt"

	"taskdaemon/internal/data"
)

// MigrateSchema 对当前配置数据库执行 schema migration。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//
// 返回值:
//   - error: 数据库打开、migration 或关闭连接失败时返回错误。
func (app *App) MigrateSchema(ctx context.Context) error {
	store, err := data.Open(ctx, app.cfg.Database)
	if err != nil {
		return fmt.Errorf("open data store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			app.logger.Warn("close data store failed", "component", "data", "error", err)
		}
	}()

	if err := app.migrateStore(ctx, store); err != nil {
		return err
	}
	return nil
}

// migrateStore 对已打开的数据层执行 schema migration。
//
// 参数:
//   - ctx: 控制 migration 生命周期的 context。
//   - store: 已打开的数据层连接。
//
// 返回值:
//   - error: migration 失败时返回带上下文的错误。
func (app *App) migrateStore(ctx context.Context, store *data.Store) error {
	app.logger.Info("schema migration started", "driver", app.cfg.Database.Driver)
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	app.logger.Info("schema migration completed", "driver", app.cfg.Database.Driver)
	return nil
}
