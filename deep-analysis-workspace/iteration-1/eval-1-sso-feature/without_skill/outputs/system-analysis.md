# System Analysis: SAML SSO for Enterprise Tenants

## Codebase Overview

The project is a Go 1.22 backend application (`github.com/acme/platform`) with the following structure:

```
cmd/server/main.go          -- Entry point, wiring, HTTP server
internal/auth/              -- Authentication: Keycloak client, middleware, handlers
internal/tenant/            -- Tenant model, repository, service, handler
internal/user/              -- User model, repository, service, handler
internal/config/            -- Configuration loading (env vars)
pkg/httputil/               -- HTTP response helpers
migrations/001_initial.sql  -- Database schema
docker-compose.yml          -- App + Postgres + Keycloak (dev environment)
```

**Key dependencies** (from `go.mod`):
- `github.com/Nerzal/gocloak/v13` -- Keycloak admin/auth client
- `github.com/go-chi/chi/v5` -- HTTP router
- `github.com/golang-jwt/jwt/v5` -- JWT parsing (declared but not directly used in source; Keycloak client handles JWT internally)
- `github.com/jackc/pgx/v5` -- PostgreSQL driver
- `github.com/rs/zerolog` -- Structured logging
- `github.com/sethvargo/go-envconfig` -- Environment-based config

## Relevant Modules and Change Points

### 1. `internal/auth/` -- Primary Change Target

#### `internal/auth/handler.go`
**Current state**: Defines `LoginRequest` (email + password + optional tenant_id) and three HTTP handlers: `Login`, `Logout`, `RefreshToken`. All routes are public (no auth middleware).

**Change points**:
- **New endpoint needed**: An SSO initiation endpoint (e.g., `GET /api/auth/sso/login?email=...` or `POST /api/auth/sso/login`) that resolves the tenant from email, checks if SSO is enabled, and returns a redirect URL to Keycloak's SAML broker flow.
- **New endpoint needed**: An SSO callback endpoint (e.g., `GET /api/auth/sso/callback`) that receives the authorization code from Keycloak after SAML authentication, exchanges it for tokens, and returns them to the client.
- **Existing `Login` handler**: Must be modified to check if the resolved tenant has SSO enabled. If SSO is enabled and enforced, direct password login should be rejected with an appropriate error (e.g., 403 with message "SSO authentication required for this organization").
- **New endpoint (optional)**: A tenant SSO check endpoint (e.g., `GET /api/auth/sso/check?email=...`) for frontend to determine whether to show password form or SSO redirect button.

#### `internal/auth/service.go`
**Current state**: `Service` struct holds `keycloak *KeycloakClient`, `userSvc *user.Service`, `tenantSvc *tenant.Service`. The `Login` method does: resolve tenant -> verify active -> authenticate via Keycloak direct grant -> ensure local user exists -> return tokens.

**Change points**:
- **New method**: `InitiateSSO(ctx, email) (redirectURL, error)` -- resolve tenant, verify SSO is enabled, build Keycloak SAML broker redirect URL.
- **New method**: `HandleSSOCallback(ctx, code, state) (*Tokens, error)` -- exchange Keycloak authorization code for tokens after SAML callback, perform JIT user provisioning.
- **Modify `Login`**: Add SSO-enabled check after tenant resolution (between lines 46-52 in current code). If `tenant.SSOEnabled`, return error before reaching `keycloak.Authenticate`.

#### `internal/auth/keycloak.go`
**Current state**: `KeycloakClient` wraps gocloak with methods: `Authenticate` (direct grant), `ValidateToken` (introspect + decode), `Logout`, `RefreshToken`. Uses a single realm (`cfg.Realm`), single client (`cfg.ClientID`), single secret (`cfg.ClientSecret`).

**Change points**:
- **New method**: Build SAML IdP broker redirect URL. Keycloak's SAML brokering works via the standard OIDC authorization endpoint with a `kc_idp_hint` parameter that tells Keycloak which IdP to redirect to. The URL format is: `{keycloak_base_url}/realms/{realm}/protocol/openid-connect/auth?client_id={client_id}&redirect_uri={callback_url}&response_type=code&scope=openid&kc_idp_hint={idp_alias}`. The `idp_alias` would come from the tenant's SSO configuration.
- **New method**: Exchange authorization code for tokens. The gocloak library does not have a direct method for authorization code exchange, but it can be done via `GetToken` with grant_type `authorization_code`. Alternatively, a direct HTTP call to Keycloak's token endpoint may be needed.
- **Existing `ValidateToken`**: Should work unchanged for SSO-brokered tokens, since Keycloak issues standard JWTs regardless of authentication method. However, the custom `tenant_id` claim extraction (line 74) depends on a Keycloak client mapper being configured to inject `tenant_id` into tokens. For SSO-brokered users, this mapper must also be configured for the brokered flow, or `tenant_id` will be absent and the middleware will fall back to email-domain resolution (line 64 of `middleware.go`).
- **Config expansion**: The `KeycloakClient` currently has no callback URL or frontend URL stored. The SSO flow requires knowing the redirect URI for Keycloak callbacks.

#### `internal/auth/middleware.go`
**Current state**: `Authenticate` middleware extracts Bearer token, validates via Keycloak, resolves tenant (from token claim or email domain fallback), verifies tenant is active, sets context values (user_id, tenant_id, roles). `RequireRole` checks realm roles.

**Change points**:
- **Likely no changes needed**. The middleware processes JWTs, and SSO-brokered tokens are standard Keycloak JWTs. The tenant resolution fallback (email domain) covers the case where `tenant_id` claim is missing from SSO tokens. The existing flow handles both scenarios.
- **Minor note**: Line 99-103 has a variable shadowing bug -- `r` is used both as the range variable and the `http.Request`, causing `r.WithContext(r.Context())` to fail compilation. This is in `RequireRole` and is a pre-existing bug unrelated to SSO, but it would surface during testing.

#### `internal/auth/models.go`
**Current state**: Only defines `Session` struct (not persisted, exists for future use).

**Change points**:
- **New model**: SSO configuration struct, e.g., `SSOConfig` with fields like `Enabled bool`, `IdPAlias string` (Keycloak IdP alias), `IdPEntityID string`, `IdPMetadataURL string`, `EnforceSSO bool` (whether to block password login).
- **Alternatively**, SSO config may live in `internal/tenant/models.go` since it is tenant-scoped configuration.

### 2. `internal/tenant/` -- SSO Configuration Storage

#### `internal/tenant/models.go`
**Current state**: `Tenant` struct has fields: `ID`, `Name`, `Slug`, `EmailDomain`, `IsActive`, `Plan`, `CreatedAt`, `UpdatedAt`. The file contains a NOTE comment explicitly acknowledging the lack of per-tenant configuration storage and suggesting a `tenant_settings` table or JSONB column.

**Change points -- two options**:

**Option A: Extend Tenant struct with SSO fields**
Add `SSOEnabled bool`, `SSOProvider string`, `SSOIdPAlias string`, `SSOEnforced bool` directly to the `Tenant` struct. Simple but mixes concerns and adds nullable columns to the tenants table.

**Option B: New `tenant_sso_config` table and model (recommended)**
Create a separate `SSOConfig` struct and table. This keeps the tenant model clean, supports future extensibility (e.g., multiple IdPs, OIDC in the future), and follows the guidance in the existing NOTE comment. Schema:
```sql
CREATE TABLE tenant_sso_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL UNIQUE REFERENCES tenants(id),
    enabled         BOOLEAN NOT NULL DEFAULT false,
    protocol        TEXT NOT NULL DEFAULT 'saml',
    idp_alias       TEXT NOT NULL,       -- Keycloak IdP alias
    idp_entity_id   TEXT,
    idp_metadata_url TEXT,
    enforce_sso     BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `internal/tenant/repository.go`
**Current state**: Repository methods use `pgxpool.Pool` directly with raw SQL queries. Methods: `GetByID`, `GetBySlug`, `GetByEmailDomain`, `Update`, `List`. All SELECT queries explicitly list columns.

**Change points**:
- If Option A: Modify all SELECT queries to include new SSO columns, modify `Update` to include new columns.
- If Option B: Add new repository (either in `internal/tenant/sso_repository.go` or `internal/sso/repository.go`) with methods: `GetSSOConfigByTenantID`, `CreateSSOConfig`, `UpdateSSOConfig`, `DeleteSSOConfig`.

#### `internal/tenant/service.go`
**Current state**: Thin service layer, mostly delegates to repository.

**Change points**:
- New method: `GetSSOConfig(ctx, tenantID) (*SSOConfig, error)` -- needed by auth service to determine SSO status during login.
- New method: `IsSSOEnabled(ctx, tenantID) (bool, error)` -- convenience method for the auth handler/service.
- If Option B is chosen, the tenant service could delegate to an SSO config repository, or a separate SSO config service could be created.

#### `internal/tenant/handler.go`
**Current state**: `GetTenant` and `UpdateTenant` handlers. `UpdateTenant` accepts partial updates for `name`, `email_domain`, `is_active`, `plan`.

**Change points**:
- **New handler or extended handler**: Admin endpoint for configuring tenant SSO (e.g., `PUT /api/admin/tenants/{tenantID}/sso`). This would accept SSO configuration parameters and create/update the SSO config.
- **Extend `GetTenant` response**: Include SSO enablement status in tenant response so that frontend can detect SSO tenants.

### 3. `internal/user/` -- JIT Provisioning Adjustments

#### `internal/user/service.go`
**Current state**: `GetOrCreateByEmail` handles user creation on first login. Creates user with `Role: "user"`, `IsActive: true`. Does NOT set `KeycloakID` on creation (the field is blank for newly created users).

**Change points**:
- **Modify `GetOrCreateByEmail`** (or create a new SSO-specific variant): Accept `keycloakID` parameter so that SSO-provisioned users get their `keycloak_id` set immediately. Currently, `KeycloakID` is never populated during user creation. For SSO, the Keycloak `sub` claim from the brokered token should be stored.
- **Account linking logic**: When an existing user (created via password login) logs in via SSO for the first time, the `keycloak_id` may differ. The `GetOrCreateByEmail` method finds by email, so it will find the existing user. But it should update the `keycloak_id` if it has changed (to reflect the brokered identity).

#### `internal/user/repository.go`
**Current state**: `Create` method uses `gen_random_uuid()` for ID, inserts email/name/tenant_id/keycloak_id/role. The `GetByKeycloakID` method exists but is unused in the current codebase.

**Change points**:
- **New method**: `UpdateKeycloakID(ctx, userID, keycloakID)` -- for account linking when SSO is first enabled.
- `GetByKeycloakID` will become relevant for SSO flows where the token `sub` is the primary user identifier.

#### `internal/user/models.go`
**Current state**: `User` struct with `KeycloakID string`. No `AuthMethod` or `SSOLinked` field.

**Change points**:
- Consider adding `AuthMethod string` ("password", "sso") to track how the user was created/last authenticated. This is useful for auditing and for the account linking flow.

### 4. `internal/config/config.go` -- New Configuration

**Current state**: `Config` struct with `Port`, `DatabaseURL`, `Keycloak` (nested `KeycloakConfig` with `BaseURL`, `Realm`, `ClientID`, `ClientSecret`).

**Change points**:
- Add SSO callback URL to config: `SSOCallbackURL string` (the URL that Keycloak redirects to after SAML authentication).
- Add frontend URL to config: `FrontendURL string` (for constructing redirect URLs back to the frontend after SSO).
- These could be added to `KeycloakConfig` or as top-level fields on `Config`.

### 5. `cmd/server/main.go` -- Wiring and Routes

**Current state**: Wires repositories -> services -> handlers -> routes. Public routes: `/api/auth/login`, `/api/auth/logout`, `/api/auth/refresh`. Protected routes: `/api/users/me`, `/api/tenants/{tenantID}`. Admin routes: `/api/admin/users`, `/api/admin/tenants/{tenantID}`.

**Change points**:
- **New public routes**:
  - `GET /api/auth/sso/check?email=...` -- Check if email domain has SSO enabled (for frontend)
  - `GET /api/auth/sso/login?email=...` -- Initiate SSO flow (redirect to Keycloak)
  - `GET /api/auth/sso/callback` -- Handle Keycloak callback after SAML
- **New admin routes** (protected + admin role):
  - `PUT /api/admin/tenants/{tenantID}/sso` -- Configure SSO for a tenant
  - `GET /api/admin/tenants/{tenantID}/sso` -- Get SSO configuration
  - `DELETE /api/admin/tenants/{tenantID}/sso` -- Disable SSO for a tenant
- **Wiring**: If SSO config repository/service are added, they need to be instantiated and injected. The `auth.NewService` constructor will need the SSO config service as an additional dependency.

### 6. `migrations/001_initial.sql` -- Schema Changes

**Current state**: Two tables: `tenants` (id, name, slug, email_domain, is_active, plan, timestamps) and `users` (id, email, name, tenant_id, keycloak_id, role, is_active, last_login, timestamps). NOTE comment acknowledges missing per-tenant config storage.

**Change points**:
- **New migration** (`002_add_sso_config.sql`): Create `tenant_sso_configs` table (see Option B above).
- **Optional**: Add `auth_method` column to `users` table.

### 7. `docker-compose.yml` -- Infrastructure

**Current state**: Three services: `app`, `postgres`, `keycloak`. Keycloak runs in dev mode (`start-dev`) on port 8180 (mapped from container 8080). Single realm `platform`, single client `platform-app`.

**Change points**:
- No structural changes needed to docker-compose.
- Keycloak will need realm-level configuration (SAML IdP brokered identity providers) but this is done through Keycloak's admin UI/API, not through docker-compose.
- Optionally: Add a test SAML IdP container (e.g., `kristophjunge/test-saml-idp` or a SimpleSAMLphp instance) for local development and testing.

## Existing Patterns to Follow

1. **Repository pattern**: Raw SQL with `pgxpool`, explicit column listing in queries, methods return `(*Model, error)`. No ORM.
2. **Service pattern**: Thin services that delegate to repositories and compose business logic. Services take dependencies as constructor parameters.
3. **Handler pattern**: HTTP handlers are methods on `Handler` structs, which take a `*Service` as constructor parameter. JSON request/response encoding.
4. **Dependency injection**: All dependencies are wired in `main.go` via constructors. No dependency injection framework.
5. **Configuration**: Environment variables via `go-envconfig` with prefix nesting (e.g., `KEYCLOAK_BASE_URL`).
6. **Error handling**: Errors are wrapped with `fmt.Errorf("context: %w", err)` and logged at handler level.
7. **Router groups**: Chi router with grouped routes for public/protected/admin separation.
8. **Middleware chain**: Global middleware (Logger, Recoverer, RequestID) -> per-group middleware (Authenticate, RequireRole).

## Technical Observations

### Keycloak as SAML Broker Architecture
The critical architectural decision (already made in requirements) is that Keycloak acts as the SAML SP. The Go backend never touches SAML XML. Instead:
1. Backend redirects user to Keycloak with `kc_idp_hint` parameter
2. Keycloak handles SAML AuthnRequest/Response with the corporate IdP
3. Keycloak issues standard OIDC/JWT tokens to the Go backend
4. Backend receives and validates JWTs exactly as it does for password login

This means the Go backend needs to implement an OAuth2 authorization code flow with Keycloak (not SAML protocol). The current codebase only uses Keycloak's direct grant (password) flow. Adding authorization code flow is the primary new capability needed.

### Single Realm Architecture
The config shows `KEYCLOAK_REALM=platform` -- a single realm. SAML IdPs configured in Keycloak will be realm-level identity providers. Each tenant's IdP gets registered as a separate identity provider in the `platform` realm with a unique alias (e.g., `saml-acmecorp`, `saml-globex`). The `kc_idp_hint` parameter in the authorization URL directs Keycloak to the correct IdP.

### Token Validation Compatibility
The `ValidateToken` method in `keycloak.go` uses `RetrospectToken` (introspection) and `DecodeAccessToken`. Both work identically for SSO-brokered tokens since Keycloak is the token issuer in both flows. The `sub` claim will be the Keycloak user ID (which may differ from the IdP user ID). The `email` claim will be mapped from the SAML assertion via Keycloak attribute mappers.

### Pre-existing Bug in middleware.go
In `RequireRole` (lines 99-103), the range variable `r` shadows the `http.Request` parameter `r`, causing `r.WithContext(r.Context())` to reference the string `r` (role) instead of the request. This would cause a compilation error. This is unrelated to SSO but will surface during development/testing.

### Missing Keycloak ID Population
The `user.Service.GetOrCreateByEmail` method creates users without setting `KeycloakID`. For password login, this means users in the database lack the Keycloak mapping. For SSO, the `keycloak_id` becomes more important because the token `sub` is the primary identifier. This gap should be addressed alongside the SSO work.

### gocloak Library Capabilities
The `gocloak/v13` library provides Keycloak admin API methods including:
- `CreateIdentityProvider` -- can programmatically register SAML IdPs
- `GetIdentityProviders` -- list configured IdPs
- `CreateIdentityProviderMapper` -- configure attribute mappings
- `GetToken` -- supports authorization_code grant type
These could be used to automate SSO configuration rather than requiring manual Keycloak admin UI work.
