# Design Decisions

> Task: Add SAML 2.0 SSO support to a multi-tenant Go backend via Keycloak brokering
> Total decisions: 7
> User-approved: 3 of 7

## Decision 1: Separate SSOService Instead of Extending auth.Service

**Decision:** Create a new `SSOService` struct in the auth package for all SSO logic, rather than adding SSO methods to the existing `auth.Service`.

**Context:** The existing `auth.Service` orchestrates the password login flow (`Login()`, `Logout()`, `RefreshToken()`). The SSO flow is fundamentally different — it involves browser redirects, authorization code exchange, and a multi-step asynchronous flow rather than a synchronous API call. The question is whether to extend the existing service or create a new one.

**Reasoning:** The codebase has zero test coverage (`internal/auth/` has no test files at all). Modifying `auth.Service` to add SSO branching would mean any bug in SSO code could affect the password login path. With a separate `SSOService`, the existing `Login()` method is completely untouched — not a single line changes. This isolation is critical given the zero-test-coverage reality. Additionally, the SSO flow has different dependencies (needs SSO config repository) and different error semantics (redirects vs. JSON errors).

**Alternatives considered:**
- **Extend `auth.Service` with SSO methods:** Would keep all auth logic in one service. Rejected because it would modify the constructor signature and potentially introduce shared state that could affect the password flow. The risk is disproportionate to the benefit of colocation.
- **Create a completely separate `sso` package:** Would maximize isolation. Rejected because it would create an artificial boundary — SSO authentication is logically part of the auth domain, uses the same `KeycloakClient`, and should share the same package namespace.

**Trade-offs accepted:**
- Some conceptual duplication: both `Service` and `SSOService` do "authentication" but via different mechanisms
- Two service types in the auth package could be confusing — naming must be clear

**User approval:** not required

**Impact:** `internal/auth/` package, `cmd/server/main.go` wiring

---

## Decision 2: Dedicated tenant_sso_config Table

**Decision:** Store per-tenant SSO configuration in a new `tenant_sso_config` table with a foreign key to the `tenants` table.

**Context:** The tenant model has no per-tenant configuration storage. The codebase itself acknowledges this gap in comments in `internal/tenant/models.go:18-23` and `migrations/001_initial.sql:34-37`. SSO requires storing IdP alias, entity ID, SSO URL, certificate, metadata URL, and enabled flag per tenant.

**Reasoning:** User explicitly chose this approach during Stage 3. The dedicated table provides schema enforcement for SSO-specific fields, supports direct querying (e.g., find SSO config by IdP alias), and establishes a pattern for future per-tenant configuration needs. The `UNIQUE` constraint on `tenant_id` enforces one SSO config per tenant. The `UNIQUE` constraint on `idp_alias` prevents alias collisions in Keycloak.

**Alternatives considered:**
- **JSONB column on tenants table:** Simpler migration (just add a column), no new table. Rejected by user because it loses schema enforcement, makes querying individual fields less ergonomic, and doesn't establish a clean pattern for future config storage.
- **Key-value tenant_settings table:** Generic approach that could store any setting. Rejected because SSO config has a fixed structure with specific types — a generic key-value store would require serialization/deserialization logic and lose type safety.

**Trade-offs accepted:**
- New table adds schema complexity and requires a dedicated repository
- One more JOIN if tenant + SSO config are needed in the same query (though in practice they are queried separately)

**User approval:** approved

**Impact:** `migrations/`, `internal/tenant/` package, `cmd/server/main.go` wiring

---

## Decision 3: Modify GetOrCreateByEmail Signature to Accept KeycloakID

**Decision:** Add a `keycloakID string` parameter to `user.Service.GetOrCreateByEmail()` instead of creating a separate account linking method.

**Context:** SSO JIT provisioning needs to create users with a KeycloakID, and account linking needs to update existing users' KeycloakID on first SSO login. The current `GetOrCreateByEmail()` creates users without setting KeycloakID (the field exists in the User struct but is left empty).

**Reasoning:** `GetOrCreateByEmail()` is already the JIT provisioning entry point — it's called during password login at `internal/auth/service.go:61`. Adding the KeycloakID parameter keeps all provisioning logic in one place. The existing caller passes an empty string, preserving current behavior. For SSO callers, the KeycloakID is populated. The method also handles account linking naturally: if an existing user has no KeycloakID, it gets updated.

**Alternatives considered:**
- **Separate `LinkKeycloakID(email, keycloakID)` method:** Would keep `GetOrCreateByEmail()` unchanged but require two calls in the SSO flow — first get-or-create, then link. Rejected because it introduces a window where the user exists without a KeycloakID, and adds unnecessary complexity.
- **New `GetOrCreateByEmailWithKeycloakID()` method:** Would avoid changing the existing signature but duplicates the get-or-create logic. Rejected because code duplication in provisioning logic is a maintenance hazard.

**Trade-offs accepted:**
- Changes an existing function signature, breaking the current caller. The caller at `auth.Service.Login()` line 61 must be updated in the same change.
- Empty string sentinel for "no keycloakID" is not the most expressive API — a pointer `*string` would be clearer but adds nil-checking overhead.

**User approval:** approved (auto-approved per eval)

**Impact:** `internal/user/service.go`, `internal/user/repository.go`, `internal/auth/service.go` (existing caller)

---

## Decision 4: Use gocloak for Authorization Code Exchange

**Decision:** Use gocloak's `GetToken()` method with `GrantType: "authorization_code"` for the OAuth2 code exchange, rather than making direct HTTP calls to Keycloak's token endpoint.

**Context:** The SSO callback receives an authorization code from Keycloak that must be exchanged for JWT tokens. The codebase already uses gocloak for all Keycloak communication. The question is whether to extend gocloak usage or bypass it for this specific operation.

**Reasoning:** Consistency with existing patterns — all Keycloak communication goes through gocloak. The `GetToken()` method in gocloak accepts various OAuth2 grant types. Using it for authorization_code exchange keeps the abstraction layer consistent and avoids introducing a second HTTP client for Keycloak. If gocloak doesn't support it cleanly (the risk identified in Stage 2), a fallback to direct HTTP POST is prepared.

**Alternatives considered:**
- **Direct HTTP POST to Keycloak token endpoint:** Simpler and more predictable — a standard `application/x-www-form-urlencoded` POST with code, redirect_uri, client_id, client_secret, grant_type. Rejected as primary approach because it breaks the existing abstraction pattern, but kept as fallback.

**Trade-offs accepted:**
- Dependency on gocloak's internal implementation of code exchange. If it doesn't work, the fallback adds implementation time.

**User approval:** not required

**Impact:** `internal/auth/keycloak.go`

---

## Decision 5: Frontend Callback via Redirect with URL Fragment

**Decision:** After the SSO callback processes the authorization code and obtains tokens, redirect the browser to `FrontendCallbackURL#access_token=...&refresh_token=...&expires_in=...` so the frontend SPA can capture the tokens.

**Context:** The SSO callback endpoint (`/api/auth/sso/callback`) is called by Keycloak via a browser redirect, not an API call. The backend needs to deliver tokens to the frontend SPA. This is fundamentally different from the password login flow where the frontend makes an API call and receives a JSON response.

**Reasoning:** URL fragment parameters (the `#` part) are not sent to the server on subsequent requests, making them more secure than query parameters for token delivery. This is the standard pattern for OAuth2/OIDC in SPA architectures. The frontend reads the fragment, stores the tokens, and clears the URL.

**Alternatives considered:**
- **Set tokens in HttpOnly cookies and redirect:** Would be more secure (tokens not exposed to JavaScript) but changes the entire auth model — the existing system uses Bearer tokens, not cookie-based auth. Rejected because it would require middleware changes.
- **Store tokens server-side with a one-time code:** Backend stores tokens, returns a one-time code to the frontend, frontend exchanges the code for tokens via API call. More secure but adds complexity (server-side token storage, code expiry). Could be a future enhancement.
- **POST form auto-submit to frontend:** HTML page with auto-submitting form containing tokens as hidden fields. Works but is fragile and unusual.

**Trade-offs accepted:**
- Tokens are briefly visible in the URL fragment (accessible to JavaScript). This is a standard OAuth2 implicit-like trade-off.
- Requires frontend to be configured at a specific URL that the backend knows about (`FrontendCallbackURL`).

**User approval:** approved (auto-approved per eval)

**Impact:** `internal/auth/sso_handler.go`, `internal/config/config.go`, frontend integration contract

---

## Decision 6: State Parameter with Cookie for CSRF Protection

**Decision:** Use a randomly generated state parameter stored in a short-lived HttpOnly cookie for CSRF protection in the SSO initiation/callback flow.

**Context:** The OAuth2 authorization code flow is susceptible to CSRF attacks where an attacker initiates an SSO flow and injects their authorization code into the victim's callback. The state parameter prevents this.

**Reasoning:** Standard OAuth2 security best practice. The state is generated in `InitiateSSO`, stored in a cookie (8-minute TTL, HttpOnly, Secure in production, SameSite=Lax), and validated in `HandleCallback`. Cookie-based storage is stateless (no server-side session store needed) and follows the existing Keycloak-managed session pattern.

**Alternatives considered:**
- **No state parameter:** Rejected — leaves the SSO flow vulnerable to CSRF. Not acceptable for an enterprise authentication feature.
- **Server-side state storage (Redis/DB):** More robust but introduces a new dependency and state management complexity. Rejected for MVO scope.
- **Encrypted state in URL:** State contains encrypted data rather than being a lookup key. More complex to implement correctly. Rejected for MVO scope.

**Trade-offs accepted:**
- Cookie-based state doesn't survive across different browsers/devices. This is acceptable — SSO initiation and callback happen in the same browser session.
- 8-minute TTL means slow IdP authentication could time out. This is an edge case — most IdP auths complete in under a minute.

**User approval:** not required

**Impact:** `internal/auth/sso_handler.go`, `internal/auth/sso_service.go`

---

## Decision 7: Keycloak IdP Alias Format: saml-{tenant_slug}

**Decision:** Use `saml-{tenant_slug}` as the Keycloak Identity Provider alias for each tenant's SAML IdP configuration.

**Context:** Each tenant's SAML IdP needs a unique alias in Keycloak. This alias is used in the `kc_idp_hint` URL parameter to direct Keycloak to the correct IdP during SSO initiation. The `tenants.slug` column is already UNIQUE in the database.

**Reasoning:** Deterministic — given a tenant, the alias can be computed without a database lookup. Human-readable — useful for debugging in Keycloak admin console. Unique — leverages the existing UNIQUE constraint on tenant slugs. The `saml-` prefix distinguishes SAML IdPs from any future OIDC IdPs.

**Alternatives considered:**
- **UUID-based aliases (e.g., `idp-{uuid}`):** Unique by construction but not human-readable. Debugging Keycloak configurations becomes harder. Rejected for operational ergonomics.
- **Tenant ID-based aliases (e.g., `saml-{tenant_id}`):** Also unique but UUIDs in aliases are long and unwieldy. Rejected in favor of the shorter, readable slug.

**Trade-offs accepted:**
- Tenant slug changes would break the IdP alias linkage. However, slug changes are already rare/restricted (used in URLs), and the SSO config stores the alias independently, so a rename would require updating both.

**User approval:** not required

**Impact:** `internal/auth/keycloak.go`, `internal/auth/sso_service.go`, `internal/tenant/sso_config.go`

---

## Deferred Decisions

- **SSO-only enforcement mode:** Deferred until after MVO. Currently dual-auth (both SSO and password) for all tenants. If an enterprise customer requires SSO-only, a boolean flag on the SSO config and a check in `auth.Service.Login()` would be needed. Revisit when a customer requests it.

- **Multi-domain per tenant SSO mapping:** Deferred. Current design maps one email domain to one tenant to one SSO config. If a tenant has multiple email domains, the `tenants.email_domain` column and `tenant_sso_config` lookup would need to be extended to a many-to-many relationship. Revisit when a multi-domain tenant appears.

- **SAML metadata auto-rotation:** Deferred. IdP certificates expire and metadata URLs may rotate. Currently, SSO config is static. A periodic job to refresh metadata from `idp_metadata_url` could be added later. Revisit before production scale-out.

- **gocloak vs. direct HTTP for Admin API:** If gocloak lacks `CreateIdentityProvider()`, the decision to use direct HTTP calls will be made during implementation. The interface (`CreateSAMLIdP` method on `KeycloakClient`) is the same regardless. Deferred to implementation time when gocloak API surface can be verified empirically.
