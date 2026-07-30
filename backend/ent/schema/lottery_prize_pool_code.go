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

// LotteryPrizePoolCode binds an unused balance redeem code to a lottery prize.
type LotteryPrizePoolCode struct {
	ent.Schema
}

func (LotteryPrizePoolCode) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "lottery_prize_pool_codes"}}
}

func (LotteryPrizePoolCode) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("redeem_code_id").Unique(),
		field.Float("prize_value").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("status").Default("available").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("assigned_to_user_id").Optional().Nillable(),
		field.Int64("assigned_draw_record_id").Optional().Nillable(),
		field.Time("assigned_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (LotteryPrizePoolCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("prize_value", "status", "id"),
	}
}
