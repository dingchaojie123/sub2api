-- Persist request-time billing ownership so later settlement is independent of
-- API-key reassignment, member removal, and organization switching.
ALTER TABLE usage_billing_dedup
    ADD COLUMN IF NOT EXISTS member_user_id BIGINT REFERENCES users(id);

ALTER TABLE usage_billing_dedup_archive
    ADD COLUMN IF NOT EXISTS organization_id BIGINT REFERENCES organizations(id);
ALTER TABLE usage_billing_dedup_archive
    ADD COLUMN IF NOT EXISTS member_user_id BIGINT REFERENCES users(id);

ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS billing_source VARCHAR(20) NOT NULL DEFAULT 'personal';
ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS billing_organization_id BIGINT REFERENCES organizations(id);
ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS billing_member_user_id BIGINT REFERENCES users(id);

ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_source VARCHAR(20) NOT NULL DEFAULT 'personal';
ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_organization_id BIGINT REFERENCES organizations(id);
ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS billing_member_user_id BIGINT REFERENCES users(id);

ALTER TABLE jimeng_video_tasks
    ADD COLUMN IF NOT EXISTS billing_source VARCHAR(20) NOT NULL DEFAULT 'personal';
ALTER TABLE jimeng_video_tasks
    ADD COLUMN IF NOT EXISTS billing_organization_id BIGINT REFERENCES organizations(id);
ALTER TABLE jimeng_video_tasks
    ADD COLUMN IF NOT EXISTS billing_member_user_id BIGINT REFERENCES users(id);

UPDATE batch_image_jobs task
SET billing_source = 'organization',
    billing_organization_id = binding.organization_id,
    billing_member_user_id = binding.member_user_id
FROM organization_api_keys binding
WHERE task.api_key_id = binding.api_key_id
  AND task.settled_at IS NULL
  AND task.billing_organization_id IS NULL;

UPDATE pp_video_tasks task
SET billing_source = 'organization',
    billing_organization_id = binding.organization_id,
    billing_member_user_id = binding.member_user_id
FROM organization_api_keys binding
WHERE task.api_key_id = binding.api_key_id
  AND task.settled_at IS NULL
  AND task.billing_organization_id IS NULL;

UPDATE jimeng_video_tasks task
SET billing_source = 'organization',
    billing_organization_id = binding.organization_id,
    billing_member_user_id = binding.member_user_id
FROM organization_api_keys binding
WHERE task.api_key_id = binding.api_key_id
  AND task.settled_at IS NULL
  AND task.billing_organization_id IS NULL;

CREATE OR REPLACE FUNCTION set_usage_log_organization_id() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NULL AND NEW.request_id IS NOT NULL THEN
        SELECT organization_id INTO NEW.organization_id
        FROM usage_billing_dedup
        WHERE request_id = NEW.request_id AND api_key_id = NEW.api_key_id;
    END IF;
    IF NEW.organization_id IS NULL THEN
        SELECT organization_id INTO NEW.organization_id
        FROM organization_api_keys
        WHERE api_key_id = NEW.api_key_id AND status = 'active';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
