CREATE TABLE tenant_sso_config (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    idp_entity_id TEXT NOT NULL,
    idp_sso_url TEXT NOT NULL,
    idp_certificate TEXT NOT NULL,
    sp_entity_id TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenant_sso_config_tenant_id ON tenant_sso_config(tenant_id);

ALTER TABLE users ADD COLUMN keycloak_id TEXT UNIQUE;
