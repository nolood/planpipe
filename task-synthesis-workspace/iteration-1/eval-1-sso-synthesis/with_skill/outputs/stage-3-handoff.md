# Stage 3 Handoff — Task Synthesis & Agreement Complete

## Task Summary

Add SAML 2.0 SSO support to a multi-tenant Go backend platform so enterprise tenants can authenticate employees through corporate identity providers. Keycloak (already in use for email/password auth) acts as the SAML SP/broker. The existing email/password login must remain fully functional for non-SSO tenants. SSO configuration is per-tenant -- each tenant independently enables SSO and provides their IdP's SAML metadata.

## Classification
- **Type:** feature
- **Complexity:** high
- **Primary risk area:** integration — depends on Keycloak SAML brokering, enterprise IdP interoperability, and the architectural shift from direct grant to authorization code flow
- **Solution direction:** safe + minimal — establish tests for existing login flow first, then implement MVO SSO (one tenant end-to-end), defer automation to when scale demands it

## Agreed Goal

Add SP-initiated SAML 2.0 SSO as a second authentication method to the platform, with Keycloak as SAML SP/broker, per-tenant configuration, and JIT user provisioning — while preserving the existing email/password flow unchanged for non-SSO tenants.

## Agreed Problem Statement

Enterprise adoption is blocked because the platform only supports email/password authentication. Enterprise organizations require SAML SSO for security compliance, credential lifecycle management, and audit trail continuity. Active deals are contingent on SSO support. Q3 delivery is a business priority.

## Agreed Scope

### Included
- SP-initiated SAML 2.0 SSO login flow (email -> tenant detection -> Keycloak redirect -> IdP auth -> JWT callback)
- Per-tenant SSO configuration storage in PostgreSQL (new schema)
- Keycloak SAML IdP broker setup per tenant (within existing single `platform` realm)
- JIT user provisioning on first SSO login (create local user record with `keycloak_id`)
- Account linking for existing password users (by email match) when their tenant enables SSO
- Authorization code flow support in Keycloak client (alongside existing direct grant)
- SSO initiation and callback HTTP endpoints
- Login flow routing: detect SSO tenant and direct to appropriate auth method
- Backward-compatible email/password login for non-SSO tenants (unchanged API contract)

### Excluded
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning/deprovisioning
- Self-service SSO configuration UI (API-only or manual config for now)
- Multiple IdPs per tenant
- OIDC/OAuth SSO (SAML 2.0 only)
- Admin UI for SSO management

## Key Scenarios for Planning

### Primary Scenario
1. Enterprise employee enters corporate email at login page
2. System extracts email domain, looks up tenant via `tenant.Repository.GetByEmailDomain()`, determines tenant has SSO enabled
3. System redirects to Keycloak's SAML broker endpoint, which redirects to tenant's corporate IdP
4. Employee authenticates at corporate IdP
5. IdP sends SAML assertion to Keycloak. Keycloak validates, maps attributes, issues authorization code
6. Application callback endpoint receives code, exchanges for JWT via Keycloak token endpoint
7. If first SSO login, local user record is JIT-provisioned with email, name, `keycloak_id`, and tenant association
8. Employee is authenticated with correct tenant context and role -- same JWT claims, same context keys as password-authenticated sessions

### Mandatory Edge Cases
- First SSO login (JIT provisioning): create user, populate `keycloak_id`, default missing attributes
- Existing password user on newly SSO-enabled tenant: link by email match, update `keycloak_id`, do not duplicate
- SSO misconfiguration: meaningful error, tenant isolation (other tenants unaffected)
- IdP outage: graceful failure with clear error (password fallback policy to be decided)
- Non-SSO tenant login: completely unchanged behavior

## System Map for Planning

### Modules to Change
| Module | Path | What Changes | Scope |
|--------|------|-------------|-------|
| Auth | `internal/auth/` | New SSO endpoints (initiation + callback), SSO service logic, Keycloak authorization code flow methods, SAML broker redirect URL generation | large |
| Tenant | `internal/tenant/` | SSO configuration model/struct, SSO config CRUD repository methods, SSO detection service methods | medium |
| User | `internal/user/` | `GetOrCreateByEmail` accepts and stores `keycloak_id` for SSO-provisioned users | small |
| Config | `internal/config/` | SAML-related base configuration (SP entity ID, ACS URL base path) | small |
| Server | `cmd/server/main.go` | Register SSO routes in public route group | small |
| Migrations | `migrations/` | New migration `002_sso_config.sql` for per-tenant SSO configuration schema | medium |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/handler.go` | New SSO initiation and callback HTTP endpoints | HTTP entry points for the SSO flow |
| `internal/auth/service.go:Login` | SSO tenant detection branch; new SSO callback processing method | Login must route by auth method per tenant |
| `internal/auth/keycloak.go` | Authorization code exchange methods; SAML broker redirect URL generation | Current client only supports direct grant |
| `internal/tenant/models.go` | SSO configuration fields/struct | No per-tenant config exists yet |
| `internal/tenant/repository.go` | SSO config CRUD queries | Data access for SSO configuration |
| `internal/tenant/service.go` | `GetSSOConfig`, `UpdateSSOConfig`, `IsSSOEnabled` methods | Service layer for SSO config |
| `migrations/002_sso_config.sql` | Per-tenant SSO configuration table/columns | Schema foundation for SSO |
| `internal/user/service.go:GetOrCreateByEmail` | Accept and store `keycloak_id` | SSO users need `keycloak_id` populated |
| `cmd/server/main.go` | Register SSO routes in public group | SSO endpoints for unauthenticated users |

### Critical Dependencies
- **Keycloak v24.0:** SAML IdP broker per tenant with unique alias (e.g., `saml-{tenant_slug}`), Standard Flow enabled on `platform-app` client, attribute mappers for `tenant_id` claim in brokered tokens. Keycloak handles all SAML protocol -- Go app never sees raw SAML assertions.
- **gocloak v13.9.0:** Authorization code token exchange. SAML IdP Admin API coverage needs early verification -- may require direct HTTP calls as fallback.
- **Enterprise IdPs (external):** SAML 2.0 metadata exchange during configuration. Runtime dependency for SSO login only. Cannot control IdP behavior or availability.

## Constraints the Plan Must Respect
- Backward compatibility (hard): `POST /api/auth/login` works identically for non-SSO tenants -- same contract, same behavior -- user confirmed
- Single Keycloak realm: all tenant IdPs in `platform` realm with unique aliases -- user confirmed
- JWT claim compatibility: SSO JWTs must include `sub`, `email`, `tenant_id`, `realm_access.roles` -- user confirmed
- Repository-service-handler pattern: all new code follows existing layering -- user confirmed
- No per-tenant config precedent: SSO config storage design sets the pattern for future features -- user confirmed
- Q3 delivery timeline: scope decisions favor MVO -- user confirmed
- SAML 2.0 only: no OIDC/OAuth SSO -- user confirmed
- Email UNIQUE globally: email-domain-to-tenant mapping must be 1:1 -- user confirmed
- Zero test coverage: high regression risk, must establish tests before modifying auth flow -- user confirmed
- No SAML assertion/token logging: sensitive data must not appear in logs -- user confirmed

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Zero test coverage makes auth flow changes dangerous | high | high | Establish integration tests for existing login flow before modifying it. All SSO code includes tests |
| Keycloak SAML broker configuration complexity at scale | medium | high | Build configuration automation (deferred to post-MVO). Create reusable IdP templates |
| Account linking undefined for existing password users | high | medium | Define strategy before implementation: email match, update `keycloak_id` |
| Keycloak client Standard Flow misconfiguration risk | medium | high | Test config changes in staging. Both flows coexist on same client |
| gocloak may not cover SAML IdP Admin API | medium | medium | Check gocloak source early. Prepare direct HTTP fallback |
| SAML attribute mapping varies across IdP vendors | medium | medium | Standard attribute mapper template. Document required IdP attributes |
| RequireRole middleware variable shadowing bug | low | medium | Fix before or during SSO work |

## Product Requirements for Planning
- **Primary scenario:** Enterprise user enters email, system detects SSO tenant, redirects through Keycloak SAML broker to corporate IdP, user authenticates, returns with JWT, lands authenticated
- **Success signals:** Enterprise deal closure rate, SSO login success rate (>95%), time-to-SSO-configuration, auth support ticket volume reduction
- **Minimum viable outcome:** SP-initiated SAML SSO for at least one tenant, with JIT provisioning, coexisting with unchanged email/password flow. No self-service UI, no IdP-initiated SSO, no SLO, no SCIM
- **Backward compatibility:** Email/password login completely unaffected for non-SSO tenants. Same API contract, same behavior, same error messages

## Solution Direction

Safe + minimal hybrid, as agreed with the user. First: establish integration tests for the existing login flow (safety net against regression in zero-test-coverage codebase). Then: implement MVO SSO -- one enterprise tenant end-to-end with manual Keycloak IdP configuration. Defer Keycloak Admin API automation to when tenant scale demands it. This balances Q3 delivery pressure with regression safety and avoids overbuilding for scale that doesn't exist yet.

## Accepted Assumptions
- Keycloak's SAML brokering is sufficient for multi-tenant SSO (one IdP per tenant, shared realm) -- based on Keycloak docs, not tested with this setup
- Auth middleware works unchanged for SSO-authenticated requests (validates JWTs regardless of issuance method) -- architecturally sound, needs verification testing
- Single `email_domain` per tenant is sufficient for MVO enterprises -- multi-domain is a deferrable edge case
- Direct Access Grants and Standard Flow can coexist on the same Keycloak client without interference

## Deferred Items
- Password fallback policy for SSO tenants: block vs. allow password login. To resolve during planning.
- Per-tenant config storage format: new table vs. JSONB column. To decide during planning.
- SSO and password coexistence within a tenant during transition. To resolve alongside fallback policy.
- Required SAML attributes specification. To define during Keycloak mapper configuration planning.
- Keycloak Admin API automation for IdP provisioning. Deferred to post-MVO.

## User Corrections from Synthesis
No corrections were made -- the user confirmed all five agreement blocks as presented without changes.

## Acceptance Criteria
- Enterprise employee can log in via SAML SSO through their corporate IdP and land authenticated with correct tenant context and role
- Non-SSO tenant email/password login is completely unchanged (same API contract, same behavior, same errors)
- First-time SSO users are automatically provisioned with correct tenant association and `keycloak_id`
- Existing password users are linked (not duplicated) when their tenant enables SSO
- SSO misconfiguration for one tenant does not affect other tenants or non-SSO login
- SSO-issued JWTs contain all required claims (`sub`, `email`, `tenant_id`, `realm_access.roles`)
- All new SSO code follows the repository-service-handler pattern
- Integration tests exist for the existing login flow before SSO modifications
- New SSO code includes tests

## Detailed References
These files contain the full analysis and agreed model:
- `analysis.md` — synthesized task analysis
- `agreement-package.md` — agreement blocks presented to user
- `agreed-task-model.md` — full agreed task model with correction log
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
