# Codebase / System Analysis

## Relevant Modules

### Auth Package
- **Path:** `internal/auth/`
- **Purpose:** Handles all authentication flows -- login, logout, token refresh, JWT validation, and Keycloak integration. This is the primary change target for SSO.
- **Key files:**
  - `handler.go` -- HTTP handlers for `/api/auth/login`, `/api/auth/logout`, `/api/auth/refresh`. The `Login` handler currently accepts `LoginRequest{Email, Password, TenantID}` and only supports direct email/password flow.
  - `service.go` -- `Service` struct orchestrating auth flows. `Login()` method resolves tenant by email domain, verifies tenant is active, authenticates against Keycloak via direct grant, then calls `userSvc.GetOrCreateByEmail()` for user sync. `Logout()` and `RefreshToken()` delegate to Keycloak.
  - `keycloak.go` -- `KeycloakClient` struct wrapping gocloak library. `Authenticate()` uses `client.Login()` (direct grant / password flow only). `ValidateToken()` uses introspection + decode to extract claims including custom `tenant_id` claim and realm roles. `Logout()` and `RefreshToken()` delegate to gocloak.
  - `middleware.go` -- `Middleware` struct with `Authenticate()` (validates Bearer token via Keycloak introspection, resolves tenant from claims or email domain fallback, verifies tenant is active, injects user_id/tenant_id/roles into context) and `RequireRole()` (role-based access control).
  - `models.go` -- `Session` struct (currently unused, exists for future session tracking).
- **Relevance to task:** The SSO login flow needs to be added alongside the existing password flow. The `Login` handler, `Service.Login()`, and `KeycloakClient` all need extension. The `Middleware.Authenticate()` should work unchanged for SSO users because Keycloak will issue standard JWTs regardless of how the user originally authenticated.

### Tenant Package
- **Path:** `internal/tenant/`
- **Purpose:** Tenant model, CRUD operations, and tenant resolution by email domain.
- **Key files:**
  - `models.go` -- `Tenant` struct with fields: `ID, Name, Slug, EmailDomain, IsActive, Plan, CreatedAt, UpdatedAt`. Contains an explicit NOTE comment stating there is no per-tenant configuration storage and that SSO config would need either new columns or a new table.
  - `repository.go` -- `Repository` struct with `GetByID()`, `GetBySlug()`, `GetByEmailDomain()` (splits email, queries by domain), `Update()`, `List()`. All queries use pgx/pgxpool directly with hardcoded SELECT column lists.
  - `service.go` -- `Service` struct as thin wrapper over repository. `GetByEmailDomain()` is used during login for tenant resolution.
  - `handler.go` -- HTTP handlers: `GetTenant()` and `UpdateTenant()`. UpdateTenant supports partial updates via pointer fields for name, email_domain, is_active, plan.
- **Relevance to task:** The Tenant model must be extended to store SSO configuration (enabled flag, IdP metadata, Keycloak IdP alias, etc.). The `GetByEmailDomain()` method is critical for the SSO detection flow -- it's how the system determines which tenant (and therefore which SSO config) applies to a given email.

### User Package
- **Path:** `internal/user/`
- **Purpose:** User model, CRUD, and the JIT provisioning mechanism.
- **Key files:**
  - `models.go` -- `User` struct with fields: `ID, Email, Name, TenantID, KeycloakID, Role, IsActive, LastLogin, CreatedAt, UpdatedAt`.
  - `repository.go` -- `Repository` with `GetByID()`, `GetByEmail()`, `GetByKeycloakID()`, `Create()`, `UpdateLastLogin()`, `ListByTenant()`.
  - `service.go` -- `Service` with `GetOrCreateByEmail()` -- this is the JIT provisioning function. On login, if user doesn't exist, creates with default "user" role. Currently does NOT set `KeycloakID` during creation (the field is left empty in `Create()`).
  - `handler.go` -- HTTP handlers: `GetCurrentUser()` reads user_id from context (set by auth middleware), `ListUsers()` lists by tenant_id from context.
- **Relevance to task:** `GetOrCreateByEmail()` is the JIT provisioning mechanism and will be used for SSO users too. `GetByKeycloakID()` exists but is currently unused -- it becomes important for SSO account linking. The `KeycloakID` field mapping is critical: SSO users authenticated via Keycloak's SAML brokering will have a Keycloak user ID (sub claim) that needs to be stored here.

### Config Package
- **Path:** `internal/config/`
- **Purpose:** Application configuration loading from environment variables, database connection pool creation.
- **Key files:**
  - `config.go` -- `Config` struct loaded via envconfig: `Port`, `DatabaseURL`, `Keycloak` (nested `KeycloakConfig` with `BaseURL`, `Realm`, `ClientID`, `ClientSecret`). Single realm configured.
- **Relevance to task:** Confirms the application uses a single Keycloak realm ("platform" by default). SSO identity provider brokering will be configured within this realm. No additional config fields needed for SSO at the application level if Keycloak handles SAML protocol -- but the `KeycloakConfig` may need extension if the application needs to call Keycloak Admin API to manage identity providers programmatically.

### HTTP Utilities Package
- **Path:** `pkg/httputil/`
- **Purpose:** Shared HTTP response helpers.
- **Key files:**
  - `response.go` -- `WriteJSON()` and `WriteError()` helper functions with `ErrorResponse` struct.
- **Relevance to task:** Minor. New SSO-related handlers should use these utilities for consistent error responses. Currently, the auth handlers use `http.Error()` directly instead of `httputil.WriteError()` -- an inconsistency in the codebase.

### Database Migrations
- **Path:** `migrations/`
- **Purpose:** SQL schema definitions.
- **Key files:**
  - `001_initial.sql` -- Creates `tenants` table (id, name, slug, email_domain, is_active, plan, timestamps + indexes on slug and email_domain) and `users` table (id, email, name, tenant_id FK, keycloak_id, role, is_active, last_login, timestamps + indexes on email, tenant_id, keycloak_id). Contains explicit NOTE about no tenant_settings table.
- **Relevance to task:** A new migration is needed for SSO configuration storage. Options: (a) add columns to tenants table (sso_enabled, sso_provider, sso_metadata, keycloak_idp_alias), (b) create a new tenant_sso_config table with FK to tenants. The migration numbering pattern is `001_initial.sql`, so the next would be `002_*.sql`.

### Application Entry Point
- **Path:** `cmd/server/main.go`
- **Purpose:** Wires up all dependencies (repos, services, middleware, routes) and starts the HTTP server.
- **Relevance to task:** New SSO-related routes would be registered here. The dependency injection is manual (no DI framework), so new services need to be wired explicitly. Current route structure: public routes (`/api/auth/*`) and protected routes (with `authMiddleware.Authenticate`). SSO callback endpoints would likely be public routes.

## Change Points

| Location | What Changes | Scope | Confidence |
|----------|-------------|-------|------------|
| `internal/auth/handler.go:Login()` | Needs pre-check: before accepting email/password, check if tenant has SSO enabled and return SSO redirect URL instead of attempting password auth. New SSO-specific endpoints needed (e.g., `/api/auth/sso/init` to initiate SAML flow, `/api/auth/sso/callback` to handle Keycloak callback after SAML assertion). | medium | high -- read the handler code, password-only flow confirmed |
| `internal/auth/service.go:Login()` | Add SSO detection logic: resolve tenant, check SSO config, branch between password flow and SSO redirect flow. New method needed for SSO callback handling (token exchange after Keycloak processes SAML assertion). | medium | high -- read the service code, single-path login confirmed |
| `internal/auth/keycloak.go:KeycloakClient` | May need new methods for Keycloak Admin API calls to manage identity providers programmatically (create/update/delete SAML IdP config per tenant). Alternatively, if IdP setup is manual, the existing client is sufficient for token operations. | medium | medium -- depends on whether IdP management is programmatic or manual |
| `internal/tenant/models.go:Tenant` | Extend with SSO-related fields OR add a separate SSO config type. At minimum: `SSOEnabled bool`, `SSOProvider string`, `KeycloakIDPAlias string`. If using a separate table, a new `TenantSSOConfig` struct is needed. | small | high -- read the model, confirmed no SSO fields exist |
| `internal/tenant/repository.go` | Update SELECT queries to include new SSO columns (if adding to tenants table) OR add new repository for tenant_sso_config table. All existing queries have hardcoded column lists that would need updating. | medium | high -- read all queries, confirmed hardcoded column lists |
| `internal/tenant/service.go` | Add method for SSO config retrieval, e.g., `GetSSOConfig(ctx, tenantID)` or `IsSSOEnabled(ctx, tenantID)`. | small | high |
| `internal/user/service.go:GetOrCreateByEmail()` | May need modification to accept and store KeycloakID during SSO user provisioning. Currently creates users without KeycloakID. For SSO users, the Keycloak sub claim should be stored during JIT provisioning. | small | high -- read the code, confirmed KeycloakID is not set on create |
| `migrations/002_add_sso_config.sql` (new file) | New migration to add SSO configuration storage. Either ALTER TABLE tenants ADD COLUMN or CREATE TABLE tenant_sso_config. | medium | high |
| `cmd/server/main.go` | Register new SSO routes, wire up any new services/dependencies. | small | high -- read the wiring code |
| `internal/config/config.go:Config` | Potentially no changes needed if Keycloak handles all SAML protocol. If the app needs Keycloak Admin API access, add admin credentials to KeycloakConfig. | small | medium |
| `docker-compose.yml` | No changes expected. Keycloak and PostgreSQL already configured. SAML IdP configuration happens within Keycloak's admin console, not in docker-compose. | none | high |

## Dependencies

### Upstream (what affected code depends on)

- **gocloak/v13 (github.com/Nerzal/gocloak/v13):** The Keycloak client library. Currently used for `Login()` (direct grant), `RetrospectToken()`, `DecodeAccessToken()`, `Logout()`, `RefreshToken()`. For SSO, may need additional gocloak methods for: (a) Keycloak Admin API to create identity providers (`CreateIdentityProvider`), (b) token exchange for brokered authentication, (c) user federation queries. Gocloak v13 supports these admin operations.
- **chi/v5 (github.com/go-chi/chi/v5):** HTTP router. Route registration pattern is established; SSO routes would follow the same pattern. No concerns.
- **pgx/v5 (github.com/jackc/pgx/v5):** PostgreSQL driver. Used directly (no ORM). Queries are hand-written SQL. New queries for SSO config follow the same pattern.
- **golang-jwt/v5:** JWT library. Currently used indirectly (gocloak handles JWT operations). SSO does not change this dependency.
- **sethvargo/go-envconfig:** Configuration loader. New env vars would follow the existing `env:` tag pattern.
- **zerolog:** Logging. SSO-related log messages should follow the existing pattern (structured logging with zerolog).

### Downstream (what depends on affected code)

- **Auth Middleware (`internal/auth/middleware.go`):** Depends on `KeycloakClient.ValidateToken()` and `tenant.Service.GetByEmailDomain()`. For SSO users, the middleware should work unchanged because Keycloak issues standard JWTs regardless of authentication method (password or SAML-brokered). The `tenant_id` custom claim and realm roles extraction would work the same way. However, this depends on Keycloak being configured to include the `tenant_id` custom claim for brokered users too.
- **User Handler (`internal/user/handler.go`):** Depends on auth middleware context keys (`ContextKeyUserID`, `ContextKeyTenantID`). No changes needed -- works the same regardless of how authentication happened.
- **Main wiring (`cmd/server/main.go`):** Instantiates all services and registers routes. Must be updated to register new SSO routes.

### External

- **Keycloak (v24.0):** The identity broker. Running in dev mode via docker-compose. Single realm "platform" with client "platform-app". SAML IdP brokering is a built-in Keycloak feature -- each tenant's SAML IdP would be configured as an identity provider in the platform realm. Keycloak handles: SAML protocol (AuthnRequest generation, assertion parsing/validation), user session management, token issuance post-SAML auth.
- **PostgreSQL (v16):** Database. Stores tenants and users. New migration needed for SSO config. No concerns about compatibility.
- **Enterprise IdPs (external to the system):** The customer's SAML identity providers (Okta, Azure AD, ADFS, etc.). These are external systems with their own availability characteristics. The platform has no control over them.

### Implicit

- **Keycloak realm configuration:** The application assumes a single realm ("platform"). SAML IdP brokering within a single realm means all tenant IdPs are configured as identity providers in the same realm, distinguished by alias. This works but requires careful naming conventions (e.g., `saml-{tenant-slug}` as IdP alias).
- **Custom claim `tenant_id` in Keycloak tokens:** The middleware extracts `tenant_id` from JWT claims. For SAML-brokered users, Keycloak needs to be configured with a mapper that sets this claim based on the IdP alias or user attributes. This is a Keycloak configuration dependency, not a code dependency, but is critical for the auth flow to work.
- **Email domain uniqueness:** The `tenants.email_domain` column has a UNIQUE constraint. Tenant resolution by email domain is a 1:1 mapping. This means one email domain can map to at most one tenant. This is fundamental to the SSO detection flow.

## Existing Patterns

- **Repository-Service-Handler layering:** Every domain (auth, tenant, user) follows the same pattern: `Repository` (direct DB queries) -> `Service` (business logic) -> `Handler` (HTTP layer). New SSO code should follow this pattern. The tenant SSO config should have its own repository methods (or a separate repository), service methods for SSO operations, and handler methods for SSO endpoints.

- **Manual dependency injection in main.go:** All dependencies are created and wired in `main()`. Pattern: create repos, create services (injecting repos and other services), create middleware, register routes. Example: `authSvc := auth.NewService(keycloakClient, userSvc, tenantSvc)`. New SSO dependencies must be wired here.

- **Tenant resolution by email domain:** `tenant.Repository.GetByEmailDomain()` splits the email at `@` and queries by domain. Used in both `auth.Service.Login()` and `auth.Middleware.Authenticate()` (as fallback when `tenant_id` claim is missing). This same mechanism would drive SSO detection: resolve tenant from email -> check if SSO is enabled -> redirect to IdP.

- **JIT user provisioning:** `user.Service.GetOrCreateByEmail()` creates a local user record if one doesn't exist. Sets default role "user", uses email as name placeholder. This pattern exists and should be reused for SSO user provisioning with modifications (store KeycloakID).

- **Context-based auth propagation:** Auth middleware stores `user_id`, `tenant_id`, and `roles` in request context using typed context keys. All downstream handlers extract from context. This pattern is stable and SSO does not need to change it.

- **Environment-based configuration:** All config loaded from env vars using `envconfig` with struct tags. Pattern for adding new config: add a field with an `env:` tag to `Config` or a nested struct.

- **Direct SQL queries (no ORM):** All repository methods use hand-written SQL with `pgx`. Column lists are explicit in every query. This means schema changes require updating every query that touches the modified table.

## Technical Observations

- **No test files exist anywhere in the project.** There are zero `_test.go` files. This means there is no automated test coverage, no test infrastructure, and no patterns for how tests should be written. Any SSO implementation has no safety net against regressions.

- **Bug in middleware.go `RequireRole()`:** On line 101, the inner closure shadows the `r` variable -- the loop variable `r` (role string) shadows the `r` parameter (http.Request). The call `r.WithContext(r.Context())` on line 101 will not compile correctly because `r` is a string at that point, not `*http.Request`. This is a pre-existing bug unrelated to SSO but indicates the middleware code has not been thoroughly tested.

- **`KeycloakID` not populated during user creation:** In `user.Service.GetOrCreateByEmail()`, the `newUser` struct does not set `KeycloakID`. The `User.KeycloakID` field exists and the repository's `GetByKeycloakID()` method exists, but neither is used in the current flow. For SSO, populating `KeycloakID` is essential for account linking -- matching a returning SAML user to their local account.

- **`httputil` package exists but is not used by auth handlers.** Auth handlers use `http.Error()` directly while `pkg/httputil/response.go` provides `WriteJSON()` and `WriteError()`. Inconsistent error response format. SSO handlers should use `httputil` for consistency.

- **Single Keycloak realm confirmed.** Config has `KEYCLOAK_REALM=platform` (default). All SAML IdP brokering happens within this single realm. Keycloak supports multiple identity providers per realm, each with a unique alias, so multi-tenant SSO within one realm is viable.

- **`Session` model exists but is unused.** `internal/auth/models.go` defines a `Session` struct with `ID, UserID, TenantID, CreatedAt, ExpiresAt`. The comment says "Currently not persisted server-side -- sessions are managed entirely by Keycloak." This struct exists for potential future use and could be relevant if SSO requires server-side session tracking (e.g., for SLO).

- **Keycloak admin credentials not in config.** The current `KeycloakConfig` has `BaseURL`, `Realm`, `ClientID`, `ClientSecret` -- these are client credentials, not admin credentials. If the application needs to call Keycloak's Admin REST API to programmatically create/manage identity providers, admin credentials (or a service account with admin privileges) would need to be added to the config.

## Test Coverage

| Area | Test Type | Coverage Level | Key Test Files | Notes |
|------|-----------|---------------|----------------|-------|
| internal/auth/ | none | none | (no test files) | No tests for auth handlers, service, middleware, or Keycloak client |
| internal/tenant/ | none | none | (no test files) | No tests for tenant CRUD or email domain resolution |
| internal/user/ | none | none | (no test files) | No tests for user provisioning or lookup |
| internal/config/ | none | none | (no test files) | No tests for config loading |
| pkg/httputil/ | none | none | (no test files) | No tests for response helpers |
| migrations/ | none | none | N/A | SQL migrations, no test infrastructure |

## Self-Critique Notes

- **No tests to verify findings against.** The complete absence of tests means there are no behavioral contracts codified in the codebase. All behavioral understanding comes from reading the source directly, which means the analysis captures what the code does, not what it's intended to do in edge cases.

- **Keycloak configuration not visible.** The codebase contains no Keycloak realm export, Terraform config, or setup scripts beyond the docker-compose service definition. The actual Keycloak realm configuration (client settings, mappers, authentication flows) cannot be verified from the code. The analysis assumes default Keycloak settings, but the actual deployment may have custom configurations that affect SSO implementation.

- **Frontend not present.** The requirements mention frontend changes for SSO detection and redirect, but no frontend code exists in this project. The analysis covers only the Go backend. Frontend SSO implementation (email domain detection UI, redirect handling, callback processing) is a separate concern that needs its own analysis.

- **The `RequireRole` bug was found during analysis but its severity is uncertain.** The variable shadowing on line 99-101 of middleware.go appears to be a real bug, but without a Go compiler available in this context, the exact behavior cannot be confirmed. It should be verified.

- **Gocloak Admin API capabilities not verified.** The analysis states gocloak v13 supports identity provider management via Admin API, based on the library's documented features. The specific methods available (`CreateIdentityProvider`, etc.) should be verified against the gocloak v13.9.0 API surface before planning relies on programmatic IdP management.
