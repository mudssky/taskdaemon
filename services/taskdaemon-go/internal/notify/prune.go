package notify

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"taskdaemon/internal/data/ent/notification"
)

// prune 按条数与天数双维度淘汰旧通知。
// 配置为 0 表示该维度不限；两维度同时生效，任一触发即淘汰。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - error: 查询或删除失败时返回错误。
func (sink *StoreSink) prune(ctx context.Context) error {
	if sink == nil || sink.store == nil {
		return nil
	}
	sink.pruneMu.Lock()
	defer sink.pruneMu.Unlock()

	cfg := sink.storeConfig()
	if err := sink.pruneByMaxRecords(ctx, cfg.MaxRecords); err != nil {
		return err
	}
	if err := sink.pruneByRetainDays(ctx, cfg.RetainDays); err != nil {
		return err
	}
	return nil
}

// pruneByMaxRecords 按条数上限淘汰最旧记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - limit: 最大保留条数；<=0 表示不限。
//
// 返回值:
//   - error: 查询或删除失败时返回错误。
func (sink *StoreSink) pruneByMaxRecords(ctx context.Context, limit int) error {
	if limit <= 0 {
		return nil
	}
	count, err := sink.store.Client().Notification.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("count notifications for prune: %w", err)
	}
	if count <= limit {
		return nil
	}
	overflow := count - limit
	ids, err := sink.store.Client().Notification.Query().
		Order(notification.ByOccurredAt(sql.OrderAsc()), notification.ByID(sql.OrderAsc())).
		Limit(overflow).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("query old notifications for prune: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := sink.store.Client().Notification.Delete().Where(notification.IDIn(ids...)).Exec(ctx); err != nil {
		return fmt.Errorf("delete overflow notifications: %w", err)
	}
	return nil
}

// pruneByRetainDays 按保留天数淘汰过期记录。
//
// 参数:
//   - ctx: 请求上下文。
//   - days: 保留天数；<=0 表示不限。
//
// 返回值:
//   - error: 删除失败时返回错误。
func (sink *StoreSink) pruneByRetainDays(ctx context.Context, days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	if _, err := sink.store.Client().Notification.Delete().
		Where(notification.OccurredAtLT(cutoff)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete expired notifications: %w", err)
	}
	return nil
}
