# Agreement Package

> Task: Add SAML 2.0 SSO support to multi-tenant Go backend via Keycloak SAML broker, preserving existing email/password login
> Based on: Stage 2 analyses (product, system, constraints/risks)
> Purpose: Confirm or correct the synthesized understanding before planning

---

## Block 1 — Goal & Problem Understanding

**Our understanding:**
Enterprise adoption is blocked because the platform only supports email/password authentication. Enterprise organizations require SAML SSO so their employees authenticate through corporate identity providers -- this is table-stakes for security compliance and credential lifecycle management. Active enterprise deals are contingent on SSO support, making Q3 delivery a business priority.

The task is to add SP-initiated SAML 2.0 SSO as a second authentication method, with Keycloak acting as the SAML SP/broker. Each tenant independently decides whether to enable SSO and provides their IdP's SAML metadata. The existing email/password flow is untouched for non-SSO tenants.

**Expected outcome:**
An enterprise employee enters their corporate email, the system detects the SSO-enabled tenant, redirects through Keycloak to the corporate IdP, and the employee lands authenticated in the application with a valid JWT -- indistinguishable from a password-authenticated session for all downstream behavior. At least one enterprise tenant can complete this flow end-to-end. Email/password login remains identical for all non-SSO tenants.

**Confirm:** Is this the right goal? Are we solving the right problem? Is this the outcome you need?

---

## Block 2 — Scope

**Included:**
- SP-initiated SAML 2.0 SSO login flow (email -> tenant detection -> Keycloak redirect -> IdP auth -> JWT callback)
- Per-tenant SSO configuration storage in PostgreSQL (new schema)
- Keycloak SAML IdP broker setup per tenant (within the existing single realm)
- JIT user provisioning on first SSO login (create local user record with keycloak_id)
- Account linking for existing password users (by email match) when their tenant enables SSO
- Authorization code flow support in the Keycloak client (alongside existing direct grant)
- SSO initiation and callback HTTP endpoints
- Login flow routing: detect SSO tenant and direct to appropriate auth method
- Backward-compatible email/password login for non-SSO tenants (unchanged API contract)

**Excluded:**
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning/deprovisioning
- Self-service SSO configuration UI (API-only or manual config for now)
- Multiple IdPs per tenant
- OIDC/OAuth SSO (SAML 2.0 only)
- Admin UI for SSO management

**Confirm:** Is the scope correct? Anything missing? Anything that shouldn't be here?

---

## Block 3 — Key Scenarios

**Primary scenario:**
Enterprise employee enters corporate email at login. System detects SSO-enabled tenant by email domain. System redirects to Keycloak SAML broker, which redirects to corporate IdP. Employee authenticates at IdP. SAML assertion flows back through Keycloak, which issues an authorization code. Application exchanges code for JWT, provisions user if first login, and the employee is authenticated.

**Mandatory edge cases:**
- First SSO login (JIT provisioning): new user record created from SAML attributes with keycloak_id
- Existing password user enabling SSO: link SSO identity to existing user by email match rather than creating a duplicate
- SSO misconfiguration: meaningful error message, no impact on other tenants
- IdP outage: graceful failure with clear error (password fallback policy is an open question)
- Non-SSO tenant login: completely unchanged behavior

**Deferred (not in this task):**
- IdP-initiated SSO (enterprise secondary requirement)
- Single Logout (session expiry + manual logout covers MVO)
- SCIM provisioning (manual deactivation acceptable initially)
- Self-service SSO configuration UI (API/manual is acceptable for launch)

**Confirm:** Is the primary scenario correct? Are the mandatory edge cases right? Can the deferred items really wait?

---

## Block 4 — Constraints

- **Backward compatibility (hard):** Existing `POST /api/auth/login` must work identically for non-SSO tenants -- same request/response contract, same behavior, same error messages
- **Single Keycloak realm:** All tenant SAML IdP configurations share the `platform` realm. Tenant isolation is at the IdP-alias level, not realm level
- **JWT claim compatibility:** SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, and `realm_access.roles` for middleware and handler compatibility
- **Repository-service-handler pattern:** All new code must follow the existing layering convention
- **No per-tenant config exists yet:** SSO config storage design sets a precedent for future per-tenant features
- **Q3 delivery timeline:** Scope decisions should favor MVO delivery
- **SAML 2.0 only:** No OIDC/OAuth SSO in this task
- **Email UNIQUE globally:** Same email cannot exist in two tenants -- email-domain-to-tenant mapping must be 1:1
- **Zero test coverage:** No existing tests anywhere in the codebase -- high regression risk for auth flow changes
- **No SAML assertion/token logging:** Sensitive authentication data must not appear in logs

**Confirm:** Are these constraints accurate? Are there constraints we missed? Can any of these be relaxed?

---

## Block 5 — Candidate Solution Directions

Based on the analysis, we see these possible directions:

- **Minimal / MVO-first:** Implement the happy-path SSO flow. Manual Keycloak IdP configuration per tenant. Separate `tenant_sso_config` table. API-only config management. Account linking by email match. Goal: one enterprise tenant through SSO end-to-end as fast as possible. Trade-off: each new tenant requires manual Keycloak admin work.

- **Systematic / automation-included:** Same core SSO flow, but include Keycloak Admin API automation for programmatic IdP provisioning from the start. Trade-off: more upfront work, but tenant onboarding scales without manual intervention. Better for 5+ tenants.

- **Safe / test-first:** Before any SSO code, establish integration tests for the existing login flow. Build SSO with tests alongside. Trade-off: slower start, significantly lower regression risk, test investment pays dividends beyond SSO.

- **Recommended hybrid:** Safe + minimal first (tests + MVO), then systematic (automation) as follow-up. Gets the essential safety net in place, delivers MVO for enterprise unblocking, and defers automation to when scale demands it.

**Confirm:** Which direction do you prefer? Minimal and fast, or safe and tested? Should automation be included upfront or deferred? Any direction we should avoid?
