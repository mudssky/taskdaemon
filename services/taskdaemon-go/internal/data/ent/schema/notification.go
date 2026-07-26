package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Notification 定义站内通知持久化记录（T2a / C-2）。
type Notification struct {
	ent.Schema
}

// Fields 返回站内通知字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_id").Unique().NotEmpty(),
		field.String("name").NotEmpty(),
		field.Enum("severity").Values("info", "warning", "error", "critical"),
		field.String("subject_kind").Optional(),
		field.String("subject_id").Optional(),
		field.String("title").NotEmpty(),
		field.Text("body").Optional(),
		field.JSON("detail", map[string]any{}).Optional(),
		field.Time("read_at").Optional().Nillable(),
		field.Time("occurred_at"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Indexes 返回站内通知查询索引。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Index: 索引定义。
func (Notification) Indexes() []ent.Index {
	return []ent.Index{
		// 未读列表：read_at 为空 + occurred_at 倒序
		index.Fields("read_at", "occurred_at"),
		// 按级别筛选
		index.Fields("severity", "occurred_at"),
	}
}
