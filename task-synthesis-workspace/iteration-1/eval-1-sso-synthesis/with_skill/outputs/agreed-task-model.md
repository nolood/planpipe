# Agreed Task Model

> Agreed on: 2026-03-14
> Based on: Stage 2 analyses + user review

## Task Goal

Add SAML 2.0 Single Sign-On support to the multi-tenant Go backend platform so that enterprise tenants can authenticate their employees through corporate identity providers, with Keycloak acting as the SAML SP/broker. The existing email/password login flow must remain fully functional for non-SSO tenants.

## Problem Statement

Enterprise adoption is blocked because the platform only supports email/password authentication. Enterprise IT departments require SAML SSO for security compliance, audit trail continuity, and credential lifecycle management. Active enterprise deals are contingent on SSO support, making Q3 delivery a business priority. The existing auth system works correctly -- the gap is that it is the only method available.

## Scope

### Included
- SP-initiated SAML 2.0 SSO login flow (email -> tenant detection -> Keycloak redirect -> IdP auth -> JWT callback)
- Per-tenant SSO configuration storage in PostgreSQL (new schema)
- Keycloak SAML IdP broker setup per tenant (within the existing single `platform` realm)
- JIT user provisioning on first SSO login (create local user record with `keycloak_id`)
- Account linking for existing password users (by email match) when their tenant enables SSO
- Authorization code flow support in the Keycloak client (alongside existing direct grant)
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

## Key Scenarios

### Primary Scenario
1. Enterprise employee navigates to the login page and enters their corporate email (e.g., jane@acmecorp.com)
2. System extracts the email domain, looks up the tenant via `tenant.Repository.GetByEmailDomain()`, and determines the tenant has SSO enabled
3. System redirects the employee to Keycloak's SAML broker endpoint, which redirects to the tenant's configured corporate IdP
4. Employee authenticates at their corporate IdP using existing corporate credentials
5. IdP sends SAML assertion back to Keycloak. Keycloak validates the assertion, maps attributes, and issues an authorization code
6. Application receives the authorization code at the callback endpoint, exchanges it for a JWT via Keycloak's token endpoint
7. If first SSO login, a local user record is created (JIT provisioning) with email, name, `keycloak_id`, and correct tenant association
8. Employee is authenticated and lands in the application with correct tenant context and role -- indistinguishable from a password-authenticated session for all downstream behavior

### Mandatory Edge Cases
- **First SSO login (JIT provisioning):** Create user record from SAML attributes. Populate `keycloak_id`. Use sensible defaults for missing optional attributes (e.g., set Name=email if display name not provided by IdP).
- **Existing password user on newly SSO-enabled tenant:** Link SSO identity to existing user by email match. Update `keycloak_id` with brokered subject. Do not create duplicate user records.
- **SSO misconfiguration:** Surface meaningful error to the user. Isolate failure -- other tenants must not be affected.
- **IdP outage:** Graceful failure with clear error message. Password fallback policy remains an open question to resolve during planning.
- **Non-SSO tenant login:** Completely unchanged behavior -- same API contract, same flow, same errors.

### Explicitly Deferred
- **IdP-initiated SSO:** Deferred -- SP-initiated covers the critical enterprise use case. User confirmed this can wait.
- **Single Logout (SLO):** Deferred -- session expiry and manual logout provide acceptable alternatives for MVO. User confirmed.
- **SCIM provisioning:** Deferred -- JIT provisioning handles creation; manual deactivation is acceptable initially. User confirmed.
- **Self-service SSO configuration UI:** Deferred -- API/manual configuration acceptable for initial launch. User confirmed.
- **Multiple IdPs per tenant:** Deferred -- most enterprises have a single IdP. User confirmed.

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Auth | `internal/auth/` | New SSO endpoints (initiation + callback), SSO service logic, Keycloak authorization code flow | large |
| Tenant | `internal/tenant/` | SSO configuration storage, tenant SSO detection for login routing | medium |
| User | `internal/user/` | JIT provisioning enhancement -- `GetOrCreateByEmail` must accept and store `keycloak_id` | small |
| Config | `internal/config/` | SAML-related base configuration (SP entity ID, ACS URL base) | small |
| Server | `cmd/server/main.go` | Route registration for new SSO endpoints in public route group | small |
| Migrations | `migrations/` | New migration for per-tenant SSO configuration schema | medium |

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

### Dependencies
- **Keycloak v24.0:** SAML IdP broker per tenant, Standard Flow on client, `tenant_id` claim mapper for brokered tokens
- **gocloak v13.9.0:** Authorization code token exchange. SAML IdP Admin API coverage needs early verification
- **Enterprise IdPs (external):** SAML 2.0 metadata exchange during configuration. Runtime dependency for SSO login
- **PostgreSQL v16:** Schema changes for SSO config via numbered SQL migration

## Confirmed Constraints
- **Backward compatibility (hard):** `POST /api/auth/login` must work identically for non-SSO tenants -- confirmed by user
- **Single Keycloak realm:** All tenant IdPs in `platform` realm with unique aliases -- confirmed by user
- **JWT claim compatibility:** SSO JWTs must include `sub`, `email`, `tenant_id`, `realm_access.roles` -- confirmed by user
- **Repository-service-handler pattern:** All new code follows existing layering -- confirmed by user
- **No per-tenant config precedent:** SSO config storage design sets the pattern for future features -- confirmed by user
- **Q3 delivery timeline:** Scope decisions favor MVO -- confirmed by user
- **SAML 2.0 only:** No OIDC/OAuth SSO -- confirmed by user
- **Email UNIQUE globally:** Email-domain-to-tenant mapping must be 1:1 -- confirmed by user
- **Zero test coverage:** High regression risk acknowledged -- confirmed by user
- **No SAML assertion/token logging:** Sensitive data must not appear in logs -- confirmed by user

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Zero test coverage makes auth flow changes dangerous | high | high | Establish integration tests for existing login flow before modifying it. All SSO code includes tests |
| Keycloak SAML broker configuration complexity at scale | medium | high | Build configuration automation layer using Keycloak Admin API. Reusable IdP templates |
| Account linking undefined for existing password users | high | medium | Define strategy before implementation: email match, update `keycloak_id` |
| Keycloak client Standard Flow misconfiguration risk | medium | high | Test config changes in staging. Both flows coexist on same client |
| gocloak may not cover SAML IdP Admin API | medium | medium | Check gocloak source early. Prepare direct HTTP fallback |
| SAML attribute mapping varies across IdP vendors | medium | medium | Standard attribute mapper template. Document required IdP attributes |
| RequireRole middleware variable shadowing bug | low | medium | Fix before or during SSO work -- signals code health risk in auth module |
| Single `email_domain` per tenant limitation | low | medium | Defer for MVO. Plan `tenant_email_domains` join table if needed later |

## Solution Direction

Recommended hybrid approach: **Safe + Minimal first, then Systematic**. Establish integration tests for the existing login flow (safety net), then implement MVO SSO (one tenant end-to-end with manual Keycloak config). Defer Keycloak Admin API automation to when scale demands it. This balances Q3 delivery pressure with regression safety. User confirmed this direction.

## Accepted Assumptions
- Keycloak's SAML brokering capabilities are sufficient for the multi-tenant SSO pattern (one IdP per tenant, shared realm). This is based on Keycloak's documented SAML support but has not been tested with this specific setup.
- The auth middleware (`internal/auth/middleware.go:Authenticate`) will work unchanged for SSO-authenticated requests because it validates JWTs regardless of issuance method. This needs verification testing but is architecturally sound.
- The `email_domain` UNIQUE constraint and single-domain-per-tenant model is sufficient for MVO enterprise customers. Enterprises with multiple domains are an edge case that can be deferred.
- Both Direct Access Grants and Standard Flow (Authorization Code) can coexist on the same Keycloak client without interference.

## Deferred Decisions
- **Password fallback policy for SSO tenants:** Whether to block or allow password login for SSO-enabled tenants. Significant UX and security implications. To be resolved during planning or early implementation.
- **Per-tenant config storage format:** New table (`tenant_sso_config`) vs. JSONB column. Both are viable. To be decided during planning based on future extensibility analysis.
- **SSO and password coexistence within a tenant during transition:** Whether enabling SSO is all-or-nothing or gradual. To be resolved alongside password fallback policy.
- **Required SAML attributes specification:** Exact list of required vs. optional IdP attributes. To be defined during Keycloak mapper configuration planning.

## User Corrections Log
No corrections were made -- the user confirmed all five blocks as presented without changes.

## Acceptance Criteria
- Enterprise employee can log in via SAML SSO through their corporate IdP and land authenticated in the application with correct tenant context and role
- Non-SSO tenant email/password login is completely unchanged (same API contract, same behavior, same errors)
- First-time SSO users are automatically provisioned with correct tenant association and `keycloak_id`
- Existing password users are linked (not duplicated) when their tenant enables SSO
- SSO misconfiguration for one tenant does not affect other tenants or non-SSO login
- SSO-issued JWTs contain all required claims (`sub`, `email`, `tenant_id`, `realm_access.roles`) for full middleware and handler compatibility
- All new SSO code follows the repository-service-handler pattern
- Integration tests exist for the existing login flow before SSO modifications
- New SSO code includes tests
