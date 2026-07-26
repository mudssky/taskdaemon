package notify

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent/notification"
)

// openTestStore 打开临时 SQLite 并完成 migration。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - *data.Store: 可用数据层。
func openTestStore(t *testing.T) *data.Store {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "notify.db")
	store, err := data.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + dbPath + "?_fk=1",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.Migrate(ctx))
	return store
}

// TestStoreSinkPersistsEventFields 验证事件落库字段正确。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkPersistsEventFields(t *testing.T) {
	store := openTestStore(t)
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 0}, StoreSinkOptions{})
	occurred := time.Unix(1700000000, 0).UTC()
	event := Event{
		ID:         "evt-1",
		Name:       NameTaskRunSucceeded,
		Severity:   SeverityInfo,
		OccurredAt: occurred,
		Subject:    Subject{Kind: "run", ID: "12"},
		Title:      "ok",
		Body:       "body",
		Detail:     map[string]any{"taskId": 3, "runId": 12},
	}
	require.NoError(t, sink.Deliver(context.Background(), event))

	record, err := store.Client().Notification.Query().Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, "evt-1", record.EventID)
	require.Equal(t, string(NameTaskRunSucceeded), record.Name)
	require.Equal(t, notification.SeverityInfo, record.Severity)
	require.Equal(t, "run", record.SubjectKind)
	require.Equal(t, "12", record.SubjectID)
	require.Equal(t, "ok", record.Title)
	require.Equal(t, "body", record.Body)
	require.Equal(t, map[string]any{"taskId": float64(3), "runId": float64(12)}, record.Detail)
	require.Nil(t, record.ReadAt)
	require.True(t, record.OccurredAt.Equal(occurred))
}

// TestStoreSinkIdempotentByEventID 验证同一 event_id 重复投递只落一条。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkIdempotentByEventID(t *testing.T) {
	store := openTestStore(t)
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true}, StoreSinkOptions{})
	event := Event{
		ID:         "same-id",
		Name:       NameTaskRunFailed,
		Severity:   SeverityError,
		OccurredAt: time.Now().UTC(),
		Title:      "failed",
	}
	require.NoError(t, sink.Deliver(context.Background(), event))
	require.NoError(t, sink.Deliver(context.Background(), event))
	count, err := store.Client().Notification.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestStoreSinkPruneByMaxRecords 验证条数上限淘汰；配置 0 不限。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkPruneByMaxRecords(t *testing.T) {
	store := openTestStore(t)
	// 先不限写入，再同步 prune，避免异步淘汰干扰断言
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 0}, StoreSinkOptions{})
	base := time.Now().UTC()
	for i := range 4 {
		require.NoError(t, sink.Deliver(context.Background(), Event{
			ID:         fmt.Sprintf("max-%d", i),
			Name:       NameTaskRunSucceeded,
			Severity:   SeverityInfo,
			OccurredAt: base.Add(time.Duration(i) * time.Minute),
			Title:      "t",
		}))
	}
	sink.UpdateConfig(config.NotifyConfig{Store: config.NotifyStoreConfig{Enabled: true, MaxRecords: 2, RetainDays: 0}})
	require.NoError(t, sink.prune(context.Background()))
	count, err := store.Client().Notification.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)

	// 0 = 不限
	sink.UpdateConfig(config.NotifyConfig{Store: config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 0}})
	for i := range 3 {
		require.NoError(t, sink.Deliver(context.Background(), Event{
			ID:         fmt.Sprintf("unlimited-%d", i),
			Name:       NameTaskRunSucceeded,
			Severity:   SeverityInfo,
			OccurredAt: base.Add(time.Duration(10+i) * time.Minute),
			Title:      "t",
		}))
	}
	require.NoError(t, sink.prune(context.Background()))
	count, err = store.Client().Notification.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

// TestStoreSinkPruneByRetainDays 验证天数淘汰；配置 0 不限。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkPruneByRetainDays(t *testing.T) {
	store := openTestStore(t)
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 7}, StoreSinkOptions{})
	now := time.Now().UTC()
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "old", Name: NameTaskRunSucceeded, Severity: SeverityInfo,
		OccurredAt: now.AddDate(0, 0, -10), Title: "old",
	}))
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "new", Name: NameTaskRunSucceeded, Severity: SeverityInfo,
		OccurredAt: now.AddDate(0, 0, -1), Title: "new",
	}))
	require.NoError(t, sink.prune(context.Background()))
	ids, err := store.Client().Notification.Query().IDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	record, err := store.Client().Notification.Get(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, "new", record.EventID)

	// 0 = 不限
	sink.UpdateConfig(config.NotifyConfig{Store: config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 0}})
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "ancient", Name: NameTaskRunSucceeded, Severity: SeverityInfo,
		OccurredAt: now.AddDate(-1, 0, 0), Title: "ancient",
	}))
	require.NoError(t, sink.prune(context.Background()))
	count, err := store.Client().Notification.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

// TestStoreSinkPruneBothDimensions 验证条数与天数同时生效。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkPruneBothDimensions(t *testing.T) {
	store := openTestStore(t)
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true, MaxRecords: 0, RetainDays: 0}, StoreSinkOptions{})
	now := time.Now().UTC()
	// 3 条新 + 1 条过期 + 1 条额外
	for i := range 3 {
		require.NoError(t, sink.Deliver(context.Background(), Event{
			ID: fmt.Sprintf("fresh-%d", i), Name: NameTaskRunSucceeded, Severity: SeverityInfo,
			OccurredAt: now.Add(time.Duration(i) * time.Minute), Title: "fresh",
		}))
	}
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "stale", Name: NameTaskRunSucceeded, Severity: SeverityInfo,
		OccurredAt: now.AddDate(0, 0, -10), Title: "stale",
	}))
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "extra", Name: NameTaskRunSucceeded, Severity: SeverityInfo,
		OccurredAt: now.Add(5 * time.Minute), Title: "extra",
	}))
	sink.UpdateConfig(config.NotifyConfig{Store: config.NotifyStoreConfig{Enabled: true, MaxRecords: 3, RetainDays: 5}})
	require.NoError(t, sink.prune(context.Background()))
	count, err := store.Client().Notification.Query().Count(context.Background())
	require.NoError(t, err)
	require.LessOrEqual(t, count, 3)
	// 过期项必须不在
	exists, err := store.Client().Notification.Query().Where(notification.EventIDEQ("stale")).Exist(context.Background())
	require.NoError(t, err)
	require.False(t, exists)
}

// TestStoreSinkMarkReadAndFilters 验证已读变更与筛选。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestStoreSinkMarkReadAndFilters(t *testing.T) {
	store := openTestStore(t)
	sink := NewStoreSink(store, config.NotifyStoreConfig{Enabled: true}, StoreSinkOptions{})
	now := time.Now().UTC()
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "a", Name: NameTaskRunSucceeded, Severity: SeverityInfo, OccurredAt: now, Title: "a",
	}))
	require.NoError(t, sink.Deliver(context.Background(), Event{
		ID: "b", Name: NameTaskRunFailed, Severity: SeverityError, OccurredAt: now.Add(time.Minute), Title: "b",
	}))

	unread, err := sink.UnreadCount(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, unread)

	list, err := sink.List(context.Background(), ListFilter{Page: 1, PageSize: 10, Severity: SeverityError})
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Equal(t, "b", list.Items[0].EventID)

	_, err = sink.MarkRead(context.Background(), list.Items[0].ID)
	require.NoError(t, err)
	readTrue := true
	readList, err := sink.List(context.Background(), ListFilter{Page: 1, PageSize: 10, Read: &readTrue})
	require.NoError(t, err)
	require.Equal(t, 1, readList.Total)

	affected, err := sink.MarkAllRead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, affected)
	unread, err = sink.UnreadCount(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, unread)

	cleared, err := sink.ClearRead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, cleared)
}
