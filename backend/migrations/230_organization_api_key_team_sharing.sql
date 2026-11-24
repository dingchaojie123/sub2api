-- 团队 Key 共享：创建者（超级管理员）可把自己的团队 Key 标记为「共享给团队成员」。
-- 标记后，之后加入团队的成员会自动获得一份同配置的副本 Key（副本归成员本人，用量与月度额度按成员单独统计）。
--
-- shared_with_team   仅对「源 Key」有意义：true 表示该 Key 已共享给团队成员。
-- source_api_key_id  仅对「副本 Key」有意义：指向它复制自哪把源 Key；源 Key 本身为 NULL。
ALTER TABLE organization_api_keys
    ADD COLUMN IF NOT EXISTS shared_with_team BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE organization_api_keys
    ADD COLUMN IF NOT EXISTS source_api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL;

-- 同一把源 Key 对同一成员只会有一份副本（保证幂等，重复加入 / 并发不会重复发 Key）。
CREATE UNIQUE INDEX IF NOT EXISTS uq_organization_api_keys_source_member
    ON organization_api_keys(source_api_key_id, member_user_id)
    WHERE source_api_key_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_organization_api_keys_shared
    ON organization_api_keys(organization_id)
    WHERE shared_with_team;
