CREATE TABLE IF NOT EXISTS lottery_chance_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    delta INTEGER NOT NULL,
    reason TEXT NOT NULL,
    source_redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE SET NULL,
    draw_record_id BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_lottery_chance_ledger_source_redeem
    ON lottery_chance_ledger(source_redeem_code_id)
    WHERE source_redeem_code_id IS NOT NULL AND delta > 0;

CREATE INDEX IF NOT EXISTS idx_lottery_chance_ledger_user_created
    ON lottery_chance_ledger(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS lottery_prize_pool_codes (
    id BIGSERIAL PRIMARY KEY,
    redeem_code_id BIGINT NOT NULL UNIQUE REFERENCES redeem_codes(id) ON DELETE CASCADE,
    prize_value NUMERIC(20,8) NOT NULL,
    status TEXT NOT NULL DEFAULT 'available',
    assigned_to_user_id BIGINT NULL,
    assigned_draw_record_id BIGINT NULL,
    assigned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lottery_pool_value_status
    ON lottery_prize_pool_codes(prize_value, status, id);

CREATE TABLE IF NOT EXISTS lottery_draw_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    prize_name TEXT NOT NULL,
    prize_value NUMERIC(20,8) NOT NULL,
    prize_pool_code_id BIGINT NOT NULL UNIQUE REFERENCES lottery_prize_pool_codes(id),
    prize_redeem_code_id BIGINT NOT NULL UNIQUE REFERENCES redeem_codes(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_lottery_chance_ledger_draw_record'
          AND conrelid = 'lottery_chance_ledger'::regclass
    ) THEN
        ALTER TABLE lottery_chance_ledger
            ADD CONSTRAINT fk_lottery_chance_ledger_draw_record
            FOREIGN KEY (draw_record_id) REFERENCES lottery_draw_records(id) ON DELETE SET NULL;
    END IF;
END $$;
