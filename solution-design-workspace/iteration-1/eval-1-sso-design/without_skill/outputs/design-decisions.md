# Design Decisions: SAML SSO Implementation

This document records every significant technical decision made during solution design, the alternatives considered, and the rationale for each choice.

---

## DD-1: SSO Endpoints as New Routes (Not Modifying Login)

**Decision:** Add three new endpoints (`/api/auth/sso/check`, `/api/auth/sso/initiate`, `/api/auth/sso/callback`) rather than modifying the existing `POST /api/auth/login` endpoint.

**Alternatives considered:**
1. **Modify `/api/auth/login` to detect SSO tenants and redirect** -- would change the API contract for all consumers
2. **Add SSO as an optional mode within the login handler** -- mixes two fundamentally different auth paradigms in one handler
3. **New dedicated SSO endpoints (chosen)** -- clean separation, zero impact on existing login

**Rationale:** The SSO flow is fundamentally different from password login. Password login is a synchronous API call (POST body -> JSON response). SSO login is a browser redirect flow (GET -> 302 -> IdP -> callback). Mixing these in a single endpoint creates complexity and API contract confusion. Separate endpoints are also easier to test, monitor, and deprecate independently.

**Impact:** Frontend must implement SSO detection logic (call `/check` before presenting login form). This is a small frontend change that cleanly separates concerns.

---

## DD-2: Three-Step SSO Flow (Check -> Initiate -> Callback)

**Decision:** Split the SSO flow into three endpoints rather than two.

**Alternatives considered:**
1. **Two-step: Initiate + Callback** -- frontend must know in advance whether to redirect to SSO
2. **Three-step: Check + Initiate + Callback (chosen)** -- frontend asks backend first, then decides

**Rationale:** The `/check` endpoint lets the frontend determine SSO availability before redirecting the user. Without it, the frontend would need to embed tenant-SSO-mapping logic or always try SSO first. The check endpoint is a lightweight JSON call that returns SSO status, keeping the frontend simple and the backend authoritative about SSO configuration. It also enables the frontend to show appropriate UI (e.g., "Sign in with Company SSO" button).

**Impact:** One additional API call during login. Negligible latency (single DB query). Significantly simpler frontend logic.

---

## DD-3: Keycloak `kc_idp_hint` for IdP Selection

**Decision:** Use Keycloak's `kc_idp_hint` query parameter to direct users to the correct SAML IdP, rather than building a custom IdP selection flow.

**Alternatives considered:**
1. **Let user choose IdP on Keycloak's login page** -- confusing for enterprise users who should go straight to their IdP
2. **Build custom IdP selection in the app** -- duplicates Keycloak functionality
3. **Use `kc_idp_hint` to skip Keycloak's login page (chosen)** -- sends users directly to their corporate IdP

**Rationale:** `kc_idp_hint` is a standard Keycloak feature designed for this exact use case. When the hint matches a configured IdP alias, Keycloak skips its own login page and redirects directly to the external IdP. This gives users the expected enterprise SSO experience (email -> immediate redirect to corporate login) without building a custom IdP selection layer.

**Impact:** Each tenant's IdP alias must be deterministic and stored in `tenant_sso_config.idp_alias`. The naming convention `saml-{tenant_slug}` ensures uniqueness within the single Keycloak realm.

---

## DD-4: Signed Cookie for CSRF State (Not Server-Side Store)

**Decision:** Use a signed HTTP cookie to carry the OAuth2 `state` parameter across the SSO redirect flow, rather than storing state server-side.

**Alternatives considered:**
1. **In-memory map with TTL** -- simple but not HA-safe, lost on restart
2. **Redis/database-backed state store** -- durable but adds infrastructure dependency
3. **Signed cookie (chosen)** -- stateless, HA-compatible, no external storage

**Rationale:** The state parameter serves one purpose: CSRF protection for the OAuth2 callback. It needs to survive one browser redirect roundtrip (typically < 60 seconds). A signed cookie with HMAC-SHA256 + short expiry achieves this without any server-side storage. The cookie is HttpOnly, Secure (in production), SameSite=Lax, with 5-minute MaxAge. The HMAC key comes from the `SSO_STATE_SECRET` environment variable.

**Impact:** Requires `SSO_STATE_SECRET` env var in all deployments. The state value is: `base64(timestamp|random_nonce|hmac(timestamp|nonce, secret))`. Validated by recomputing HMAC and checking timestamp freshness.

---

## DD-5: Separate `SSORepository` (Not Extending Existing `tenant.Repository`)

**Decision:** Create a new `SSORepository` struct in a new file rather than adding SSO methods to the existing `tenant.Repository`.

**Alternatives considered:**
1. **Add methods to existing `tenant.Repository`** -- fewer files, but violates SRP as the repo grows
2. **Separate `SSORepository` (chosen)** -- clean separation, follows pattern of one repo per table
3. **SSO package with its own repo** -- over-engineering for a feature that is conceptually part of tenant config

**Rationale:** The existing `tenant.Repository` handles the `tenants` table. SSO config lives in a separate `tenant_sso_config` table. Having a separate repository for a separate table follows the existing codebase pattern (each table has its own repository). It also makes the SSO repository independently testable and keeps the existing tenant repository unchanged (zero regression risk).

**Impact:** The `tenant.Service` gains an `ssoRepo` field. Its constructor changes from `NewService(repo)` to `NewService(repo, ssoRepo)`. This is a breaking signature change that requires updating `cmd/server/main.go`.

---

## DD-6: `GetOrCreateByEmailWithKeycloakID` as New Method (Not Modifying Existing)

**Decision:** Add a new `GetOrCreateByEmailWithKeycloakID` method in the user service rather than modifying the existing `GetOrCreateByEmail`.

**Alternatives considered:**
1. **Modify `GetOrCreateByEmail` to accept optional KeycloakID parameter** -- changes the existing method signature, all callers must be updated
2. **Add `keycloakID` as a field that `GetOrCreateByEmail` always sets if available** -- still modifies existing method semantics
3. **New parallel method (chosen)** -- existing method untouched, SSO has its own dedicated path

**Rationale:** The existing `GetOrCreateByEmail` is called by `auth.Service.Login()` (the password flow). Modifying it risks breaking password login. A new method isolates SSO-specific logic (account linking, KeycloakID mismatch detection, tenant verification) from the simpler password-flow provisioning. If SSO logic needs to evolve (e.g., attribute sync), it can change independently.

**Impact:** Some code duplication between the two methods (both do find-or-create by email). This is acceptable because the SSO variant has additional logic (KeycloakID handling, tenant validation) that makes it semantically different.

---

## DD-7: Token Delivery via HTML Bridge Page (Not JSON API)

**Decision:** The SSO callback endpoint returns an HTML page that communicates tokens to the frontend SPA, rather than returning a JSON response.

**Alternatives considered:**
1. **JSON response** -- not possible because the callback is a browser redirect from Keycloak, not an XHR call
2. **Redirect to frontend with tokens in URL fragment** -- exposes tokens in browser history, URL length limits
3. **Redirect to frontend with a short-lived auth code** -- adds complexity (need our own code-to-token exchange)
4. **HTML page with `window.postMessage` (chosen)** -- secure communication to the SPA, tokens never in URL

**Rationale:** The SSO callback is a full browser navigation (302 redirect from Keycloak). The browser expects an HTTP response, not a JSON API response. The HTML bridge page approach:
- Renders a minimal HTML page with embedded JavaScript
- Calls `window.opener.postMessage({tokens}, frontendOrigin)` if SSO was initiated in a popup
- Or stores tokens in `sessionStorage` and redirects to the frontend URL
- Tokens never appear in the URL or browser history
- The frontend origin is validated against `SSO_FRONTEND_URL` config to prevent token leakage

**Impact:** Frontend must implement either popup-based SSO initiation (with postMessage listener) or check sessionStorage on load after SSO redirect. Both are standard patterns for SPA + OAuth2 flows.

---

## DD-8: Admin API for SSO Configuration (Not Direct DB/Keycloak)

**Decision:** Provide admin-protected API endpoints for SSO configuration that handle both database storage and Keycloak IdP setup atomically.

**Alternatives considered:**
1. **Direct database manipulation** -- error-prone, Keycloak and DB can get out of sync
2. **Keycloak admin console only** -- no link to tenant in our DB, no automation
3. **Admin API endpoints that coordinate DB + Keycloak (chosen)** -- single source of truth, atomic operations

**Rationale:** SSO configuration requires two systems to be in sync: the `tenant_sso_config` table (which maps tenants to SSO settings) and the Keycloak realm (which has the actual SAML IdP configuration). An admin API endpoint that handles both ensures:
- Consistency: If Keycloak IdP creation fails, the DB record is not created
- Discoverability: Admins can query SSO config via the same API they use for tenant management
- Automation: CI/CD or onboarding scripts can configure SSO programmatically

**Impact:** Admin endpoints are behind auth middleware + admin role check. The `PUT` operation creates or updates (upsert semantics). Keycloak Admin API authentication uses the existing client credentials.

---

## DD-9: Single Keycloak Realm with Per-Tenant IdP Aliases

**Decision:** Configure all tenant SAML IdPs within the single `platform` realm using unique aliases (`saml-{tenant_slug}`), rather than creating per-tenant realms.

**Alternatives considered:**
1. **Per-tenant Keycloak realms** -- strong isolation but massive operational complexity
2. **Single realm, per-tenant IdP aliases (chosen, confirmed by user)** -- simpler, sufficient for <50 tenants

**Rationale:** The codebase is designed around a single Keycloak realm. The `KeycloakClient` stores one `realm` field. The auth middleware validates tokens against one realm. Introducing multi-realm would require fundamental changes to the auth architecture. Per-tenant IdP aliases within a single realm:
- Are a standard Keycloak pattern for multi-tenant SSO
- Support up to hundreds of IdPs per realm (well beyond the initial <50 tenant target)
- Keep the auth middleware unchanged (all tokens come from the same realm)
- Keep the Keycloak client configuration unchanged

**Impact:** IdP alias naming convention `saml-{tenant_slug}` must be enforced. Tenant slugs are already UNIQUE in the database, ensuring IdP alias uniqueness.

---

## DD-10: Email-Domain Tenant Resolution for SSO (Reusing Existing Pattern)

**Decision:** Use the existing email-domain-to-tenant resolution mechanism for SSO tenant detection, rather than introducing a new lookup mechanism.

**Alternatives considered:**
1. **Dedicated SSO domain mapping table** -- more flexible (multi-domain) but over-engineering for MVO
2. **Tenant ID in the SSO initiation URL** -- requires frontend to know tenant ID
3. **Email domain resolution (chosen, existing pattern)** -- reuses `tenant.GetByEmailDomain`, already proven in the codebase

**Rationale:** The codebase already resolves tenants from email domains in two places: `auth.Service.Login()` (line 38) and `auth.Middleware.Authenticate()` (line 64). The SSO flow follows the same pattern: user provides email, backend extracts domain, looks up tenant. This is consistent, requires no new database structures, and works for the initial deployment where email domain maps 1:1 to tenant.

**Impact:** Multi-domain tenants (e.g., acme.com and acme.co.uk mapping to the same tenant) are not supported. This is documented as a deferred item consistent with the agreed task model.

---

## DD-11: No Modification to Existing Login Endpoint for SSO Tenants

**Decision:** The existing `POST /api/auth/login` endpoint works identically for SSO-enabled tenants and non-SSO tenants. Password login is not blocked for SSO tenants.

**Alternatives considered:**
1. **Block password login for SSO tenants** -- user explicitly rejected this (chose dual-auth)
2. **Return a warning/hint for SSO tenants** -- adds complexity to the login response contract
3. **No change to login endpoint (chosen, per user decision)** -- dual-auth, both methods available

**Rationale:** User explicitly chose dual-auth over SSO-only enforcement. This simplifies implementation (zero changes to the login endpoint) and provides availability during the SSO transition period. Enterprise admins can still use password login if their IdP has issues.

**Impact:** Security consideration: enterprise security policies that require SSO enforcement are not met. This is documented as a deferred item ("SSO-only enforcement mode"). The platform depends on enterprise admins communicating SSO availability to their users.

---

## DD-12: Keycloak Admin API via gocloak with HTTP Fallback

**Decision:** Attempt SAML IdP management via gocloak's admin API methods first, with a fallback to direct HTTP calls if gocloak does not support the required SAML-specific fields.

**Alternatives considered:**
1. **gocloak only** -- risk: may not support SAML IdP config fields
2. **Direct HTTP only** -- works but reinvents what gocloak provides
3. **gocloak with HTTP fallback (chosen)** -- best of both, verified at implementation time

**Rationale:** gocloak v13.9.0 provides `CreateIdentityProvider` and `GetIdentityProvider` methods. These may support SAML provider types, but the SAML-specific config fields (entityId, singleSignOnServiceUrl, signingCertificate, etc.) are provider-specific and may not have typed fields in gocloak's structs. The design: try gocloak's generic `IdentityProviderRepresentation` first. If the SAML config map is not properly serialized, implement a `keycloakAdminHTTP` wrapper that makes direct `POST /admin/realms/{realm}/identity-provider/instances` calls with the correct JSON payload.

**Impact:** Implementation should verify gocloak support early (Phase 2 of implementation). If fallback is needed, it adds a `keycloak_admin_http.go` file with a thin HTTP client. No architectural impact either way.

---

## DD-13: JWT Tenant Resolution via Email Domain Fallback (Not Custom Mapper)

**Decision:** For SSO-issued JWTs, rely on the existing email-domain-based tenant resolution fallback in the auth middleware rather than configuring a custom Keycloak protocol mapper for tenant_id.

**Alternatives considered:**
1. **Custom Keycloak protocol mapper for tenant_id** -- requires Keycloak customization (custom authenticator or script mapper)
2. **Email domain fallback (chosen, already implemented)** -- works today, no Keycloak changes needed

**Rationale:** The auth middleware at `internal/auth/middleware.go:62-70` already handles the case where `tenant_id` is not in the JWT: it falls back to `tenantSvc.GetByEmailDomain(claims.Email)`. This fallback works correctly for SSO users because:
- SSO users always have an email in their JWT (NameID format is email)
- The email domain maps to exactly one tenant
- The tenant is verified as active (line 74-78)

Adding a custom Keycloak mapper would eliminate one DB query per request but requires non-trivial Keycloak configuration (either a script mapper that somehow knows the tenant mapping, or a custom authenticator in the first broker login flow). For MVO, the DB query is acceptable.

**Impact:** One additional DB query per authenticated request for SSO users (same as password users without tenant_id in JWT). This can be optimized later with a Keycloak protocol mapper or by caching tenant-by-domain lookups.

---

## DD-14: SSO Config as Separate Table with CASCADE Delete

**Decision:** Store SSO configuration in a dedicated `tenant_sso_config` table with `ON DELETE CASCADE` from the `tenants` table.

**This decision was made by the user during Stage 3 task synthesis.** The design implements it as specified.

**Rationale (from user):** A dedicated table provides better query flexibility, establishes a precedent for future per-tenant configuration tables, and keeps the tenants table focused on core tenant properties.

**Impact:** Schema migration required. FK relationship ensures SSO config is cleaned up when a tenant is deleted. UNIQUE constraint on tenant_id enforces one SSO config per tenant.

---

## DD-15: Pre-Existing Bug Handling (RequireRole Variable Shadowing)

**Decision:** Document the variable shadowing bug in `internal/auth/middleware.go:99-104` but do not fix it as part of the SSO implementation.

**Alternatives considered:**
1. **Fix it in the SSO PR** -- scope creep, risks git blame confusion
2. **Fix it in a separate PR before SSO** -- ideal but not blocking SSO
3. **Document and defer (chosen)** -- SSO does not use RequireRole in a way that triggers the bug

**Rationale:** The bug is at line 99-101 where the loop variable `r` (role string) shadows the `r` parameter (http.Request). This means `r.WithContext(r.Context())` calls string methods on the role, not request methods, which would cause a compilation error. However, the SSO admin endpoints will use `RequireRole("admin")`, so this bug must be fixed before SSO admin endpoints work correctly. Recommendation: fix in a preparatory PR.

**Impact:** If not fixed, the SSO admin endpoints (`GET/PUT/DELETE /api/admin/tenants/{tenantID}/sso`) will not work because they are behind `RequireRole("admin")`. The fix is straightforward: rename the loop variable from `r` to `role` (or rename the request parameter).
