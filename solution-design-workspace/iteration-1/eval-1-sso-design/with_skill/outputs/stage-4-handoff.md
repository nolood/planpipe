# Stage 4 Handoff — Solution Design Complete

## Task Summary
Add SAML 2.0 Single Sign-On support to a multi-tenant Go backend platform so that enterprise tenants can authenticate their employees through corporate identity providers. Keycloak acts as the SAML SP/broker. Existing email/password login remains fully functional for non-SSO tenants. SSO configuration is per-tenant, stored in a dedicated database table.

## Classification
- **Type:** feature
- **Complexity:** high
- **Change scope:** 7 files modified, 5 new, 0 deleted across 6 modules
- **Solution direction:** systematic — proper SSO subsystem with dedicated config storage, automated Keycloak setup, clean separation from password flow

## Implementation Approach
The design adds SSO as an additive layer that runs alongside the existing password authentication, without modifying any existing auth code. A new `SSOService` handles the SSO flow (check, initiate, callback) separately from the existing `auth.Service`, eliminating regression risk in a zero-test-coverage codebase. The Keycloak client is extended with authorization code exchange and IdP management methods. SSO configuration is stored in a dedicated `tenant_sso_config` table (user decision). JIT provisioning reuses the existing `GetOrCreateByEmail()` pattern, enhanced to populate KeycloakID for account linking.

## Solution Overview
The frontend detects SSO by calling `/api/auth/sso/check` with the user's email. If the tenant has SSO enabled, the user is redirected to `/api/auth/sso/initiate`, which resolves the tenant's IdP alias and redirects to Keycloak with a `kc_idp_hint`. Keycloak brokers SAML authentication with the corporate IdP. After authentication, Keycloak redirects to `/api/auth/sso/callback` with an authorization code. The backend exchanges the code for JWT tokens, performs JIT provisioning (find or create user by email, link KeycloakID), and delivers tokens to the frontend via a redirect with URL fragment parameters. The JWT tokens produced are identical in format to password-flow tokens, so all downstream middleware and protected routes work without modification.

## Change Summary

### Modules Affected
| Module | Path | Changes | Scope |
|--------|------|---------|-------|
| Auth | `internal/auth/` | New SSOService, SSOHandler, extended KeycloakClient with code exchange and IdP management | large |
| Tenant | `internal/tenant/` | New SSOConfig model, SSOConfigRepository, service methods for SSO config | medium |
| User | `internal/user/` | GetOrCreateByEmail gains keycloakID param, new UpdateKeycloakID repo method | small |
| Config | `internal/config/` | SSO sub-config with callback URLs | small |
| Server | `cmd/server/` | Wire SSO dependencies, register SSO routes | small |
| Migrations | `migrations/` | New tenant_sso_config table | medium |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/sso_service.go` (new) | SSO flow orchestration: CheckSSO, InitiateSSO, HandleCallback | Core SSO business logic |
| `internal/auth/sso_handler.go` (new) | HTTP handlers for /api/auth/sso/check, /initiate, /callback | SSO endpoint entry points |
| `internal/auth/keycloak.go` | Add ExchangeCode(), BuildAuthURL(), CreateSAMLIdP(), GetSAMLIdP() | Keycloak integration for authorization code flow and IdP management |
| `internal/tenant/sso_config_repository.go` (new) | CRUD for tenant_sso_config table | SSO config persistence |
| `internal/user/service.go:GetOrCreateByEmail` | Add keycloakID parameter for JIT provisioning with identity linking | Account linking on first SSO login |
| `internal/auth/service.go:61` | Update GetOrCreateByEmail call to pass empty keycloakID | Keep password flow working with modified signature |
| `cmd/server/main.go` | Wire SSOConfigRepository, SSOService, SSOHandler; register routes | Application integration |
| `migrations/002_sso_config.sql` (new) | CREATE TABLE tenant_sso_config with FK to tenants | Per-tenant SSO configuration schema |

### New Entities
| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| SSOService | struct | `internal/auth/sso_service.go` | Orchestrates SSO check, initiation, and callback flows |
| SSOHandler | struct | `internal/auth/sso_handler.go` | HTTP handlers for SSO endpoints |
| SSOConfig | struct | `internal/tenant/sso_config.go` | Per-tenant SAML SSO configuration model |
| SSOConfigRepository | struct | `internal/tenant/sso_config_repository.go` | Database operations for tenant_sso_config table |
| tenant_sso_config | table | `migrations/002_sso_config.sql` | Database table for per-tenant SSO configuration |

### Interface Changes
| Interface | Change | Consumers Affected |
|-----------|--------|-------------------|
| `user.Service.GetOrCreateByEmail()` | Add `keycloakID string` parameter | `auth.Service.Login()` at line 61 (update to pass ""), new `auth.SSOService.HandleCallback()` |
| `tenant.NewService()` | Add `ssoRepo *SSOConfigRepository` parameter | `cmd/server/main.go` line 41 |
| `auth.KeycloakClient` | Add 5 new public methods (no existing methods change) | New `auth.SSOService` (new consumer only) |

## Implementation Sequence
| Step | What | Validates |
|------|------|-----------|
| 1 | Create `migrations/002_sso_config.sql` | Migration applies, table exists |
| 2 | Create `internal/tenant/sso_config.go` + `sso_config_repository.go` | Can CRUD SSO configs in DB |
| 3 | Modify `internal/tenant/service.go` — SSO config methods | SSO config accessible through service layer |
| 4 | Modify `internal/user/repository.go` + `service.go` — KeycloakID handling | Users can be created/linked with KeycloakID |
| 5 | Modify `internal/auth/service.go` — update GetOrCreateByEmail caller | Password login still works with new signature |
| 6 | Modify `internal/config/config.go` — SSO config fields | Config loads SSO env vars |
| 7 | Modify `internal/auth/keycloak.go` — add code exchange and IdP methods | Can exchange auth codes, build auth URLs, manage IdPs |
| 8 | Create `internal/auth/sso_service.go` | SSO business logic works |
| 9 | Create `internal/auth/sso_handler.go` | SSO endpoints respond correctly |
| 10 | Modify `cmd/server/main.go` — wire and register | Full SSO flow works end-to-end |

## Key Technical Decisions
| Decision | Reasoning | User Approved |
|----------|-----------|---------------|
| Separate SSOService from existing auth.Service | Isolates SSO from password flow — zero test coverage makes modification of existing Login() too risky | yes (dual-auth confirmed) |
| Dedicated tenant_sso_config table | User chose this over JSONB for schema enforcement and query flexibility | yes |
| Modify GetOrCreateByEmail to accept keycloakID | Keeps provisioning logic in one place, enables account linking with minimal change | yes |
| gocloak for authorization code exchange | Consistent with existing Keycloak integration pattern; fallback to direct HTTP if gocloak lacks support | not required |
| Frontend callback via URL fragment redirect | Standard OAuth2/SPA pattern; avoids server-side session storage | yes |
| State parameter with HttpOnly cookie for CSRF | Standard OAuth2 CSRF protection; stateless approach | not required |
| IdP alias format: saml-{tenant_slug} | Deterministic, human-readable, leverages existing slug uniqueness | not required |

## Constraints Respected
- **Backward compatibility:** `POST /api/auth/login` is completely unchanged — not a single line modified
- **Single Keycloak realm:** All IdPs configured in the `platform` realm with unique `saml-{tenant_slug}` aliases
- **Repository-service-handler pattern:** All new code follows this pattern strictly (SSOConfigRepository -> Service -> Handler)
- **JWT claim consistency:** SSO-issued JWTs contain same claims (sub, email, tenant_id, realm_access.roles) — middleware works unchanged
- **Dedicated SSO config table:** Per user decision, not JSONB
- **Q3 timeline / MVO scope:** No SLO, no SCIM, no admin UI, no IdP-initiated SSO

## Risks and Mitigations
| Risk | Mitigation | Severity |
|------|------------|----------|
| gocloak may not support authorization_code grant cleanly | Fallback to direct HTTP POST to Keycloak token endpoint prepared | high |
| Keycloak client needs Standard Flow enabled (config change) | Enable in staging first, verify both flows coexist | high |
| Zero test coverage on modified GetOrCreateByEmail | Include unit tests with the change, test both password and SSO call paths | medium |
| SAML attribute mapping varies across IdPs | Create Keycloak IdP config template with standard attribute mappers | medium |
| Auth middleware needs tenant_id in JWT for SSO users | Configure Keycloak protocol mapper for tenant_id claim; email domain fallback exists | medium |
| Account linking race condition on concurrent SSO logins | Idempotent UPDATE with WHERE keycloak_id IS NULL condition | low |

## Backward Compatibility
All changes are additive. No existing API endpoints, database columns, or behaviors are modified. New SSO endpoints are added alongside existing ones. The `tenant_sso_config` table is new and does not affect existing tables. The `users.keycloak_id` column already exists and is simply populated for SSO users. The only behavioral change is `GetOrCreateByEmail()` now accepting an additional parameter — the existing caller is updated to pass an empty string, preserving identical behavior.

## User Decisions Log
- **Dual-auth over SSO-only:** User chose to keep password login available for SSO tenants during transition. Design respects this by not modifying the existing Login() flow.
- **Dedicated table over JSONB:** User chose `tenant_sso_config` table for schema enforcement. Design implements this with proper FK and indexes.
- **Automatic email-based account linking:** User chose automatic over admin-driven linking. Design implements this in the enhanced GetOrCreateByEmail().
- **Frontend callback mechanism (auto-approved):** URL fragment redirect for token delivery to SPA.
- **GetOrCreateByEmail parameter change (auto-approved):** Adding keycloakID parameter accepted.

## Acceptance Criteria
- Enterprise user can complete SP-initiated SAML SSO login through their corporate IdP
- Existing email/password login works identically before and after the change
- New SSO users are JIT-provisioned with correct tenant association
- Existing password users are automatically linked on first SSO login via email match
- Per-tenant SSO configuration is stored and retrievable
- Keycloak SAML IdP configuration can be automated via Admin API
- JWT tokens from SSO flow contain all required claims for middleware compatibility

## Detailed References
- `implementation-design.md` — complete implementation design
- `change-map.md` — detailed file-level change map
- `design-decisions.md` — full decision journal with 7 decisions
- `design-review-package.md` — user review document with 3 approval points
- `agreed-task-model.md` — agreed task model (Stage 3)
- `stage-3-handoff.md` — Stage 3 handoff document
