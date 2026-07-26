package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Task 定义可被调度和手动触发的任务配置。
type Task struct {
	ent.Schema
}

// Fields 返回任务定义字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("description").Optional(),
		field.Bool("enabled").Default(true),
		field.String("cron_expression").Optional(),
		field.String("timezone").Default("Local"),
		field.Enum("runner_type").Values("shell", "bash", "pwsh", "python", "node", "typescript", "agent").Default("shell"),
		field.JSON("runner_config", map[string]any{}).Optional(),
		field.Int("timeout_seconds").Default(3600).Positive(),
		field.Enum("overlap_policy").Values("skip").Default("skip"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges 返回任务与执行记录的关系。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Edge: Ent 关系定义。
func (Task) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("runs", Run.Type),
	}
}
