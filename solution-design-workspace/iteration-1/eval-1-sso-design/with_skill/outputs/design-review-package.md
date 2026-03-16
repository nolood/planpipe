# Design Review Package

> Task: Add SAML 2.0 SSO support to multi-tenant Go backend via Keycloak brokering
> Solution direction: systematic
> Changes: 7 files modified, 5 new, 0 deleted across 6 modules

## Proposed Solution Summary

The implementation adds SAML 2.0 SSO as an additive layer alongside the existing email/password login — no existing authentication code is modified. Three new endpoints handle the SSO flow: one to check if a user's email domain has SSO enabled, one to initiate the SSO redirect to Keycloak (which brokers to the corporate IdP), and one to handle the callback after authentication. A new `tenant_sso_config` database table stores per-tenant SSO configuration (IdP details, certificates, enabled status). When an SSO user logs in for the first time, they are automatically provisioned as a local user with their Keycloak identity linked. Existing password users whose tenant enables SSO are automatically linked on their first SSO login via email matching.

## Key Changes

### SSO Authentication Flow
Three new public HTTP endpoints are added:
- `GET /api/auth/sso/check?email=...` — detects whether the user's tenant has SSO enabled
- `GET /api/auth/sso/initiate?email=...` — redirects the user to Keycloak, which redirects to the corporate IdP
- `GET /api/auth/sso/callback?code=...&state=...` — exchanges the authorization code for tokens, provisions/links the user, delivers tokens to the frontend

The existing `POST /api/auth/login` endpoint is completely unchanged. Password login continues to work for all tenants, including SSO-enabled ones (dual-auth as agreed).

### Per-Tenant SSO Configuration Storage
A new `tenant_sso_config` table (with FK to `tenants`) stores: IdP alias, entity ID, SSO URL, certificate, metadata URL, SP entity ID, and an enabled flag. One config per tenant. Admin creates/updates this via API or direct database access (no self-service UI in this iteration).

### JIT Provisioning and Account Linking
SSO users are provisioned on first login using the existing `GetOrCreateByEmail()` pattern, enhanced to populate the KeycloakID. Existing password users are automatically linked when they first use SSO — their KeycloakID is populated by matching on email.

### Keycloak Integration Extension
The existing Keycloak client gains methods for authorization code exchange (the SSO flow uses browser redirects instead of the direct password grant) and SAML IdP management via the Keycloak Admin API (to programmatically configure tenant IdPs).

## Approval Points

### Point 1: GetOrCreateByEmail Signature Change

**Context:** The existing `user.Service.GetOrCreateByEmail(email, tenantID)` function is the JIT provisioning entry point used during password login. SSO needs it to also accept a KeycloakID parameter so new SSO users are created with their Keycloak identity, and existing users are auto-linked.

**Options:**
- **Option A: Add keycloakID parameter to existing function** — Minimal change, one place for all provisioning logic. Requires updating the existing caller in `auth.Service.Login()` to pass an empty string.
- **Option B: Create separate GetOrCreateByEmailWithKeycloakID function** — Keeps existing function unchanged but duplicates provisioning logic.

**Recommendation:** Option A — it keeps provisioning logic in one place. The existing caller change is trivial (add `""` as the fourth argument) and both changes go in the same commit.

**Question:** Is it acceptable to modify the `GetOrCreateByEmail` function signature, knowing it requires a corresponding change to the existing password login caller?

---

### Point 2: Frontend Token Delivery Mechanism

**Context:** After the SSO callback processes the authorization code, the backend has JWT tokens that need to get to the frontend SPA. Unlike password login (where the frontend makes an API call and receives JSON), the SSO callback is a browser redirect from Keycloak — the backend can't return JSON directly.

**Options:**
- **Option A: Redirect with URL fragment** — `FrontendCallbackURL#access_token=...&refresh_token=...` — standard OAuth2/SPA pattern. Tokens are in the URL fragment (not sent to server). Frontend reads and clears them.
- **Option B: Server-side token storage with one-time code** — Backend stores tokens, returns a code, frontend exchanges code for tokens. More secure but adds server-side state management.

**Recommendation:** Option A for MVO. It's the industry standard for SPAs and avoids introducing server-side session storage. Option B can be added later as a security hardening measure.

**Question:** Is the URL fragment approach acceptable for token delivery to the frontend, or does the frontend require a different integration pattern?

---

### Point 3: Keycloak Client Standard Flow Requirement

**Context:** The SSO flow uses the OAuth2 authorization code flow, which requires the Keycloak client (`platform-app`) to have "Standard Flow" enabled. Currently, the client likely only has "Direct Access Grants" (password flow) enabled.

**Options:**
- **Option A: Enable both Standard Flow and Direct Access Grants on the same client** — Simplest. Both authentication methods use the same Keycloak client.
- **Option B: Create a separate Keycloak client for SSO** — More isolated but doubles the Keycloak client management.

**Recommendation:** Option A. Keycloak explicitly supports having both flows enabled on a single client. There is no security or functional conflict. This avoids doubling the configuration surface.

**Question:** Should Standard Flow be enabled alongside Direct Access Grants on the existing `platform-app` Keycloak client, or should a separate client be created for SSO?

## Risk Zones

- **gocloak authorization code exchange:** The gocloak library may not cleanly support the authorization_code grant type in its `GetToken()` method. If it doesn't work, direct HTTP calls to Keycloak's token endpoint will be used as a fallback. This should be verified early in implementation.

- **Keycloak SAML attribute mapping:** Different enterprise IdPs (Okta, Azure AD, ADFS) use different SAML attribute names. Keycloak attribute mappers must be configured per IdP to normalize these differences. This is a Keycloak configuration task, not a code task, but it requires documentation and potentially a template.

- **Zero test coverage:** The existing codebase has no tests. While the SSO code is isolated from the password flow, the `GetOrCreateByEmail()` signature change touches shared code. Implementation should include tests for the modified function and for all new SSO code.

- **Auth middleware JWT claim dependency:** The middleware expects `tenant_id` as a custom JWT claim. For SSO-authenticated users, Keycloak must include this claim in the brokered JWT via a protocol mapper. If not configured, the middleware falls back to email domain lookup (works but adds a DB query per request). The Keycloak tenant_id mapper configuration is a manual step that must be verified.

## Scope Confirmation

**In scope (per agreed model):**
- SP-initiated SAML 2.0 SSO login flow via Keycloak broker
- Per-tenant SSO configuration in dedicated table
- JIT user provisioning for SSO users
- Automatic email-based account linking
- Keycloak SAML IdP configuration automation via Admin API
- SSO check, initiate, and callback endpoints
- Password login remains as fallback (dual-auth)

**Not in scope (confirmed):**
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning
- Self-service SSO configuration UI
- OIDC/OAuth SSO (SAML 2.0 only)
- Multi-realm Keycloak architecture
- SSO-only enforcement mode
- Multi-domain per tenant SSO mapping

**Question:** Does this implementation scope match what you agreed to? Anything missing or extra?
