package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Session 定义管理员登录态。
type Session struct {
	ent.Schema
}

// Fields 返回登录态字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").NotEmpty().Unique().Sensitive(),
		field.String("csrf_token_hash").Optional().Sensitive(),
		field.Time("expires_at"),
		field.String("user_agent").Optional(),
		field.String("ip").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges 返回登录态和管理员的关系。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Edge: Ent 关系定义。
func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("admin", Admin.Type).Ref("sessions").Unique().Required(),
	}
}
