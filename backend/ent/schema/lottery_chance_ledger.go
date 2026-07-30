package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LotteryChanceLedger records immutable lottery chance grants and spends.
type LotteryChanceLedger struct {
	ent.Schema
}

func (LotteryChanceLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "lottery_chance_ledger"}}
}

func (LotteryChanceLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int("delta"),
		field.String("reason").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("source_redeem_code_id").Optional().Nillable(),
		field.Int64("draw_record_id").Optional().Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryChanceLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
	}
}
