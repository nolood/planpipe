# Stage 3 Handoff -- Ready for Implementation Planning

## Stage Completed

**Stage 2 -> Stage 3:** Deep analysis synthesis and user agreement complete. The task model has been confirmed and is ready for implementation planning.

## Task Summary

Add SAML 2.0 SSO to a multi-tenant Go backend (chi router, gocloak, PostgreSQL, Keycloak). Enterprise tenants authenticate employees via corporate SAML IdPs through Keycloak brokering. Existing email/password login remains unchanged for non-SSO tenants. Password login is blocked for SSO-enabled tenants (enforcement model).

**Type:** Feature | **Complexity:** High | **Timeline:** Q3 (MVO)

## Confirmed Decisions

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | Password fallback for SSO tenants | **Blocked** | Enterprise enforcement; no password bypass of IdP MFA |
| D2 | Account linking strategy | **Automatic email-based** | Least friction; match by email, update keycloak_id |
| D3 | Per-tenant config storage | **New `tenant_sso_config` table** | Clean precedent for future per-tenant config |
| Pre | RequireRole bug fix | **Include in SSO work** | Low effort, reduces auth module risk |

## Deliverables for Planning

The plan must produce an implementation sequence for these deliverables:

| # | Deliverable | Module(s) | Scope |
|---|------------|-----------|-------|
| S1 | SSO initiation endpoint (`POST /api/auth/sso/initiate`) | auth | Large |
| S2 | SSO callback endpoint (`GET /api/auth/sso/callback`) | auth | Large |
| S3 | Per-tenant SSO config storage + admin API | tenant, migrations | Medium |
| S4 | SSO tenant detection in login flow (block password for SSO tenants) | auth | Medium |
| S5 | Keycloak authorization code flow methods | auth (keycloak.go) | Medium |
| S6 | JIT user provisioning with KeycloakID | user | Small |
| S7 | Automatic email-based account linking | user | Small |
| S8 | Database migration (`002_sso_config.sql`) | migrations | Medium |
| S9 | Keycloak SAML IdP configuration automation | auth or new keycloak admin module | Medium |
| S10 | Integration tests (existing login + new SSO) | auth, user, tenant | Medium |
| S11 | RequireRole bug fix | auth (middleware.go) | Small |

## System Map for Planning

### Files to Modify

| File | Changes | Priority |
|------|---------|----------|
| `internal/auth/handler.go` | Add `SSOInitiate()` and `SSOCallback()` handlers | High |
| `internal/auth/service.go` | Add `InitiateSSO()`, `ProcessSSOCallback()` methods; modify `Login()` for SSO tenant detection | High |
| `internal/auth/keycloak.go` | Add `GetSAMLBrokerRedirectURL()`, `ExchangeCode()` methods | High |
| `internal/tenant/models.go` | Add `SSOConfig` struct | Medium |
| `internal/tenant/repository.go` | Add SSO config CRUD queries | Medium |
| `internal/tenant/service.go` | Add `GetSSOConfig()`, `UpdateSSOConfig()`, `IsSSOEnabled()` | Medium |
| `internal/tenant/handler.go` | Add SSO admin endpoints (create/read/delete SSO config) | Medium |
| `internal/user/service.go` | Modify `GetOrCreateByEmail()` to accept and store KeycloakID | Medium |
| `cmd/server/main.go` | Register SSO routes (public group) and SSO admin routes (protected group) | Low |
| `internal/config/config.go` | Add SAML base config if needed (SP entity ID, ACS URL base) | Low |
| `internal/auth/middleware.go` | Fix RequireRole variable shadowing bug (lines 99-104) | Low |

### Files to Create

| File | Purpose |
|------|---------|
| `migrations/002_sso_config.sql` | `tenant_sso_config` table schema |
| Test files (various) | Integration tests for login + SSO flows |

### Files Unchanged

| File | Reason |
|------|--------|
| `internal/auth/middleware.go:Authenticate` | Validates JWTs regardless of issuance method; SSO JWTs contain same claims |
| `internal/auth/models.go` | Session struct unused; no SSO-specific models needed here |
| `internal/user/repository.go` | `GetByKeycloakID`, `GetByEmail`, `Create` already exist; no changes needed |
| `internal/user/handler.go` | User endpoints unaffected |
| `pkg/httputil/response.go` | Utility functions unchanged; used by new SSO handlers |

## Hard Constraints for Planning

1. `POST /api/auth/login` unchanged for non-SSO tenants (same contract, behavior, errors)
2. SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, `realm_access.roles`
3. Auth middleware context keys produce same values for SSO requests
4. All SAML IdP configs in single `platform` Keycloak realm with unique aliases
5. Repository-service-handler layering for all new code
6. SAML 2.0 only (no OIDC/OAuth SSO)
7. Q3 delivery (MVO scope)
8. Password login blocked for SSO-enabled tenants
9. Automatic email-based account linking
10. Separate `tenant_sso_config` table

## Risk Mitigations to Embed in Plan

| # | Risk | Mitigation | Planning Implication |
|---|------|-----------|---------------------|
| R1 | Zero test coverage | Write tests for existing login flow first | **Must be an early phase** -- tests before modifications |
| R2 | Keycloak SAML complexity | Admin API automation with templates | Keycloak integration should be its own phase with staging validation |
| R3 | Account linking edge cases | Auto-match by email; error on keycloak_id conflict | Edge case handling in user provisioning phase |
| R4 | Keycloak Standard Flow config | Test in staging | Keycloak setup validation step in plan |
| R5 | gocloak Admin API gaps | Check source early; prepare HTTP fallback | Early spike/verification task in plan |
| R6 | IdP attribute mapping variance | Standard mapper template | Documentation deliverable alongside Keycloak config |
| R7 | RequireRole bug | Fix early | Include in first phase |

## Suggested Phase Structure (for planner reference)

Based on the dependency graph and risk mitigations, the natural implementation phases are:

1. **Foundation:** Fix RequireRole bug. Write integration tests for existing login flow. Verify gocloak Admin API coverage. Verify Keycloak client Standard Flow status.
2. **Schema + Config:** Database migration for `tenant_sso_config`. Tenant model, repository, service for SSO config. SSO admin API endpoints.
3. **Keycloak Integration:** Authorization code flow methods in KeycloakClient. SAML IdP configuration automation via Admin API. Keycloak attribute mapper setup.
4. **SSO Flow:** SSO initiation endpoint. SSO callback endpoint. Login endpoint modification (block password for SSO tenants). JIT provisioning with KeycloakID. Account linking.
5. **Validation:** Integration tests for SSO flow. End-to-end testing with a real SAML IdP (or Keycloak-to-Keycloak test IdP). Verify backward compatibility of password login.

## Open Items (Non-Blocking)

These can be resolved during implementation:

- gocloak SAML Admin API coverage (check source; fallback to HTTP)
- Keycloak `platform-app` Standard Flow status (verify; enable if needed)
- Required vs optional SAML attributes (default: email required, name optional)
- Keycloak hot-reload of IdP configs (verify during integration testing)
- SSO at scale considerations (defer to post-MVO)
- SSO-specific monitoring (defer to post-MVO; use standard logging)

## Artifact References

| Artifact | Location | Purpose |
|----------|----------|---------|
| Synthesized analysis | `analysis.md` (this directory) | Full cross-analysis synthesis with consolidated findings |
| Agreement package | `agreement-package.md` (this directory) | Decisions presented to user for confirmation |
| Agreed task model | `agreed-task-model.md` (this directory) | Confirmed task model with all decisions, scope, and constraints |
| Product analysis | `sso-stage2-artifacts/product-analysis.md` | Detailed product/business analysis |
| System analysis | `sso-stage2-artifacts/system-analysis.md` | Detailed codebase/system analysis |
| Constraints/risks analysis | `sso-stage2-artifacts/constraints-risks-analysis.md` | Detailed constraints and risk analysis |
| Stage 2 handoff | `sso-stage2-artifacts/stage-2-handoff.md` | Previous stage handoff document |
