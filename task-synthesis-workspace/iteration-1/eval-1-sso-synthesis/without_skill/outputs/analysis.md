# Synthesized Analysis: SAML 2.0 SSO Feature

## 1. Unified Task Definition

**Task:** Add SAML 2.0 Single Sign-On support to a multi-tenant Go backend platform so that enterprise tenants can authenticate employees through their corporate identity providers, while preserving the existing email/password login flow for non-SSO tenants.

**Type:** New feature (not a fix or enhancement -- this adds a capability that does not exist today)

**Complexity:** High -- spans authentication flow architecture, schema design, external system integration (Keycloak SAML brokering), per-tenant configuration (a first for this codebase), and backward compatibility with zero existing test coverage.

**Primary Risk Area:** Integration -- the task depends on Keycloak SAML brokering configuration, enterprise IdP interoperability, and a fundamental shift from direct grant to authorization code flow.

---

## 2. Cross-Analysis Alignment

All three analysis streams (product, system, constraints/risks) converge on the same core challenges. No contradictions were found during synthesis. The areas of strong consensus are:

| Theme | Product View | System View | Constraints View |
|-------|-------------|-------------|-----------------|
| No per-tenant config storage | MVO requires SSO config per tenant | `tenant/models.go` has explicit NOTE about missing config storage | Identified as technical constraint; schema design sets a precedent |
| Single Keycloak realm | SSO config happens within one realm | `keycloak.go:18` stores single `realm` string | Architectural constraint; tenant isolation is at IdP-alias level, not realm level |
| Auth flow paradigm shift | User is redirected, not submitting credentials to app | `keycloak.go:42` uses `client.Login()` (direct grant); SSO needs authorization code flow | Fundamental shift from API-to-API to browser-redirect pattern |
| Zero test coverage | (implicit risk to delivery timeline) | No test files exist in any module | High likelihood, high impact regression risk during auth changes |
| Backward compatibility | Email/password login must be completely unaffected | Login API contract (`POST /api/auth/login`) must be preserved | Hard business requirement; same API contract, behavior, and error messages |
| Account linking undefined | Edge case: existing password user on newly SSO-enabled tenant | `GetOrCreateByEmail` creates users but doesn't handle dual-auth-method scenario | High likelihood risk; must be resolved before implementation |

---

## 3. Consolidated System Map

### Modules and Change Scope

| Module | Path | Role | Change Scope | Confidence |
|--------|------|------|-------------|------------|
| **Auth** | `internal/auth/` | Primary target: new SSO endpoints, service logic, Keycloak authorization code flow | Large | High |
| **Tenant** | `internal/tenant/` | SSO config storage, tenant detection for SSO routing | Medium | High |
| **User** | `internal/user/` | JIT provisioning enhancement (KeycloakID population) | Small | High |
| **Config** | `internal/config/` | Possible SAML-related base configuration (SP entity ID, ACS URL) | Small | Medium |
| **Server** | `cmd/server/main.go` | Route registration for new SSO endpoints | Small | High |
| **Migrations** | `migrations/` | New migration for SSO config schema | Medium | High |

### Critical Change Points (Ordered by Risk)

1. **`internal/auth/service.go:Login`** -- Must add SSO tenant detection branch without breaking password login. Highest regression risk since all logins flow through here.
2. **`internal/auth/handler.go`** -- New SSO initiation and callback HTTP endpoints. Large scope, but additive (new endpoints, not modifying existing ones).
3. **`internal/auth/keycloak.go`** -- Authorization code exchange methods. New capability added to an existing critical client.
4. **`internal/tenant/models.go` + `repository.go`** -- SSO configuration fields/struct and CRUD queries. Schema design decision with precedent-setting implications.
5. **`migrations/002_sso_config.sql`** -- New table or columns for per-tenant SSO configuration. Must be rollback-safe.
6. **`internal/user/service.go:GetOrCreateByEmail`** -- Accept and store KeycloakID for SSO-provisioned users. Small, targeted change.
7. **`cmd/server/main.go`** -- Register SSO routes in public route group. Mechanical change.

### Unchanged (But Must Be Verified)

- **`internal/auth/middleware.go:Authenticate`** -- Validates JWTs regardless of issuance method. Should work unchanged, but SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, and `realm_access.roles` claims.

---

## 4. Consolidated Dependencies

### Internal
- Auth middleware validates JWTs and injects context keys (`ContextKeyUserID`, `ContextKeyTenantID`, `ContextKeyRoles`). All protected routes depend on this. SSO must produce identical context values.
- Tenant resolution by email domain (`tenant.Repository.GetByEmailDomain()`) is the natural SSO detection point.
- User provisioning via `GetOrCreateByEmail()` is reusable for SSO JIT but needs KeycloakID support.

### External
- **Keycloak v24.0:** Must be configured with SAML IdP broker per tenant, Standard Flow enabled on client, attribute mappers for `tenant_id` claim. Keycloak handles all SAML protocol operations -- the Go app never sees raw SAML assertions.
- **gocloak v13.9.0:** Must support authorization code token exchange. SAML IdP management via Admin API coverage is uncertain -- may need direct HTTP calls as fallback.
- **Enterprise IdPs:** External SAML IdPs controlled by customers. Metadata exchange during configuration; IdP availability is a runtime dependency for SSO login only.

### Implicit (Easy to Miss)
- Keycloak `platform-app` client likely needs Standard Flow enabled (currently may be Direct Access Grants only).
- SAML ACS URL is a function of Keycloak base URL + realm + IdP alias -- must be communicated to each enterprise IdP during setup.
- `tenant_id` custom claim may need a Keycloak protocol mapper configured for brokered logins.

---

## 5. Consolidated Constraints

### Hard Constraints (Non-Negotiable)
1. **Backward compatibility:** `POST /api/auth/login` with email/password must continue working identically for non-SSO tenants. Same API contract, same behavior, same error messages.
2. **JWT claim compatibility:** SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, and `realm_access.roles` for middleware and handler compatibility.
3. **Context key contract:** Protected route handlers depend on `ContextKeyUserID`, `ContextKeyTenantID`, and `ContextKeyRoles` being set by auth middleware.
4. **Single Keycloak realm:** All SAML IdP configurations must coexist in the `platform` realm with unique aliases per tenant.
5. **Repository-service-handler pattern:** All new code must follow the existing layering convention.
6. **SAML 2.0 protocol only:** No OIDC/OAuth SSO in scope.
7. **Q3 delivery timeline:** Scope decisions must favor MVO delivery within Q3.

### Soft Constraints (Preferred but Negotiable)
8. **Email UNIQUE constraint:** Global uniqueness on users.email means one email maps to one tenant. Acceptable for MVO but may need revisiting.
9. **No migration framework:** Raw SQL migrations with numbered convention. New migrations must follow this pattern.
10. **Error response consistency:** New SSO handlers should prefer `pkg/httputil.WriteError()` over direct `http.Error()` calls.

---

## 6. Consolidated Risk Matrix

| # | Risk | Likelihood | Impact | Category | Mitigation Strategy |
|---|------|-----------|--------|----------|-------------------|
| R1 | Zero test coverage makes auth flow changes dangerous | High | High | Regression | Establish integration tests for existing login flow BEFORE modifying it. All new SSO code must include tests. |
| R2 | Keycloak SAML broker configuration complexity at scale | Medium | High | Integration | Build automation layer using Keycloak Admin API. Create reusable IdP configuration templates. |
| R3 | Account linking undefined for existing password users enabling SSO | High | Medium | Scope | Define strategy before implementation: match by email, update KeycloakID. Plan for edge cases. |
| R4 | Keycloak client may need Standard Flow enabled, risking config error | Medium | High | Technical | Test in staging. Both direct grant and standard flow can coexist on the same client. |
| R5 | gocloak may not cover SAML IdP Admin API | Medium | Medium | Technical | Check gocloak source early. Prepare fallback: direct HTTP client for Keycloak Admin REST API. |
| R6 | SAML attribute mapping varies across IdP vendors | Medium | Medium | Integration | Design standard attribute mapper template. Document required IdP attributes. |
| R7 | Variable shadowing bug in RequireRole middleware indicates untested code paths | Low | Medium | Regression | Fix pre-existing bug before SSO work begins. |
| R8 | Single `email_domain` per tenant may not support enterprises with multiple domains | Low | Medium | Scope | Defer for MVO. Most enterprises have a primary domain. |

---

## 7. Consolidated Open Questions

### Must Resolve Before Planning

| # | Question | Impact Area | Blocking? |
|---|----------|------------|-----------|
| Q1 | **Password fallback policy for SSO tenants:** Should SSO-enabled tenants have password login blocked (security enforcement) or available as fallback (availability)? | Login endpoint behavior, auth service logic | Yes -- determines login endpoint branching design |
| Q2 | **Account linking strategy:** How should existing password users be handled when their tenant enables SSO? Automatic email-based linking, admin-driven, or re-registration? | User service logic, data migration, rollout planning | Yes -- affects user provisioning implementation |
| Q3 | **Per-tenant config storage design:** New `tenant_sso_config` table vs JSONB column on tenants table? Sets a precedent for all future per-tenant configuration. | Schema design, repository pattern, migration | Yes -- foundational design decision |

### Should Resolve Before Implementation

| # | Question | Impact Area | Blocking? |
|---|----------|------------|-----------|
| Q4 | **gocloak SAML IdP Admin API coverage:** Does gocloak v13.9.0 support `CreateIdentityProvider` and related SAML IdP management endpoints? | Implementation approach for IdP configuration | No -- can design with fallback |
| Q5 | **Keycloak client Standard Flow configuration:** Is `platform-app` already configured for Authorization Code flow? | Deployment steps, testing strategy | No -- can be verified and fixed |
| Q6 | **SSO and password coexistence within a tenant:** Can both methods exist simultaneously during transition? | Tenant config model, login routing logic | Partially -- linked to Q1 |
| Q7 | **Required SAML attributes:** Which attributes are required from IdPs (email, name, groups?) and how to handle missing optional attributes? | Keycloak mapper configuration, JIT provisioning | No -- can default to email-only |

### Can Defer

| # | Question | Impact Area |
|---|----------|------------|
| Q8 | SSO configuration management at scale (50+ tenants) | Operations, monitoring |
| Q9 | Keycloak upgrade impact on SAML IdP broker configurations | Deployment process |
| Q10 | SSO-specific monitoring and alerting | Operations |
| Q11 | Data retention or audit requirements for SSO login events | Compliance |

---

## 8. Pre-Existing Issues Discovered

1. **Variable shadowing bug in `internal/auth/middleware.go:99-104`:** The `RequireRole` closure uses `r` for both the role string (loop variable) and the `http.Request` parameter. This is a compilation error that indicates this code path is not exercised. Should be fixed before SSO work begins.
2. **Zero test coverage across entire codebase:** No test files exist in any module. This is not an SSO-specific issue but creates significant regression risk for any auth flow changes.
3. **`httputil` package underutilization:** `pkg/httputil/response.go` provides `WriteJSON()` and `WriteError()` but handlers inconsistently use `http.Error()` directly.
4. **Unused `Session` struct:** `auth.models.go` defines a `Session` struct that is not used anywhere (sessions are Keycloak-managed).

---

## 9. Minimum Viable Outcome (MVO) Definition

**Core (cannot be cut):**
- SP-initiated SAML SSO login for at least one enterprise tenant
- JIT user provisioning for new SSO users with correct tenant association
- Per-tenant SSO configuration storage (even if API-only, no UI)
- Tenant detection at login that routes SSO-enabled tenants to SAML flow
- Keycloak configured as SAML SP/broker for tenant IdPs
- Email/password login completely unaffected for non-SSO tenants

**Deferred (explicitly out of scope for MVO):**
- Self-service SSO configuration UI
- IdP-initiated SSO
- Single Logout (SLO)
- Automated account linking for existing password users (can be admin-driven initially)
- SCIM provisioning
- Multiple IdPs per tenant
- Multiple email domains per tenant

---

## 10. Synthesis Confidence Assessment

| Area | Confidence | Rationale |
|------|-----------|-----------|
| Task definition and scope | High | All three analyses agree on what is being built and what is excluded |
| System architecture understanding | High | System analysis verified all claims from actual code reads with specific file paths and line numbers |
| Change point identification | High | Every change point maps to a specific file and function with scope estimate |
| Risk assessment | High | Risks are calibrated (not all "high"), evidence-based, and have concrete mitigations |
| Dependency mapping | High | Upstream, downstream, external, and implicit dependencies all identified |
| Open questions | Medium | Questions are well-identified, but three blocking questions (Q1-Q3) require product decisions that could shift the design significantly |
| MVO scope | High | Clear line between what's in and what's out, with explicit rationale |
