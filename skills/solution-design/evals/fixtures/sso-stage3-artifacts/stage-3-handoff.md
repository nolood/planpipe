# Stage 3 Handoff — Task Synthesis Complete

> Status: draft — pending user confirmation in Stage 4

## Task Summary
Add SAML 2.0 Single Sign-On support to a multi-tenant Go backend platform so that enterprise tenants can authenticate their employees through corporate identity providers. Keycloak acts as SAML SP/broker. Existing email/password login remains fully functional for non-SSO tenants. SSO configuration is per-tenant.

## Classification
- **Type:** feature
- **Complexity:** high
- **Primary risk area:** integration — Keycloak SAML brokering, enterprise IdP interoperability, auth flow paradigm shift
- **Solution direction:** systematic — build a proper per-tenant SSO subsystem rather than minimal patches, but MVO-scoped for Q3

## Synthesized Goal
Enable enterprise tenants to authenticate their users via SAML 2.0 SSO through corporate identity providers, with Keycloak brokering the SAML protocol, while preserving the existing email/password flow completely.

## Synthesized Problem Statement
Enterprise adoption is blocked because organizations cannot enforce their corporate authentication policies on the platform. SAML SSO is table-stakes for enterprise sales, and Q3 delivery is prioritized to unblock active enterprise pipeline.

## Synthesized Scope

### Included
- SP-initiated SAML 2.0 SSO login flow via Keycloak broker
- Per-tenant SSO configuration storage (new `tenant_sso_config` table — user chose dedicated table over JSONB)
- JIT user provisioning for SSO users (match by email, create if new)
- Account linking by email for existing password users enabling SSO
- Keycloak SAML IdP configuration automation via Admin API
- SSO login endpoint (`GET /api/auth/sso/initiate`) and callback endpoint (`GET /api/auth/sso/callback`)
- Password login remains available for SSO tenants as fallback (user chose dual-auth over SSO-only)

### Excluded
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning
- Self-service SSO configuration UI (admin does it via API/DB)
- OIDC/OAuth SSO (SAML 2.0 only)
- Multi-realm Keycloak architecture

## Key Scenarios for Planning

### Primary Scenario
1. User enters email on login page
2. Frontend calls `/api/auth/sso/check` with email to detect SSO tenant
3. If SSO-enabled: frontend redirects to `/api/auth/sso/initiate?email=...`
4. Backend resolves tenant by email domain, finds SSO config, redirects to Keycloak auth URL with IdP hint
5. Keycloak brokers SAML authentication with the tenant's corporate IdP
6. User authenticates at corporate IdP
7. Keycloak receives SAML assertion, creates/links user, issues JWT
8. Keycloak redirects to `/api/auth/sso/callback` with authorization code
9. Backend exchanges code for tokens via Keycloak, extracts user info
10. Backend does JIT provisioning (find or create user by email, link KeycloakID)
11. Backend returns JWT tokens to frontend
12. User is authenticated in the application

### Mandatory Edge Cases
- Existing password user whose tenant enables SSO → automatic email-based linking, KeycloakID populated on first SSO login
- SSO-enabled tenant user attempts password login → allowed (dual-auth confirmed)
- Email domain not mapped to any SSO tenant → normal password login
- Keycloak/IdP unavailable → graceful error, password fallback available
- SAML assertion missing required attributes → reject with clear error

## System Map for Planning

### Modules to Change
| Module | Path | What Changes | Scope |
|--------|------|-------------|-------|
| Auth | `internal/auth/` | New SSO endpoints, SSO service methods, Keycloak authorization code flow | large |
| Tenant | `internal/tenant/` | SSO config model, repository methods, email domain lookup | medium |
| User | `internal/user/` | JIT provisioning enhancement (KeycloakID population) | small |
| Config | `internal/config/` | SSO-related base configuration (callback URLs, SAML defaults) | small |
| Server | `cmd/server/main.go` | Route registration for SSO endpoints | small |
| Migrations | `migrations/` | New `tenant_sso_config` table migration | medium |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/handler.go` | New SSO initiation and callback HTTP handlers | Entry points for SSO flow |
| `internal/auth/service.go` | New SSO methods: CheckSSO, InitiateSSO, HandleCallback | Core SSO business logic |
| `internal/auth/keycloak.go` | Authorization code exchange, IdP management via Admin API | Keycloak integration for SSO |
| `internal/tenant/models.go` | SSOConfig struct added | Per-tenant SSO data model |
| `internal/tenant/repository.go` | GetSSOConfigByDomain, SaveSSOConfig methods | SSO config persistence |
| `migrations/002_sso_config.sql` | New `tenant_sso_config` table | Schema for SSO configuration |
| `internal/user/service.go` | UpdateKeycloakID method, modify GetOrCreateByEmail | Link users to Keycloak accounts |
| `cmd/server/main.go` | Register `/api/auth/sso/*` routes | Expose SSO endpoints |

### Critical Dependencies
- **Keycloak v24.0:** SAML IdP broker config per tenant, Standard Flow on client, attribute mappers for tenant_id
- **gocloak v13.9.0:** Authorization code exchange + IdP management (needs verification)
- **Enterprise IdPs:** External, customer-controlled. SAML metadata exchange during setup

## Constraints for Planning
- Existing `POST /api/auth/login` must work identically for non-SSO tenants
- Single Keycloak realm — all tenant IdPs coexist with unique aliases
- Repository-service-handler pattern — no shortcuts
- JWT claims must include `sub`, `email`, `tenant_id`, `realm_access.roles`
- Per-tenant SSO config goes in a dedicated table, not JSONB (synthesized recommendation)
- Q3 delivery timeline — scope decisions favor MVO

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Zero test coverage makes auth flow changes dangerous | high | high | Write integration tests for existing login before modifying, all SSO code includes tests |
| Keycloak SAML broker configuration complexity at scale | medium | high | Automate via Admin API, create IdP config templates |
| Account linking edge cases (email mismatch, multiple accounts) | medium | medium | Match by email only, reject mismatches with clear error, log for admin review |
| gocloak may not cover SAML IdP Admin API | medium | medium | Check early, prepare fallback HTTP client |
| Keycloak client Standard Flow configuration | medium | high | Test in staging, both direct grant and standard flow can coexist |

## Product Requirements for Planning
- **Primary scenario:** SP-initiated SAML SSO login via Keycloak broker
- **Success signals:** Enterprise deal closure rate, SSO login success rate (>95%), time-to-SSO-configuration
- **Minimum viable outcome:** SP-initiated SAML SSO for at least one tenant, JIT provisioning, coexisting with email/password
- **Backward compatibility:** Email/password login completely unaffected for all tenants

## Solution Direction
Systematic — build a proper SSO subsystem with dedicated config storage, automated Keycloak setup, and clean separation from the password flow. But scoped to MVO: no UI, no SLO, no SCIM. Pending user confirmation in Stage 4.

## Assumptions (pending confirmation)
- Keycloak v24.0 supports SAML IdP brokering (well-documented feature)
- Single realm architecture is sufficient for initial deployment (< 50 tenants)
- Email domain is sufficient for tenant SSO detection (one domain per tenant initially)

## Deferred Items
- Multi-domain per tenant SSO mapping
- SSO-only enforcement (blocking password login for SSO tenants)
- Self-service SSO configuration UI
- SAML metadata auto-rotation

## Acceptance Criteria
- Enterprise user can complete SP-initiated SAML SSO login through their corporate IdP
- Existing email/password login works identically before and after the change
- New SSO users are JIT-provisioned with correct tenant association
- Existing password users are automatically linked on first SSO login via email match
- Per-tenant SSO configuration is stored and retrievable
- Keycloak SAML IdP configuration can be automated via Admin API
- JWT tokens from SSO flow contain all required claims for middleware compatibility

## Detailed References
- `analysis.md` — synthesized task analysis
- `agreement-package.md` — agreement blocks for Stage 4's combined review
- `agreed-task-model.md` — draft task model (pending user confirmation)
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
