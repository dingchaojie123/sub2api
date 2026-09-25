ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_price_2k DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS video_price_4k DECIMAL(20,8);

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_fallback_unit_price DECIMAL(20,8) NOT NULL DEFAULT 0;
