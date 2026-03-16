# Change Map: SAML SSO Implementation

This document maps every file that must be created or modified, what changes, why, and in what order.

## Summary

| Category | Files | New | Modified |
|----------|-------|-----|----------|
| Database migrations | 1 | 1 | 0 |
| Tenant module | 3 | 2 | 1 |
| User module | 2 | 0 | 2 |
| Auth module | 3 | 1 | 2 |
| Config module | 1 | 0 | 1 |
| Server entry point | 1 | 0 | 1 |
| Infrastructure | 1 | 0 | 1 |
| **Total** | **12** | **3** | **8** |

---

## New Files

### 1. `migrations/002_sso_config.sql`

**Purpose:** Create the `tenant_sso_config` table for per-tenant SAML SSO storage.

**Content:**
- `CREATE TABLE tenant_sso_config` with columns: id (UUID PK), tenant_id (UUID FK UNIQUE), enabled (bool), idp_alias (text), idp_display_name (text), metadata_url (text nullable), metadata_xml (text nullable), entity_id (text), sso_url (text), certificate (text), name_id_format (text), sign_requests (bool), created_at, updated_at
- Indexes on tenant_id and idp_alias
- ON DELETE CASCADE from tenants

**Dependencies:** `migrations/001_initial.sql` (tenants table must exist)

**Risk:** Low. Purely additive schema change. No existing tables modified.

---

### 2. `internal/tenant/sso_config.go`

**Purpose:** Define the `SSOConfig` struct and related types for SAML SSO configuration.

**Content:**
- `SSOConfig` struct matching the `tenant_sso_config` table columns
- No behavioral logic, pure data model

**Dependencies:** None (standalone struct)

**Risk:** None. New file, no existing code touched.

---

### 3. `internal/tenant/sso_repository.go`

**Purpose:** Data access layer for the `tenant_sso_config` table.

**Content:**
- `SSORepository` struct with `*pgxpool.Pool` field
- `NewSSORepository(db *pgxpool.Pool) *SSORepository`
- `GetByTenantID(ctx, tenantID) (*SSOConfig, error)` -- returns nil/ErrNotFound if no config
- `GetByIdPAlias(ctx, alias) (*SSOConfig, error)` -- lookup by Keycloak IdP alias
- `Create(ctx, *SSOConfig) error`
- `Update(ctx, *SSOConfig) error`
- `Delete(ctx, tenantID) error`

**Dependencies:** `internal/tenant/sso_config.go` (SSOConfig struct), pgxpool

**Risk:** None. New file, follows existing repository pattern from `internal/tenant/repository.go`.

---

### 4. `internal/auth/sso_handler.go`

**Purpose:** SSO admin API handlers for managing tenant SSO configuration. Separated from the main auth handler to keep the file focused.

**Content:**
- `SSOAdminHandler` struct with `tenantSvc` and `keycloakClient` dependencies
- `GetSSOConfig(w, r)` -- `GET /api/admin/tenants/{tenantID}/sso`
- `SaveSSOConfig(w, r)` -- `PUT /api/admin/tenants/{tenantID}/sso` -- accepts SAML IdP metadata, stores in DB, creates/updates Keycloak IdP
- `DeleteSSOConfig(w, r)` -- `DELETE /api/admin/tenants/{tenantID}/sso` -- removes config from DB and Keycloak

**Dependencies:** `internal/tenant/service.go`, `internal/auth/keycloak.go`

**Risk:** Low. New file. Admin-only endpoints behind auth middleware + role check.

---

## Modified Files

### 5. `internal/tenant/service.go`

**Current state:** `Service` struct with `repo *Repository`. Methods: GetByID, GetBySlug, GetByEmailDomain, Update, List.

**Changes:**
- Add `ssoRepo *SSORepository` field to `Service` struct
- Update `NewService` to accept `*SSORepository` parameter: `NewService(repo *Repository, ssoRepo *SSORepository) *Service`
- Add method `GetSSOConfig(ctx, tenantID) (*SSOConfig, error)` -- delegates to `ssoRepo.GetByTenantID`
- Add method `GetSSOConfigByEmailDomain(ctx, email) (*Tenant, *SSOConfig, error)` -- resolves tenant, then fetches SSO config. Returns (tenant, nil, nil) if tenant found but no SSO config
- Add method `SaveSSOConfig(ctx, *SSOConfig) error` -- create or update in ssoRepo
- Add method `DeleteSSOConfig(ctx, tenantID) error` -- delete from ssoRepo

**Breaking change:** `NewService` signature changes from `NewService(repo *Repository)` to `NewService(repo *Repository, ssoRepo *SSORepository)`. This affects `cmd/server/main.go` wiring.

**Risk:** Medium. Signature change requires coordinated update in main.go. But all existing methods remain unchanged.

---

### 6. `internal/user/repository.go`

**Current state:** CRUD methods: GetByID, GetByEmail, GetByKeycloakID, Create, UpdateLastLogin, ListByTenant.

**Changes:**
- Add method `UpdateKeycloakID(ctx, userID, keycloakID string) error` -- `UPDATE users SET keycloak_id = $1, updated_at = NOW() WHERE id = $2`

**Risk:** Low. New method, no existing methods modified.

---

### 7. `internal/user/service.go`

**Current state:** `GetOrCreateByEmail(ctx, email, tenantID)` -- finds or creates user but does NOT set KeycloakID.

**Changes:**
- Add method `GetOrCreateByEmailWithKeycloakID(ctx, email, tenantID, keycloakID) (*User, error)` -- extends GetOrCreateByEmail logic:
  - User exists + no KeycloakID: sets KeycloakID (account linking)
  - User exists + matching KeycloakID: returns user
  - User exists + mismatched KeycloakID: returns error
  - User exists + mismatched TenantID: returns error
  - User does not exist: creates with KeycloakID set

**Existing `GetOrCreateByEmail` is NOT modified.** The new method is a parallel path used only by the SSO flow.

**Risk:** Low. New method alongside existing, no modification to GetOrCreateByEmail.

---

### 8. `internal/auth/keycloak.go`

**Current state:** `KeycloakClient` with methods: Authenticate (direct grant), ValidateToken, Logout, RefreshToken. Helper: getStringClaim.

**Changes:**
- Add `SSOCallbackConfig` field or accept callback URL as parameter
- Add method `BuildSSORedirectURL(idpAlias, callbackURL, state string) string` -- constructs Keycloak authorization URL with `kc_idp_hint`
- Add method `ExchangeCode(ctx, code, callbackURL string) (*gocloak.JWT, error)` -- authorization code grant via gocloak `GetToken`
- Add method `GetAdminToken(ctx) (string, error)` -- obtains admin service account token
- Add method `CreateSAMLIdentityProvider(ctx, SAMLIdPConfig) error` -- creates SAML IdP in Keycloak via Admin REST API
- Add method `UpdateSAMLIdentityProvider(ctx, SAMLIdPConfig) error`
- Add method `DeleteSAMLIdentityProvider(ctx, alias string) error`
- Add struct `SAMLIdPConfig` -- parameters for Keycloak SAML IdP creation

**No existing methods are modified.** All additions are new methods.

**Risk:** Medium. The admin API methods may need direct HTTP calls if gocloak does not fully support SAML IdP configuration fields. Design includes fallback approach.

---

### 9. `internal/auth/service.go`

**Current state:** `Service` with Login, Logout, RefreshToken. Constructor: `NewService(kc, userSvc, tenantSvc)`.

**Changes:**
- Add `ssoCallbackURL string` and `stateSecret string` fields to `Service` struct
- Update `NewService` signature to accept SSO config: `NewService(kc, userSvc, tenantSvc, ssoConfig config.SSOConfig)`
- Add struct `SSOCheckResponse` with fields: SSOEnabled, InitiateURL, IdPName
- Add method `CheckSSO(ctx, email) (*SSOCheckResponse, error)`
- Add method `InitiateSSO(ctx, email) (redirectURL string, stateCookie *http.Cookie, err error)`
- Add method `HandleSSOCallback(ctx, code, state, cookieState string) (*Tokens, error)`
- Add helper `generateCSRFState() string` -- crypto/rand based random state
- Add helper `signState(state string) string` -- HMAC-SHA256 with stateSecret
- Add helper `validateState(state, expected string) bool`

**Breaking change:** `NewService` signature changes. Affects `cmd/server/main.go`.

**Existing Login/Logout/RefreshToken methods are NOT modified.**

**Risk:** Medium. Constructor change requires coordinated update. But login flow is untouched.

---

### 10. `internal/auth/handler.go`

**Current state:** `Handler` with Login, Logout, RefreshToken handlers.

**Changes:**
- Add method `CheckSSO(w, r)` -- `GET /api/auth/sso/check?email=...`
- Add method `InitiateSSO(w, r)` -- `GET /api/auth/sso/initiate?email=...` -- sets state cookie, returns 302
- Add method `SSOCallback(w, r)` -- `GET /api/auth/sso/callback?code=...&state=...` -- validates state cookie, exchanges code, returns tokens
- Add helper `renderSSOCompletePage(w, *Tokens)` -- renders HTML page that posts tokens to frontend

**Existing Login/Logout/RefreshToken handlers are NOT modified.**

**Risk:** Low. All additions, no modifications to existing handlers.

---

### 11. `internal/config/config.go`

**Current state:** `Config` struct with Port, DatabaseURL, Keycloak fields.

**Changes:**
- Add `SSO SSOConfig` field with `env:",prefix=SSO_"` tag
- Add `SSOConfig` struct: CallbackURL, FrontendURL, StateSecret (HMAC key)

**Risk:** Low. Additive config fields. Existing fields unchanged. New env vars: `SSO_CALLBACK_URL`, `SSO_FRONTEND_URL`, `SSO_STATE_SECRET`.

---

### 12. `cmd/server/main.go`

**Current state:** Wires repos -> services -> handlers -> routes. Public routes: login/logout/refresh. Protected routes: users/me, tenants/{id}, admin routes.

**Changes:**
- Create `ssoRepo := tenant.NewSSORepository(db)`
- Update `tenantSvc := tenant.NewService(tenantRepo, ssoRepo)` (was `NewService(tenantRepo)`)
- Update `authSvc := auth.NewService(keycloakClient, userSvc, tenantSvc, cfg.SSO)` (was `NewService(kc, us, ts)`)
- Create `ssoAdminHandler := auth.NewSSOAdminHandler(tenantSvc, keycloakClient)`
- Register SSO public routes: GET /api/auth/sso/check, GET /api/auth/sso/initiate, GET /api/auth/sso/callback
- Register SSO admin routes: GET/PUT/DELETE /api/admin/tenants/{tenantID}/sso

**Risk:** Medium. Wiring changes must be done correctly. But all existing route registrations remain identical.

---

### 13. `docker-compose.yml`

**Current state:** Services: app, postgres, keycloak. Environment vars for app.

**Changes:**
- Add SSO environment variables to `app` service:
  - `SSO_CALLBACK_URL=http://localhost:8080/api/auth/sso/callback`
  - `SSO_FRONTEND_URL=http://localhost:3000`
  - `SSO_STATE_SECRET=dev-secret-change-in-production`

**Risk:** Low. Additive environment variables only.

---

## Implementation Order

The order is designed so that each step compiles and the system remains functional throughout:

### Phase 1: Foundation (no behavioral changes)
1. `migrations/002_sso_config.sql` -- schema first
2. `internal/tenant/sso_config.go` -- data model
3. `internal/tenant/sso_repository.go` -- persistence
4. `internal/tenant/service.go` -- add ssoRepo + SSO methods
5. `internal/config/config.go` -- SSO config struct
6. `internal/user/repository.go` -- add UpdateKeycloakID
7. `internal/user/service.go` -- add GetOrCreateByEmailWithKeycloakID

### Phase 2: Keycloak Integration
8. `internal/auth/keycloak.go` -- add ExchangeCode, BuildSSORedirectURL, Admin API methods

### Phase 3: SSO Flow
9. `internal/auth/service.go` -- add CheckSSO, InitiateSSO, HandleSSOCallback
10. `internal/auth/handler.go` -- add SSO endpoint handlers
11. `internal/auth/sso_handler.go` -- admin SSO config handlers

### Phase 4: Wiring
12. `cmd/server/main.go` -- wire everything, register routes
13. `docker-compose.yml` -- add env vars

---

## Files NOT Changed

These files are explicitly unmodified:

| File | Why Not Changed |
|------|-----------------|
| `internal/auth/middleware.go` | Validates JWTs regardless of issuance method. SSO tokens pass through unchanged. The email-domain fallback for tenant_id already exists (line 62-70). |
| `internal/auth/models.go` | Session struct is unused and remains unused. SSO sessions are Keycloak-managed. |
| `internal/tenant/handler.go` | Tenant CRUD handlers unchanged. SSO config has its own admin handler. |
| `internal/tenant/repository.go` | Existing tenant queries unchanged. SSO config has its own repository. |
| `internal/tenant/models.go` | Tenant struct unchanged. SSOConfig is a separate struct in a separate file. |
| `internal/user/models.go` | User struct already has KeycloakID field. No changes needed. |
| `internal/user/handler.go` | User handlers read from auth context, which works the same for SSO-authenticated users. |
| `pkg/httputil/response.go` | Used by new SSO handlers but not modified. |
| `migrations/001_initial.sql` | Existing schema unchanged. SSO config is a new table. |
| `go.mod` | No new dependencies needed. gocloak already present. If SAML admin API requires direct HTTP calls, `net/http` is stdlib. |

---

## Dependency Graph

```
migrations/002_sso_config.sql
    |
    v
tenant/sso_config.go
    |
    v
tenant/sso_repository.go --------> tenant/service.go (modified)
                                        |
config/config.go (modified) <-----------+
    |                                   |
    v                                   v
auth/keycloak.go (modified)         user/service.go (modified)
    |                               user/repository.go (modified)
    v                                   |
auth/service.go (modified) <------------+
    |
    v
auth/handler.go (modified)
auth/sso_handler.go (new)
    |
    v
cmd/server/main.go (modified)
```

---

## Risk Assessment by File

| File | Risk Level | Reason |
|------|-----------|--------|
| `migrations/002_sso_config.sql` | Low | New table, no existing schema modified |
| `internal/tenant/sso_config.go` | None | New data struct |
| `internal/tenant/sso_repository.go` | None | New file, follows existing pattern |
| `internal/tenant/service.go` | Medium | Constructor signature change |
| `internal/user/repository.go` | Low | New method only |
| `internal/user/service.go` | Low | New method only, existing method untouched |
| `internal/auth/keycloak.go` | Medium | New methods, gocloak API gaps possible |
| `internal/auth/service.go` | Medium | Constructor signature change, core SSO logic |
| `internal/auth/handler.go` | Low | New methods only |
| `internal/auth/sso_handler.go` | Low | New file |
| `internal/config/config.go` | Low | Additive struct fields |
| `cmd/server/main.go` | Medium | Wiring changes, multiple constructor updates |
