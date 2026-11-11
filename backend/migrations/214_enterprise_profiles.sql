-- 企业账号档案。
-- 账号类型不在 users 表加字段：存在本表记录即为企业账号，否则为个人账号。
-- verify_status:
--   pending     已提交，等待后台审核（注册后立即可用，仅徽标显示「待认证」）
--   approved    后台审核通过，显示「已认证」
--   rejected    后台驳回，可修改后重新提交
--   incomplete  存量团队创建者迁移而来，企业信息待补全
CREATE TABLE IF NOT EXISTS enterprise_profiles (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id),
    company_name   VARCHAR(200) NOT NULL DEFAULT '',
    credit_code    VARCHAR(18)  NOT NULL DEFAULT '',
    address        VARCHAR(500) NOT NULL DEFAULT '',
    contact_name   VARCHAR(100) NOT NULL DEFAULT '',
    contact_phone  VARCHAR(40)  NOT NULL DEFAULT '',
    verify_status  VARCHAR(20)  NOT NULL DEFAULT 'pending'
        CHECK (verify_status IN ('pending', 'approved', 'rejected', 'incomplete')),
    reject_reason  VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by    BIGINT REFERENCES users(id),
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_enterprise_profiles_user
    ON enterprise_profiles(user_id);

-- 统一社会信用代码全局唯一，防止同一家公司重复注册；存量迁移数据的空值不参与约束。
CREATE UNIQUE INDEX IF NOT EXISTS uq_enterprise_profiles_credit_code
    ON enterprise_profiles(credit_code) WHERE credit_code <> '';

CREATE INDEX IF NOT EXISTS idx_enterprise_profiles_status_created
    ON enterprise_profiles(verify_status, created_at DESC);

-- 存量数据：已经创建过团队的账号视为企业账号，信息待其登录后补全。
INSERT INTO enterprise_profiles (user_id, company_name, verify_status)
SELECT DISTINCT ON (o.owner_user_id) o.owner_user_id, LEFT(o.name, 200), 'incomplete'
FROM organizations o
JOIN users u ON u.id = o.owner_user_id AND u.deleted_at IS NULL
ORDER BY o.owner_user_id, o.id
ON CONFLICT (user_id) DO NOTHING;
