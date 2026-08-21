CREATE TABLE IF NOT EXISTS pp_video_tasks (
    id BIGSERIAL PRIMARY KEY,
    local_task_id VARCHAR(64) NOT NULL UNIQUE,
    task_id VARCHAR(128),
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    group_id BIGINT,
    account_id BIGINT NOT NULL,
    platform VARCHAR(32) NOT NULL,
    operation VARCHAR(48) NOT NULL,
    model VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'submitting',
    billing_status VARCHAR(32) NOT NULL DEFAULT 'held',
    request_hash VARCHAR(128) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(255),
    requested_video_duration_seconds INT NOT NULL DEFAULT 0,
    requested_video_duration_milliseconds BIGINT NOT NULL DEFAULT 0,
    input_video_duration_milliseconds BIGINT NOT NULL DEFAULT 0,
    generated_video_duration_milliseconds BIGINT NOT NULL DEFAULT 0,
    video_count INT NOT NULL DEFAULT 1,
    video_resolution VARCHAR(32) NOT NULL DEFAULT '',
    output_width INT NOT NULL DEFAULT 0,
    output_height INT NOT NULL DEFAULT 0,
    frame_rate NUMERIC(12,4) NOT NULL DEFAULT 0,
    has_audio BOOLEAN NOT NULL DEFAULT FALSE,
    kling_mode VARCHAR(32) NOT NULL DEFAULT '',
    billing_formula VARCHAR(32) NOT NULL DEFAULT '',
    billing_units NUMERIC(30,6) NOT NULL DEFAULT 0,
    billing_unit_price NUMERIC(30,12) NOT NULL DEFAULT 0,
    response_status INT NOT NULL DEFAULT 200,
    response_content_type VARCHAR(128) NOT NULL DEFAULT 'application/json',
    response_body TEXT NOT NULL DEFAULT '',
    hold_id VARCHAR(160) NOT NULL,
    capture_id VARCHAR(160) NOT NULL,
    release_id VARCHAR(160) NOT NULL,
    estimated_total_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    hold_amount DECIMAL(20,10) NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20,10),
    currency VARCHAR(16) NOT NULL DEFAULT 'USD',
    last_error_code VARCHAR(128),
    last_error_message TEXT,
    poll_lease_token VARCHAR(128),
    poll_lease_owner VARCHAR(128),
    poll_lease_until TIMESTAMPTZ,
    poll_attempts INT NOT NULL DEFAULT 0,
    last_polled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ
);

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS poll_lease_token VARCHAR(128);

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS requested_video_duration_milliseconds BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS input_video_duration_milliseconds BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS output_width INT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS output_height INT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS frame_rate NUMERIC(12,4) NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS has_audio BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS kling_mode VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_formula VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_units NUMERIC(30,6) NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_unit_price NUMERIC(30,12) NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS uq_pp_video_tasks_task_id
    ON pp_video_tasks(task_id)
    WHERE task_id IS NOT NULL AND task_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_pp_video_tasks_owner_idempotency_key
    ON pp_video_tasks(user_id, api_key_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

CREATE INDEX IF NOT EXISTS idx_pp_video_tasks_owner_task
    ON pp_video_tasks(user_id, api_key_id, task_id);

CREATE INDEX IF NOT EXISTS idx_pp_video_tasks_polling
    ON pp_video_tasks(billing_status, status, poll_lease_until, updated_at)
    WHERE billing_status IN ('held', 'none', 'settling', 'settling_none') AND settled_at IS NULL;
