package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AudioRecord 定义一次外部音频播放请求的历史记录。
type AudioRecord struct {
	ent.Schema
}

// Fields 返回音频播放请求历史字段。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []ent.Field: Ent 字段定义。
func (AudioRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("source_kind").Values("url", "upload").Default("url"),
		field.String("source").Optional(),
		field.String("source_url").Optional().Sensitive(),
		field.String("original_filename").Optional(),
		field.String("stored_path").NotEmpty(),
		field.String("mime_type").Optional(),
		field.Int64("size_bytes").NonNegative(),
		field.String("sha256").NotEmpty(),
		field.Enum("status").Values("received", "queued", "playing", "played", "failed", "skipped", "queued_skipped").Default("received"),
		field.String("error_summary").Optional(),
		field.Time("played_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
