-- Repair the legacy TikTok--Seedance account mapping that collapsed both
-- Seedance 2.5 One variants into an upstream-internal model ID. The provider
-- accepts the two public model IDs directly.

UPDATE accounts AS account
SET credentials = jsonb_set(
    account.credentials,
    '{model_mapping,seedance2.5-one-720p}',
    '"seedance2.5-one-720p"'::jsonb,
    false
)
WHERE account.deleted_at IS NULL
  AND account.platform IN ('seedance', 'ctmo-seedance')
  AND account.credentials->'model_mapping'->>'seedance2.5-one-720p' = 'SD2.5-30-10-10'
  AND EXISTS (
      SELECT 1
      FROM account_groups AS account_group
      JOIN groups AS bound_group ON bound_group.id = account_group.group_id
      WHERE account_group.account_id = account.id
        AND bound_group.deleted_at IS NULL
        AND bound_group.name = 'TikTok--Seedance'
  );

UPDATE accounts AS account
SET credentials = jsonb_set(
    account.credentials,
    '{model_mapping,seedance2.5-one-1080p}',
    '"seedance2.5-one-1080p"'::jsonb,
    false
)
WHERE account.deleted_at IS NULL
  AND account.platform IN ('seedance', 'ctmo-seedance')
  AND account.credentials->'model_mapping'->>'seedance2.5-one-1080p' = 'SD2.5-30-10-10'
  AND EXISTS (
      SELECT 1
      FROM account_groups AS account_group
      JOIN groups AS bound_group ON bound_group.id = account_group.group_id
      WHERE account_group.account_id = account.id
        AND bound_group.deleted_at IS NULL
        AND bound_group.name = 'TikTok--Seedance'
  );
