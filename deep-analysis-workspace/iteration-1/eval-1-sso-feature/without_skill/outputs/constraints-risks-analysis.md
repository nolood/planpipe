# Constraints, Risks, and Integration Analysis: SAML SSO for Enterprise Tenants

## Hard Constraints

### 1. SAML 2.0 Protocol Only
The scope explicitly excludes OIDC/OAuth-based SSO. Only SAML 2.0 is in scope. This constrains the Keycloak IdP brokering configuration to SAML identity providers. However, the Go backend's interaction with Keycloak will use OIDC/OAuth2 (authorization code flow) since Keycloak abstracts the SAML protocol. The constraint applies to the IdP-side protocol, not the backend-to-Keycloak protocol.

### 2. Per-Tenant Configuration
SSO must be configurable per tenant, not system-wide. Each tenant independently enables/disables SSO and connects to their own IdP. This rules out a global SSO toggle and requires tenant-scoped storage for SSO configuration. The current `Tenant` model (`internal/tenant/models.go`) has no extensible configuration mechanism -- only fixed columns.

### 3. Backward Compatibility
The existing email/password login flow (`POST /api/auth/login`) must remain fully functional for non-SSO tenants. No behavioral changes are acceptable for tenants that do not enable SSO. This means all changes to `auth.Service.Login` must be gated behind a tenant SSO check.

### 4. Existing Keycloak Instance
Must integrate with the existing Keycloak instance (version 24.0, as specified in `docker-compose.yml`). Cannot replace Keycloak or add a second authentication system. The single-realm architecture (`platform` realm) must be maintained unless there is a strong reason to change it.

### 5. Q3 Timeline
Delivery target is Q3. This constrains scope to the essential SSO flow (SP-initiated SAML via Keycloak brokering) and defers nice-to-haves like self-service SSO configuration UI, IdP-initiated SSO, SCIM provisioning, and SLO.

### 6. Go + Chi Technology Stack
All backend changes must use Go, the chi router, and the existing patterns (repository, service, handler layers). No new frameworks or languages.

## Risks

### Risk 1: Authorization Code Flow is New to the Codebase
**Severity**: High
**Description**: The current codebase only uses Keycloak's direct grant (password) flow via `kc.client.Login()` in `internal/auth/keycloak.go`. SSO requires the OAuth2 authorization code flow, which involves browser redirects, callback endpoints, state parameters, and code exchange. This is a fundamentally different interaction pattern that does not exist in the codebase today.
**Impact**: Requires new endpoint types (redirect-based rather than JSON API), session/state management for CSRF protection during the OAuth2 flow, and correct callback URL configuration in Keycloak.
**Mitigation**: The gocloak library and Go's `oauth2` standard library both support authorization code flow. The implementation pattern is well-documented. Design the callback handler carefully with proper state validation.

### Risk 2: Keycloak IdP Configuration Complexity
**Severity**: Medium
**Description**: Each tenant's SAML IdP must be registered as an identity provider in Keycloak. This involves: uploading IdP metadata XML, configuring attribute mappers (to map SAML attributes to Keycloak user attributes), setting up first login flows (for JIT provisioning behavior), and configuring the correct `kc_idp_hint` alias. If this is done manually through Keycloak admin UI, it is error-prone and does not scale. If automated via Keycloak Admin API (gocloak), the API surface is large and poorly documented.
**Impact**: Incorrect Keycloak configuration leads to authentication failures, missing user attributes, or broken tenant isolation.
**Mitigation**: Start with manual Keycloak configuration for a single test tenant. Document the exact steps. Consider automating via gocloak's `CreateIdentityProvider` / `CreateIdentityProviderMapper` methods once the manual process is proven.

### Risk 3: Account Linking / Keycloak ID Mismatch
**Severity**: Medium
**Description**: When an existing password-based user logs in via SSO for the first time, Keycloak may create a new user or link to the existing one depending on the "First Login Flow" configuration. If Keycloak creates a new user, the `sub` claim in the token will be a new Keycloak user ID, different from the one associated with the password-based account. The `users.keycloak_id` column in the database will not match.
**Impact**: The user could end up with a duplicate account, or the `GetByKeycloakID` lookup could fail to find them.
**Mitigation**: Configure Keycloak's first login flow to detect existing users by email and link accounts automatically. In the Go backend, use email as the primary matching key (which `GetOrCreateByEmail` already does) rather than `keycloak_id`. Update `keycloak_id` after successful SSO login to reflect the current Keycloak identity.

### Risk 4: Tenant Isolation in SSO Flow
**Severity**: High
**Description**: The SSO flow introduces a new attack surface for tenant isolation violations. If the `kc_idp_hint` parameter is manipulated, a user could potentially be directed to a different tenant's IdP. After authentication, if tenant resolution relies solely on email domain, a malicious IdP could return an assertion with an email from a different domain.
**Impact**: Cross-tenant authentication bypass.
**Mitigation**: After SSO callback, validate that the authenticated user's email domain matches the tenant that initiated the SSO flow. Use the `state` parameter in the OAuth2 flow to encode the expected tenant ID and verify it after callback. Keycloak's attribute mapping should be trusted, but the Go backend should perform its own validation.

### Risk 5: Missing `tenant_id` Custom Claim in SSO Tokens
**Severity**: Medium
**Description**: The current middleware (`internal/auth/middleware.go`, line 61-71) first tries to read `tenant_id` from the token claims, then falls back to email domain resolution. For password-based login, the `tenant_id` custom claim may or may not be present (depends on Keycloak mapper configuration). For SSO-brokered tokens, this claim is even less likely to be present unless an explicit mapper is configured.
**Impact**: SSO tokens will always hit the fallback path (email domain resolution). This works correctly but adds a database query per authenticated request.
**Mitigation**: This is acceptable for v1. The email domain fallback is functional and correct. Optionally, configure a Keycloak protocol mapper to inject `tenant_id` as a custom claim, but this is an optimization, not a requirement.

### Risk 6: SSO Enforcement Bypass
**Severity**: High
**Description**: If a tenant enables SSO, users of that tenant should not be able to bypass SSO by calling `POST /api/auth/login` directly with email and password. Currently, the `Login` method in `auth.Service` does not check for SSO enablement -- it proceeds directly to Keycloak direct grant authentication.
**Impact**: SSO enforcement is ineffective. Enterprise IT policies are bypassed.
**Mitigation**: Add a check in `auth.Service.Login` after tenant resolution: if the tenant has SSO enabled (and enforcement is turned on), reject the login attempt with a clear error message indicating that SSO must be used.

### Risk 7: Frontend Coordination
**Severity**: Low (for backend scope)
**Description**: The SSO flow requires frontend changes: detecting SSO-enabled tenants (to show redirect button instead of password field), handling the redirect flow, and processing the callback. The requirements mention frontend changes are in scope, but the codebase under analysis is backend-only.
**Impact**: Backend SSO endpoints may be built without validated frontend integration, leading to API design issues discovered late.
**Mitigation**: Design the SSO API endpoints with clear contracts. The `GET /api/auth/sso/check?email=...` endpoint provides a clean frontend integration point. Document the expected frontend flow alongside the backend implementation.

## Integration Dependencies

### 1. Keycloak Admin Configuration
SSO depends on Keycloak being configured with:
- SAML Identity Provider entries (one per SSO-enabled tenant) in the `platform` realm
- Attribute mappers on each IdP to map SAML attributes (email, name) to Keycloak user attributes
- A "First Broker Login" flow configured for account linking (match by email)
- A client configuration for the Go backend that supports authorization code flow (currently likely configured only for direct grant / service account)

This is external to the Go codebase but is a hard dependency for the feature to function.

### 2. Keycloak Client Configuration Change
The Keycloak client `platform-app` (referenced in `docker-compose.yml` as `KEYCLOAK_CLIENT_ID`) may need its "Valid Redirect URIs" updated to include the SSO callback URL (e.g., `http://localhost:8080/api/auth/sso/callback`). It may also need "Standard Flow Enabled" turned on (for authorization code flow), which might be currently disabled if only direct grant is used.

### 3. Database Migration
A new migration (`002_add_sso_config.sql`) must be applied before the SSO feature can function. The migration adds the `tenant_sso_configs` table. This is a forward-only schema change with no impact on existing data (additive table, not altering existing tables).

### 4. Enterprise IdP Metadata
Each enterprise tenant must provide their IdP metadata (typically an XML file with the IdP's SSO URL, signing certificate, entity ID, and attribute mappings). This metadata is consumed by Keycloak, not by the Go backend. The Go backend only needs to know the Keycloak IdP alias for each tenant.

### 5. gocloak Library Capabilities
The authorization code flow (token exchange) may require using gocloak methods not currently used in the codebase, or direct HTTP calls to Keycloak's token endpoint. Verify that gocloak v13.9.0 supports `GetToken` with `grant_type=authorization_code`.

## Backward Compatibility Analysis

### Fully Backward-Compatible Areas
- **`POST /api/auth/login`**: Will continue to work for non-SSO tenants. For SSO tenants, it will return an error (new behavior, but only for tenants that have explicitly enabled SSO).
- **`POST /api/auth/logout`**: No changes needed.
- **`POST /api/auth/refresh`**: No changes needed. Refresh token flow is identical for SSO and password tokens.
- **`Authenticate` middleware**: No changes needed. Processes JWTs identically regardless of how they were obtained.
- **`RequireRole` middleware**: No changes needed (after fixing the pre-existing shadowing bug).
- **All protected routes**: Unchanged. They rely on context values set by middleware.
- **Database schema**: Additive only (new table). Existing `tenants` and `users` tables are not altered.

### Areas Requiring Careful Backward-Compatibility Handling
- **`auth.Service.Login`**: The SSO enforcement check must only apply to tenants that have explicitly enabled SSO. A missing or empty SSO config must default to password-login-allowed.
- **`user.Service.GetOrCreateByEmail`**: If modified to accept `keycloakID`, the existing call site in `auth.Service.Login` must continue to work (either with an empty `keycloakID` or via a separate code path).
- **Keycloak client configuration**: Enabling "Standard Flow" on the `platform-app` client should not affect existing direct grant functionality. Both flows can coexist on the same client.

## Sensitive Areas

### Security-Sensitive
1. **SSO callback endpoint**: Must validate the `state` parameter to prevent CSRF attacks. Must validate the authorization code is legitimate (Keycloak token exchange handles this).
2. **Tenant isolation**: The callback must verify that the authenticated user belongs to the tenant that initiated the SSO flow.
3. **SSO enforcement**: The password login endpoint must correctly reject SSO-enforced tenants.
4. **Token handling**: Tokens from SSO flow must not be treated differently security-wise. Same validation, same expiry, same middleware.

### Data-Sensitive
1. **IdP metadata storage**: Contains the IdP's signing certificate and SSO URL. Not highly sensitive (public keys), but should not be exposed to unauthorized users.
2. **Keycloak client secret**: Already present in config. No new secrets are introduced (the SSO flow uses the same Keycloak client).

### Operationally Sensitive
1. **Keycloak availability**: SSO introduces a hard runtime dependency on Keycloak for the authentication redirect flow (Keycloak was already a dependency for token validation, but SSO adds the interactive redirect dependency).
2. **External IdP availability**: If the enterprise IdP is down, SSO users cannot log in. The Go backend cannot control or mitigate this. Clear error messages are important.
3. **Migration ordering**: The database migration must be applied before the SSO code is deployed. Rolling deployments should apply migration first.

## Open Questions Requiring Resolution Before Implementation

1. **Where should SSO configuration live?** New `tenant_sso_configs` table (recommended) vs. columns on `tenants` table. This affects model design, repository structure, and migration.
2. **Should the Go backend automate Keycloak IdP configuration via admin API, or is manual configuration acceptable for v1?** Automation is complex but scales; manual is simpler but error-prone.
3. **What is the account linking strategy?** Match by email (simple, works with current code) vs. Keycloak-level account linking (more robust, requires Keycloak "First Broker Login" flow configuration).
4. **Should password login be blocked for SSO tenants, or should SSO be additive (both methods allowed)?** The `enforce_sso` flag approach supports both modes, but the default must be decided.
5. **What SAML attributes should be mapped?** At minimum: email, name. Possibly: groups/roles (for automatic role assignment).
