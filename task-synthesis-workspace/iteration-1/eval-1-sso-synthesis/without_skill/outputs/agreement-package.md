# Agreement Package: SAML 2.0 SSO Feature

This document presents the synthesized understanding of the SSO SAML feature for your review and confirmation before planning begins. Each section requires your agreement or correction.

---

## A. Task Understanding

**What we are building:**
Add SAML 2.0 Single Sign-On to the multi-tenant Go backend so enterprise tenants can authenticate employees through their corporate identity providers (Okta, Azure AD, ADFS, etc.) via Keycloak acting as SAML SP/broker.

**What we are NOT building (explicitly excluded):**
- Self-service SSO configuration UI
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM provisioning
- Multiple IdPs per tenant
- OIDC/OAuth-based SSO

**Why it matters:**
Enterprise adoption is blocked. Organizations cannot enforce corporate authentication policies on the platform. Q3 delivery is prioritized to unblock active enterprise pipeline.

> **Please confirm:** Is this understanding of scope and exclusions correct?

---

## B. Primary Scenario

The main user flow we will implement:

1. Enterprise employee navigates to login and enters their corporate email (e.g., `jane@acmecorp.com`)
2. System extracts email domain, resolves tenant via `email_domain` field, detects SSO is enabled for this tenant
3. System redirects employee to Keycloak's SAML broker endpoint, which redirects to the tenant's corporate IdP
4. Employee authenticates at their corporate IdP (password, MFA, etc. -- whatever the IdP requires)
5. IdP sends SAML assertion to Keycloak. Keycloak validates it, maps attributes, issues a JWT
6. Application receives JWT via authorization code flow callback
7. If first SSO login: local user record is created (JIT provisioning) with data from SAML attributes
8. Employee is authenticated and lands in the application with correct tenant context and role

**Unchanged flow:** Non-SSO tenant users continue using email/password login via `POST /api/auth/login` with zero changes to behavior, API contract, or error messages.

> **Please confirm:** Is this flow description accurate? Any steps missing or incorrect?

---

## C. Architecture Decisions Requiring Confirmation

### C1. Modules to Modify

| Module | What Changes | Your Confirmation Needed |
|--------|-------------|------------------------|
| `internal/auth/` | New SSO initiation + callback endpoints, authorization code flow in Keycloak client, SSO branch in login service | Expected primary target |
| `internal/tenant/` | SSO configuration storage (model + repository + service) | Expected medium change |
| `internal/user/` | `GetOrCreateByEmail` enhanced to accept KeycloakID for SSO users | Expected small change |
| `internal/config/` | Possible SAML base config (SP entity ID, ACS URL) | Expected small change |
| `cmd/server/main.go` | Register new SSO routes in public group | Expected small change |
| `migrations/` | New migration for SSO config schema | Expected medium change |
| `internal/auth/middleware.go` | NO changes expected (validates JWTs regardless of issuance method) | Needs verification testing |

> **Please confirm:** Are the module change scopes accurate? Any modules missing?

### C2. What Must Not Break

These are hard compatibility requirements the plan must enforce:

1. `POST /api/auth/login` continues to work identically for non-SSO tenants (same request/response contract)
2. JWTs from SSO contain the same claims as password-issued JWTs: `sub`, `email`, `tenant_id`, `realm_access.roles`
3. Auth middleware context keys (`ContextKeyUserID`, `ContextKeyTenantID`, `ContextKeyRoles`) produce the same values for SSO-authenticated requests
4. All protected routes continue working without modification
5. Token format remains the same for downstream consumers

> **Please confirm:** Are these backward compatibility requirements complete?

---

## D. Decisions We Need From You

These are blocking questions that must be answered before planning can produce a concrete implementation plan. Each has significant architectural implications.

### D1. Password Fallback Policy for SSO Tenants

**Question:** When a tenant has SSO enabled, should password login be blocked or available as fallback?

**Option A -- Block password login (enterprise-preferred):**
- `POST /api/auth/login` with email/password returns an error for SSO tenants, directing user to SSO flow
- Enforces corporate security policies (no password bypass of IdP MFA)
- If the IdP is down, SSO users cannot authenticate at all
- Simpler to reason about security

**Option B -- Allow password fallback (availability-preferred):**
- Password login still works for SSO tenants as a backup
- Reduces availability risk during IdP outages
- Undermines the security enforcement that enterprises want SSO for
- More complex login routing logic

**Option C -- Configurable per-tenant:**
- Each tenant chooses whether password fallback is allowed
- Most flexible but adds complexity to config model and login routing
- Adds a field to the tenant SSO configuration

> **Your decision:** A, B, or C?

### D2. Account Linking Strategy

**Question:** When a tenant that already has password users enables SSO, what happens to existing users?

**Option A -- Automatic email-based linking:**
- When an existing user logs in via SSO, the system matches by email and updates their record with the Keycloak brokered subject ID
- Seamless for users
- Risk: email mismatch between IdP and existing account (rare but possible)

**Option B -- Admin-driven linking:**
- Admin manually maps existing users to their SSO identities
- Most controlled, no automatic assumptions
- Higher operational burden, especially for large tenants

**Option C -- Defer for MVO:**
- For MVO, SSO is only for new users or tenants without existing password users
- Existing password users at SSO-enabled tenants continue using passwords
- Account linking is added in a future iteration
- Simplest to implement now

> **Your decision:** A, B, or C?

### D3. Per-Tenant Config Storage Design

**Question:** How should per-tenant SSO configuration be stored? This is the first per-tenant configuration in the system and sets a precedent.

**Option A -- New `tenant_sso_config` table:**
- Separate table with FK to tenants
- Clean separation, normalized schema
- Easier to extend with additional fields later
- Slightly more complex queries (JOIN or separate fetch)
- Example: `tenant_sso_config(id, tenant_id FK, sso_enabled, provider_alias, metadata_url, metadata_xml, created_at, updated_at)`

**Option B -- JSONB column on tenants table:**
- Add `sso_config JSONB` column to existing tenants table
- Simpler migration, less schema change
- Flexible structure (can add fields without migration)
- Less type safety, harder to query/index specific fields
- Mixes configuration with core tenant data

**Option C -- New columns on tenants table:**
- Add `sso_enabled BOOL`, `sso_provider_alias TEXT`, `sso_metadata_url TEXT` directly to tenants table
- Simplest to implement
- Clutters tenants table if more per-tenant config is added later
- Rigid -- schema migration needed for every new field

> **Your decision:** A, B, or C?

---

## E. Risks We Plan to Mitigate

The following risks will be addressed in the implementation plan. Confirm you are comfortable with the mitigation approaches.

| # | Risk | Our Planned Mitigation |
|---|------|----------------------|
| R1 | Zero test coverage makes auth changes dangerous | Write integration tests for existing login flow BEFORE modifying it. All new SSO code includes tests. |
| R2 | Keycloak SAML broker config complexity at scale | Build automation via Keycloak Admin API. Create reusable IdP configuration templates. |
| R3 | Account linking for existing password users | Resolve via your answer to D2 above. |
| R4 | Keycloak client may need Standard Flow enabled | Test in staging first. Both flows coexist on the same client. |
| R5 | gocloak may not cover SAML IdP Admin API | Check gocloak source early in implementation. Prepare fallback: direct HTTP calls to Keycloak Admin REST API. |
| R6 | SAML attribute mapping varies by IdP vendor | Design standard attribute mapper template. Document required IdP attributes. |
| R7 | RequireRole middleware has a pre-existing variable shadowing bug | Fix before SSO work begins (low effort, reduces overall risk). |

> **Please confirm:** Are these mitigations acceptable? Any risks you want handled differently?

---

## F. MVO Boundary

What the Minimum Viable Outcome includes:

- [x] SP-initiated SAML SSO login for at least one enterprise tenant
- [x] JIT user provisioning for new SSO users
- [x] Per-tenant SSO configuration (API-only, no UI)
- [x] Tenant detection at login that routes to SAML flow
- [x] Keycloak as SAML SP/broker
- [x] Email/password login unaffected for non-SSO tenants
- [x] Integration tests for login flows (existing + new)

What is explicitly deferred:

- [ ] Self-service SSO configuration UI
- [ ] IdP-initiated SSO
- [ ] Single Logout (SLO)
- [ ] SCIM provisioning
- [ ] Multiple IdPs per tenant
- [ ] Multiple email domains per tenant
- [ ] SSO-specific monitoring/alerting dashboard

> **Please confirm:** Is this MVO scope correct? Anything that should move between the included/deferred lists?

---

## G. Pre-Existing Issues

During analysis, these pre-existing issues were discovered. They are not part of the SSO feature but affect it:

1. **Variable shadowing bug in `internal/auth/middleware.go:99-104`** -- `RequireRole` has a compilation error. Recommend fixing before SSO work.
2. **Zero test coverage** -- No tests exist anywhere. SSO work will establish the first tests.
3. **Unused `Session` struct in `auth/models.go`** -- Dead code. No action needed for SSO.

> **Please confirm:** Should we fix the RequireRole bug as part of SSO work, or track it separately?

---

## Summary of Decisions Needed

| # | Decision | Options | Default Recommendation |
|---|----------|---------|----------------------|
| D1 | Password fallback for SSO tenants | A (block) / B (allow) / C (configurable) | A -- Block password login. Enterprises want enforcement. |
| D2 | Account linking strategy | A (auto-email) / B (admin) / C (defer) | A -- Automatic email-based linking. Least friction. |
| D3 | Per-tenant config storage | A (new table) / B (JSONB) / C (columns) | A -- New table. Clean precedent for future per-tenant config. |
| Pre | Fix RequireRole bug | With SSO / Separately | With SSO -- low effort, reduces risk. |

Once you confirm or correct the above, we will proceed to create the implementation plan.
