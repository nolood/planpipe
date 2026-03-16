# Constraints / Risks Analysis

## Constraints

### Architectural
- **Single Keycloak realm architecture:** The application uses one Keycloak realm (`platform`, per `internal/config/config.go:19` and `docker-compose.yml:13`). All SAML IdP broker configurations must coexist within this realm. This is workable (Keycloak supports multiple IdPs per realm) but means tenant isolation is at the IdP-alias level, not realm level. Source: `internal/auth/keycloak.go:18` stores a single `realm` field used for all operations
- **Direct grant to authorization code flow shift:** The current `KeycloakClient.Authenticate()` uses `client.Login()` -- the Resource Owner Password Credentials (direct grant) flow where the app collects and forwards credentials. SAML SSO requires the Authorization Code flow where the user is redirected to Keycloak, then back to the app with a code. This is a fundamentally different flow pattern that adds browser redirects to what is currently a pure API-to-API interaction. Source: `internal/auth/keycloak.go:42-48`
- **Repository-service-handler layering:** The codebase follows a strict repo -> service -> handler pattern across all modules. Any new SSO functionality must follow this pattern. Source: consistent structure across `internal/auth/`, `internal/tenant/`, `internal/user/`

### Technical
- **gocloak v13.9.0 API surface:** The gocloak library wraps the Keycloak REST API. While it supports token operations (login, introspect, refresh), its support for SAML IdP management via the Admin REST API needs verification. If gocloak does not expose `CreateIdentityProvider` or equivalent methods, direct HTTP calls to the Keycloak Admin REST API (`/admin/realms/{realm}/identity-provider/instances`) will be needed. Source: `go.mod:6`
- **Go 1.22 requirement:** Module declares Go 1.22. No SSO-specific constraint, but any SAML libraries or HTTP client usage must be compatible. Source: `go.mod:3`
- **PostgreSQL schema migration:** The database uses raw SQL migrations (`migrations/001_initial.sql`). No migration framework (golang-migrate, goose) is evident. New migrations must follow the existing numbered convention and be applied manually or via a deployment process. Source: `migrations/` directory structure
- **No per-tenant config storage mechanism:** The tenant model has only fixed columns. The code explicitly notes this gap: "There is currently no per-tenant configuration storage." Adding SSO config requires either schema extension (new columns/table) or a JSONB approach. Source: `internal/tenant/models.go:18-23`, `migrations/001_initial.sql:34-37`
- **Email UNIQUE constraint across all tenants:** `users.email` has a global UNIQUE constraint, not scoped to tenant. This means the same email cannot exist in two tenants. For SSO this means email-domain-to-tenant mapping must be strictly 1:1. Source: `migrations/001_initial.sql:19`

### Business
- **Q3 delivery timeline:** Product team has prioritized this for Q3. This constrains scope decisions -- features that expand the timeline (IdP-initiated SSO, SCIM, SLO) are explicitly deferred. Source: requirements draft
- **Backward compatibility mandate:** Current email/password login must remain fully functional for non-SSO tenants. This is a hard business requirement, not a nice-to-have. Source: requirements draft constraints section
- **Enterprise customer dependency:** SSO is blocking enterprise deals. Delivery delays have direct revenue impact. This creates pressure to ship MVO rather than a complete solution. Source: requirements draft problem statement

### Compatibility
- **Login API contract preservation:** The existing `POST /api/auth/login` endpoint accepts `{email, password, tenant_id}` and returns `{access_token, refresh_token, expires_in, token_type}`. This contract must be preserved exactly for non-SSO tenants. SSO introduces new endpoints rather than modifying this one. Source: `internal/auth/handler.go:18-29`
- **JWT token format:** The auth middleware expects JWTs with `sub`, `email`, `tenant_id` (custom claim), and `realm_access.roles` claims. SSO-issued JWTs must contain the same claims for downstream compatibility. If Keycloak doesn't automatically include `tenant_id` in brokered tokens, a protocol mapper must be configured. Source: `internal/auth/keycloak.go:68-88`
- **Context key contract:** Protected route handlers depend on `ContextKeyUserID`, `ContextKeyTenantID`, and `ContextKeyRoles` being set by the auth middleware. SSO-authenticated requests must produce the same context values. Source: `internal/auth/middleware.go:15-18`, `internal/user/handler.go:19-23`

### Regulatory/Compliance
- **Enterprise security policy enforcement:** The entire purpose of SSO is to enable enterprises to enforce their security policies. The implementation must not introduce security gaps (e.g., allowing password fallback that bypasses IdP MFA). SAML assertion validation must be robust -- Keycloak handles this, but misconfiguration could weaken it
- **SAML assertion handling:** SAML assertions contain sensitive authentication data. While Keycloak handles the SAML protocol, any logging or error handling in the Go application must not inadvertently log SAML assertions, tokens, or sensitive user attributes

## Risks

| Risk | Category | Likelihood | Impact | Evidence | Mitigation Idea |
|------|----------|-----------|--------|----------|-----------------|
| Keycloak SAML broker configuration is complex and error-prone, especially with per-tenant IdP setup at scale | integration | medium | high | Keycloak SAML IdP config requires: metadata import, attribute mappers, first broker login flow configuration, client scope settings. Each tenant is a separate IdP config | Build a configuration automation layer (Keycloak Admin API calls) rather than requiring manual Keycloak admin console work |
| gocloak library may not support SAML IdP management operations, requiring direct Keycloak Admin REST API calls | technical | medium | medium | gocloak wraps Keycloak REST API but SAML IdP management endpoints may not be covered. Would need to check gocloak source or docs | Design the Keycloak admin client to fall back to direct HTTP calls if gocloak doesn't expose needed endpoints |
| Zero test coverage means any auth flow changes carry high regression risk | regression | high | high | No test files exist anywhere in the codebase. The auth middleware, which all protected routes depend on, has zero tests | Establish at minimum integration tests for the login flow before making changes. SSO changes should include tests |
| Account linking for existing password users is undefined and will surface as a real problem when the first tenant enables SSO | scope | high | medium | Requirements list this as an unknown. The `GetOrCreateByEmail` method creates new users but doesn't handle the case where a user with the same email already exists with a different auth method | Define account linking strategy before implementation. Simplest: match by email, update KeycloakID with brokered subject |
| Keycloak client may need reconfiguration for Authorization Code flow, which could affect existing password auth if done incorrectly | technical | medium | high | Current setup likely uses Direct Access Grants only. Enabling Standard Flow shouldn't break direct grants, but misconfiguration could | Test Keycloak client config changes in a staging environment. Both flows can coexist on the same client |
| Single `email_domain` per tenant assumption may break with enterprise customers who have multiple email domains | scope | low | medium | `tenants.email_domain` is a single TEXT column with UNIQUE constraint. Some enterprises use multiple domains (acme.com, acme.co.uk). Only one domain can map to one tenant currently | This could be deferred -- most enterprises have a primary domain. If needed, a `tenant_email_domains` join table could replace the single column |
| SAML attribute mapping variability across IdPs (Okta vs Azure AD vs ADFS send different attribute names) | integration | medium | medium | Different IdPs use different SAML attribute names for the same data (email, name, groups). Keycloak attribute mappers must handle each IdP's specific format | Design a standard attribute mapper template in Keycloak. Document required IdP attributes in setup guide |
| Variable shadowing bug in RequireRole middleware may indicate untested code paths in the auth module | regression | low | medium | `internal/auth/middleware.go:99-104` has a variable shadowing issue where loop variable `r` shadows the request parameter, causing a compilation error. This suggests the role-based auth path may not be exercised | Fix the bug before SSO work. Its existence suggests this area of code may have other issues |

## Integration Dependencies

- **Keycloak (v24.0):** Primary integration point. Contract: Keycloak Admin REST API for SAML IdP management, OIDC/OAuth2 authorization code flow for token issuance after SAML brokering. Stability: Keycloak is mature and the Admin REST API is stable. Change flexibility: high -- Keycloak configuration is under our control. Failure mode: if Keycloak is down, all authentication fails (both SSO and password). This is an existing single point of failure, not new to SSO
- **Enterprise IdPs (Okta, Azure AD, ADFS, etc.):** Per-tenant external dependency. Contract: SAML 2.0 protocol -- metadata exchange during configuration, SAML assertions during login. Stability: varies by customer's IdP. Change flexibility: zero -- we cannot modify customer IdP behavior. Failure mode: if a specific IdP is down, only that tenant's SSO users are affected. Non-SSO tenants and other SSO tenants are unaffected
- **PostgreSQL (v16):** Schema changes for SSO config storage. Contract: SQL DDL/DML. Stability: high. Change flexibility: high -- we control the schema. Failure mode: database unavailability blocks all operations (existing risk, not new)

## Backward Compatibility

| What Changes | Current Consumers | Migration Needed? | Rollback Safe? | Notes |
|-------------|-------------------|-------------------|----------------|-------|
| New SSO endpoints added (SSO initiate, callback) | None (new endpoints) | no | yes | New endpoints don't affect existing consumers. Can be removed without breaking anything |
| Tenant model/table extended with SSO config | Tenant repository, service, handler, auth service | yes -- schema migration | yes -- new columns/table can be dropped, existing columns unaffected | Migration adds data; doesn't modify or remove existing columns. Rollback requires reverse migration |
| Login endpoint behavior for SSO tenants | Frontend login flow, any API consumers | unknown | unknown | If `POST /api/auth/login` rejects password login for SSO tenants, existing API consumers hitting that endpoint will get errors. Need to decide: reject with error + redirect hint, or silently accept both methods? |
| User creation flow gains KeycloakID | Auth service (Login flow), user service | no | yes | Adding KeycloakID to user creation is additive. Existing users without KeycloakID are unaffected |
| Keycloak client config (enable Standard Flow) | Auth middleware token validation, login flow | no | yes | Enabling Standard Flow alongside Direct Access Grants doesn't break existing password auth |

## Sensitive Areas

- **Auth middleware (`internal/auth/middleware.go`):** This is the single gatekeeper for all protected routes. Every API request passes through `Authenticate()`. While SSO should not require middleware changes (it validates JWTs regardless of issuance method), any inadvertent change here could break all authenticated endpoints for all tenants. Risk level: **high**
- **Auth service Login flow (`internal/auth/service.go:Login`):** The core login orchestration. Adding SSO tenant detection here means modifying a critical path that all password logins use. A bug in the SSO-detection branch could break password login. Risk level: **high**
- **Tenant resolution by email domain (`internal/tenant/repository.go:GetByEmailDomain`):** Used by both login and middleware for tenant detection. If SSO config checking is added to this path, performance could degrade (additional DB query per request) or errors could cascade. Risk level: **medium**
- **Keycloak client (`internal/auth/keycloak.go`):** Wraps all Keycloak communication. Adding authorization code exchange methods is an extension, but any error in the existing methods (Authenticate, ValidateToken) would break all auth. The single `realm` field is a potential fragility point if multi-realm is ever needed. Risk level: **medium**
- **Database schema (`migrations/001_initial.sql`):** Production data. Schema migration must be forward-compatible and rollback-safe. The `email UNIQUE` constraint and `email_domain UNIQUE` constraint are architectural decisions that constrain SSO design. Risk level: **medium**

## Critique Review

The critic found this analysis SUFFICIENT across all five criteria. Constraint specificity scored PASS -- constraints are backed by specific code references and evidence (file paths, line numbers, SQL schema details). Risk calibration is well-balanced -- not all risks are rated high; they span low to high with differentiated likelihood and impact scores. Backward compatibility is thoroughly addressed with a concrete table mapping what changes, who consumes it, and rollback safety. Integration dependencies cover all three external systems with contract types and failure modes. Sensitive areas identify specific modules with reasoning for their sensitivity levels.

Minor observation from the critic: The analysis could more explicitly address whether the gocloak library's Admin API coverage has been verified (beyond noting it "needs verification"). However, this is appropriately captured as an open question rather than asserted as a constraint.

## Open Questions

- Is password fallback for SSO-enabled tenants a security requirement to block, or an availability feature to support? This has significant architectural implications -- blocking password login for SSO tenants requires careful error handling in the login endpoint
- What happens during a Keycloak upgrade? The docker-compose pins v24.0. SAML IdP broker configurations should survive upgrades, but this needs verification in the deployment process
- Should SSO configuration changes (enable/disable, IdP metadata update) require Keycloak restart or is hot-reload supported? Keycloak generally supports hot-reload of IdP configs, but this should be verified
- How will SSO configuration be managed at scale? If 50 enterprise tenants each have SSO, that's 50 IdP configurations in Keycloak. Is the Keycloak Admin API performant enough for this, and is there a management/monitoring story?
- What monitoring and alerting should exist for SSO failures? SSO introduces a new failure mode (IdP-side issues) that password auth doesn't have. Operations needs visibility into SSO-specific failures
- Is there a data retention or audit requirement for SSO login events that differs from password login events? Enterprise customers often require detailed audit trails of SSO authentications
