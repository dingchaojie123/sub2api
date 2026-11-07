ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS provider_billing_seconds BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    DROP CONSTRAINT IF EXISTS pp_video_tasks_provider_billing_seconds_check;
ALTER TABLE pp_video_tasks
    ADD CONSTRAINT pp_video_tasks_provider_billing_seconds_check
    CHECK (provider_billing_seconds >= 0);

DO $migration$
DECLARE
    allowed_platforms TEXT;
BEGIN
    SELECT string_agg(quote_literal(platform), ', ' ORDER BY platform)
      INTO allowed_platforms
      FROM (
          SELECT unnest(ARRAY[
              '88api-video', 'anthropic', 'openai', 'gemini', 'antigravity',
              'grok', 'jimeng', 'doubao', 'qwen', 'kimi', 'deepseek',
              'midjourney', 'kling', 'happyhourse', 'seedance',
              'ctmo-seedance', 'guangying', 'huli-ai', 'opone',
              'xingapi-seedance', 'mai-token-seedance', 'bytedance', 'wan3',
              'minimax-h3', 'minimax-h3-compshare', 'minimax-speech',
              'qwen-tts', 'pixverse-v6', 'grok-imagine-video', 'kuaishou'
          ]) AS platform
          UNION
          SELECT DISTINCT platform
            FROM user_platform_quotas
      ) AS preserved_platforms;

    ALTER TABLE user_platform_quotas
        DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

    EXECUTE format(
        'ALTER TABLE user_platform_quotas ADD CONSTRAINT user_platform_quotas_platform_check CHECK (platform IN (%s))',
        allowed_platforms
    );
END
$migration$;
