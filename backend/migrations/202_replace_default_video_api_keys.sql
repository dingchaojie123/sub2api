-- Replace the legacy aggregate-video signup key with TikTok--Seedance.
-- Existing customer credentials bound to 视频【聚合】 are revoked via the
-- same tombstone + soft-delete convention used by APIKeyRepository.DeleteWithAudit.
-- The regular default-key ensure flow securely creates or adopts the replacement
-- TikTok--Seedance key the next time the customer reads their defaults.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TEMP TABLE legacy_default_video_api_keys (
    api_key_id BIGINT PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO legacy_default_video_api_keys (api_key_id)
SELECT api_key.id
FROM api_keys AS api_key
JOIN users AS customer
  ON customer.id = api_key.user_id
JOIN groups AS bound_group
  ON bound_group.id = api_key.group_id
WHERE api_key.deleted_at IS NULL
  AND customer.deleted_at IS NULL
  AND customer.role = 'user'
  AND bound_group.name = '视频【聚合】'
ON CONFLICT (api_key_id) DO NOTHING;

-- Soft deletion does not invoke the FK's ON DELETE SET NULL behavior. Delete
-- only default slots that actually point at a legacy key so custom video
-- defaults remain untouched.
DELETE FROM user_default_api_keys AS defaults
USING legacy_default_video_api_keys AS legacy
WHERE defaults.purpose = 'video'
  AND defaults.api_key_id = legacy.api_key_id;

UPDATE api_keys AS api_key
SET key = '__deleted__legacy_video_group__' || api_key.id::TEXT,
    deleted_at = NOW(),
    updated_at = NOW()
FROM legacy_default_video_api_keys AS legacy
WHERE api_key.id = legacy.api_key_id
  AND api_key.deleted_at IS NULL;
