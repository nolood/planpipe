# Constraints / Risks Analysis

## Constraints

### Architectural

- **Single Keycloak realm architecture:** The application is configured with a single realm ("platform", set via `KEYCLOAK_REALM` env var in `internal/config/config.go`). All tenant SAML identity providers must be configured as identity providers within this one realm. This is workable (Keycloak supports multiple IdPs per realm) but means IdP aliases must be unique across tenants and naming conventions must be enforced. Source: `config.go` line 19, `docker-compose.yml` line 13.

- **Repository-Service-Handler pattern must be followed:** Every module in the codebase follows this three-layer pattern with manual dependency injection in `main.go`. SSO code must conform to this structure. Source: consistent pattern across `internal/auth/`, `internal/tenant/`, `internal/user/`, wired in `cmd/server/main.go`.

- **Keycloak as the SAML broker, not the Go app:** The Go application does not implement any SAML protocol handling. It delegates all authentication to Keycloak via gocloak. SSO must follow the same delegation model -- Keycloak handles SAML AuthnRequest/Response, the Go app handles the resulting Keycloak tokens. This is actually a beneficial constraint (no SAML library needed in Go) but it means the Go app cannot customize SAML behavior directly. Source: `internal/auth/keycloak.go` -- entire Keycloak interaction is via gocloak, no SAML libraries in `go.mod`.

- **Direct SQL with explicit column lists:** The codebase does not use an ORM. All queries in `tenant/repository.go` and `user/repository.go` have explicit column lists in SELECT statements. Any schema change to the `tenants` table requires updating every query that touches it (4 queries in tenant repository: `GetByID`, `GetBySlug`, `GetByEmailDomain`, `List`; plus the `Update` statement). Source: `internal/tenant/repository.go`.

### Technical

- **Go 1.22 / gocloak v13.9.0:** The application uses Go 1.22 and gocloak v13.9.0. Any SSO implementation must be compatible with these versions. Gocloak v13 provides Admin API methods for identity provider management, but the specific methods available need verification against v13.9.0 release notes. Source: `go.mod`.

- **pgx/v5 for database access:** All database operations use `pgxpool`. Migrations are plain SQL files with numeric prefixes. No migration framework (like golang-migrate or goose) is visible in the project -- migrations appear to be applied manually or by a process not captured in the codebase. Source: `migrations/001_initial.sql`, `go.mod`.

- **Single email domain per tenant:** The `tenants.email_domain` column is `TEXT NOT NULL UNIQUE`. Each tenant has exactly one email domain. Enterprise organizations with multiple domains would need either multiple tenant records or a schema change to support multiple domains per tenant. Source: `migrations/001_initial.sql` line 6, `internal/tenant/models.go`.

- **No Keycloak Admin API credentials available:** The current config only has client credentials (`ClientID`, `ClientSecret`), not admin credentials. The Keycloak Admin API (needed for programmatic IdP management) requires either admin username/password or a service account with realm-management roles. Source: `internal/config/config.go` lines 17-22.

### Business

- **Q3 delivery target:** The product team has prioritized this for Q3. This is a timeline constraint that bounds the scope of the initial release. The minimum viable outcome (SP-initiated SSO, per-tenant config, JIT provisioning) must be achievable within this window. Source: requirements draft.

- **Coexistence with password login is non-negotiable:** The existing email/password flow must remain fully functional for non-SSO tenants. This is explicitly called out in the requirements and confirmed by the codebase -- there is no SSO fallback mechanism, so password login must keep working independently. Source: requirements draft scope section.

### Compatibility

- **Token format must remain unchanged for downstream consumers:** The auth middleware (`internal/auth/middleware.go`) extracts `user_id`, `tenant_id`, and `roles` from Keycloak JWTs and injects them into request context. All protected route handlers depend on these context values. SSO-authenticated users must produce tokens with the same claims structure. Source: `internal/auth/middleware.go` lines 52-84, `internal/user/handler.go` lines 18-24.

- **`tenant_id` custom claim dependency:** The middleware relies on a custom `tenant_id` claim in the JWT (line 74 of keycloak.go). For SAML-brokered users, Keycloak must be configured with a mapper that populates this claim. Without it, the middleware falls back to email domain lookup, which works but is less reliable. Source: `internal/auth/keycloak.go` lines 73-76, `internal/auth/middleware.go` lines 61-71.

- **Login API response contract:** The current login endpoint returns `{access_token, refresh_token, expires_in, token_type}`. Any SSO-related changes to the login flow must not alter this response format for password-based logins. SSO login may return a different response (e.g., redirect URL), but the existing contract must be preserved for password clients. Source: `internal/auth/handler.go` lines 24-29.

### Regulatory/Compliance

- **SAML 2.0 protocol compliance:** The integration must conform to SAML 2.0 specification for SP-initiated SSO. Since Keycloak handles the protocol, this is largely Keycloak's responsibility, but the application must correctly handle the post-authentication flow and respect SAML assertions. Source: requirements draft.

- **Enterprise security expectations:** Enterprise clients deploying SSO typically expect: encrypted SAML assertions, signed AuthnRequests, certificate-based trust, and audit logging of SSO events. These are Keycloak configuration concerns but need to be verified during setup. Source: industry standard expectations for enterprise SAML SSO.

## Risks

| Risk | Category | Likelihood | Impact | Evidence | Mitigation Idea |
|------|----------|-----------|--------|----------|-----------------|
| `tenant_id` claim not populated for SAML-brokered users, causing middleware to fall back to email domain lookup or fail | integration | medium | high | The `tenant_id` claim is a custom claim (`keycloak.go` line 74). Keycloak needs explicit mapper configuration for brokered authentication flows. If this mapper is not configured for SAML IdP users, the claim will be absent. | Verify Keycloak mapper configuration is part of the IdP setup checklist. Test with actual SAML flow before go-live. |
| Account linking conflicts when existing password users switch to SSO -- duplicate accounts or lost access | scope | high | high | `user.Service.GetOrCreateByEmail()` matches by email. If a user's email in the IdP differs from their platform email (e.g., case sensitivity, alias), a duplicate user record is created. `KeycloakID` is not set during creation (`user/service.go` line 37), so there's no secondary matching key. | Implement account linking by email with KeycloakID update on first SSO login. Normalize emails to lowercase. |
| Multiple email domains per enterprise tenant not supported | scope | medium | high | Schema has single `email_domain TEXT NOT NULL UNIQUE` per tenant (`001_initial.sql` line 6). Real enterprises often have multiple domains. If tenant employees use different email domains, SSO detection by email domain will fail for some users. | Consider a `tenant_email_domains` junction table or accept single-domain limitation for MVO with documented workaround. |
| No test coverage means regressions in password login flow go undetected | regression | medium | high | Zero test files in the entire project. Changes to `auth/service.go`, `auth/handler.go`, and `auth/middleware.go` could break existing password login with no automated detection. | Add tests for the existing password login flow before modifying auth code. At minimum, integration tests for Login handler. |
| Keycloak IdP configuration drift or misconfiguration for specific tenants | knowledge | medium | medium | No Keycloak configuration management (no Terraform, no realm export, no setup scripts) visible in the codebase. IdP configuration is presumably manual. Manual configuration of SAML IdPs per tenant is error-prone and hard to audit. | Document a standard IdP setup procedure. Consider programmatic IdP management via Keycloak Admin API. |
| Variable shadowing bug in `RequireRole` middleware causes runtime error for admin routes | regression | high | medium | In `middleware.go` line 99-101, loop variable `r` (string) shadows `r` (*http.Request). The call `r.WithContext(r.Context())` will fail because `r` is a string. This is a pre-existing bug that would surface if admin routes are tested during SSO work. | Fix the variable shadowing before SSO changes. Rename loop variable to `role` or `roleName`. |
| Keycloak brokered login may use different user IDs than direct login, breaking user-KeycloakID mapping | integration | medium | medium | Keycloak may assign different user IDs for brokered vs. direct-grant users depending on identity provider configuration and user federation settings. The `keycloak_id` stored in the users table maps to the `sub` claim. If Keycloak creates a new user for SAML-brokered login instead of linking to existing, the IDs diverge. | Configure Keycloak's "first broker login" flow to link with existing users by email. Verify sub claim consistency. |
| JIT provisioning without authorization check creates accounts for any IdP-authenticated user | scope | low | medium | `user.Service.GetOrCreateByEmail()` creates users unconditionally. If an enterprise IdP authenticates a user who should not have access to the platform (not assigned to the SAML app in the IdP), they will still get an account. | Rely on IdP-side application assignment to control access. Document this as a shared responsibility model. |

## Integration Dependencies

- **Keycloak (v24.0, dev mode):** Primary integration point. Contract is implicit -- the application uses gocloak library methods and expects Keycloak to return standard JWT tokens with specific claims. Stability: Keycloak is mature and SAML brokering is a core feature. Change flexibility: Keycloak configuration is controlled by the platform ops team. Failure mode: if Keycloak is down, all authentication (password and SSO) fails -- it's a single point of failure.

- **Enterprise Identity Providers (Okta, Azure AD, ADFS, etc.):** External systems controlled by the customer. Contract: SAML 2.0 protocol, IdP metadata XML defines endpoints and certificates. Stability: varies by customer. Change flexibility: none -- the platform must adapt to each IdP's configuration. Failure mode: if an IdP is down, only that tenant's SSO users are affected; other tenants and password login continue working.

- **PostgreSQL (v16):** Schema changes needed (new migration). Contract: SQL schema defined by migrations. Stability: managed by the platform team. Change flexibility: full -- the migration pattern is established. Failure mode: database downtime affects all operations, not SSO-specific.

## Backward Compatibility

| What Changes | Current Consumers | Migration Needed? | Rollback Safe? | Notes |
|-------------|-------------------|-------------------|----------------|-------|
| Tenants table schema (new SSO columns or related table) | `tenant.Repository` (4 SELECT queries + 1 UPDATE), `tenant.Handler`, `auth.Service.Login()`, `auth.Middleware.Authenticate()` | yes -- SQL migration to add columns/table. All repository queries with explicit column lists need updating. | yes -- new columns can be nullable, migration is additive (ADD COLUMN / CREATE TABLE), rollback drops column/table | If using ALTER TABLE ADD COLUMN with defaults, no data migration needed. Existing rows get default values. |
| Login API endpoint behavior (`POST /api/auth/login`) | Frontend client, any API consumers | no -- password login behavior unchanged. SSO adds new behavior (redirect response) only for SSO-enabled tenants. New SSO endpoints are additive. | yes -- if SSO is disabled for a tenant, login reverts to password-only behavior | Existing clients sending email+password to non-SSO tenants see identical behavior. New SSO detection could return a different response code (e.g., 302 or a JSON body with redirect URL) for SSO tenants, which clients must handle. |
| User creation flow (KeycloakID population) | `user.Service.GetOrCreateByEmail()`, `user.Repository.Create()` | no -- adding KeycloakID to creation is additive. Existing users without KeycloakID continue to work. | yes -- KeycloakID is already nullable in schema | Backfilling KeycloakID for existing users is optional but recommended for account linking. |

## Sensitive Areas

- **`internal/auth/service.go:Login()` -- the login orchestration function:** This is the most critical function in the authentication flow. It resolves tenants, validates credentials, and provisions users. Any modification risks breaking the existing password login flow. Risk level: high. Mitigating factor: the function is short (30 lines) and straightforward, making it easy to understand. Aggravating factor: zero test coverage.

- **`internal/auth/middleware.go:Authenticate()` -- the authentication middleware:** Every protected request passes through this function. It validates tokens and sets context values that all downstream handlers depend on. While SSO should not require changes here (Keycloak issues standard JWTs regardless of auth method), any accidental modification could break all protected routes. Risk level: high.

- **`internal/auth/keycloak.go:ValidateToken()` -- token introspection and claim extraction:** This function parses JWT claims including the custom `tenant_id` claim. If SAML-brokered tokens have a different claims structure, this function needs verification. Risk level: medium.

- **`internal/tenant/repository.go` -- hardcoded column lists:** All four SELECT queries and the UPDATE query list columns explicitly. Adding SSO columns to the tenants table requires updating all of them. Missing one would cause runtime errors (scan mismatches). Risk level: medium. This is a mechanical task but error-prone without tests.

- **Keycloak "first broker login" flow configuration:** When a SAML-brokered user logs in for the first time, Keycloak runs its "first broker login" authentication flow. By default, this flow may prompt the user to link or create an account, review profile information, etc. The behavior of this flow directly affects the user experience and account linking. This is configured in Keycloak, not in the Go app, but misconfiguration here causes the most visible user-facing problems. Risk level: high. Not directly auditable from the codebase.

## Self-Critique Notes

- **Keycloak configuration constraints could not be fully verified.** The analysis states that SAML brokering works within a single realm and that custom claim mappers are needed, but the actual Keycloak configuration is not in the codebase. These are based on Keycloak documentation and typical configurations, not observed code. The actual realm setup may have additional constraints.

- **Migration framework uncertainty.** The analysis notes that no migration framework (golang-migrate, goose) is visible, but one might be used outside this codebase (e.g., a Makefile, CI script, or separate tool). The rollback safety assessment assumes additive schema changes, which is generally safe, but the exact rollback mechanism is unknown.

- **Risk calibration may underweight the "no tests" risk.** The analysis rates the regression risk as "medium likelihood, high impact." In practice, modifying core authentication code with zero test coverage is one of the highest-risk activities in software development. The likelihood of introducing a regression is arguably "high" rather than "medium." The rating was kept at "medium" because the codebase is small and changes can be manually traced, but this is a judgment call.

- **Frontend impact not assessed.** The constraints analysis covers only the Go backend. The frontend must implement SSO detection (email domain check), redirect handling, and callback processing. These are separate constraints that were not analyzed because no frontend code exists in this project.

- **The account linking risk is the highest-severity finding, but the mitigation is vague.** "Implement account linking by email with KeycloakID update" sounds straightforward but has its own edge cases (email format differences, multiple IdPs for the same user, race conditions during concurrent first-logins). A deeper analysis of the account linking strategy would be warranted if this is in scope for MVO.
