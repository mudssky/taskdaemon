package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Admin 定义单管理员账号。
type Admin struct {
	ent.Schema
}

// Fields 返回管理员账号字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (Admin) Fields() []ent.Field {
	return []ent.Field{
		field.String("singleton_key").NotEmpty().Unique().Immutable(),
		field.String("username").NotEmpty().Unique(),
		field.String("password_hash").Sensitive(),
		field.Bool("active").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges 返回管理员和登录态的关系。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Edge: Ent 关系定义。
func (Admin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type),
	}
}
