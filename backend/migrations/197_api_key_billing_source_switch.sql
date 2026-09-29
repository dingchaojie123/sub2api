-- Allow an API key to retain its historical organization binding after the
-- owner explicitly switches billing back to their personal balance.
COMMENT ON COLUMN organization_api_keys.status IS
    'Billing binding state: active, inactive, or detached (explicit personal billing)';

CREATE OR REPLACE FUNCTION set_usage_log_organization_id() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NULL THEN
        SELECT organization_id INTO NEW.organization_id
        FROM organization_api_keys
        WHERE api_key_id = NEW.api_key_id AND status <> 'detached';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
