package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/data/ent/notification"
)

// StoreSinkName 是站内通知 sink 的稳定名称。
const StoreSinkName = "store"

// ErrNotificationNotFound 表示通知记录不存在。
var ErrNotificationNotFound = fmt.Errorf("notification not found")

// StoreSink 是站内通知 sink：持久化事件并提供查询/已读/删除能力。
// 它是 T4/D2 实现其他 sink 时的参考样板。
type StoreSink struct {
	store  *data.Store
	logger *slog.Logger
	mu     sync.RWMutex
	cfg    config.NotifyStoreConfig
	// pruneMu 串行化淘汰，避免并发 count/delete 互相覆盖。
	pruneMu sync.Mutex
}

// StoreSinkOptions 控制站内 sink 可选依赖。
type StoreSinkOptions struct {
	Logger *slog.Logger
}

// NewStoreSink 创建站内通知 sink。
//
// 参数:
//   - store: 数据层实例。
//   - cfg: 站内 sink 配置。
//   - opts: 可选 logger。
//
// 返回值:
//   - *StoreSink: 站内 sink 实例。
func NewStoreSink(store *data.Store, cfg config.NotifyStoreConfig, opts StoreSinkOptions) *StoreSink {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &StoreSink{
		store:  store,
		logger: logger,
		cfg:    cfg,
	}
}

// Name 返回 sink 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 固定为 "store"。
func (sink *StoreSink) Name() string {
	return StoreSinkName
}

// Enabled 返回站内 sink 是否启用。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 启用时返回 true。
func (sink *StoreSink) Enabled() bool {
	if sink == nil {
		return false
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg.Enabled
}

// UpdateConfig 热更新通知配置中的站内 sink 段。
//
// 参数:
//   - cfg: 完整通知配置。
//
// 返回值:
//   - 无。
func (sink *StoreSink) UpdateConfig(cfg config.NotifyConfig) {
	if sink == nil {
		return
	}
	sink.mu.Lock()
	sink.cfg = cfg.Store
	sink.mu.Unlock()
}

// storeConfig 返回当前站内 sink 配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.NotifyStoreConfig: 当前配置。
func (sink *StoreSink) storeConfig() config.NotifyStoreConfig {
	if sink == nil {
		return config.Default().Notify.Store
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg
}

// Deliver 将事件写入站内通知表；重复 event_id 视为幂等成功。
//
// 参数:
//   - ctx: 请求上下文。
//   - event: 待落库事件。
//
// 返回值:
//   - error: 非幂等写入失败时返回错误。
func (sink *StoreSink) Deliver(ctx context.Context, event Event) error {
	if sink == nil || sink.store == nil {
		return fmt.Errorf("store sink is not configured")
	}
	cfg := sink.storeConfig()
	if !cfg.Enabled {
		return nil
	}
	if !severityMeetsMinimum(event.Severity, cfg.MinSeverity) {
		return nil
	}

	severity, err := toEntSeverity(event.Severity)
	if err != nil {
		return err
	}
	create := sink.store.Client().Notification.Create().
		SetEventID(event.ID).
		SetName(string(event.Name)).
		SetSeverity(severity).
		SetTitle(event.Title).
		SetOccurredAt(event.OccurredAt.UTC()).
		SetCreatedAt(time.Now().UTC())
	if event.Subject.Kind != "" {
		create.SetSubjectKind(event.Subject.Kind)
	}
	if event.Subject.ID != "" {
		create.SetSubjectID(event.Subject.ID)
	}
	if event.Body != "" {
		create.SetBody(event.Body)
	}
	if len(event.Detail) > 0 {
		create.SetDetail(event.Detail)
	}
	if _, err := create.Save(ctx); err != nil {
		if ent.IsConstraintError(err) {
			// event_id 唯一约束：重复投递幂等成功
			return nil
		}
		return fmt.Errorf("persist notification: %w", err)
	}

	// 淘汰在写入后异步触发，不在写入事务内。
	go func() {
		pruneCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := sink.prune(pruneCtx); err != nil {
			sink.logger.Warn("notify store prune failed", "error", err)
		}
	}()
	return nil
}

// ListFilter 描述站内通知列表筛选条件。
type ListFilter struct {
	Page     int
	PageSize int
	Read     *bool
	Severity Severity
}

// ListResult 是分页列表结果。
type ListResult struct {
	Items    []*ent.Notification
	Total    int
	Page     int
	PageSize int
}

// List 按时间倒序分页查询通知。
//
// 参数:
//   - ctx: 请求上下文。
//   - filter: 分页与筛选条件。
//
// 返回值:
//   - ListResult: 列表结果。
//   - error: 查询失败时返回错误。
func (sink *StoreSink) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := sink.store.Client().Notification.Query()
	if filter.Read != nil {
		if *filter.Read {
			query = query.Where(notification.ReadAtNotNil())
		} else {
			query = query.Where(notification.ReadAtIsNil())
		}
	}
	if filter.Severity != "" {
		severity, err := toEntSeverity(filter.Severity)
		if err != nil {
			return ListResult{}, err
		}
		query = query.Where(notification.SeverityEQ(severity))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("count notifications: %w", err)
	}
	items, err := query.
		Order(notification.ByOccurredAt(sql.OrderDesc()), notification.ByID(sql.OrderDesc())).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("list notifications: %w", err)
	}
	return ListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UnreadCount 返回未读通知数量。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - int: 未读数。
//   - error: 查询失败时返回错误。
func (sink *StoreSink) UnreadCount(ctx context.Context) (int, error) {
	count, err := sink.store.Client().Notification.Query().
		Where(notification.ReadAtIsNil()).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead 标记单条通知已读。
//
// 参数:
//   - ctx: 请求上下文。
//   - id: 通知 ID。
//
// 返回值:
//   - *ent.Notification: 更新后的记录。
//   - error: 不存在或更新失败时返回错误。
func (sink *StoreSink) MarkRead(ctx context.Context, id int) (*ent.Notification, error) {
	record, err := sink.store.Client().Notification.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("get notification: %w", err)
	}
	if record.ReadAt != nil {
		return record, nil
	}
	now := time.Now().UTC()
	updated, err := sink.store.Client().Notification.UpdateOneID(id).
		SetReadAt(now).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("mark notification read: %w", err)
	}
	return updated, nil
}

// MarkReadBatch 批量标记已读。
//
// 参数:
//   - ctx: 请求上下文。
//   - ids: 通知 ID 列表。
//
// 返回值:
//   - int: 实际更新条数。
//   - error: 更新失败时返回错误。
func (sink *StoreSink) MarkReadBatch(ctx context.Context, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	affected, err := sink.store.Client().Notification.Update().
		Where(
			notification.IDIn(ids...),
			notification.ReadAtIsNil(),
		).
		SetReadAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("batch mark notifications read: %w", err)
	}
	return affected, nil
}

// MarkAllRead 标记全部未读为已读。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - int: 实际更新条数。
//   - error: 更新失败时返回错误。
func (sink *StoreSink) MarkAllRead(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	affected, err := sink.store.Client().Notification.Update().
		Where(notification.ReadAtIsNil()).
		SetReadAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return affected, nil
}

// Delete 删除单条通知。
//
// 参数:
//   - ctx: 请求上下文。
//   - id: 通知 ID。
//
// 返回值:
//   - error: 不存在或删除失败时返回错误。
func (sink *StoreSink) Delete(ctx context.Context, id int) error {
	err := sink.store.Client().Notification.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotificationNotFound
		}
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

// ClearRead 清空全部已读通知。
//
// 参数:
//   - ctx: 请求上下文。
//
// 返回值:
//   - int: 删除条数。
//   - error: 删除失败时返回错误。
func (sink *StoreSink) ClearRead(ctx context.Context) (int, error) {
	affected, err := sink.store.Client().Notification.Delete().
		Where(notification.ReadAtNotNil()).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("clear read notifications: %w", err)
	}
	return affected, nil
}

// toEntSeverity 将 notify.Severity 映射为 Ent enum。
//
// 参数:
//   - severity: 通知严重级别。
//
// 返回值:
//   - notification.Severity: Ent 枚举值。
//   - error: 非法值时返回错误。
func toEntSeverity(severity Severity) (notification.Severity, error) {
	switch severity {
	case SeverityInfo:
		return notification.SeverityInfo, nil
	case SeverityWarning:
		return notification.SeverityWarning, nil
	case SeverityError:
		return notification.SeverityError, nil
	case SeverityCritical:
		return notification.SeverityCritical, nil
	default:
		return "", fmt.Errorf("invalid notify severity %q", severity)
	}
}

// severityMeetsMinimum 判断事件级别是否达到配置的最低投递级别。
//
// 参数:
//   - severity: 事件级别。
//   - min: 配置的最低级别字符串；空表示不限。
//
// 返回值:
//   - bool: 应投递时返回 true。
func severityMeetsMinimum(severity Severity, min string) bool {
	if min == "" {
		return true
	}
	return severityRank(severity) >= severityRank(Severity(min))
}

// severityRank 返回严重级别排序权重。
//
// 参数:
//   - severity: 严重级别。
//
// 返回值:
//   - int: 权重；未知级别返回 -1。
func severityRank(severity Severity) int {
	switch severity {
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	case SeverityCritical:
		return 4
	default:
		return -1
	}
}
