-- 视频任务记录页：保存用户提交的提示词，并为「按用户 + 创建时间倒序」的列表查询建立索引。
-- prompt 只用于展示，不参与计费或幂等判断；历史任务保持为空串。
ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS prompt TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_pp_video_tasks_user_created
    ON pp_video_tasks(user_id, created_at DESC, id DESC);
