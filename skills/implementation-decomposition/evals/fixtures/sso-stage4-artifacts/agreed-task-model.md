# Agreed Task Model

> Agreed on: 2026-03-10
> Based on: Stage 2 analyses + user review

## Task Goal
Enable enterprise tenants to authenticate their users via SAML 2.0 SSO through corporate identity providers, with Keycloak brokering the SAML protocol, while preserving the existing email/password flow completely.

## Problem Statement
Enterprise adoption is blocked because organizations cannot enforce their corporate authentication policies on the platform. SAML SSO is table-stakes for enterprise sales, and Q3 delivery is prioritized to unblock active enterprise pipeline.

## Scope

### Included
- SP-initiated SAML 2.0 SSO login flow via Keycloak broker
- Per-tenant SSO configuration storage (dedicated `tenant_sso_config` table)
- JIT user provisioning for SSO users
- Account linking by email for existing password users
- Keycloak SAML IdP configuration automation via Admin API
- SSO login and callback endpoints
- Password login available as fallback for SSO tenants (dual-auth)

### Excluded
- IdP-initiated SSO
- Single Logout (SLO)
- SCIM user provisioning
- Self-service SSO configuration UI
- OIDC/OAuth SSO
- Multi-realm Keycloak architecture

## Key Scenarios

### Primary Scenario
1. User enters email on login page
2. Frontend calls SSO check endpoint with email
3. SSO-enabled → redirect to SSO initiation endpoint
4. Backend resolves tenant, redirects to Keycloak with IdP hint
5. Keycloak brokers SAML auth with corporate IdP
6. User authenticates at IdP
7. Keycloak receives assertion, issues JWT
8. Callback with authorization code → backend exchanges for tokens
9. JIT provisioning: find or create user by email, link KeycloakID
10. Return JWT tokens to frontend

### Mandatory Edge Cases
- Existing password user + SSO enabled → auto email linking
- SSO user attempts password login → allowed (dual-auth)
- Unknown email domain → normal password login
- Keycloak/IdP unavailable → graceful error, password fallback
- Missing SAML attributes → reject with clear error

### Explicitly Deferred
- Multi-domain per tenant SSO mapping
- SSO-only enforcement mode
- Self-service SSO config UI
- SAML metadata auto-rotation

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Auth | `internal/auth/` | New SSO endpoints, service logic, Keycloak integration | large |
| Tenant | `internal/tenant/` | SSO config storage, domain-based lookup | medium |
| User | `internal/user/` | JIT provisioning, KeycloakID linking | small |
| Config | `internal/config/` | SSO-related configuration | small |
| Server | `cmd/server/main.go` | Route registration | small |
| Migrations | `migrations/` | New table for SSO config | medium |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/auth/handler.go` | New SSO handlers | SSO flow entry points |
| `internal/auth/service.go` | SSO business logic | Core SSO flow |
| `internal/auth/keycloak.go` | Auth code exchange, IdP management | Keycloak SSO integration |
| `internal/tenant/models.go` | SSOConfig struct | Per-tenant SSO data |
| `internal/tenant/repository.go` | SSO config queries | SSO config persistence |
| `migrations/002_sso_config.sql` | New table | SSO config schema |
| `internal/user/service.go` | KeycloakID update | Account linking |
| `cmd/server/main.go` | SSO routes | Endpoint exposure |

### Dependencies
- **Keycloak v24.0:** SAML broker, Standard Flow, attribute mappers
- **gocloak v13.9.0:** Auth code exchange, IdP management (needs verification)
- **Enterprise IdPs:** External SAML providers

## Confirmed Constraints
- **Backward compatibility:** `POST /api/auth/login` unchanged — confirmed
- **Single Keycloak realm:** all IdPs coexist — confirmed
- **Pattern compliance:** repository-service-handler — confirmed
- **JWT claims:** `sub`, `email`, `tenant_id`, `realm_access.roles` — confirmed
- **Dedicated table:** `tenant_sso_config` not JSONB — user decision
- **Q3 timeline:** MVO-scoped delivery — confirmed

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Zero test coverage + auth changes | high | high | Tests before modifying, tests for all SSO code |
| Keycloak SAML config complexity | medium | high | Automate via Admin API |
| Account linking edge cases | medium | medium | Email-only matching, reject mismatches |
| gocloak SAML API gaps | medium | medium | Check early, fallback HTTP client |
| Standard Flow config risk | medium | high | Test in staging |

## Solution Direction
Systematic — proper SSO subsystem, dedicated storage, automated Keycloak setup, clean separation. MVO-scoped for Q3.

## Accepted Assumptions
- Keycloak v24.0 supports SAML IdP brokering
- Single realm sufficient for <50 tenants
- Email domain sufficient for tenant SSO detection

## Deferred Decisions
- Multi-domain mapping: deferred until multi-domain tenants exist
- SSO enforcement mode: deferred, dual-auth is sufficient for MVO

## User Corrections Log
- **Password fallback:** Proposed SSO-only for security → User chose dual-auth for availability during transition
- **Config storage:** Proposed JSONB column for simplicity → User chose dedicated table for precedent and query flexibility
- **Account linking:** Proposed admin-driven linking → User chose automatic email-based linking for lower friction

## Acceptance Criteria
- SP-initiated SAML SSO login completes successfully through corporate IdP
- Email/password login works identically before and after
- New SSO users are JIT-provisioned with correct tenant
- Existing users auto-linked on first SSO login via email
- Per-tenant SSO config stored and retrievable
- Keycloak IdP config automated via Admin API
- JWT tokens from SSO contain all required claims
