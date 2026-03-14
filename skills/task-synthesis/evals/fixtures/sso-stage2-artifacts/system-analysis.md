# Codebase / System Analysis

## Relevant Modules

### Auth Module
- **Path:** `internal/auth/`
- **Purpose:** Handles all authentication concerns -- HTTP handlers for login/logout/refresh, Keycloak client wrapper for token operations, JWT validation middleware, and auth-related models. This is the primary change target for SSO
- **Key files:**
  - [`handler.go` -- HTTP handlers for POST /api/auth/login, /api/auth/logout, /api/auth/refresh. `LoginRequest` struct accepts email+password+optional tenant_id. Returns `TokenResponse` with access/refresh tokens]
  - [`service.go` -- `Service` struct orchestrating auth flow. `Login()` resolves tenant by email domain if tenant_id not provided, verifies tenant is active, authenticates against Keycloak via direct grant, then calls `userSvc.GetOrCreateByEmail()` to ensure local user record exists]
  - [`keycloak.go` -- `KeycloakClient` wrapping gocloak library. `Authenticate()` uses `client.Login()` (direct grant/password). `ValidateToken()` introspects token and extracts claims including custom `tenant_id` claim and `realm_access.roles`. Uses single realm config (field: `realm string`)]
  - [`middleware.go` -- `Middleware` with `Authenticate()` (validates Bearer JWT, extracts user_id/tenant_id/roles into context) and `RequireRole()` (role-based authorization). Context keys: `ContextKeyUserID`, `ContextKeyTenantID`, `ContextKeyRoles`]
  - [`models.go` -- `Session` struct (currently unused -- sessions managed by Keycloak). `TokenClaims` struct in keycloak.go holds Subject, Email, TenantID, RealmRoles]
- **Relevance to task:** This module must be extended to support SAML SSO flow alongside the existing password flow. New endpoints for SSO initiation and callback are needed. The `KeycloakClient` may need methods for Keycloak's SAML broker endpoints. The middleware should work unchanged since it validates JWTs regardless of how they were obtained

### Tenant Module
- **Path:** `internal/tenant/`
- **Purpose:** Manages tenant lifecycle -- CRUD operations, tenant resolution by ID/slug/email domain. The `Tenant` struct is a flat model with no extensible config storage
- **Key files:**
  - [`models.go` -- `Tenant` struct with fields: ID, Name, Slug, EmailDomain, IsActive, Plan, CreatedAt, UpdatedAt. Explicit NOTE comment: "There is currently no per-tenant configuration storage" and suggests "a tenant_settings table or a JSONB column" for things like SSO config]
  - [`repository.go` -- PostgreSQL queries: `GetByID`, `GetBySlug`, `GetByEmailDomain` (splits email on @ and matches domain), `Update` (updates name/slug/email_domain/is_active/plan), `List`]
  - [`service.go` -- Thin service layer delegating to repository. `GetByEmailDomain()` used heavily by auth module for tenant resolution]
  - [`handler.go` -- HTTP handlers: `GetTenant` (GET by ID) and `UpdateTenant` (PUT with partial update for name/email_domain/is_active/plan)]
- **Relevance to task:** Must be extended with SSO configuration storage. The `GetByEmailDomain()` method is the critical tenant detection point during SSO flow. The `UpdateTenant` handler may need to accept SSO configuration fields

### User Module
- **Path:** `internal/user/`
- **Purpose:** User CRUD and provisioning. Key method `GetOrCreateByEmail()` provides the JIT provisioning pattern that SSO will reuse
- **Key files:**
  - [`models.go` -- `User` struct: ID, Email, Name, TenantID, KeycloakID (maps to Keycloak sub claim), Role (user/admin/viewer), IsActive, LastLogin, CreatedAt, UpdatedAt]
  - [`repository.go` -- Queries: `GetByID`, `GetByEmail`, `GetByKeycloakID`, `Create` (generates UUID, sets is_active=true), `UpdateLastLogin`, `ListByTenant`]
  - [`service.go` -- `GetOrCreateByEmail()` finds user by email or creates with default role "user" and Name=email. Does NOT set `KeycloakID` during creation -- this is a gap for SSO]
  - [`handler.go` -- `GetCurrentUser` (reads user_id from auth context), `ListUsers` (reads tenant_id from auth context)]
- **Relevance to task:** SSO JIT provisioning will use or extend `GetOrCreateByEmail()`. The `KeycloakID` field needs to be populated during SSO user creation. The `GetByKeycloakID()` repository method already exists and can support account linking by Keycloak subject

### Config Module
- **Path:** `internal/config/`
- **Purpose:** Environment-based configuration loading using go-envconfig
- **Key files:**
  - [`config.go` -- `Config` struct: Port, DatabaseURL, `KeycloakConfig` (BaseURL, Realm, ClientID, ClientSecret). `NewDB()` creates pgxpool connection]
- **Relevance to task:** May need additional SAML/SSO-related configuration (e.g., SAML SP entity ID, ACS URL base). The `KeycloakConfig` currently assumes a single realm -- this is relevant because SAML IdP brokering configuration happens at the realm level in Keycloak

### Server Entry Point
- **Path:** `cmd/server/main.go`
- **Purpose:** Application bootstrap -- creates repos, services, middleware, and configures chi router with all routes
- **Key files:**
  - [`main.go` -- Wiring: tenantRepo/userRepo -> keycloakClient/tenantSvc/userSvc -> authSvc -> authMiddleware. Routes: public group (login/logout/refresh) and protected group (users/me, tenants/{id}, admin routes with RequireRole)]
- **Relevance to task:** New SSO routes must be registered here (SSO initiation endpoint, SAML callback endpoint). These would likely be public routes (unauthenticated users initiate SSO)

### Database Migrations
- **Path:** `migrations/`
- **Purpose:** PostgreSQL schema definitions
- **Key files:**
  - [`001_initial.sql` -- Creates `tenants` table (id, name, slug, email_domain UNIQUE, is_active, plan) and `users` table (id, email UNIQUE, name, tenant_id FK, keycloak_id, role, is_active, last_login). NOTE comment: "No tenant_settings or tenant_config table exists yet"]
- **Relevance to task:** New migration needed for SSO configuration storage -- either new columns on tenants table or a new `tenant_sso_config` table

### HTTP Utilities
- **Path:** `pkg/httputil/`
- **Purpose:** Shared HTTP response helpers
- **Key files:**
  - [`response.go` -- `WriteJSON()` and `WriteError()` with `ErrorResponse` struct (error, code, details)]
- **Relevance to task:** SSO error responses should use these utilities for consistency

## Change Points

| Location | What Changes | Scope | Confidence |
|----------|-------------|-------|------------|
| `internal/auth/handler.go` | New SSO endpoints needed: (1) SSO initiation -- accepts email, detects SSO tenant, returns redirect URL to Keycloak SAML broker. (2) SSO callback -- receives authorization code from Keycloak after SAML assertion, exchanges for JWT, provisions user | large | high |
| `internal/auth/service.go:Login` | Login method needs a branch: if tenant has SSO enabled, reject direct password login and return error directing to SSO flow. New method needed for SSO callback processing (exchange code for tokens, extract user info, JIT provision) | medium | high |
| `internal/auth/keycloak.go` | New methods needed: (1) Generate Keycloak SAML broker redirect URL for a specific IdP alias. (2) Exchange authorization code for tokens (authorization code flow, not direct grant). May need gocloak's `GetToken` with code grant or direct HTTP calls to Keycloak's token endpoint | medium | high |
| `internal/tenant/models.go` | Tenant struct needs SSO configuration fields or a reference to SSO config. Options: add SSOEnabled bool + SSOConfig pointer to Tenant struct, or create separate SSOConfig struct loaded separately | medium | high |
| `internal/tenant/repository.go` | New queries for SSO config: load SSO settings by tenant ID, update SSO settings. If using a separate table, needs new repository methods | medium | high |
| `internal/tenant/service.go` | New methods: `GetSSOConfig(tenantID)`, `UpdateSSOConfig(tenantID, config)`, `IsSSOEnabled(tenantID)` | small | high |
| `internal/user/service.go:GetOrCreateByEmail` | Needs to accept and store `KeycloakID` during user creation for SSO-provisioned users. Currently creates users without KeycloakID | small | high |
| `migrations/002_sso_config.sql` | New migration: either add columns to tenants table (sso_enabled, sso_provider_alias, sso_metadata_url, etc.) or create new `tenant_sso_config` table with FK to tenants | medium | high |
| `cmd/server/main.go` | Register new SSO routes in the public route group (SSO init + callback endpoints) | small | high |
| `internal/config/config.go` | Possibly add SAML-related base config (SP entity ID, ACS URL base path) to `Config` struct | small | medium |
| `internal/auth/middleware.go:Authenticate` | Should work unchanged -- validates JWT from Keycloak regardless of how it was obtained. No changes expected, but needs verification testing | none | high |

## Dependencies

### Upstream (what affected code depends on)
- **gocloak v13.9.0:** The Keycloak client library. Currently used for `Login()` (direct grant), `RetrospectToken()`, `DecodeAccessToken()`, `Logout()`, `RefreshToken()`. For SSO, will need authorization code exchange capabilities. gocloak does provide `GetToken()` which can accept authorization code grant parameters. It also provides admin API methods for managing identity providers (`CreateIdentityProvider`, `GetIdentityProvider`, etc.) which would be needed for programmatic SAML IdP setup
- **chi/v5 v5.0.12:** HTTP router. SSO adds new routes but doesn't change how routing works. No constraint
- **pgx/v5 v5.5.5:** PostgreSQL driver with connection pooling. New queries for SSO config will use existing pool. No constraint
- **golang-jwt/v5 v5.2.1:** JWT parsing library. Used implicitly by gocloak. SSO tokens are still JWTs. No constraint
- **go-envconfig v1.0.0:** Config loading. Minor extension for any new env vars. No constraint
- **zerolog v1.32.0:** Structured logging. SSO flow should log key events for debugging. No constraint

### Downstream (what depends on affected code)
- **Auth middleware consumers:** All protected routes depend on `auth.Middleware.Authenticate()`. Since the middleware validates JWTs (not how they were obtained), SSO should not affect downstream consumers -- as long as SSO-issued JWTs contain the same claims (sub, email, tenant_id, realm_access.roles)
- **User handler:** `user.Handler.GetCurrentUser()` reads `auth.ContextKeyUserID` from context. Will work for SSO users if their user records exist and have correct IDs
- **Tenant handler:** `tenant.Handler.GetTenant()` and `UpdateTenant()` are consumed by admin routes. If `UpdateTenant` is extended for SSO config, the admin route consumers are affected
- **Auth service consumers:** `auth.Handler` is the only direct consumer of `auth.Service`. Adding SSO methods to the service doesn't break existing handler methods

### External
- **Keycloak (v24.0 per docker-compose):** Central authentication server. For SAML SSO, Keycloak must be configured with: (1) a SAML Identity Provider per tenant, (2) appropriate client settings for authorization code flow (the current client may be configured for direct grant only), (3) attribute mappers for SAML-to-JWT claim mapping. The Keycloak Admin REST API allows programmatic management of IdP configurations
- **PostgreSQL (v16):** Schema changes required for SSO config storage. Standard migration approach. No special concerns
- **Enterprise IdPs (external):** Each enterprise customer runs their own SAML IdP (Okta, Azure AD, ADFS, etc.). The platform depends on IdP availability for SSO login. Metadata exchange (IdP metadata XML -> Keycloak, SP metadata -> IdP) happens during configuration, not at runtime

### Implicit
- **Keycloak client configuration:** The current Keycloak client (`platform-app` per docker-compose) is likely configured for direct access grants only. SAML brokered login uses the authorization code flow, which requires the client to have "Standard Flow" enabled in Keycloak. This is a Keycloak admin configuration change, not a code change, but it's a hidden dependency that will cause SSO to fail if not addressed
- **Keycloak realm as IdP namespace:** All SAML IdP broker configurations in Keycloak are scoped to a realm. With a single `platform` realm, all tenant IdPs are configured as identity providers within that realm. Each IdP needs a unique alias (e.g., `saml-{tenant_slug}`). This works but means all tenant IdP configs share the same Keycloak realm namespace
- **SAML ACS URL:** Keycloak generates an Assertion Consumer Service URL based on the realm and IdP alias. The enterprise IdP must be configured to send SAML assertions to this URL. This URL is a function of Keycloak's base URL, realm name, and IdP alias -- it is deterministic but must be communicated during configuration
- **Token claim consistency:** The auth middleware expects `tenant_id` as a custom claim in the JWT. For SSO users, this claim must be populated by Keycloak via mapper configuration (mapping from the brokered IdP session to the JWT). If the Keycloak client doesn't have a `tenant_id` mapper configured for brokered logins, the middleware's tenant resolution will fall back to email domain lookup, which works but is less efficient

## Existing Patterns

- **Repository pattern:** All data access follows repo -> service -> handler layering. Example: `tenant.Repository.GetByID()` -> `tenant.Service.GetByID()` -> `tenant.Handler.GetTenant()`. SSO config storage should follow this same pattern with a repository for SSO config data
- **Tenant resolution by email domain:** `tenant.Repository.GetByEmailDomain()` splits email on `@` and queries by domain. Used in `auth.Service.Login()` and `auth.Middleware.Authenticate()`. This is the natural detection point for SSO-enabled tenants -- after resolving the tenant, check SSO config. Example at: `internal/tenant/repository.go:43-58`
- **JIT user provisioning via GetOrCreateByEmail:** `user.Service.GetOrCreateByEmail()` finds or creates a user record during login. Currently sets Name=email and Role="user" for new users, does NOT set KeycloakID. This pattern is directly reusable for SSO JIT provisioning but needs enhancement to accept KeycloakID. Example at: `internal/user/service.go:26-47`
- **Environment-based configuration:** All config loaded from environment variables via `go-envconfig` with prefix nesting (e.g., `KEYCLOAK_` prefix). Any new SSO base config should follow this pattern. Example at: `internal/config/config.go:11-22`
- **Context-based auth propagation:** Auth middleware injects user_id, tenant_id, and roles into request context via `context.WithValue()`. Downstream handlers extract via typed context keys. SSO must produce the same context values. Example at: `internal/auth/middleware.go:80-83`
- **Partial update pattern:** `tenant.Handler.UpdateTenant()` uses pointer fields (`*string`, `*bool`) for optional updates. If SSO config is managed through tenant update API, it should follow this pattern. Example at: `internal/tenant/handler.go:40-45`
- **Error handling:** Mix of `http.Error()` direct calls in handlers and `fmt.Errorf` wrapping in services. The `pkg/httputil` package provides `WriteError()` but it's not consistently used. SSO handlers should prefer `httputil.WriteError()` for consistency

## Technical Observations

- **No test files exist anywhere in the codebase:** Zero test coverage. No unit tests, no integration tests, no test fixtures. This means any changes to the auth flow carry high regression risk with no safety net. There is no established testing pattern to follow
- **Variable shadowing bug in RequireRole:** In `internal/auth/middleware.go:99-104`, the `RequireRole` closure uses `r` for both the role string (loop variable) and the `http.Request` parameter. Line 101 `r.WithContext(r.Context())` references the string `r` (role), not the request. This is a compilation error in the current code. While tangential to SSO, it indicates that this code path may not be exercised or compiled in production
- **Sessions not persisted server-side:** `auth.models.go` defines a `Session` struct but it's unused. Sessions are entirely Keycloak-managed. This means there's no server-side session store to extend for SSO sessions -- which is actually simpler, since SSO sessions will also be Keycloak-managed
- **Single Keycloak realm assumption:** `KeycloakClient` stores a single `realm` string. All operations use this one realm. SAML IdP configurations in Keycloak are per-realm, so all tenant IdPs will be siblings within the same realm. This works for multi-tenant SAML but means tenant isolation at the Keycloak level is not realm-based
- **Keycloak client uses direct grant only:** The `Authenticate()` method in `keycloak.go:42` uses `client.Login()` which is the Resource Owner Password Credentials (direct grant) flow. SAML SSO requires the Authorization Code flow, which is fundamentally different -- the user is redirected to Keycloak, not submitting credentials to the app. This is the most significant architectural shift in the auth flow
- **Tenant model has no extensible config:** The explicit NOTE in `internal/tenant/models.go:18-23` and `migrations/001_initial.sql:34-37` acknowledge the lack of per-tenant configuration storage and suggest either a `tenant_settings` table or JSONB column. This is a known gap that must be addressed for SSO
- **Email uniqueness constraint:** The users table enforces `email UNIQUE` globally (not per-tenant). This means a user email can only belong to one tenant. For SSO this is fine as long as email domains map 1:1 to tenants, but it could be a constraint if an organization uses multiple email domains or if the same person needs access to multiple tenants
- **httputil package underused:** `pkg/httputil/response.go` provides `WriteJSON()` and `WriteError()` but most handlers use `http.Error()` directly. This is cosmetic but new SSO handlers should prefer the httputil package

## Test Coverage

| Area | Test Type | Coverage Level | Key Test Files | Notes |
|------|-----------|---------------|----------------|-------|
| internal/auth/ | unit/integration/e2e | none | (none found) | No test files exist. The auth middleware, handler, service, and keycloak client are all untested |
| internal/tenant/ | unit/integration/e2e | none | (none found) | No test files exist. Repository queries, service logic, and handlers are untested |
| internal/user/ | unit/integration/e2e | none | (none found) | No test files exist. GetOrCreateByEmail logic is untested |
| internal/config/ | unit | none | (none found) | Config loading is untested |
| cmd/server/ | e2e | none | (none found) | No integration or smoke tests for the full application |

## Critique Review

The critic found this analysis SUFFICIENT across all six criteria. Module specificity scored PASS -- all modules were explored with file paths and actual code details (structs, methods, SQL schemas). Change points are concrete and pinpointed to specific functions with scope estimates. Dependencies are thoroughly mapped across all four categories (upstream, downstream, external, implicit), with particular strength in identifying the implicit Keycloak client configuration dependency. Existing patterns were identified from actual code with specific file references. Test coverage was checked and the complete absence of tests is clearly documented. All claims are verified from actual code reads.

Minor observation from the critic: The RequireRole bug discovery (variable shadowing in middleware.go:99-104) is a valuable finding but tangential to the SSO task. It could be noted as a pre-existing issue to fix opportunistically.

## Open Questions

- Does gocloak v13.9.0 provide methods for managing SAML Identity Providers via the Keycloak Admin REST API, or will direct HTTP calls to the admin API be needed for programmatic IdP configuration?
- Is the current Keycloak client (`platform-app`) configured for Authorization Code flow (Standard Flow) in addition to Direct Grant, or does it need reconfiguration?
- Should SSO configuration storage use a new `tenant_sso_config` table (normalized, cleaner separation) or a JSONB column on the existing tenants table (simpler migration, less schema change)? The codebase NOTE suggests both options
- How should the authorization code callback URL be structured? It needs to be a public endpoint that Keycloak redirects to after SAML brokering. Typical pattern: `/api/auth/sso/callback`
- The `email UNIQUE` constraint on the users table means one email can only belong to one tenant. Is this acceptable for all SSO use cases, or could there be scenarios where the same email needs multi-tenant access?
- Should there be a Keycloak admin service/client separate from the auth KeycloakClient for managing SAML IdP configurations (separation of authentication operations from admin operations)?
