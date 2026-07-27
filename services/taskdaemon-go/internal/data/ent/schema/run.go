package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Run 定义一次任务执行历史。
type Run struct {
	ent.Schema
}

// Fields 返回执行历史字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (Run) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("trigger").Values("cron", "manual").Default("cron"),
		field.Enum("status").Values("queued", "running", "success", "failed", "timeout", "cancelled", "skipped").Default("queued"),
		field.Int("exit_code").Optional().Nillable(),
		field.Time("started_at").Default(time.Now),
		field.Time("finished_at").Optional().Nillable(),
		field.Int64("duration_ms").Optional(),
		field.String("error_summary").Optional(),
		field.String("stdout").Optional(),
		field.String("stderr").Optional(),
		// T6: run 完整日志归档元数据（append-only）
		field.Enum("log_archive_status").
			Values("absent", "archived", "pruned").
			Default("absent"),
		field.Int64("log_size_bytes").Optional().Nillable(),
		field.Bool("log_write_failed").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges 返回执行历史与任务定义的关系。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Edge: Ent 关系定义。
func (Run) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("task", Task.Type).Ref("runs").Unique().Required(),
	}
}
