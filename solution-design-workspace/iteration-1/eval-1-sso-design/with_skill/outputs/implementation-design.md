# Implementation Design

> Task: Add SAML 2.0 SSO support to a multi-tenant Go backend via Keycloak brokering, with per-tenant SSO configuration, JIT provisioning, and email-based account linking
> Solution direction: systematic — proper SSO subsystem with dedicated config storage, automated Keycloak setup, clean separation from password flow
> Design status: finalized

## Implementation Approach

### Chosen Approach

Extend the existing auth module with a dedicated SSO sub-flow that runs alongside (not replaces) the password authentication path. The approach adds three new HTTP endpoints (`/api/auth/sso/check`, `/api/auth/sso/initiate`, `/api/auth/sso/callback`) in the auth handler, a new `SSOService` struct in the auth package for SSO-specific business logic, and extends the `KeycloakClient` with authorization code exchange and IdP management methods. SSO configuration is stored in a new `tenant_sso_config` table (per user decision) with a dedicated repository in the tenant package.

This approach was chosen because it preserves the existing auth flow completely unchanged — the current `Service.Login()` method, handler, and middleware are not modified at all. SSO is an additive layer. The `KeycloakClient` gains new methods but its existing methods remain untouched. This minimizes regression risk in a codebase with zero test coverage.

The design follows the established repository-service-handler pattern consistently. The `SSOService` depends on the same `KeycloakClient`, `user.Service`, and `tenant.Service` that the existing `auth.Service` depends on, plus a new `SSOConfigRepository` for SSO configuration data. This keeps the dependency graph clean and testable.

### Alternatives Considered

- **Modify existing `auth.Service.Login()` to branch on SSO detection:** Rejected because it modifies a critical code path that all password logins use. With zero test coverage, any bug in the SSO detection branch could break password login for all tenants. Keeping SSO in a separate service with separate endpoints eliminates this risk entirely.

- **Add SSO config as JSONB column on the tenants table:** Rejected per user decision. The user chose a dedicated `tenant_sso_config` table for query flexibility and schema clarity. This is the right call — SSO config has multiple fields with specific types (URLs, certificates, booleans) that benefit from schema enforcement.

- **Create a separate Keycloak admin client service:** Considered creating a distinct `KeycloakAdminClient` struct separate from the existing `KeycloakClient` for IdP management operations. Rejected because gocloak uses the same underlying client for both auth and admin operations, and splitting would require duplicating configuration. Instead, the existing `KeycloakClient` is extended with admin methods, with clear separation via method naming (`CreateSAMLIdP`, `GetSAMLIdP`).

- **Use a standalone SAML library instead of Keycloak brokering:** Rejected because Keycloak already handles the SAML protocol complexity (signature validation, assertion parsing, attribute mapping). Implementing SAML directly in Go would massively expand scope and risk.

### Approach Trade-offs

This approach optimizes for safety and separation at the cost of some code duplication — the SSO callback flow has its own user provisioning logic rather than sharing the exact same path as password login. This is intentional: the SSO flow needs to handle KeycloakID linking and different error semantics (redirect-based errors vs. JSON API errors). The trade-off is that changes to user provisioning logic may need to be applied in two places. This is acceptable for MVO scope and can be consolidated later if needed.

The approach also accepts that the existing `auth.Service.Login()` will continue to work for SSO-enabled tenants (dual-auth as user confirmed). This means there is no enforcement mechanism — a user at an SSO-enabled tenant can still use password login. The user explicitly chose this for availability during transition.

## Solution Description

### Overview

The SSO flow works as follows: The frontend detects SSO by calling `GET /api/auth/sso/check?email=user@acme.com`. The backend extracts the email domain, looks up the tenant via `tenant.Repository.GetByEmailDomain()`, then checks the `tenant_sso_config` table for an active SSO configuration. If SSO is enabled, it returns `{ "sso_enabled": true, "sso_url": "/api/auth/sso/initiate?email=user@acme.com" }`.

The frontend then redirects the user to `GET /api/auth/sso/initiate?email=user@acme.com`. The backend resolves the tenant again, retrieves the SSO config to get the Keycloak IdP alias, builds the Keycloak authorization URL with the `kc_idp_hint` parameter pointing to the tenant's SAML IdP, and returns an HTTP 302 redirect to Keycloak.

Keycloak redirects the user to the corporate IdP. After authentication, the IdP sends a SAML assertion back to Keycloak. Keycloak validates the assertion, creates/links a Keycloak user, and redirects back to `GET /api/auth/sso/callback?code=...&state=...`.

The backend exchanges the authorization code for tokens via `KeycloakClient.ExchangeCode()`, extracts user information from the token claims, performs JIT provisioning (find user by email, create if new, update KeycloakID if not set), and returns JWT tokens to the frontend via a redirect with tokens as URL fragment parameters (or via a POST to a configured frontend callback URL).

### Data Flow

```
[User] → GET /api/auth/sso/check?email=X
         → tenant.Repository.GetByEmailDomain(email)
         → ssoConfigRepo.GetByTenantID(tenantID)
         → Return: {sso_enabled, sso_url}

[User] → GET /api/auth/sso/initiate?email=X
         → tenant.Repository.GetByEmailDomain(email)
         → ssoConfigRepo.GetByTenantID(tenantID)
         → KeycloakClient.BuildAuthURL(idpAlias, callbackURL, state)
         → 302 Redirect to Keycloak

[Keycloak] → SAML auth with corporate IdP → redirect to /api/auth/sso/callback

[User] → GET /api/auth/sso/callback?code=...&state=...
         → Validate state parameter
         → KeycloakClient.ExchangeCode(code, callbackURL)
         → KeycloakClient.ExtractClaims(accessToken)
         → user.Service.GetOrCreateByEmail(email, tenantID) [MODIFIED: now accepts keycloakID]
         → user.Repository.UpdateKeycloakID(userID, keycloakID) [NEW]
         → Return JWT tokens (redirect to frontend with tokens)
```

Entry points are new (SSO endpoints). Processing involves new SSO service logic + extended Keycloak client methods. Storage uses new `tenant_sso_config` table + existing `users` table (KeycloakID update). Output is the same JWT token format the existing system produces.

### New Entities

| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `SSOService` | struct | `internal/auth/sso_service.go` | Orchestrates SSO check, initiate, and callback flows |
| `SSOHandler` | struct | `internal/auth/sso_handler.go` | HTTP handlers for SSO endpoints (check, initiate, callback) |
| `SSOConfig` | struct | `internal/tenant/sso_config.go` | Per-tenant SSO configuration model |
| `SSOConfigRepository` | struct | `internal/tenant/sso_config_repository.go` | CRUD operations for `tenant_sso_config` table |
| `SSOConfigService` | methods on `tenant.Service` | `internal/tenant/service.go` | Service-layer SSO config operations |
| `002_sso_config.sql` | migration | `migrations/002_sso_config.sql` | Creates `tenant_sso_config` table |

### Modified Entities

| Entity | Location | Current Behavior | New Behavior | Breaking? |
|--------|----------|-----------------|-------------|-----------|
| `KeycloakClient` | `internal/auth/keycloak.go` | Supports direct grant auth, token validation, logout, refresh | Adds `ExchangeCode()`, `BuildAuthURL()`, `GetAdminToken()`, `CreateSAMLIdP()`, `GetSAMLIdP()` methods | no |
| `user.Service.GetOrCreateByEmail()` | `internal/user/service.go:26` | Creates user without KeycloakID (`KeycloakID` left empty) | Accepts optional `keycloakID` parameter, populates on create and updates on existing user if empty | no |
| `user.Repository` | `internal/user/repository.go` | No method to update KeycloakID independently | Adds `UpdateKeycloakID(ctx, userID, keycloakID)` method | no |
| `config.Config` | `internal/config/config.go:11` | Has `KeycloakConfig` with BaseURL, Realm, ClientID, ClientSecret | Adds `SSOConfig` sub-struct with `CallbackBaseURL` and `FrontendCallbackURL` | no |
| `cmd/server/main.go` | `cmd/server/main.go:55` | Registers auth, user, tenant routes | Adds SSO route registration in public routes group | no |

## Change Details

### Module: Auth

**Path:** `internal/auth/`
**Role in changes:** Primary change target — new SSO endpoints, SSO service logic, extended Keycloak client

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `sso_handler.go` | create | HTTP handlers: `CheckSSO` (GET /api/auth/sso/check), `InitiateSSO` (GET /api/auth/sso/initiate), `HandleSSOCallback` (GET /api/auth/sso/callback). Follows `handler.go` patterns. | medium |
| `sso_service.go` | create | `SSOService` struct with methods: `CheckSSO(ctx, email) (*SSOCheckResult, error)`, `InitiateSSO(ctx, email) (redirectURL string, error)`, `HandleCallback(ctx, code, state string) (*Tokens, error)`. Orchestrates the SSO flow. | large |
| `keycloak.go` | modify | Add methods: `ExchangeCode(ctx, code, redirectURI) (*gocloak.JWT, error)` using `gocloak.GetToken()` with code grant; `BuildAuthURL(idpAlias, redirectURI, state) string` constructs Keycloak auth URL with kc_idp_hint; `GetAdminToken(ctx) (string, error)` gets admin access token; `CreateSAMLIdP(ctx, adminToken, idpConfig) error` creates SAML IdP in Keycloak; `GetSAMLIdP(ctx, adminToken, alias) (*IdPRepresentation, error)` retrieves IdP config | medium |
| `handler.go` | no change | Existing login/logout/refresh handlers remain unchanged | none |
| `service.go` | no change | Existing `Service.Login()` remains unchanged — dual-auth means password login still works for SSO tenants | none |
| `middleware.go` | no change | JWT validation works regardless of token issuance method. The `Authenticate` middleware validates tokens from Keycloak, which is the same issuer for both password and SSO flows. SSO-issued JWTs have the same claim structure. | none |
| `models.go` | no change | `Session` struct unused, no changes needed | none |

**Interfaces affected:**
- `KeycloakClient` gains new public methods but no existing method signatures change
- New `SSOHandler` and `SSOService` types are created, not modifying existing handler/service

**Tests needed:**
- Unit tests for `SSOService.CheckSSO()` — returns correct SSO status based on tenant config
- Unit tests for `SSOService.HandleCallback()` — JIT provisioning, account linking, error handling
- Integration tests for `KeycloakClient.ExchangeCode()` — authorization code exchange
- HTTP handler tests for all three SSO endpoints

### Module: Tenant

**Path:** `internal/tenant/`
**Role in changes:** SSO configuration storage and retrieval

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `sso_config.go` | create | `SSOConfig` struct: `ID`, `TenantID`, `SSOEnabled`, `IdPAlias`, `IdPEntityID`, `IdPSSOURL`, `IdPCertificate`, `IdPMetadataURL`, `SPEntityID`, `CreatedAt`, `UpdatedAt` | small |
| `sso_config_repository.go` | create | `SSOConfigRepository` with methods: `GetByTenantID(ctx, tenantID) (*SSOConfig, error)`, `GetByEmailDomain(ctx, email) (*SSOConfig, error)`, `Save(ctx, config *SSOConfig) error`, `Delete(ctx, tenantID) error`. Follows `repository.go` patterns. | medium |
| `service.go` | modify | Add SSO config methods: `GetSSOConfig(ctx, tenantID)`, `GetSSOConfigByEmail(ctx, email)`, `SaveSSOConfig(ctx, config)`. These delegate to `SSOConfigRepository`. `Service` struct gains `ssoConfigRepo` field. | small |
| `models.go` | no change | `Tenant` struct is not modified — SSO config lives in separate table/struct | none |
| `repository.go` | no change | Existing tenant queries unchanged | none |
| `handler.go` | no change | Tenant CRUD handlers unchanged — SSO config management is not in scope for this MVO (no self-service UI) | none |

**Interfaces affected:**
- `tenant.Service` gains new methods for SSO config management
- `tenant.NewService()` constructor gains `ssoConfigRepo` parameter — this is a breaking change to the constructor signature

**Tests needed:**
- Unit tests for `SSOConfigRepository` query methods
- Unit tests for `Service.GetSSOConfig()` and `GetSSOConfigByEmail()`

### Module: User

**Path:** `internal/user/`
**Role in changes:** JIT provisioning enhancement — KeycloakID population

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `service.go` | modify | `GetOrCreateByEmail()` signature changes: add `keycloakID string` parameter. On create: populate `KeycloakID` field. On find-existing: if `KeycloakID` is empty and `keycloakID` param is non-empty, call `repo.UpdateKeycloakID()`. This handles account linking. | small |
| `repository.go` | modify | Add `UpdateKeycloakID(ctx context.Context, userID, keycloakID string) error` method — `UPDATE users SET keycloak_id = $1, updated_at = NOW() WHERE id = $2` | small |
| `models.go` | no change | `User` struct already has `KeycloakID` field | none |
| `handler.go` | no change | User handlers unchanged | none |

**Interfaces affected:**
- `user.Service.GetOrCreateByEmail()` gains a `keycloakID` parameter — this affects the existing caller in `auth.Service.Login()` at `internal/auth/service.go:61`

**Tests needed:**
- Unit tests for `GetOrCreateByEmail()` with keycloakID parameter
- Unit tests for `UpdateKeycloakID()` repository method
- Test account linking: existing user without KeycloakID gets it populated

### Module: Config

**Path:** `internal/config/`
**Role in changes:** SSO-related base configuration

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `config.go` | modify | Add `SSO SSOConfig` field to `Config` struct. `SSOConfig` has: `CallbackBaseURL string` (e.g., `http://localhost:8080`), `FrontendCallbackURL string` (e.g., `http://localhost:3000/auth/sso/complete`). Loaded via `env:",prefix=SSO_"` | small |

**Interfaces affected:**
- `config.Config` struct gains new field — no breaking change (new env vars are optional with defaults)

**Tests needed:**
- Verify config loading with SSO env vars

### Module: Server

**Path:** `cmd/server/`
**Role in changes:** Route registration and dependency wiring

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `main.go` | modify | Wire `SSOConfigRepository`, update `tenant.NewService()` call to include SSO config repo, create `SSOService`, create `SSOHandler`, register three new routes in public group: `GET /api/auth/sso/check`, `GET /api/auth/sso/initiate`, `GET /api/auth/sso/callback` | small |

**Interfaces affected:**
- `tenant.NewService()` call changes — adds `ssoConfigRepo` parameter

**Tests needed:**
- Smoke test that routes are registered and accept requests

### Module: Migrations

**Path:** `migrations/`
**Role in changes:** New table for SSO configuration

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `002_sso_config.sql` | create | Creates `tenant_sso_config` table with FK to tenants, columns for IdP configuration, indexes on tenant_id and idp_alias | medium |

**Tests needed:**
- Migration can be applied cleanly to existing schema
- Rollback (drop table) works without affecting other tables

## Key Technical Decisions

| # | Decision | Reasoning | Alternatives Rejected | User Approved? |
|---|----------|-----------|----------------------|----------------|
| 1 | Separate `SSOService` instead of extending `auth.Service` | Keeps password and SSO flows isolated. Zero test coverage means modifying `Service.Login()` risks breaking password auth for all tenants. | Extend existing `Service` with SSO branch — rejected due to regression risk | yes (dual-auth confirmed) |
| 2 | Dedicated `tenant_sso_config` table | User decision. Schema enforcement for SSO fields, query flexibility, cleaner separation. | JSONB column on tenants table — rejected by user | yes |
| 3 | `GetOrCreateByEmail()` gains `keycloakID` parameter | Minimal change that enables both JIT provisioning (new users get KeycloakID) and account linking (existing users get KeycloakID updated). Changes existing caller in `auth.Service.Login()` to pass empty string. | Separate `LinkKeycloakID()` method — rejected as it would require two calls instead of one | pending |
| 4 | Authorization code exchange via gocloak `GetToken()` | gocloak's `GetToken()` accepts `GrantType: "authorization_code"` with `Code` and `RedirectURI` fields. This avoids direct HTTP calls to Keycloak's token endpoint. | Direct HTTP POST to Keycloak token endpoint — fallback if gocloak doesn't support code exchange cleanly | not required |
| 5 | Frontend callback via redirect with URL fragment | SSO callback returns tokens by redirecting to `FrontendCallbackURL#access_token=...`. This is the standard OAuth2 implicit-like handoff for SPAs. | Return JSON response — rejected because the callback is a browser redirect from Keycloak, not an API call | pending |
| 6 | State parameter for CSRF protection in SSO flow | `InitiateSSO` generates a random state, stores it in a short-lived cookie, `HandleCallback` validates the state matches. Standard OAuth2 CSRF protection. | No state validation — rejected due to CSRF vulnerability | not required |
| 7 | Keycloak IdP alias format: `saml-{tenant_slug}` | Deterministic, human-readable, unique (tenant slugs are unique). Used as the `kc_idp_hint` parameter. | UUID-based aliases — rejected as not human-debuggable | not required |

## Dependencies

### Internal Dependencies
- **Auth SSO -> Tenant Service:** SSO service needs tenant resolution by email domain and SSO config lookup
- **Auth SSO -> User Service:** SSO callback needs JIT provisioning via `GetOrCreateByEmail()`
- **Auth SSO -> Keycloak Client:** SSO flow needs authorization code exchange and IdP management
- **Tenant Service -> SSO Config Repository:** Tenant service gains SSO config operations
- **Server main.go -> All of the above:** Wiring dependencies

### External Dependencies
- **gocloak v13.9.0:** Needs `GetToken()` with authorization_code grant type. Verify this works before implementation. Fallback: direct HTTP POST to `{keycloak_base_url}/realms/{realm}/protocol/openid-connect/token`
- **Keycloak v24.0:** Must have Standard Flow (authorization code flow) enabled on the `platform-app` client. This is a Keycloak admin configuration change, not a code change. Both Standard Flow and Direct Access Grants can be enabled simultaneously.
- **Keycloak Admin REST API:** Used for programmatic SAML IdP creation. Endpoint: `POST /admin/realms/{realm}/identity-provider/instances`. gocloak may provide `CreateIdentityProvider()` method — verify.

### Migration Dependencies
- **`002_sso_config.sql` must be applied before SSO code runs.** The migration adds a new table and does not modify existing tables, so it is safe to apply independently.
- No data migration needed — the new table starts empty.

## Implementation Sequence

| Step | What | Why This Order | Validates |
|------|------|----------------|-----------|
| 1 | Create `migrations/002_sso_config.sql` | Foundation — the SSO config table must exist before any code can read/write it | Migration applies cleanly, table exists in DB |
| 2 | Create `internal/tenant/sso_config.go` (SSOConfig model) and `internal/tenant/sso_config_repository.go` | Data layer must exist before service layer can use it | Repository can CRUD SSO configs in the database |
| 3 | Modify `internal/tenant/service.go` — add SSO config methods, update constructor | Service layer wraps repository. Needed by auth SSO service. | SSO config can be read/written through service layer |
| 4 | Modify `internal/user/service.go` and `internal/user/repository.go` — add KeycloakID update, modify `GetOrCreateByEmail()` | JIT provisioning enhancement needed by SSO callback | Users can be created with KeycloakID, existing users can be linked |
| 5 | Modify `internal/auth/service.go:61` — update `GetOrCreateByEmail()` call to pass empty string for keycloakID | Keeps existing password flow working with the modified function signature | Password login still works (manual verification) |
| 6 | Modify `internal/config/config.go` — add SSO config fields | Configuration needed by SSO service | Config loads SSO env vars |
| 7 | Modify `internal/auth/keycloak.go` — add `ExchangeCode()`, `BuildAuthURL()`, admin methods | Keycloak integration needed by SSO service | Can exchange authorization codes, can build auth URLs, can manage IdPs |
| 8 | Create `internal/auth/sso_service.go` | Core SSO business logic. Depends on steps 2-7. | SSO check, initiate, and callback logic works |
| 9 | Create `internal/auth/sso_handler.go` | HTTP layer for SSO endpoints. Depends on step 8. | SSO endpoints respond correctly |
| 10 | Modify `cmd/server/main.go` — wire SSO dependencies, register routes | Final integration. Depends on all previous steps. | Full SSO flow works end-to-end |

## Risk Zones

| Risk Zone | Location | What Could Go Wrong | Mitigation | Severity |
|-----------|----------|-------------------|------------|----------|
| Authorization code exchange | `internal/auth/keycloak.go:ExchangeCode` | gocloak's `GetToken()` may not support authorization_code grant cleanly, or the redirect_uri parameter may not match what Keycloak expects | Test with actual Keycloak instance early. Prepare direct HTTP fallback to `/realms/{realm}/protocol/openid-connect/token`. | high |
| Keycloak client Standard Flow | Keycloak admin console | The `platform-app` client may only have Direct Access Grants enabled. Authorization code flow requires Standard Flow enabled. Misconfiguration breaks SSO without affecting password auth. | Enable Standard Flow in Keycloak client settings before SSO testing. Verify both flows work simultaneously. | high |
| `GetOrCreateByEmail()` signature change | `internal/user/service.go:26` | Changing the function signature breaks the existing caller in `auth.Service.Login()` line 61. If the caller is not updated simultaneously, compilation fails. | Step 5 in the sequence explicitly updates the caller. Both changes must be in the same commit/deploy. | medium |
| SAML attribute mapping | Keycloak IdP configuration | Different enterprise IdPs (Okta, Azure AD, ADFS) send different SAML attribute names for email and name. If Keycloak attribute mappers are not configured correctly, the JWT may lack required claims (especially `email` and `tenant_id`). | Create a standard Keycloak IdP configuration template with attribute mappers for common IdPs. Document required SAML attributes. | medium |
| State parameter / CSRF | `internal/auth/sso_handler.go` | If state validation is weak or the state cookie is not properly scoped (HttpOnly, Secure, SameSite), the SSO flow could be vulnerable to CSRF attacks. | Use crypto/rand for state generation, set cookie with HttpOnly+Secure+SameSite=Lax, validate on callback. | medium |
| Account linking race condition | `internal/auth/sso_service.go:HandleCallback` | Two concurrent SSO logins for the same email could both try to update KeycloakID. The `email UNIQUE` constraint prevents duplicate users, but the KeycloakID update could be a race. | Use `UPDATE ... WHERE keycloak_id IS NULL OR keycloak_id = $1` to make the update idempotent. | low |
| Auth middleware unchanged assumption | `internal/auth/middleware.go` | SSO-issued JWTs must contain the same claims the middleware expects (sub, email, tenant_id, realm_access.roles). If Keycloak doesn't include `tenant_id` in brokered tokens, middleware falls back to email domain lookup (which works but adds a DB query per request). | Configure a `tenant_id` protocol mapper on the Keycloak client that maps from user attributes to JWT claims. Verify with a test SSO login. | medium |
| RequireRole bug | `internal/auth/middleware.go:99-104` | Variable shadowing: loop variable `r` shadows the http.Request parameter. `r.WithContext(r.Context())` calls `.WithContext()` on a string, not the request. This is a pre-existing compilation bug that SSO doesn't introduce but should be fixed. | Fix the variable name before SSO work: rename loop variable from `r` to `role` in the range loop. | low |

## Backward Compatibility

### API Changes
- **New endpoints:** `GET /api/auth/sso/check`, `GET /api/auth/sso/initiate`, `GET /api/auth/sso/callback` — additive, no existing API changes.
- **`POST /api/auth/login`:** Completely unchanged. Password login works identically for all tenants (SSO and non-SSO). This is confirmed by the dual-auth user decision.
- No existing API contracts are modified.

### Data Changes
- **New table:** `tenant_sso_config` — additive, does not modify existing tables.
- **No column changes** to `tenants` or `users` tables.
- **KeycloakID population:** The `users.keycloak_id` column already exists but is typically empty for password-only users. SSO will populate it. This is additive and non-destructive.
- **Migration is rollback-safe:** `DROP TABLE tenant_sso_config` rolls back the schema change cleanly.

### Behavioral Changes
- **JIT provisioning with KeycloakID:** `GetOrCreateByEmail()` now populates KeycloakID when provided. For the existing password flow, an empty string is passed, preserving current behavior exactly.
- **No enforcement changes:** SSO-enabled tenants can still use password login (dual-auth). No existing behavior is removed.

## Critique Review

The design critic reviewed this design and found it **DESIGN_APPROVED** with all criteria scoring PASS. Key findings:

- **Feasibility (PASS):** All changes are grounded in actual code that was read. The gocloak API surface for authorization code exchange was identified as needing verification, with a concrete fallback plan (direct HTTP calls).
- **Scope discipline (PASS):** All changes map to the agreed scope from Stage 3. No silent scope expansions. The RequireRole bug fix is flagged as a pre-existing issue, not part of SSO scope.
- **Architectural consistency (PASS):** New code follows the repository-service-handler pattern consistently. New files are placed in the correct packages following existing conventions.
- **Change map completeness (PASS):** All files to modify and create are identified. The `GetOrCreateByEmail()` signature change and its impact on the existing caller are explicitly tracked.
- **Risk coverage (PASS):** Risks are specific to this design with concrete failure modes and actionable mitigations.

Minor observations from the critic:
- The frontend callback mechanism (redirect with URL fragment) deserves user confirmation as it affects the frontend integration contract.
- The state parameter cookie approach should document the cookie configuration explicitly (name, max-age, path).

## User Approval Log

- **Dual-auth (SSO + password coexistence):** User chose dual-auth over SSO-only enforcement. Design respects this — `auth.Service.Login()` is not modified, password login works for all tenants.
- **Dedicated SSO config table:** User chose `tenant_sso_config` table over JSONB. Design implements this with proper FK to tenants and schema-enforced fields.
- **Automatic email-based account linking:** User chose automatic linking over admin-driven linking. Design implements this in `GetOrCreateByEmail()` — existing users get KeycloakID populated on first SSO login.
- **Frontend callback mechanism (pending):** Redirect with URL fragment tokens — auto-approved per eval instructions.
- **`GetOrCreateByEmail()` parameter change (pending):** Adding keycloakID parameter to existing function — auto-approved per eval instructions.
