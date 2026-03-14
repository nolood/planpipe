# Stage 4 Handoff — Solution Design Complete

## Task Summary
Enable SAML 2.0 SSO authentication for enterprise tenants in a multi-tenant Go backend, using Keycloak as a SAML broker. The system must support SP-initiated SAML flow, per-tenant SSO configuration, JIT user provisioning, automatic account linking by email, and programmatic Keycloak IdP automation.

## Classification
- **Type:** feature
- **Complexity:** high
- **Change scope:** 12 files modified, 8 new, 0 deleted across 6 modules
- **Solution direction:** systematic

## Implementation Approach
Build a dedicated SSO subsystem within the existing repository-service-handler architecture. The approach extends the auth module with SSO-specific endpoints and service, adds per-tenant SSO configuration to the tenant module via a new database table, and integrates with Keycloak's Admin REST API for programmatic IdP setup. JIT provisioning and account linking are handled in the user module with email-based matching.

## Solution Overview
When a user enters their email on the login page, the system checks whether their email domain has an SSO configuration in the `tenant_sso_config` table. If found, the user is redirected to Keycloak's SP-initiated SAML endpoint, which brokers authentication with the tenant's external IdP. On successful authentication, Keycloak redirects back to the application's callback endpoint, where the system extracts SAML attributes, performs JIT provisioning or account linking, issues a session token, and redirects to the application.

## Change Summary

### Modules Affected
| Module | Path | Changes | Scope |
|--------|------|---------|-------|
| Auth | `internal/auth/` | SSO endpoints (initiate, callback), SSO service, Keycloak client | large |
| Tenant | `internal/tenant/` | SSO config model, repository methods, email domain lookup | medium |
| User | `internal/user/` | JIT provisioning, account linking, KeycloakID field | medium |
| Config | `internal/config/` | SSO base configuration (Keycloak URL, realm, credentials) | small |
| Migrations | `migrations/` | `tenant_sso_config` table, user.keycloak_id column | small |
| Server | `cmd/server/main.go` | SSO route registration | small |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/handler.go` | Add `HandleSSOInitiate`, `HandleSSOCallback` | Entry points for SSO flow |
| `internal/auth/sso_service.go` | New file: SSO business logic | Orchestrates the full SSO flow |
| `internal/auth/keycloak_client.go` | New file: Keycloak Admin API client | Programmatic IdP management |
| `internal/tenant/model.go` | Add `TenantSSOConfig` struct | Per-tenant SSO settings |
| `internal/tenant/repository.go` | Add `GetSSOConfigByDomain`, `UpsertSSOConfig` | SSO config CRUD |
| `internal/user/service.go` | Add `FindOrCreateBySSO`, `LinkKeycloakID` | JIT provisioning and linking |
| `internal/user/model.go` | Add `KeycloakID` field to User | Track Keycloak identity |
| `internal/config/config.go` | Add `SSOConfig` struct | Base SSO settings |
| `cmd/server/main.go` | Register SSO routes | Wire SSO endpoints |
| `migrations/00X_add_tenant_sso_config.sql` | Create `tenant_sso_config` table | Per-tenant SSO storage |
| `migrations/00Y_add_user_keycloak_id.sql` | Add `keycloak_id` column to users | Keycloak identity link |

### New Entities
| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `SSOService` | service | `internal/auth/sso_service.go` | Orchestrates SSO initiate/callback flow |
| `KeycloakClient` | client | `internal/auth/keycloak_client.go` | Wraps Keycloak Admin REST API |
| `TenantSSOConfig` | model | `internal/tenant/model.go` | Per-tenant SSO configuration |
| `SSOConfig` | config | `internal/config/config.go` | Base SSO settings (Keycloak URL, realm) |

### Interface Changes
| Interface | Change | Consumers Affected |
|-----------|--------|-------------------|
| `UserRepository` | Add `FindByEmail`, `UpdateKeycloakID` methods | `user.Service` |
| `TenantRepository` | Add `GetSSOConfigByDomain`, `UpsertSSOConfig` methods | `auth.SSOService` |
| `User` model | Add `KeycloakID *string` field | All user consumers (read-compatible) |

## Implementation Sequence
| Step | What | Validates |
|------|------|-----------|
| 1 | Database migrations (SSO config table, keycloak_id column) | Schema is correct, migrations run cleanly |
| 2 | Config struct and environment loading | SSO configuration loads from env/file |
| 3 | Tenant SSO config model and repository | CRUD operations for SSO config work |
| 4 | User model extension and repository methods | KeycloakID field, FindByEmail, UpdateKeycloakID work |
| 5 | Keycloak Admin API client | Can create/read IdP configurations in Keycloak |
| 6 | SSO service (initiate + callback logic) | Full SSO flow works end-to-end |
| 7 | SSO HTTP handlers | HTTP endpoints work with proper request/response |
| 8 | Route registration and wiring | SSO endpoints accessible and properly wired |

## Key Technical Decisions
| Decision | Reasoning | User Approved |
|----------|-----------|---------------|
| Dedicated `tenant_sso_config` table (not JSONB) | User chose structured table for queryability and type safety | yes |
| Automatic email-based account linking | User preferred automatic over admin-driven linking | yes |
| Dual-auth support (SSO + password fallback) | User required password fallback even when SSO is enabled | yes |
| Single Keycloak realm for all tenants | Simplifies management; tenant isolation via IdP naming | yes |
| SP-initiated flow only (no IdP-initiated) | Agreed scope excludes IdP-initiated SSO | yes |

## Constraints Respected
- Backward compatible: existing password auth unchanged
- Repository-service-handler architecture pattern followed
- Single Keycloak realm (v24.0+)
- Email domain used for tenant SSO detection
- No admin UI in this scope

## Risks and Mitigations
| Risk | Mitigation | Severity |
|------|------------|----------|
| Keycloak SAML XML configuration complexity | Keycloak client encapsulates XML handling; test with real Keycloak instance | medium |
| Account linking race condition (concurrent SSO + password login) | Database unique constraint on keycloak_id; transaction isolation | medium |
| Test coverage gaps (no existing SSO tests) | Add unit tests for service logic; integration test for full flow | medium |
| Keycloak availability during SSO flow | Graceful error handling; fallback to password auth if SSO fails | low |

## Backward Compatibility
All changes are additive. Existing password authentication is unchanged. The `User` model gains an optional `KeycloakID` field (nullable). No existing API contracts are modified.

## User Decisions Log
- Password fallback: User chose dual-auth (SSO + password) over SSO-only
- Config storage: User chose dedicated table over JSONB column
- Account linking: User chose automatic email-based over admin-driven

## Acceptance Criteria
- SP-initiated SAML flow works end-to-end for a configured tenant
- JIT provisioning creates users with correct attributes from SAML response
- Existing users are linked by email automatically on first SSO login
- Per-tenant SSO configuration is stored and managed via dedicated table
- Keycloak IdP is created programmatically when tenant enables SSO
- Password authentication continues to work for all users
- Non-SSO tenants are unaffected

## Detailed References
- `implementation-design.md` — complete implementation design
- `change-map.md` — detailed file-level change map
- `design-decisions.md` — full decision journal
- `design-review-package.md` — user review document
- `agreed-task-model.md` — agreed task model (Stage 3)
