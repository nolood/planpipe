# Agreement Package

> Task: Add SAML 2.0 SSO to multi-tenant Go backend
> Based on: Stage 2 analyses (product, system, constraints/risks)
> Purpose: Confirm or correct the synthesized understanding before planning

---

## Block 1 — Goal & Problem Understanding

**Our understanding:**
Enable enterprise tenants to authenticate their users via SAML 2.0 SSO through corporate identity providers, with Keycloak brokering the SAML protocol. Existing email/password login remains fully functional. This is table-stakes for enterprise sales and Q3 delivery is prioritized.

**Expected outcome:**
Enterprise users can log in through their corporate IdP via SAML. Non-SSO tenants see no changes. SSO config is per-tenant.

**Confirm:** Is this the right goal? Are we solving the right problem? Is this the outcome you need?

---

## Block 2 — Scope

**Included:**
- SP-initiated SAML 2.0 SSO login flow via Keycloak broker
- Per-tenant SSO configuration storage (dedicated `tenant_sso_config` table)
- JIT user provisioning for SSO users
- Account linking by email for existing password users
- Keycloak SAML IdP configuration automation via Admin API
- SSO login and callback endpoints
- Password login available as fallback for SSO tenants (dual-auth)

**Excluded:**
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning
- Self-service SSO configuration UI
- OIDC/OAuth SSO
- Multi-realm Keycloak architecture

**Confirm:** Is the scope correct? Anything missing? Anything that shouldn't be here?

---

## Block 3 — Key Scenarios

**Primary scenario:**
User enters email → SSO check → redirect to Keycloak with IdP hint → SAML auth at corporate IdP → callback with auth code → JIT provisioning → JWT tokens returned.

**Mandatory edge cases:**
- Existing password user + SSO enabled → auto email linking
- SSO user attempts password login → allowed (dual-auth)
- Unknown email domain → normal password login
- Keycloak/IdP unavailable → graceful error, password fallback
- Missing SAML attributes → reject with clear error

**Deferred (not in this task):**
- Multi-domain per tenant SSO mapping
- SSO-only enforcement mode
- Self-service SSO config UI
- SAML metadata auto-rotation

**Confirm:** Is the primary scenario correct? Are the mandatory edge cases right? Can the deferred items really wait?

---

## Block 4 — Constraints

- Existing `POST /api/auth/login` must work identically for non-SSO tenants
- Single Keycloak realm — all tenant IdPs coexist with unique aliases
- Repository-service-handler pattern must be followed
- JWT claims must include `sub`, `email`, `tenant_id`, `realm_access.roles`
- Per-tenant SSO config goes in a dedicated table, not JSONB
- Q3 delivery timeline — scope decisions favor MVO

**Confirm:** Are these constraints accurate? Are there constraints we missed? Can any of these be relaxed?

---

## Block 5 — Candidate Solution Directions

Based on the analysis, we see these possible directions:

- **Minimal MVO:** Hardcoded SAML config, minimal Keycloak integration, manual IdP setup. Fast but doesn't scale.
- **Systematic:** Proper SSO subsystem with dedicated config storage, automated Keycloak setup, clean separation. More work but foundational for enterprise sales.

**Confirm:** Which direction do you prefer? Minimal and safe, or systematic and thorough? Any direction we should avoid?
