# Synthesized Task Analysis

## Task Goal

Add SAML 2.0 Single Sign-On support to the multi-tenant Go backend platform so that enterprise tenants can authenticate their employees through corporate identity providers, with Keycloak acting as the SAML SP/broker. The existing email/password login flow must remain fully functional for non-SSO tenants. SSO configuration is per-tenant -- each tenant independently enables SSO and provides their IdP's SAML metadata.

## Problem Statement

Enterprise adoption of the platform is blocked because organizations cannot enforce their corporate authentication policies. Enterprise IT departments require employees to authenticate through centralized identity providers (via SAML) for security compliance, audit trail continuity, and credential lifecycle management. Without SSO, enterprise deals require employees to maintain separate credentials outside corporate identity governance -- a non-starter for security-conscious organizations. Active enterprise pipeline pressure (Q3 prioritization) indicates deals are directly contingent on SSO support. The existing email/password flow works correctly; the gap is that it is the only authentication method available.

## Key Scenarios

### Primary Scenario
1. **Trigger:** Enterprise employee navigates to the login page and enters their corporate email (e.g., jane@acmecorp.com)
2. **Tenant detection:** System extracts the email domain, looks up the tenant via `tenant.Repository.GetByEmailDomain()`, and determines the tenant has SSO enabled by checking SSO configuration
3. **Redirect to IdP:** System redirects the employee to Keycloak's SAML broker endpoint, which in turn redirects to the tenant's configured corporate identity provider (Okta, Azure AD, ADFS, etc.)
4. **Corporate authentication:** Employee authenticates at their corporate IdP using existing corporate credentials (password, MFA, smart card -- whatever the IdP requires)
5. **SAML assertion return:** IdP sends a SAML assertion back to Keycloak. Keycloak validates the assertion, maps SAML attributes to Keycloak user attributes, and issues an authorization code
6. **Application callback:** Application receives the authorization code at the callback endpoint, exchanges it for a JWT via Keycloak's token endpoint. If this is the user's first SSO login, a local user record is created (JIT provisioning) with data from the SAML attributes
7. **End state:** Employee is authenticated and lands in the application with correct tenant context and role. The experience is indistinguishable from a password-authenticated session from this point forward -- same JWT claims, same context keys, same downstream behavior

### Mandatory Edge Cases
- **First SSO login (JIT provisioning):** User authenticates via SSO for the first time with no local user record. System must create a user record from SAML attributes (email, name) and associate with the correct tenant. The existing `GetOrCreateByEmail` pattern in `user.Service` provides a precedent, but SSO users also need `keycloak_id` populated from the brokered identity. Missing optional attributes (e.g., display name) need sensible defaults.
- **Existing password user on newly SSO-enabled tenant:** When a tenant enables SSO after users exist with email/password accounts, first SSO login must link the SSO identity to the existing local user record rather than creating a duplicate. Email-based matching is the natural approach -- find existing user by email, update their `keycloak_id` with the brokered subject.
- **SSO misconfiguration:** Tenant admin provides incorrect IdP metadata or IdP is misconfigured. SAML assertion fails validation at Keycloak. System must surface a meaningful error (not generic 500), and the error path must not affect other tenants.
- **IdP outage:** Tenant's corporate IdP is temporarily unavailable. SSO users at that tenant cannot authenticate. System should fail gracefully with a clear error message. Password fallback policy is an open question (see below).
- **Non-SSO tenant login unchanged:** Employee at a tenant without SSO enters email and password. System detects no SSO configuration and proceeds with the standard email/password direct grant flow -- identical behavior to current system.

### Deferred Scenarios
- **IdP-initiated SSO:** User starts login from the IdP portal rather than the application. Can be deferred -- SP-initiated covers the critical enterprise use case. Risk of deferring: some enterprise workflows expect IdP-initiated login, but this is typically a secondary requirement.
- **Single Logout (SLO):** Coordinated logout across the application and the IdP. Can be deferred -- session expiry and manual logout provide acceptable alternatives for MVO. Risk: enterprise security teams may ask for it during procurement review.
- **SCIM provisioning:** Automated user provisioning/deprovisioning from the IdP. Can be deferred -- JIT provisioning handles creation, and manual deactivation is acceptable initially. Risk: enterprises with strict offboarding requirements will need this eventually.
- **Self-service SSO configuration UI:** Admin UI for tenant SSO setup. Can be deferred -- API-only or manual configuration is acceptable for initial launch. Risk: onboarding friction increases with every new tenant.
- **Multiple IdPs per tenant:** A single tenant connecting to more than one IdP. Can be deferred -- most enterprises have a single IdP. Risk is low for MVO.

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Auth | `internal/auth/` | Primary change target -- new SSO endpoints (initiation + callback), SSO service logic, Keycloak authorization code flow integration | large |
| Tenant | `internal/tenant/` | SSO configuration storage, tenant SSO detection for routing login flow | medium |
| User | `internal/user/` | JIT provisioning enhancement -- `GetOrCreateByEmail` must accept and store `keycloak_id` for SSO-provisioned users | small |
| Config | `internal/config/` | Possible SAML-related base configuration (SP entity ID, ACS URL base) | small |
| Server | `cmd/server/main.go` | Route registration for new SSO endpoints in public route group | small |
| Migrations | `migrations/` | New migration for per-tenant SSO configuration schema | medium |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/handler.go` | New SSO initiation endpoint (accepts email, detects SSO tenant, returns Keycloak SAML broker redirect URL) and SSO callback endpoint (receives authorization code, exchanges for JWT, provisions user) | These are the two HTTP entry points for the SSO flow |
| `internal/auth/service.go:Login` | SSO tenant detection branch -- if tenant has SSO enabled, reject direct password login with error directing to SSO flow. New method for SSO callback processing | Login must become tenant-aware for auth method routing |
| `internal/auth/keycloak.go` | New methods: (1) Generate Keycloak SAML broker redirect URL for a specific IdP alias, (2) Exchange authorization code for tokens (authorization code flow instead of direct grant) | Current client only supports direct grant; SSO requires authorization code flow |
| `internal/tenant/models.go` | SSO configuration fields added to tenant model or new `SSOConfig` struct | No per-tenant config storage exists yet; SSO is the first feature requiring it |
| `internal/tenant/repository.go` | New queries for SSO config CRUD (load SSO settings by tenant ID, update SSO settings) | Data access layer for SSO configuration |
| `internal/tenant/service.go` | New methods: `GetSSOConfig(tenantID)`, `UpdateSSOConfig(tenantID, config)`, `IsSSOEnabled(tenantID)` | Service layer for SSO config operations |
| `migrations/002_sso_config.sql` | New table or columns for per-tenant SSO configuration (sso_enabled, provider_alias, metadata_url, etc.) | Schema foundation for SSO config storage |
| `internal/user/service.go:GetOrCreateByEmail` | Accept and store `keycloak_id` during user creation for SSO-provisioned users | Currently creates users without `keycloak_id` -- SSO users need this populated |
| `cmd/server/main.go` | Register SSO routes (initiation + callback) in the public route group | SSO endpoints must be accessible to unauthenticated users |

### Dependencies
- **Keycloak v24.0:** Must be configured with a SAML Identity Provider per tenant (unique alias, e.g., `saml-{tenant_slug}`), Standard Flow enabled on the `platform-app` client, and attribute mappers for `tenant_id` claim in brokered tokens. Keycloak handles all SAML protocol operations -- the Go application never sees raw SAML assertions.
- **gocloak v13.9.0:** Required for authorization code token exchange. SAML IdP management via Admin API coverage needs verification -- may require direct HTTP calls to Keycloak Admin REST API as fallback.
- **Enterprise IdPs (external):** Each enterprise customer runs their own SAML IdP. Metadata exchange happens during configuration. IdP availability is a runtime dependency for SSO login only -- non-SSO tenants are unaffected.
- **PostgreSQL v16:** Schema changes for SSO config storage via numbered SQL migration.

## Constraints
- **Backward compatibility (hard):** `POST /api/auth/login` must continue working identically for non-SSO tenants. Same API contract (`{email, password, tenant_id}` -> `{access_token, refresh_token, expires_in, token_type}`), same behavior, same error messages. Source: business requirement + `internal/auth/handler.go:18-29`.
- **Single Keycloak realm:** All SAML IdP configurations must coexist in the `platform` realm. Each tenant IdP needs a unique alias. Source: `internal/auth/keycloak.go:18`, `internal/config/config.go:19`.
- **Repository-service-handler pattern:** All new code must follow the existing layering convention -- no shortcuts. Source: consistent architecture across all modules.
- **JWT claim compatibility:** SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, and `realm_access.roles` claims. Without these, the auth middleware will break for SSO-authenticated users. Source: `internal/auth/keycloak.go:68-88`, `internal/auth/middleware.go:61-83`.
- **No per-tenant config storage exists yet:** SSO is the first feature requiring per-tenant configuration. The schema design sets a precedent for future per-tenant features. Source: `internal/tenant/models.go:18-23`.
- **Q3 delivery timeline:** Scope decisions should favor MVO delivery within Q3. Source: requirements draft.
- **SAML 2.0 protocol only:** No OIDC/OAuth SSO in scope for this task. Source: requirements draft.
- **Email UNIQUE constraint (global):** `users.email` has a global UNIQUE constraint, not scoped per tenant. Same email cannot exist in two tenants. SSO email-domain-to-tenant mapping must remain strictly 1:1. Source: `migrations/001_initial.sql:19`.
- **Login API contract preservation:** Existing login endpoint contract must be preserved exactly. SSO introduces new endpoints rather than modifying the login endpoint signature. Source: `internal/auth/handler.go:18-29`.
- **Context key contract:** Protected route handlers depend on `ContextKeyUserID`, `ContextKeyTenantID`, and `ContextKeyRoles` being set by auth middleware. SSO-authenticated requests must produce the same context values. Source: `internal/auth/middleware.go:15-18`.
- **No logging of SAML assertions or tokens:** SAML assertions and tokens contain sensitive authentication data. Error handling and logging must not inadvertently expose them.

## Risks

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Zero test coverage makes auth flow changes dangerous -- any regression in login/middleware breaks all authenticated users | high | high | Establish integration tests for existing login flow before modifying it. All new SSO code should include tests |
| Keycloak SAML broker configuration complexity at scale (per-tenant IdP setup, attribute mappers, first broker login flow) | medium | high | Build configuration automation layer using Keycloak Admin API. Create reusable IdP configuration templates |
| Account linking undefined for existing password users when their tenant enables SSO -- will surface as a real problem immediately | high | medium | Define strategy before implementation: match by email, update `keycloak_id`. Plan for edge cases (email mismatch) |
| Keycloak client may need Standard Flow enabled alongside Direct Access Grants -- misconfiguration could affect existing password auth | medium | high | Test Keycloak client configuration changes in staging. Both flows can coexist on the same client |
| gocloak may not cover SAML IdP Admin API endpoints | medium | medium | Check gocloak source early. Prepare fallback: direct HTTP client for Keycloak Admin REST API |
| SAML attribute mapping varies across IdP vendors (Okta, Azure AD, ADFS use different attribute names) | medium | medium | Design standard attribute mapper template in Keycloak. Document required IdP attributes |
| RequireRole middleware has a variable shadowing bug (`internal/auth/middleware.go:99-104`) indicating untested auth code paths | low | medium | Fix the bug before or during SSO work. Pre-existing, tangential, but signals code health risk in the auth module |
| Single `email_domain` per tenant may break with enterprises using multiple domains (acme.com, acme.co.uk) | low | medium | Defer for MVO -- most enterprises have a primary domain. If needed later, a `tenant_email_domains` join table can replace the single column |

## Candidate Solution Directions

- **Minimal / MVO-first:** Implement SP-initiated SAML SSO for the happy path first. Manual Keycloak IdP configuration (no automation layer). Separate `tenant_sso_config` table for clean schema. API-only SSO config management. Account linking by email match only. No password fallback for SSO tenants. Target: get one enterprise tenant through SSO login end-to-end. Trade-off: faster delivery, but each new tenant onboarding requires manual Keycloak work.
- **Systematic / automation-included:** Same core SSO flow, but include Keycloak Admin API automation for IdP provisioning from the start. This means building a Keycloak admin client layer that can create/update/delete SAML IdP configurations programmatically. Trade-off: more upfront work, but tenant onboarding scales without manual Keycloak admin console intervention. Better for 5+ tenants.
- **Safe / test-first:** Before any SSO code, establish integration tests for the existing login flow. Then build SSO with tests alongside. This adds time to initial delivery but significantly reduces regression risk given the zero test coverage baseline. Trade-off: slower start, safer execution, and the test investment pays dividends beyond SSO.

All three directions are compatible -- the question is which combination and what order. A recommended hybrid: safe + minimal first (tests + MVO), then systematic (automation) as a follow-up.

## Resolved Contradictions

All three Stage 2 analyses were closely aligned. No significant contradictions were found between them. The critic review of Stage 2 also confirmed no cross-analysis contradictions. Specific areas of alignment:

- **All three analyses agree** that the fundamental challenge is the shift from direct grant to authorization code flow in the auth module.
- **All three analyses agree** that zero test coverage is the highest-likelihood risk, with the system analysis documenting the complete absence of test files and the constraints analysis rating it high/high.
- **All three analyses agree** on the single Keycloak realm architecture as a workable constraint (not a blocker), with each providing complementary detail -- system analysis on code structure, constraints analysis on operational implications.
- **All three analyses agree** that per-tenant config storage is a prerequisite gap with the same options (new table vs. JSONB column), with the system analysis noting the explicit NOTE comments in the codebase acknowledging this gap.
- **Minor emphasis difference:** The product analysis emphasizes the account linking edge case as a UX concern (existing users shouldn't be duplicated), while the constraints analysis frames it as a scope risk (strategy is undefined). Both are correct -- the issue is real from both angles. Resolution: flagged as an open question requiring a decision before implementation.

## Remaining Open Questions

1. **Password fallback policy for SSO tenants:** Should SSO-enabled tenants have password login blocked (enterprise security enforcement) or available as fallback (availability during IdP outage)? Product analysis notes enterprise customers typically want absolute enforcement. This is a product decision with significant architectural implications -- it determines login endpoint behavior for SSO tenants.
2. **Account linking strategy:** How should existing password users be handled when their tenant enables SSO? Options: automatic email-based linking (simplest), admin-driven linking, or re-registration. Affects user service logic, potential data migration, and rollout planning.
3. **Per-tenant config storage design:** New table (`tenant_sso_config`) vs. JSONB column on tenants table. Sets a precedent for all future per-tenant configuration. Both options are noted in the codebase. New table offers cleaner separation; JSONB offers simpler migration.
4. **gocloak SAML IdP Admin API coverage:** Does gocloak v13.9.0 support `CreateIdentityProvider` and related SAML IdP management endpoints? Determines whether direct HTTP calls to Keycloak Admin REST API are needed. Should be verified early.
5. **Keycloak client Standard Flow status:** Is the current `platform-app` client already configured for Authorization Code flow, or does it need reconfiguration? Affects deployment steps and testing strategy.
6. **SSO and password coexistence within a tenant:** Can a tenant support both SSO and email/password simultaneously during transition, or is it all-or-nothing? Affects tenant config model and login routing logic.
7. **Required SAML attributes:** Which SAML attributes are required from IdPs (email minimum, but name? groups?) and how should missing optional attributes be handled? Affects Keycloak mapper configuration and JIT provisioning logic.
8. **SSO configuration management at scale:** How will 50+ tenant IdP configurations be managed, monitored, and debugged? Lower priority -- planning can proceed without this, but it influences whether to invest in automation upfront.

## Critique Review

The synthesis was reviewed by an independent critic against eight criteria. Summary of findings:

**Verdict: CONSISTENT** -- the synthesis accurately consolidates the three Stage 2 analyses without hiding contradictions, dropping important findings, or promoting assumptions to facts.

Criteria results:
- **Goal fidelity:** PASS -- synthesized goal captures both the product intent (enterprise adoption unblocked) and technical scope (SAML SSO via Keycloak broker) from all sources.
- **Scenario coverage:** PASS -- primary SP-initiated scenario preserved with full detail from product analysis. All mandatory edge cases (JIT provisioning, account linking, misconfiguration, IdP outage) included. Deferred scenarios listed with deferral reasoning.
- **System scope accuracy:** PASS -- modules, change points, and dependencies match system analysis findings with specific file paths and function references.
- **Constraint completeness:** PASS -- constraints from all three analyses are present, deduplicated, and include code-level evidence. Added email UNIQUE and context key constraints from system analysis.
- **Risk calibration:** PASS -- risks consolidated from system and constraints analyses with consistent likelihood/impact calibration. No risk inflation or deflation.
- **Contradiction resolution:** PASS -- honestly documented that no significant contradictions exist between analyses. Minor emphasis differences (account linking framing) noted and resolved.
- **Assumption honesty:** PASS -- open questions are preserved as open questions, not promoted to resolved decisions. gocloak coverage, password fallback policy, and config storage design all remain explicitly unresolved.
- **Information preservation:** PASS -- key findings preserved including the RequireRole bug, the email UNIQUE constraint implication, the implicit Keycloak client configuration dependency, and the absence of test coverage.

No significant issues found. The synthesis is ready for user agreement review.
