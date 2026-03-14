# Change Map

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Total files affected: 12 modified, 8 new, 0 deleted

## Files to Modify

| File | Module | Change Description | Scope | Dependencies |
|------|--------|-------------------|-------|-------------|
| `internal/auth/handler.go` | Auth | Add `HandleSSOInitiate` and `HandleSSOCallback` methods to existing handler struct | medium | `sso_service.go` must exist |
| `internal/tenant/model.go` | Tenant | Add `TenantSSOConfig` struct definition | small | none |
| `internal/tenant/repository.go` | Tenant | Add `GetSSOConfigByDomain` and `UpsertSSOConfig` methods | medium | `model.go` must have TenantSSOConfig |
| `internal/tenant/repository_test.go` | Tenant | Add tests for new repository methods | small | `repository.go` changes |
| `internal/user/model.go` | User | Add `KeycloakID *string` field to User struct | small | none |
| `internal/user/repository.go` | User | Add `FindByEmail` and `UpdateKeycloakID` methods | small | `model.go` must have KeycloakID |
| `internal/user/service.go` | User | Add `FindOrCreateBySSO` and `LinkKeycloakID` methods | medium | `repository.go` changes |
| `internal/user/service_test.go` | User | Add tests for SSO-related service methods | small | `service.go` changes |
| `internal/config/config.go` | Config | Add `SSOConfig` struct and embed in main Config | small | none |
| `cmd/server/main.go` | Server | Initialize KeycloakClient, SSOService; register SSO routes | small | All auth module files |

## Files to Create

| File | Module | Purpose | Template/Pattern |
|------|--------|---------|-----------------|
| `internal/auth/sso_service.go` | Auth | SSO service: InitiateSSO, ProcessCallback logic | `internal/auth/service.go` (existing auth service pattern) |
| `internal/auth/keycloak_client.go` | Auth | Keycloak Admin REST API client: CreateIdP, GetIdP, DeleteIdP | `internal/auth/service.go` (HTTP client pattern) |
| `internal/auth/sso_service_test.go` | Auth | Unit tests for SSO service | `internal/auth/service_test.go` |
| `internal/auth/keycloak_client_test.go` | Auth | Unit tests for Keycloak client with HTTP mocking | `internal/auth/service_test.go` |
| `migrations/00X_add_tenant_sso_config.sql` | Migrations | CREATE TABLE tenant_sso_config with domain unique index | `migrations/001_initial.sql` |
| `migrations/00Y_add_user_keycloak_id.sql` | Migrations | ALTER TABLE users ADD COLUMN keycloak_id with unique index | `migrations/001_initial.sql` |

## Files to Delete

No files to delete.

## Interfaces Changed

| Interface | Location | Current Signature | New Signature | Consumers |
|-----------|----------|------------------|---------------|-----------|
| `TenantRepository` | `internal/tenant/repository.go` | `Create`, `GetByID`, `Update`, `Delete`, `List` | Adds: `GetSSOConfigByDomain(ctx, domain) (*TenantSSOConfig, error)`, `UpsertSSOConfig(ctx, config *TenantSSOConfig) error` | `auth.SSOService` |
| `UserRepository` | `internal/user/repository.go` | `Create`, `GetByID`, `Update`, `Delete`, `List` | Adds: `FindByEmail(ctx, email) (*User, error)`, `UpdateKeycloakID(ctx, userID, keycloakID string) error` | `user.Service` |
| `UserService` | `internal/user/service.go` | `Create`, `GetByID`, `Update`, `Delete` | Adds: `FindOrCreateBySSO(ctx, email string, attrs SSOAttributes) (*User, bool, error)`, `LinkKeycloakID(ctx, userID, keycloakID string) error` | `auth.SSOService` |

## Data / Schema Changes

| What | Type | Description | Migration Needed? |
|------|------|-------------|-------------------|
| `tenant_sso_config` table | add | New table: id (UUID PK), tenant_id (FK), domain (UNIQUE), entity_id, metadata_url, certificate (TEXT), enabled (BOOL), created_at, updated_at | yes — `00X_add_tenant_sso_config.sql` |
| `users.keycloak_id` column | add | New nullable VARCHAR(255) column with UNIQUE index | yes — `00Y_add_user_keycloak_id.sql` |

## Configuration Changes

| What | Location | Description |
|------|----------|-------------|
| `SSO_KEYCLOAK_URL` | `internal/config/config.go` | Keycloak base URL (e.g., `https://keycloak.example.com`) |
| `SSO_KEYCLOAK_REALM` | `internal/config/config.go` | Keycloak realm name (default: `master`) |
| `SSO_KEYCLOAK_ADMIN_USER` | `internal/config/config.go` | Keycloak admin username for API access |
| `SSO_KEYCLOAK_ADMIN_PASSWORD` | `internal/config/config.go` | Keycloak admin password for API access |

## Change Dependency Order

```
migrations (00X, 00Y) → config.go → tenant/model.go → tenant/repository.go
                                   → user/model.go → user/repository.go → user/service.go
                                                                         → keycloak_client.go
                                   → sso_service.go → handler.go → main.go
```
