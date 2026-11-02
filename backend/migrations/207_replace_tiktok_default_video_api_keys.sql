-- Move the system-provisioned video default from TikTok--Seedance to
-- TT--Seedance. Custom video defaults remain untouched, and the old API key
-- remains available as a regular credential. The default-key ensure flow
-- adopts or creates a TT--Seedance key on the customer's next defaults read.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

DELETE FROM user_default_api_keys AS defaults
USING api_keys AS api_key, users AS customer, groups AS bound_group
WHERE defaults.purpose = 'video'
  AND defaults.api_key_id = api_key.id
  AND customer.id = api_key.user_id
  AND bound_group.id = api_key.group_id
  AND api_key.deleted_at IS NULL
  AND customer.deleted_at IS NULL
  AND customer.role = 'user'
  AND bound_group.name = 'TikTok--Seedance';
