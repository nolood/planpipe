# Product Analysis: SAML SSO for Enterprise Tenants

## Business Intent

Enterprise adoption of the platform is currently blocked because organizations cannot enforce their corporate authentication policies. IT departments require employees to authenticate through their own identity provider (IdP) rather than maintaining separate credentials in a third-party application. Without SSO, the platform cannot close enterprise deals that require compliance with centralized identity management policies.

The immediate business goal is to unblock enterprise sales pipeline by delivering SAML 2.0 SSO, allowing per-tenant IdP integration through the existing Keycloak infrastructure. This is a Q3 delivery target, signaling active sales pressure.

## User Scenarios

### Scenario 1: Enterprise Admin Configures SSO for Their Tenant
An IT administrator at an enterprise client (e.g., plan = "enterprise") provides their SAML IdP metadata XML to the platform team. The platform team configures the tenant's SSO connection in Keycloak and marks the tenant as SSO-enabled. After configuration, all users with that tenant's email domain are redirected to the corporate IdP for authentication.

### Scenario 2: Enterprise Employee Logs In via SSO (SP-Initiated)
1. User navigates to the platform login page.
2. User enters their corporate email (e.g., `jane@acmecorp.com`).
3. The system resolves the tenant via `email_domain` lookup (existing `GetByEmailDomain` in `internal/tenant/repository.go`).
4. The system detects that this tenant has SSO enabled.
5. Instead of showing a password field, the user is redirected to Keycloak, which brokers the SAML authentication to the tenant's corporate IdP.
6. After successful IdP authentication, Keycloak issues tokens, and the user is redirected back to the platform with valid session tokens.
7. The Go backend receives standard Keycloak JWT tokens -- the same token format as password-based login. The `ValidateToken` method in `internal/auth/keycloak.go` processes them identically.

### Scenario 3: First-Time SSO User (JIT Provisioning)
A new employee at an SSO-enabled tenant logs in for the first time. They authenticate successfully through the corporate IdP, but no local user record exists in the `users` table. The system must create a user record on the fly (JIT provisioning). The existing `GetOrCreateByEmail` method in `internal/user/service.go` already handles this pattern for password-based login -- it creates a user with default role "user" if none exists. This same mechanism should work for SSO users, but the `KeycloakID` field (currently nullable in the schema) needs to be populated from the Keycloak `sub` claim in the SSO token.

### Scenario 4: Non-SSO Tenant User Logs In (Unchanged)
A user whose tenant does not have SSO enabled (e.g., plan = "free" or "pro") continues to log in with email/password via the existing `POST /api/auth/login` endpoint. The flow through `auth.Service.Login` -> `keycloak.Authenticate` (direct grant) remains completely unchanged.

### Scenario 5: Mixed Tenant Transition
An existing tenant with users already using email/password authentication enables SSO. Existing users must be able to authenticate via SSO without losing their accounts. This requires an account linking strategy: when an SSO-authenticated user's email matches an existing local user record, the system links them rather than creating a duplicate. The `GetOrCreateByEmail` method already does email-based matching, which provides a natural linking mechanism -- but the `keycloak_id` may differ between the password-based Keycloak account and the SSO-brokered identity.

### Scenario 6: SSO Login Failure / IdP Unavailable
The corporate IdP is down or returns an error. Keycloak handles the SAML error response and redirects back with an error. The platform must display a meaningful error to the user. No fallback to password authentication should occur for SSO-enforced tenants (this is a security requirement -- allowing password fallback defeats the purpose of SSO enforcement).

## Expected Outcomes

1. Enterprise tenants can authenticate users through their corporate SAML IdP.
2. Non-SSO tenants experience zero changes to their authentication flow.
3. Per-tenant SSO configuration is stored and queryable (new data model required -- current `Tenant` struct has no SSO fields).
4. Keycloak acts as the SAML SP/broker -- the Go backend does NOT implement SAML protocol directly.
5. Post-authentication token handling is identical for SSO and password users (same JWT format from Keycloak).
6. New SSO users are provisioned automatically on first login.

## Edge Cases

### Email Domain Collision
The current schema has `email_domain TEXT NOT NULL UNIQUE` on the `tenants` table. This enforces one-tenant-per-domain. If an enterprise acquires a subsidiary with a different domain, they cannot have one SSO configuration cover both domains. This is a schema limitation, not immediately blocking, but worth noting.

### Multiple IdPs per Tenant
Some large enterprises have multiple IdPs (e.g., different divisions). The current requirement scopes to one SAML IdP per tenant. This simplifies the initial implementation but should be designed with extensibility in mind (e.g., using a separate `tenant_sso_configs` table rather than adding columns to `tenants`).

### Account Linking Conflicts
If a user already exists with email `jane@acmecorp.com` (created via password login) and then SSO is enabled for `acmecorp.com`, the SSO login will return a different Keycloak `sub` (the brokered identity ID). The `keycloak_id` in the `users` table may not match. The system must handle this gracefully -- either by updating `keycloak_id` on first SSO login or by matching on email alone.

### SSO-Only Enforcement
Once SSO is enabled for a tenant, should password login be blocked for that tenant's users? If not blocked, users could bypass SSO by using `POST /api/auth/login` directly. The `auth.Service.Login` method must check whether the resolved tenant has SSO enabled and, if so, reject direct-grant (password) authentication.

### Tenant Plan Gating
SSO is typically an enterprise-tier feature. The `plan` field on `Tenant` (values: "free", "pro", "enterprise") could be used to gate SSO configuration. Only tenants with `plan = "enterprise"` should be allowed to enable SSO.

### Token Claims Differences
Tokens from SSO-brokered authentication may have different claim structures than direct-grant tokens. The `ValidateToken` method in `keycloak.go` extracts `sub`, `email`, `tenant_id` (custom claim), and `realm_access.roles`. The custom `tenant_id` claim may not be present in SSO-brokered tokens unless Keycloak is configured with a mapper to inject it. This is a Keycloak configuration concern that must be addressed.

### User Deactivation
If a user is deactivated in the corporate IdP but still has an active local record (`is_active = true` in `users` table), what happens? Keycloak should reject their SAML assertion, so the token will never be issued. But if there is a timing gap (e.g., session still valid), the `Authenticate` middleware will still accept the token until it expires. This is standard JWT behavior and acceptable for v1.

## Success Signals

1. An enterprise tenant admin can provide IdP metadata and have SSO configured for their tenant.
2. Users with email domains matching an SSO-enabled tenant are redirected to their corporate IdP login.
3. After IdP authentication, users land in the application with valid sessions and correct tenant/role context.
4. JIT-provisioned users appear in the `users` table with correct `tenant_id` and `keycloak_id`.
5. Password-based login for non-SSO tenants works exactly as before (regression-free).
6. Password-based login is blocked for SSO-enabled tenants (security enforcement).
7. The `GET /api/users/me` endpoint returns correct user data for SSO-authenticated users.
8. Admin endpoints (`GET /api/admin/users`, `PUT /api/admin/tenants/{tenantID}`) work correctly with SSO-authenticated admin users.
