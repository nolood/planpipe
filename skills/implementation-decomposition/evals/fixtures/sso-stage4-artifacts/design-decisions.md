# Design Decisions

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Total decisions: 5
> User-approved: 4 of 5

## Decision 1: Dedicated `tenant_sso_config` Table

**Decision:** Store per-tenant SSO configuration in a dedicated database table rather than a JSONB column on the tenants table.

**Context:** SSO configuration includes multiple structured fields (domain, entity ID, metadata URL, certificate, enabled flag). The question was whether to store this as a JSONB blob on the existing tenants table or as a separate normalized table.

**Reasoning:** User chose the dedicated table for queryability (can query by domain directly) and type safety (database enforces constraints). The trade-off of an extra table and JOIN is acceptable for the benefits.

**Alternatives considered:**
- **JSONB column on tenants:** Simpler schema, single table → Rejected because: user valued queryability and type safety over schema simplicity
- **Separate config service/table with foreign key:** Over-normalized for the current need → Rejected because: SSO config is inherently tied to a tenant

**Trade-offs accepted:**
- Extra table adds a JOIN for tenant queries that need SSO info
- Migration adds a new table (minor operational overhead)

**User approval:** approved

**Impact:** Tenant module (model + repository), migrations, auth module (queries by domain)

---

## Decision 2: Automatic Email-Based Account Linking

**Decision:** When a user authenticates via SSO for the first time, automatically link their Keycloak identity to an existing account if the email matches.

**Context:** Users may already exist in the system (created via password registration). When they first use SSO, we need to decide how to connect their SSO identity with their existing account.

**Reasoning:** User preferred automatic linking over admin-driven because it's frictionless. Email is a reliable identifier in this system (emails are unique and verified). The alternative of requiring admin action would create a support burden.

**Alternatives considered:**
- **Admin-driven linking:** Admin manually maps SSO users to existing accounts → Rejected because: creates support burden, user preferred frictionless experience
- **User self-service linking:** User logs in with password, then links SSO in settings → Rejected because: adds UI complexity outside of scope

**Trade-offs accepted:**
- Relies on email uniqueness and verification (both true in this system)
- Race condition possible if same email used in concurrent SSO + password registration (mitigated by DB unique constraint)

**User approval:** approved

**Impact:** User module (FindOrCreateBySSO logic), auth module (callback flow)

---

## Decision 3: Dual-Auth (SSO + Password Fallback)

**Decision:** When SSO is enabled for a tenant, users can still log in with their password. SSO does not replace password authentication.

**Context:** The original proposal was SSO-only mode for configured tenants. The user explicitly requested that password authentication remain available even when SSO is enabled.

**Reasoning:** Provides a safety net if SSO/Keycloak has issues. Allows gradual SSO adoption within a tenant. Reduces risk of locking users out.

**Alternatives considered:**
- **SSO-only mode:** Disable password auth when SSO is enabled → Rejected because: user required fallback capability
- **Configurable per tenant:** Allow each tenant to choose SSO-only or dual-auth → Rejected because: adds complexity, can be added later if needed

**Trade-offs accepted:**
- Users might continue using passwords even after SSO is available (acceptable per user)
- No forced SSO adoption — tenant admins can't enforce SSO-only login (can be added as future enhancement)

**User approval:** approved

**Impact:** Auth module (login flow remains unchanged), no enforcement logic needed

---

## Decision 4: Single Keycloak Realm with Per-Tenant IdP

**Decision:** Use a single Keycloak realm for all tenants, with one Identity Provider configuration per tenant (named `tenant-{id}-saml`).

**Context:** Keycloak supports multiple realms for isolation, but managing many realms adds operational complexity. The question was whether to isolate tenants in separate realms or use a single realm with multiple IdPs.

**Reasoning:** Single realm simplifies operations (one set of clients, one set of roles). Tenant isolation is achieved via unique IdP alias naming. This is sufficient because Keycloak's IdP broker correctly routes users based on the IdP they authenticate with.

**Alternatives considered:**
- **Realm per tenant:** Full isolation per tenant → Rejected because: operational overhead of managing N realms; not needed for the current security model

**Trade-offs accepted:**
- Logical isolation only (not physical realm isolation)
- All tenants share the same Keycloak client configuration
- If one IdP misconfiguration causes issues, it could theoretically affect the realm (low risk)

**User approval:** approved

**Impact:** Keycloak client (IdP naming convention), SSO service (realm is a config parameter, not per-tenant)

---

## Decision 5: Keycloak Admin REST API (Not CLI)

**Decision:** Use Keycloak's Admin REST API directly from the Go application for programmatic IdP management, rather than using the `kcadm.sh` CLI tool or a Terraform provider.

**Context:** When a tenant enables SSO, their external IdP configuration needs to be created in Keycloak. This can be done via the Admin REST API, the `kcadm.sh` CLI, or infrastructure-as-code tools like Terraform.

**Reasoning:** The Admin REST API allows the application to manage IdP configurations programmatically without external tooling. This is simpler to deploy and test than shelling out to a CLI tool or maintaining separate Terraform configs.

**Alternatives considered:**
- **`kcadm.sh` CLI:** Shell out to Keycloak CLI tool → Rejected because: requires CLI installed alongside app, harder to test, subprocess management complexity
- **Terraform provider:** Manage IdP configs as infrastructure → Rejected because: adds IaC dependency, slower feedback loop, not suitable for dynamic tenant onboarding

**Trade-offs accepted:**
- Application needs Keycloak admin credentials (stored in config, not hardcoded)
- HTTP client adds some complexity to the auth module

**User approval:** not required (implementation detail, no user-facing impact)

**Impact:** Auth module (new KeycloakClient), config (admin credentials)

---

## Deferred Decisions

- **Multi-domain mapping:** A tenant may have multiple email domains. Deferred because: current model supports one domain per config row; multiple rows can be added per tenant if needed. Revisit when multi-domain support is explicitly requested.
- **SSO enforcement mode:** The ability for tenant admins to disable password auth and enforce SSO-only. Deferred because: user chose dual-auth for now. Can be added as a boolean flag on `tenant_sso_config` later.
