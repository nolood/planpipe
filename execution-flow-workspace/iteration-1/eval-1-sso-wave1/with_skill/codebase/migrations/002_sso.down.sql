ALTER TABLE users DROP COLUMN IF EXISTS keycloak_id;

DROP INDEX IF EXISTS idx_tenant_sso_config_tenant_id;
DROP TABLE IF EXISTS tenant_sso_config;
