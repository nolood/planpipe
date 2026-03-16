# Change Map

> Task: Add SAML 2.0 SSO support to a multi-tenant Go backend via Keycloak brokering
> Total files affected: 7 modified, 5 new, 0 deleted

## Files to Modify

| File | Module | Change Description | Scope | Dependencies |
|------|--------|-------------------|-------|-------------|
| `internal/auth/keycloak.go` | Auth | Add `ExchangeCode()` method for authorization code grant exchange via gocloak, `BuildAuthURL()` to construct Keycloak auth URL with kc_idp_hint, `GetAdminToken()` for admin API access, `CreateSAMLIdP()` and `GetSAMLIdP()` for IdP management | medium | None — extends existing struct |
| `internal/user/service.go` | User | Modify `GetOrCreateByEmail()` to accept `keycloakID string` parameter. On create: set KeycloakID. On find-existing with empty KeycloakID: update via `repo.UpdateKeycloakID()` | small | `internal/user/repository.go` (needs UpdateKeycloakID) |
| `internal/user/repository.go` | User | Add `UpdateKeycloakID(ctx, userID, keycloakID string) error` method — single UPDATE query | small | None |
| `internal/tenant/service.go` | Tenant | Add `ssoConfigRepo *SSOConfigRepository` field to Service struct, update `NewService()` constructor to accept it, add `GetSSOConfig()`, `GetSSOConfigByEmail()`, `SaveSSOConfig()` methods | small | `internal/tenant/sso_config_repository.go` |
| `internal/auth/service.go` | Auth | Update `GetOrCreateByEmail()` call at line 61 to pass empty string as keycloakID parameter: `s.userSvc.GetOrCreateByEmail(ctx, email, tenantID, "")` | small | `internal/user/service.go` signature change |
| `internal/config/config.go` | Config | Add `SSO SSOConfig` struct field with `CallbackBaseURL` and `FrontendCallbackURL` fields, loaded via `env:",prefix=SSO_"` | small | None |
| `cmd/server/main.go` | Server | Create `SSOConfigRepository`, pass to `tenant.NewService()`, create `SSOService`, create `SSOHandler`, register three SSO routes in public group | small | All new SSO types must exist |

## Files to Create

| File | Module | Purpose | Template/Pattern |
|------|--------|---------|-----------------|
| `migrations/002_sso_config.sql` | Migrations | Creates `tenant_sso_config` table with columns: id (UUID PK), tenant_id (UUID FK UNIQUE), sso_enabled (BOOL), idp_alias (TEXT UNIQUE), idp_entity_id (TEXT), idp_sso_url (TEXT), idp_certificate (TEXT), idp_metadata_url (TEXT), sp_entity_id (TEXT), created_at, updated_at. Indexes on tenant_id and idp_alias | Based on `migrations/001_initial.sql` conventions |
| `internal/tenant/sso_config.go` | Tenant | `SSOConfig` struct with fields matching the `tenant_sso_config` table columns | Based on `internal/tenant/models.go` struct style |
| `internal/tenant/sso_config_repository.go` | Tenant | `SSOConfigRepository` with `GetByTenantID()`, `GetByEmailDomain()`, `Save()`, `Delete()` methods using pgxpool queries | Based on `internal/tenant/repository.go` query patterns |
| `internal/auth/sso_service.go` | Auth | `SSOService` struct with `CheckSSO(ctx, email)`, `InitiateSSO(ctx, email)`, `HandleCallback(ctx, code, state)` methods. Depends on KeycloakClient, tenant.Service, user.Service | Based on `internal/auth/service.go` service pattern |
| `internal/auth/sso_handler.go` | Auth | `SSOHandler` struct with `CheckSSO`, `InitiateSSO`, `HandleCallback` HTTP handler methods. Uses httputil for responses. | Based on `internal/auth/handler.go` handler pattern |

## Files to Delete

No files to delete.

## Interfaces Changed

| Interface | Location | Current Signature | New Signature | Consumers |
|-----------|----------|------------------|---------------|-----------|
| `user.Service.GetOrCreateByEmail` | `internal/user/service.go:26` | `GetOrCreateByEmail(ctx context.Context, email, tenantID string) (*User, error)` | `GetOrCreateByEmail(ctx context.Context, email, tenantID, keycloakID string) (*User, error)` | `internal/auth/service.go:61` (existing caller — must be updated), `internal/auth/sso_service.go` (new caller) |
| `tenant.NewService` | `internal/tenant/service.go:12` | `NewService(repo *Repository) *Service` | `NewService(repo *Repository, ssoRepo *SSOConfigRepository) *Service` | `cmd/server/main.go:41` (must be updated) |
| `KeycloakClient` | `internal/auth/keycloak.go` | Has: `Authenticate`, `ValidateToken`, `Logout`, `RefreshToken` | Adds: `ExchangeCode`, `BuildAuthURL`, `GetAdminToken`, `CreateSAMLIdP`, `GetSAMLIdP` | `internal/auth/service.go` (existing, unchanged), `internal/auth/sso_service.go` (new), `cmd/server/main.go` (unchanged) |
| `config.Config` | `internal/config/config.go:11` | `Port`, `DatabaseURL`, `Keycloak` fields | Adds `SSO SSOConfig` field | `cmd/server/main.go:24` (config used for wiring) |

## Data / Schema Changes

| What | Type | Description | Migration Needed? |
|------|------|-------------|-------------------|
| `tenant_sso_config` table | add | New table with FK to tenants(id). Stores per-tenant SAML SSO configuration: IdP alias, entity ID, SSO URL, certificate, metadata URL, SP entity ID, enabled flag | yes — `002_sso_config.sql` |
| `users.keycloak_id` column | modify (data) | Column already exists but is typically NULL for password users. SSO flow will populate it. No schema change needed. | no |

## Configuration Changes

| What | Location | Description |
|------|----------|-------------|
| `SSO_CALLBACK_BASE_URL` | env var → `internal/config/config.go` | Base URL for SSO callback endpoint (e.g., `http://localhost:8080`). Used to construct the redirect_uri for Keycloak authorization code flow. |
| `SSO_FRONTEND_CALLBACK_URL` | env var → `internal/config/config.go` | Frontend URL to redirect to after SSO callback completes (e.g., `http://localhost:3000/auth/sso/complete`). Tokens are passed as URL fragment parameters. |
| Keycloak `platform-app` client | Keycloak admin console | Enable "Standard Flow" (authorization code flow) in addition to existing "Direct Access Grants". This is a Keycloak config change, not a code change. |
| Keycloak SAML IdP configurations | Keycloak admin API / console | Per-tenant SAML IdP broker configs. Created programmatically via `KeycloakClient.CreateSAMLIdP()` or manually via Keycloak admin console. |

## Change Dependency Order

```
[002_sso_config.sql migration] → [tenant/sso_config.go model] → [tenant/sso_config_repository.go] → [tenant/service.go SSO methods]
                                                                                                         ↓
[user/repository.go UpdateKeycloakID] → [user/service.go GetOrCreateByEmail change] → [auth/service.go caller update]
                                                                                         ↓
[config/config.go SSO config] → [auth/keycloak.go new methods] → [auth/sso_service.go] → [auth/sso_handler.go] → [cmd/server/main.go wiring + routes]
```

Independent tracks:
- Migration + tenant SSO config (data layer)
- User KeycloakID enhancement (user module)
- Config SSO fields (config module)

These three tracks converge at `auth/sso_service.go` which depends on all of them.
