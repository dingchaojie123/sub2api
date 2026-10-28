-- One-time cleanup for customer API keys that predate the fixed signup group set.
-- Keep administrator keys untouched and identify retained keys by their bound
-- group, not by the mutable API-key display name.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TEMP TABLE retained_customer_api_key_groups (
    name TEXT PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO retained_customer_api_key_groups (name) VALUES
    ('OC--ChatGPT【文本模型，官key】'),
    ('Claude Code 【官方】'),
    ('视频【聚合】'),
    ('OCY--MiniMax-H3【视频】'),
    ('GPT图片【聚合】'),
    ('Gemini图片【聚合】'),
    ('OC--Qwen-TTS【音频】');

-- Soft deletion does not fire the API-key foreign key's ON DELETE action.
-- Remove affected default slots explicitly so the normal ensure flow can bind
-- them to the retained text/image/video/audio groups on the next request.
DELETE FROM user_default_api_keys AS defaults
USING api_keys AS api_key, users AS customer
WHERE defaults.api_key_id = api_key.id
  AND api_key.user_id = customer.id
  AND api_key.deleted_at IS NULL
  AND customer.deleted_at IS NULL
  AND customer.role = 'user'
  AND NOT EXISTS (
      SELECT 1
      FROM groups AS bound_group
      JOIN retained_customer_api_key_groups AS retained
        ON retained.name = bound_group.name
      WHERE bound_group.id = api_key.group_id
        AND bound_group.deleted_at IS NULL
  );

-- Match APIKeyRepository.DeleteWithAudit semantics: release the credential by
-- replacing it with a unique tombstone, then soft-delete the row. Migration 184's
-- trigger observes OLD.key and enqueues cross-instance auth-cache invalidation.
UPDATE api_keys AS api_key
SET key = '__deleted__legacy_group_prune__' || api_key.id::TEXT,
    deleted_at = NOW(),
    updated_at = NOW()
FROM users AS customer
WHERE api_key.user_id = customer.id
  AND api_key.deleted_at IS NULL
  AND customer.deleted_at IS NULL
  AND customer.role = 'user'
  AND NOT EXISTS (
      SELECT 1
      FROM groups AS bound_group
      JOIN retained_customer_api_key_groups AS retained
        ON retained.name = bound_group.name
      WHERE bound_group.id = api_key.group_id
        AND bound_group.deleted_at IS NULL
  );
