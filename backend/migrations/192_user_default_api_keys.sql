CREATE TABLE IF NOT EXISTS user_default_api_keys (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose VARCHAR(16) NOT NULL CHECK (purpose IN ('text', 'image', 'video')),
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, purpose),
    UNIQUE (api_key_id)
);
