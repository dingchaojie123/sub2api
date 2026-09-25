ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'jimeng', 'doubao', 'qwen', 'kimi', 'deepseek', 'midjourney', 'kling', 'happyhourse', 'seedance', 'bytedance', 'wan3', 'minimax-h3', 'minimax-h3-compshare', 'minimax-speech', 'qwen-tts', 'pixverse-v6', 'grok-imagine-video', 'kuaishou'));
