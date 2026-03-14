# Implementation Design

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Solution direction: systematic
> Design status: finalized

## Implementation Approach

### Chosen Approach
Build a dedicated SSO subsystem following the existing repository-service-handler pattern. The auth module gains an `SSOService` that orchestrates the full SAML flow and a `KeycloakClient` that wraps the Keycloak Admin REST API. Per-tenant SSO configuration is stored in a dedicated `tenant_sso_config` database table, accessed through the tenant repository. The user module is extended with JIT provisioning and automatic email-based account linking.

This approach was chosen because it integrates naturally with the existing architecture — every other feature in the codebase follows the same repository-service-handler pattern. The dedicated SSO config table (rather than JSONB) was a user decision, prioritizing queryability and type safety.

The systematic approach is justified by the feature's complexity: SSO involves cross-module coordination (auth, tenant, user), external service integration (Keycloak), and security-sensitive flows that benefit from explicit, well-tested code paths.

### Alternatives Considered
- **JSONB column on tenants table:** Simpler schema, but user rejected — harder to query, no type safety at database level
- **Separate SSO microservice:** Over-engineered for current scale; adds network hop and deployment complexity
- **Direct SAML handling (no Keycloak):** Requires implementing SAML SP from scratch; Keycloak provides battle-tested SAML handling

### Approach Trade-offs
This approach adds Keycloak as an infrastructure dependency. If Keycloak is unavailable, SSO logins fail (password fallback mitigates this). The single-realm model simplifies operations but means tenant isolation is logical (via IdP naming), not physical.

## Solution Description

### Overview
The SSO flow starts when a user enters their email on the login page. The `HandleSSOInitiate` endpoint checks the email domain against `tenant_sso_config`. If an SSO config exists, it generates a SAML AuthnRequest URL via the `SSOService` and redirects the user to Keycloak. Keycloak brokers authentication with the tenant's external IdP. On success, Keycloak redirects to `HandleSSOCallback` with a SAML assertion. The callback handler extracts user attributes, calls `SSOService.ProcessCallback` which performs JIT provisioning or account linking via the user service, issues a session token, and redirects to the application.

### Data Flow
1. **Entry:** `POST /auth/sso/initiate` with `{"email": "user@company.com"}`
2. **Domain lookup:** `TenantRepository.GetSSOConfigByDomain("company.com")` → `TenantSSOConfig`
3. **Redirect:** `SSOService.InitiateSSO(config)` → generates SAML AuthnRequest → redirect to Keycloak
4. **External auth:** Keycloak → external IdP → SAML assertion → Keycloak
5. **Callback:** `GET /auth/sso/callback?SAMLResponse=...`
6. **Attribute extraction:** Parse SAML response → extract email, name, groups
7. **User resolution:** `UserService.FindOrCreateBySSO(email, attributes)` → find existing user by email OR create new user with JIT provisioning
8. **Account linking:** If existing user found, `UserService.LinkKeycloakID(user, keycloakID)`
9. **Session:** Issue JWT session token (reuses existing `AuthService.IssueToken`)
10. **Redirect:** Redirect to application with token

### New Entities

| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `SSOService` | service | `internal/auth/sso_service.go` | Orchestrates SSO initiate and callback flows |
| `KeycloakClient` | client | `internal/auth/keycloak_client.go` | Wraps Keycloak Admin REST API for IdP CRUD |
| `TenantSSOConfig` | model | `internal/tenant/model.go` | Per-tenant SSO configuration (entity ID, metadata URL, certificate, domain) |
| `SSOConfig` | config struct | `internal/config/config.go` | Base SSO settings (Keycloak URL, realm, admin credentials) |
| `HandleSSOInitiate` | handler | `internal/auth/handler.go` | HTTP handler for SSO initiation |
| `HandleSSOCallback` | handler | `internal/auth/handler.go` | HTTP handler for SAML callback |
| `tenant_sso_config` | table | `migrations/00X_add_tenant_sso_config.sql` | Database table for SSO configs |

### Modified Entities

| Entity | Location | Current Behavior | New Behavior | Breaking? |
|--------|----------|-----------------|-------------|-----------|
| `User` | `internal/user/model.go` | No Keycloak identity | Gains optional `KeycloakID *string` field | no |
| `UserRepository` | `internal/user/repository.go` | CRUD by ID/email | Adds `FindByEmail`, `UpdateKeycloakID` methods | no |
| `UserService` | `internal/user/service.go` | Standard user operations | Adds `FindOrCreateBySSO`, `LinkKeycloakID` methods | no |
| `TenantRepository` | `internal/tenant/repository.go` | Tenant CRUD | Adds `GetSSOConfigByDomain`, `UpsertSSOConfig` methods | no |
| `Router` | `cmd/server/main.go` | Auth routes only | Adds `/auth/sso/initiate`, `/auth/sso/callback` routes | no |

## Change Details

### Module: Auth
**Path:** `internal/auth/`
**Role in changes:** Core SSO flow implementation — service orchestration, Keycloak integration, HTTP handlers

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `sso_service.go` | create | SSO service with `InitiateSSO(config) → redirectURL` and `ProcessCallback(samlResponse) → (user, token)` | large |
| `keycloak_client.go` | create | Keycloak Admin API client with `CreateIdP(tenantID, metadata)`, `GetIdP(tenantID)`, `DeleteIdP(tenantID)` | medium |
| `handler.go` | modify | Add `HandleSSOInitiate` and `HandleSSOCallback` handler methods | medium |
| `sso_service_test.go` | create | Unit tests for SSO service logic | medium |
| `keycloak_client_test.go` | create | Unit tests for Keycloak client (with HTTP mock) | small |

**Interfaces affected:**
- New `SSOService` interface: `InitiateSSO(ctx, email) (redirectURL, error)`, `ProcessCallback(ctx, samlResponse) (user, token, error)`
- New `KeycloakClient` interface: `CreateIdP(ctx, tenantID, metadata) error`, `GetIdP(ctx, tenantID) (IdPConfig, error)`

**Tests needed:**
- SSO initiation with valid/invalid domain
- SAML callback processing with new user (JIT) and existing user (linking)
- Keycloak client CRUD operations (mocked HTTP)
- Error handling: Keycloak unavailable, invalid SAML response, unknown domain

### Module: Tenant
**Path:** `internal/tenant/`
**Role in changes:** SSO configuration storage and domain-based lookup

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `model.go` | modify | Add `TenantSSOConfig` struct with fields: ID, TenantID, Domain, EntityID, MetadataURL, Certificate, Enabled, CreatedAt, UpdatedAt | small |
| `repository.go` | modify | Add `GetSSOConfigByDomain(domain) → (TenantSSOConfig, error)` and `UpsertSSOConfig(config) error` methods | medium |
| `repository_test.go` | modify | Add tests for new repository methods | small |

**Interfaces affected:**
- `TenantRepository` gains two new methods (additive, non-breaking)

**Tests needed:**
- GetSSOConfigByDomain: found, not found, disabled config
- UpsertSSOConfig: insert new, update existing

### Module: User
**Path:** `internal/user/`
**Role in changes:** JIT provisioning and account linking

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `model.go` | modify | Add `KeycloakID *string` field to `User` struct, add `json` and `db` tags | small |
| `service.go` | modify | Add `FindOrCreateBySSO(email, attributes) → (user, created, error)` and `LinkKeycloakID(userID, keycloakID) error` methods | medium |
| `repository.go` | modify | Add `FindByEmail(email) → (user, error)` and `UpdateKeycloakID(userID, keycloakID) error` methods | small |
| `service_test.go` | modify | Add tests for SSO-related service methods | small |

**Interfaces affected:**
- `UserRepository` gains two new methods (additive)
- `UserService` gains two new methods (additive)
- `User` struct gains one new optional field (read-compatible)

**Tests needed:**
- FindOrCreateBySSO: new user created, existing user found and linked
- Account linking: successful link, already linked, race condition handling

### Module: Config
**Path:** `internal/config/`
**Role in changes:** SSO base configuration loading

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `config.go` | modify | Add `SSOConfig` struct (KeycloakURL, Realm, AdminUser, AdminPassword) and embed in main Config | small |

**Tests needed:**
- Config loads SSO settings from environment variables

### Module: Migrations
**Path:** `migrations/`
**Role in changes:** Schema changes for SSO support

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `00X_add_tenant_sso_config.sql` | create | CREATE TABLE tenant_sso_config (id, tenant_id, domain, entity_id, metadata_url, certificate, enabled, created_at, updated_at); UNIQUE INDEX on domain | small |
| `00Y_add_user_keycloak_id.sql` | create | ALTER TABLE users ADD COLUMN keycloak_id VARCHAR(255); UNIQUE INDEX on keycloak_id | small |

**Tests needed:**
- Migrations run cleanly up and down

### Module: Server
**Path:** `cmd/server/`
**Role in changes:** SSO route registration and dependency wiring

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `main.go` | modify | Initialize KeycloakClient and SSOService; register `/auth/sso/initiate` (POST) and `/auth/sso/callback` (GET) routes | small |

**Tests needed:**
- Routes are registered and accessible

## Key Technical Decisions

| # | Decision | Reasoning | Alternatives Rejected | User Approved? |
|---|----------|-----------|----------------------|----------------|
| 1 | Dedicated `tenant_sso_config` table | User chose structured table for queryability and type safety over JSONB | JSONB column on tenants | yes |
| 2 | Automatic email-based account linking | User preferred automatic over admin-driven; email is reliable identifier | Admin-driven linking, manual claim | yes |
| 3 | Dual-auth (SSO + password fallback) | User required password fallback even when SSO is enabled for the tenant | SSO-only mode | yes |
| 4 | Single Keycloak realm, IdP per tenant | Simplifies ops; tenant isolation via unique IdP alias naming (`tenant-{id}-saml`) | Realm per tenant | yes |
| 5 | Keycloak Admin REST API (not CLI) | Programmatic IdP management from within the application; no external tooling needed | `kcadm.sh` CLI, Terraform provider | not required |

## Dependencies

### Internal Dependencies
- **Auth → Tenant:** SSOService calls TenantRepository.GetSSOConfigByDomain
- **Auth → User:** SSOService calls UserService.FindOrCreateBySSO and LinkKeycloakID
- **Auth → Config:** SSOService reads SSOConfig for Keycloak base URL and realm
- **Server → Auth:** main.go initializes and wires SSOService and KeycloakClient

### External Dependencies
- **Keycloak v24.0+:** Required for SAML broker functionality; Admin REST API used for IdP management
- **crewjam/saml Go library:** For SAML request/response handling (SP implementation)

### Migration Dependencies
- `00X_add_tenant_sso_config.sql` must run before tenant repository methods are used
- `00Y_add_user_keycloak_id.sql` must run before user linking methods are used
- Both migrations must run before the SSO service is started

## Implementation Sequence

| Step | What | Why This Order | Validates |
|------|------|----------------|-----------|
| 1 | Database migrations | Foundation: schema must exist before any code uses it | Tables created, constraints work |
| 2 | Config struct and env loading | Foundation: other modules need config values | Config loads from env/file |
| 3 | Tenant SSO config model and repository | Foundation: SSO service needs to look up tenant configs | CRUD for SSO configs works |
| 4 | User model extension and repository | Foundation: SSO service needs user provisioning/linking | KeycloakID field, new methods work |
| 5 | Keycloak Admin API client | Independent: can be developed alongside steps 3-4 | IdP CRUD operations work |
| 6 | SSO service (initiate + callback) | Core: depends on steps 2-5 | Full SSO flow logic works |
| 7 | SSO HTTP handlers | Interface: wraps service for HTTP access | Endpoints work with proper I/O |
| 8 | Route registration and wiring | Integration: connects everything in main.go | End-to-end flow accessible |

## Risk Zones

| Risk Zone | Location | What Could Go Wrong | Mitigation | Severity |
|-----------|----------|-------------------|------------|----------|
| SAML XML handling | `sso_service.go` | Malformed SAML responses, XML signature validation failures | Use crewjam/saml library; validate signatures; test with real Keycloak | medium |
| Account linking race | `user/service.go:FindOrCreateBySSO` | Concurrent SSO + password login creates duplicate accounts | DB unique constraint on keycloak_id; transaction with SELECT FOR UPDATE | medium |
| Keycloak availability | `keycloak_client.go` | Keycloak down during SSO initiation or IdP setup | Timeout + retry for admin ops; graceful error for auth flow; password fallback | medium |
| Domain ownership | `tenant/repository.go:GetSSOConfigByDomain` | Tenant claims domain they don't own | Out of scope for this task (noted as future work) | low |

## Backward Compatibility

### API Changes
No existing API contracts are modified. Two new endpoints added: `POST /auth/sso/initiate`, `GET /auth/sso/callback`.

### Data Changes
- New table `tenant_sso_config` — no impact on existing data
- New column `users.keycloak_id` (nullable) — no impact on existing users

### Behavioral Changes
No existing behaviors change. Password authentication continues to work unchanged. SSO is purely additive.

## Critique Review
Design critic returned DESIGN_APPROVED. All 8 criteria scored PASS. Minor observation: consider adding structured logging to the SSO flow for debugging SAML issues. Incorporated as a note in the implementation design.

## User Approval Log
- **Config storage:** User chose dedicated table over JSONB → design uses `tenant_sso_config` table
- **Account linking:** User chose automatic email-based over admin-driven → design uses email matching
- **Password fallback:** User chose dual-auth over SSO-only → design preserves password auth
