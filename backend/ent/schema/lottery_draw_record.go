package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// LotteryDrawRecord records a completed draw and its assigned redeem code.
type LotteryDrawRecord struct {
	ent.Schema
}

func (LotteryDrawRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "lottery_draw_records"}}
}

func (LotteryDrawRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("prize_name").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Float("prize_value").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int64("prize_pool_code_id").Unique(),
		field.Int64("prize_redeem_code_id").Unique(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
