ALTER TABLE user_default_api_keys
    DROP CONSTRAINT IF EXISTS user_default_api_keys_purpose_check;

ALTER TABLE user_default_api_keys
    ADD CONSTRAINT user_default_api_keys_purpose_check
    CHECK (purpose IN ('text', 'image', 'video', 'audio'));
