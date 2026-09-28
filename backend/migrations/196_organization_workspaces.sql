-- Enterprise team workspaces. Monetary values are stored in USD and the
-- database remains the source of truth; Redis is only an enforcement cache.
CREATE TABLE IF NOT EXISTS organizations (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    seat_limit INTEGER NOT NULL DEFAULT 5 CHECK (seat_limit > 0),
    balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    display_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    frozen_display_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);

CREATE TABLE IF NOT EXISTS organization_members (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    role VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    monthly_limit NUMERIC(20,8) NOT NULL DEFAULT 0,
    monthly_used NUMERIC(20,8) NOT NULL DEFAULT 0,
    monthly_frozen NUMERIC(20,8) NOT NULL DEFAULT 0,
    usage_period_start TIMESTAMPTZ NOT NULL DEFAULT date_trunc('month', NOW()),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_organization_members_user ON organization_members(user_id, status);
CREATE INDEX IF NOT EXISTS idx_organization_members_org ON organization_members(organization_id, status);

CREATE TABLE IF NOT EXISTS organization_invitations (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255),
    token_hash CHAR(64) NOT NULL UNIQUE,
    role VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    max_uses INTEGER NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    used_count INTEGER NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_org ON organization_invitations(organization_id, created_at DESC);

CREATE TABLE IF NOT EXISTS organization_api_keys (
    api_key_id BIGINT PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    member_user_id BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_api_keys_org ON organization_api_keys(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_organization_api_keys_member ON organization_api_keys(member_user_id, status);

CREATE TABLE IF NOT EXISTS organization_quota_ledger (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    actor_user_id BIGINT REFERENCES users(id),
    member_user_id BIGINT REFERENCES users(id),
    api_key_id BIGINT REFERENCES api_keys(id),
    request_id VARCHAR(128),
    entry_type VARCHAR(30) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_quota_ledger_request
    ON organization_quota_ledger(organization_id, request_id, entry_type)
    WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_organization_quota_ledger_org_created
    ON organization_quota_ledger(organization_id, created_at DESC);

CREATE TABLE IF NOT EXISTS organization_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    action VARCHAR(80) NOT NULL,
    target_type VARCHAR(40),
    target_id VARCHAR(80),
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_audit_logs_org_created
    ON organization_audit_logs(organization_id, created_at DESC);

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS organization_id BIGINT REFERENCES organizations(id);
ALTER TABLE usage_billing_dedup ADD COLUMN IF NOT EXISTS organization_id BIGINT REFERENCES organizations(id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_organization_created
    ON usage_logs(organization_id, created_at DESC) WHERE organization_id IS NOT NULL;

CREATE OR REPLACE FUNCTION set_usage_log_organization_id() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NULL THEN
        SELECT organization_id INTO NEW.organization_id
        FROM organization_api_keys
        WHERE api_key_id = NEW.api_key_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_usage_logs_organization_id ON usage_logs;
CREATE TRIGGER trg_usage_logs_organization_id
    BEFORE INSERT ON usage_logs
    FOR EACH ROW EXECUTE FUNCTION set_usage_log_organization_id();
