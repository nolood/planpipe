# Execution Backlog

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Implementation approach: Systematic — dedicated SSO subsystem following repository-service-handler pattern, dedicated tenant_sso_config table, automatic email-based account linking, Keycloak Admin REST API integration, dual-auth support
> Total subtasks: 10
> Execution waves: 4
> Decomposition status: draft

## Execution Overview

The implementation is decomposed into 10 subtasks organized in 4 execution waves. Wave 1 establishes the foundation — database schema, configuration, and data models. Wave 2 builds the core implementation — repository methods, user service extensions, and the Keycloak client. Wave 3 constructs the SSO orchestration layer — the SSO service and HTTP handlers. Wave 4 handles integration wiring and convergence testing. The structure maximizes parallelism within waves (up to 3 subtasks running simultaneously in Waves 1 and 2) while respecting the natural dependency chain from schema through models, repositories, services, handlers, to wiring.

## Execution Waves

### Wave 1 — Foundation
Establishes the database schema, application configuration, and data models that all subsequent work depends on. These subtasks have no inter-dependencies and can run fully in parallel.

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-1 | Database Migrations for SSO | foundation | small | ST-2, ST-3 |
| ST-2 | SSO Configuration Struct | foundation | small | ST-1, ST-3 |
| ST-3 | Tenant and User Model Extensions | foundation | small | ST-1, ST-2 |

### Wave 2 — Core Implementation
Builds the repository layer, user service extensions, and the Keycloak client. These subtasks depend on the foundation models from Wave 1 but are independent of each other.

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-4 | Tenant SSO Config Repository Methods | implementation | medium | ST-5, ST-6 |
| ST-5 | User Repository and Service SSO Extensions | implementation | medium | ST-4, ST-6 |
| ST-6 | Keycloak Admin API Client | implementation | medium | ST-4, ST-5 |

### Wave 3 — SSO Orchestration
Builds the SSO service that orchestrates the full SAML flow and the HTTP handlers that expose it. The SSO service depends on the tenant repository, user service, Keycloak client, and config from Waves 1-2. The HTTP handlers depend on the SSO service.

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-7 | SSO Service Implementation | implementation | large | — |
| ST-8 | SSO HTTP Handlers | implementation | medium | — |

### Wave 4 — Integration & Convergence
Wires everything together in main.go and runs end-to-end verification. Route registration depends on all handlers and services being complete.

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-9 | Route Registration and Dependency Wiring | integration | small | — |
| ST-10 | End-to-End Integration Testing | testing | medium | — |

## Dependency Graph

```
ST-1 (migrations)        ──→ ST-4 (tenant repo)
                          ──→ ST-5 (user repo+service)
ST-2 (config)             ──→ ST-6 (keycloak client)
                          ──→ ST-7 (sso service)
ST-3 (models)             ──→ ST-4 (tenant repo)
                          ──→ ST-5 (user repo+service)
                          ──→ ST-6 (keycloak client)
ST-4 (tenant repo)        ──→ ST-7 (sso service)
ST-5 (user repo+service)  ──→ ST-7 (sso service)
ST-6 (keycloak client)    ──→ ST-7 (sso service)
ST-7 (sso service)        ──→ ST-8 (handlers)
ST-8 (handlers)           ──→ ST-9 (wiring)
ST-7 + ST-8               ──→ ST-9 (wiring)
ST-9 (wiring)             ──→ ST-10 (integration testing)
```

## Conflict Zones

| # | Zone | Subtasks Involved | Conflict Type | Severity | Resolution |
|---|------|-------------------|---------------|----------|------------|
| 1 | `internal/tenant/model.go` | ST-3, ST-4 | file collision | low | Sequenced: ST-3 adds the struct definition in Wave 1, ST-4 adds repository methods in a different file (`repository.go`) in Wave 2. The model.go change is fully contained in ST-3. No actual overlap. |
| 2 | `internal/user/model.go` + `internal/user/service.go` | ST-3, ST-5 | file collision | low | Sequenced: ST-3 adds the `KeycloakID` field to `model.go` in Wave 1. ST-5 adds service methods in `service.go` and repository methods in `repository.go` in Wave 2. ST-5 depends on ST-3. Clean boundary. |
| 3 | `internal/auth/handler.go` | ST-8 | file collision | low | Only ST-8 modifies `handler.go`. No conflict — single ownership. |
| 4 | `internal/auth/` directory | ST-6, ST-7, ST-8 | semantic collision | low | These subtasks create/modify different files in the same directory. ST-6 creates `keycloak_client.go`, ST-7 creates `sso_service.go`, ST-8 modifies `handler.go`. All are sequenced across Waves 2-3. No actual file overlap. |

---

## Subtasks

### ST-1: Database Migrations for SSO

**ID:** ST-1
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Creates the database schema required for SSO functionality. Two migrations establish the `tenant_sso_config` table for per-tenant SSO configuration and add the `keycloak_id` column to the `users` table for Keycloak identity linking. All subsequent repository and service code depends on these schema changes.

#### Goal
Both migration files exist, run cleanly (up and down), and the database schema supports SSO configuration storage and user Keycloak identity linking.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Migrations | `migrations/00X_add_tenant_sso_config.sql` | create | CREATE TABLE tenant_sso_config with columns: id (UUID PK), tenant_id (FK to tenants), domain (VARCHAR, UNIQUE), entity_id (VARCHAR), metadata_url (VARCHAR), certificate (TEXT), enabled (BOOL DEFAULT false), created_at (TIMESTAMP), updated_at (TIMESTAMP). Unique index on domain. |
| Migrations | `migrations/00Y_add_user_keycloak_id.sql` | create | ALTER TABLE users ADD COLUMN keycloak_id VARCHAR(255) NULL. Unique index on keycloak_id. |

#### Boundaries

**In scope:**
- Creating both migration SQL files
- UP and DOWN migration directions
- Table structure, column types, constraints, and indexes as specified in the implementation design
- Unique index on `tenant_sso_config.domain`
- Unique index on `users.keycloak_id`

**Out of scope:**
- Go model structs (handled by ST-3)
- Repository methods that query these tables (handled by ST-4 and ST-5)
- Populating any data in the tables

#### Context

**Related design decisions:**
- DD-1: Dedicated `tenant_sso_config` table — this subtask creates the table structure that implements this decision. The table is normalized with typed columns rather than JSONB, per the user's choice for queryability and type safety.

**Applicable constraints:**
- Migrations must support both UP and DOWN directions
- `tenant_sso_config.domain` must have a UNIQUE constraint (used for email domain lookup)
- `users.keycloak_id` must be nullable (existing users don't have Keycloak IDs) and UNIQUE (one user per Keycloak identity)

**Key scenarios covered:**
- Foundation for per-tenant SSO configuration storage
- Foundation for Keycloak identity linking

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] `migrations/00X_add_tenant_sso_config.sql` exists with correct CREATE TABLE statement and UNIQUE index on domain
- [ ] `migrations/00Y_add_user_keycloak_id.sql` exists with correct ALTER TABLE statement and UNIQUE index on keycloak_id
- [ ] Both migrations include DOWN (rollback) statements
- [ ] Migrations run cleanly on a fresh database and roll back cleanly

---

### ST-2: SSO Configuration Struct

**ID:** ST-2
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Adds the `SSOConfig` struct to the application's configuration module. This struct holds the base Keycloak connection settings (URL, realm, admin credentials) that the SSO service and Keycloak client need to communicate with Keycloak. Without this, no SSO-related module can connect to Keycloak.

#### Goal
The `SSOConfig` struct is added to `internal/config/config.go`, embedded in the main `Config` struct, and loads from environment variables.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Config | `internal/config/config.go` | modify | Add `SSOConfig` struct with fields: KeycloakURL (string), Realm (string), AdminUser (string), AdminPassword (string). Embed `SSOConfig` in the main `Config` struct. Add env variable loading for SSO_KEYCLOAK_URL, SSO_KEYCLOAK_REALM, SSO_KEYCLOAK_ADMIN_USER, SSO_KEYCLOAK_ADMIN_PASSWORD. |

#### Boundaries

**In scope:**
- `SSOConfig` struct definition with all four fields
- Embedding in the main `Config` struct
- Environment variable loading for the four SSO settings
- Default value for `Realm` field (default: `master`)

**Out of scope:**
- Keycloak client implementation that uses this config (handled by ST-6)
- SSO service that reads this config (handled by ST-7)
- Any validation beyond basic env loading

#### Context

**Related design decisions:**
- DD-4: Single Keycloak realm — the `Realm` field is a single value (not per-tenant). This is because all tenants share one realm, with isolation via per-tenant IdP naming.
- DD-5: Keycloak Admin REST API — the `AdminUser` and `AdminPassword` fields are needed because the application manages IdP configurations programmatically via the Admin API.

**Applicable constraints:**
- Must follow existing config loading pattern in `config.go`
- Admin credentials are stored in config, not hardcoded

**Key scenarios covered:**
- All SSO operations require Keycloak connection settings

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] `SSOConfig` struct exists in `internal/config/config.go` with KeycloakURL, Realm, AdminUser, AdminPassword fields
- [ ] `SSOConfig` is embedded in the main `Config` struct
- [ ] Environment variables SSO_KEYCLOAK_URL, SSO_KEYCLOAK_REALM, SSO_KEYCLOAK_ADMIN_USER, SSO_KEYCLOAK_ADMIN_PASSWORD are loaded
- [ ] Config loads SSO settings correctly from environment variables

---

### ST-3: Tenant and User Model Extensions

**ID:** ST-3
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Extends the data models in the tenant and user modules to support SSO. Adds the `TenantSSOConfig` struct to the tenant module (representing a row in the `tenant_sso_config` table) and adds the `KeycloakID` optional field to the `User` struct. These model changes are the foundation for all repository and service methods that follow.

#### Goal
The `TenantSSOConfig` struct exists in `internal/tenant/model.go` with all required fields, and the `User` struct in `internal/user/model.go` has an optional `KeycloakID` field with proper tags.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Tenant | `internal/tenant/model.go` | modify | Add `TenantSSOConfig` struct with fields: ID (uuid), TenantID (uuid), Domain (string), EntityID (string), MetadataURL (string), Certificate (string), Enabled (bool), CreatedAt (time.Time), UpdatedAt (time.Time). Add appropriate `json` and `db` tags. |
| User | `internal/user/model.go` | modify | Add `KeycloakID *string` field to the existing `User` struct with `json:"keycloak_id,omitempty"` and `db:"keycloak_id"` tags. |

#### Boundaries

**In scope:**
- `TenantSSOConfig` struct definition with all fields and tags
- `KeycloakID *string` field on the `User` struct with tags
- Any supporting types (e.g., `SSOAttributes` struct if needed by the user service interface)

**Out of scope:**
- Database migrations that create the underlying tables (handled by ST-1)
- Repository methods that query/persist these models (handled by ST-4 and ST-5)
- Service methods that operate on these models (handled by ST-5 and ST-7)

#### Context

**Related design decisions:**
- DD-1: Dedicated `tenant_sso_config` table — the `TenantSSOConfig` struct maps to this table. Fields match the table schema from ST-1.
- DD-2: Automatic email-based account linking — the `KeycloakID` field on `User` enables tracking which Keycloak identity is linked to which user.

**Applicable constraints:**
- `KeycloakID` must be a pointer (`*string`) so it's nullable — existing users won't have a Keycloak ID
- The `User` struct change must be read-compatible (adding an optional field doesn't break existing consumers)
- `TenantSSOConfig` struct field types must match the database column types from the migration

**Key scenarios covered:**
- Per-tenant SSO configuration data model
- User Keycloak identity linking data model

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] `TenantSSOConfig` struct exists in `internal/tenant/model.go` with all fields (ID, TenantID, Domain, EntityID, MetadataURL, Certificate, Enabled, CreatedAt, UpdatedAt) and proper tags
- [ ] `User` struct in `internal/user/model.go` has `KeycloakID *string` field with `json` and `db` tags
- [ ] Struct field types match the database schema from ST-1

---

### ST-4: Tenant SSO Config Repository Methods

**ID:** ST-4
**Type:** implementation
**Wave:** 2
**Priority:** high
**Estimated scope:** medium

#### Purpose
Adds two repository methods to the tenant module for SSO configuration persistence: `GetSSOConfigByDomain` (used by the SSO service to resolve email domain to tenant SSO config during login) and `UpsertSSOConfig` (used for creating/updating SSO configurations when tenants enable SSO). These methods are the data access layer for the `tenant_sso_config` table.

#### Goal
The tenant repository has `GetSSOConfigByDomain` and `UpsertSSOConfig` methods that work correctly against the `tenant_sso_config` table, with unit tests covering found, not found, disabled config, insert, and update scenarios.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Tenant | `internal/tenant/repository.go` | modify | Add `GetSSOConfigByDomain(ctx context.Context, domain string) (*TenantSSOConfig, error)` method — queries by domain, returns nil if not found or if config is disabled. Add `UpsertSSOConfig(ctx context.Context, config *TenantSSOConfig) error` method — inserts or updates SSO config by tenant_id. |
| Tenant | `internal/tenant/repository_test.go` | modify | Add tests: GetSSOConfigByDomain with found config, not found, disabled config. UpsertSSOConfig with new insert and update existing. |

#### Boundaries

**In scope:**
- `GetSSOConfigByDomain` repository method implementation and tests
- `UpsertSSOConfig` repository method implementation and tests
- Updating the `TenantRepository` interface to include the new methods (if the interface is defined in `repository.go`)
- SQL queries against the `tenant_sso_config` table

**Out of scope:**
- The `TenantSSOConfig` struct definition (handled by ST-3)
- The `tenant_sso_config` table creation (handled by ST-1)
- Service-layer logic that calls these methods (handled by ST-7)

#### Context

**Related design decisions:**
- DD-1: Dedicated `tenant_sso_config` table — these methods query the dedicated table. The domain column has a UNIQUE index enabling efficient domain lookup.

**Applicable constraints:**
- Must follow existing repository pattern in `internal/tenant/repository.go`
- `GetSSOConfigByDomain` should only return enabled configs (where `enabled = true`) for the SSO flow
- `UpsertSSOConfig` should use INSERT ON CONFLICT or equivalent for idempotent upserts
- Repository interface must be additive (non-breaking to existing consumers)

**Key scenarios covered:**
- SSO initiation: look up SSO config by email domain
- SSO config management: create/update per-tenant SSO configuration

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| Database schema for tenant_sso_config table | blocking | ST-1 | Migration `00X_add_tenant_sso_config.sql` exists and can run. Repository tests need the table. |
| TenantSSOConfig struct definition | blocking | ST-3 | `TenantSSOConfig` struct exists in `internal/tenant/model.go` with all fields. |

#### Completion Criteria
- [ ] `GetSSOConfigByDomain` method exists on the tenant repository and returns the correct `TenantSSOConfig` for a given domain
- [ ] `GetSSOConfigByDomain` returns nil/not-found for unknown domains and disabled configs
- [ ] `UpsertSSOConfig` method exists and correctly inserts new configs and updates existing ones
- [ ] `TenantRepository` interface includes both new methods
- [ ] Tests cover: found, not found, disabled config, insert new, update existing
- [ ] All tests pass

---

### ST-5: User Repository and Service SSO Extensions

**ID:** ST-5
**Type:** implementation
**Wave:** 2
**Priority:** high
**Estimated scope:** medium

#### Purpose
Extends the user module with SSO-specific repository methods and service methods. The repository gains `FindByEmail` (for email-based account lookup during SSO callback) and `UpdateKeycloakID` (for linking a Keycloak identity to a user). The service gains `FindOrCreateBySSO` (JIT provisioning — find existing user by email or create new one with SAML attributes) and `LinkKeycloakID` (associate Keycloak ID with user). These are called by the SSO service during the callback flow.

#### Goal
The user repository has `FindByEmail` and `UpdateKeycloakID` methods, and the user service has `FindOrCreateBySSO` and `LinkKeycloakID` methods, all with unit tests covering the primary flow and edge cases.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| User | `internal/user/repository.go` | modify | Add `FindByEmail(ctx context.Context, email string) (*User, error)` — returns user by email or nil if not found. Add `UpdateKeycloakID(ctx context.Context, userID string, keycloakID string) error` — sets the keycloak_id column on the user row. |
| User | `internal/user/service.go` | modify | Add `FindOrCreateBySSO(ctx context.Context, email string, attrs SSOAttributes) (*User, bool, error)` — finds existing user by email or creates new user with attributes from SAML response; returns (user, created_flag, error). Add `LinkKeycloakID(ctx context.Context, userID string, keycloakID string) error` — delegates to repository. |
| User | `internal/user/service_test.go` | modify | Add tests: FindOrCreateBySSO with new user (JIT creation), existing user found and linked. LinkKeycloakID success, already linked, race condition handling via DB constraint. |

#### Boundaries

**In scope:**
- `FindByEmail` repository method
- `UpdateKeycloakID` repository method
- `FindOrCreateBySSO` service method (JIT provisioning logic)
- `LinkKeycloakID` service method
- Updating `UserRepository` and `UserService` interfaces
- Unit tests for all new methods
- `SSOAttributes` type definition (if not already in ST-3)

**Out of scope:**
- `KeycloakID` field on the `User` struct (handled by ST-3)
- Database migration for `keycloak_id` column (handled by ST-1)
- SSO callback orchestration that calls these methods (handled by ST-7)

#### Context

**Related design decisions:**
- DD-2: Automatic email-based account linking — `FindOrCreateBySSO` implements this: it looks up users by email and links their Keycloak identity automatically. No admin action needed.
- DD-3: Dual-auth — `FindOrCreateBySSO` must NOT disable password auth when linking. The user keeps both authentication paths.

**Applicable constraints:**
- Race condition on concurrent SSO + password registration: the DB UNIQUE constraint on `keycloak_id` prevents duplicate Keycloak linking. `FindOrCreateBySSO` should handle constraint violations gracefully.
- Email is assumed unique and verified in this system (per DD-2 trade-offs).
- All new interface methods are additive (non-breaking).

**Key scenarios covered:**
- JIT provisioning: new SSO user → create user with SAML attributes
- Account linking: existing password user → find by email, link Keycloak ID
- Race condition: concurrent linking → DB constraint prevents duplicates

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| Database schema for users.keycloak_id column | blocking | ST-1 | Migration `00Y_add_user_keycloak_id.sql` exists and can run. |
| KeycloakID field on User struct | blocking | ST-3 | `User` struct has `KeycloakID *string` field in `internal/user/model.go`. |

#### Completion Criteria
- [ ] `FindByEmail` repository method exists and returns correct user or nil
- [ ] `UpdateKeycloakID` repository method exists and sets keycloak_id on the user row
- [ ] `FindOrCreateBySSO` service method implements JIT provisioning: finds existing user by email or creates new user with SAML attributes
- [ ] `FindOrCreateBySSO` links Keycloak ID on existing users
- [ ] `LinkKeycloakID` service method delegates to repository correctly
- [ ] `UserRepository` and `UserService` interfaces include the new methods
- [ ] Tests cover: new user creation (JIT), existing user linking, already-linked user, constraint violation handling
- [ ] All tests pass

---

### ST-6: Keycloak Admin API Client

**ID:** ST-6
**Type:** implementation
**Wave:** 2
**Priority:** high
**Estimated scope:** medium

#### Purpose
Creates the `KeycloakClient` that wraps Keycloak's Admin REST API for programmatic IdP management. When a tenant enables SSO, the application needs to create an Identity Provider configuration in Keycloak for that tenant's SAML IdP. This client handles authentication with Keycloak's admin API, and CRUD operations on IdP configurations. The client is consumed by the SSO service.

#### Goal
A `KeycloakClient` exists in `internal/auth/keycloak_client.go` with `CreateIdP`, `GetIdP`, and `DeleteIdP` methods that communicate with Keycloak's Admin REST API, with unit tests using HTTP mocks.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Auth | `internal/auth/keycloak_client.go` | create | New file: `KeycloakClient` struct with constructor accepting `SSOConfig`. Methods: `CreateIdP(ctx, tenantID string, metadata SamlIdPMetadata) error` — creates a SAML IdP in Keycloak with alias `tenant-{id}-saml`; `GetIdP(ctx, tenantID string) (*IdPConfig, error)` — retrieves existing IdP config; `DeleteIdP(ctx, tenantID string) error` — removes IdP config. Internal: admin token management (obtain/refresh admin access token). |
| Auth | `internal/auth/keycloak_client_test.go` | create | Unit tests with HTTP mocking: CreateIdP success and error, GetIdP found and not found, DeleteIdP success. Admin token acquisition. |

#### Boundaries

**In scope:**
- `KeycloakClient` struct and constructor
- `CreateIdP`, `GetIdP`, `DeleteIdP` methods
- Admin token management (obtain access token from Keycloak)
- `KeycloakClient` interface definition
- HTTP request/response handling for Keycloak Admin REST API
- IdP alias naming convention: `tenant-{id}-saml`
- Supporting types: `SamlIdPMetadata`, `IdPConfig`
- Unit tests with HTTP mocks

**Out of scope:**
- SSO service that calls the Keycloak client (handled by ST-7)
- SAML request/response handling for the login flow (handled by ST-7)
- Configuration loading (handled by ST-2)
- Wiring in main.go (handled by ST-9)

#### Context

**Related design decisions:**
- DD-4: Single Keycloak realm with per-tenant IdP — the client operates within a single realm. IdP alias naming convention `tenant-{id}-saml` provides logical tenant isolation within the shared realm.
- DD-5: Keycloak Admin REST API — this subtask implements the decision to use the REST API directly rather than `kcadm.sh` CLI or Terraform.

**Applicable constraints:**
- Must use the `SSOConfig` from ST-2 for Keycloak URL, realm, and admin credentials
- Admin token should be obtained using Keycloak's token endpoint with admin credentials
- Keycloak v24.0+ Admin REST API endpoints for Identity Provider management
- Error handling: timeouts, Keycloak unavailability, conflict (IdP already exists)

**Key scenarios covered:**
- Tenant SSO enablement: create IdP in Keycloak
- SSO config verification: get existing IdP config
- Tenant SSO disablement: delete IdP from Keycloak

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| SSOConfig struct for Keycloak connection settings | blocking | ST-2 | `SSOConfig` struct exists in `internal/config/config.go` with KeycloakURL, Realm, AdminUser, AdminPassword fields. |
| TenantSSOConfig model for metadata types | soft | ST-3 | `TenantSSOConfig` struct available for reference. The KeycloakClient can define its own input types, but alignment with the model is beneficial. |

#### Completion Criteria
- [ ] `internal/auth/keycloak_client.go` exists with `KeycloakClient` struct and constructor
- [ ] `CreateIdP` method creates a SAML IdP in Keycloak using the Admin REST API with alias `tenant-{id}-saml`
- [ ] `GetIdP` method retrieves an existing IdP configuration
- [ ] `DeleteIdP` method removes an IdP configuration
- [ ] Admin token management (obtain/refresh) is implemented
- [ ] `KeycloakClient` interface is defined for mockability
- [ ] Unit tests with HTTP mocks cover success and error paths for all methods
- [ ] All tests pass

---

### ST-7: SSO Service Implementation

**ID:** ST-7
**Type:** implementation
**Wave:** 3
**Priority:** critical-path
**Estimated scope:** large

#### Purpose
Creates the `SSOService` that orchestrates the full SSO flow. This is the core service that ties together the tenant SSO config lookup, SAML AuthnRequest generation, SAML response processing, Keycloak integration, JIT user provisioning, and account linking. It has two primary operations: `InitiateSSO` (takes email, resolves tenant, generates redirect URL) and `ProcessCallback` (takes SAML response, extracts attributes, provisions/links user, issues token).

#### Goal
An `SSOService` exists in `internal/auth/sso_service.go` with `InitiateSSO` and `ProcessCallback` methods that correctly orchestrate the full SSO flow, with unit tests covering all primary and edge case scenarios.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Auth | `internal/auth/sso_service.go` | create | New file: `SSOService` struct with constructor accepting `SSOConfig`, `TenantRepository`, `UserService`, `KeycloakClient`. Methods: `InitiateSSO(ctx, email string) (redirectURL string, error)` — extracts domain from email, calls `TenantRepository.GetSSOConfigByDomain`, generates SAML AuthnRequest URL targeting Keycloak with IdP hint. `ProcessCallback(ctx, samlResponse string) (*User, string, error)` — parses SAML response, extracts user attributes (email, name, groups), calls `UserService.FindOrCreateBySSO`, issues JWT token via existing `AuthService.IssueToken`. |
| Auth | `internal/auth/sso_service_test.go` | create | Unit tests: InitiateSSO with valid domain (SSO config found), unknown domain, disabled SSO config. ProcessCallback with new user (JIT provisioning), existing user (account linking), invalid SAML response, missing attributes. Error handling for Keycloak unavailability. |

#### Boundaries

**In scope:**
- `SSOService` struct and constructor
- `InitiateSSO` method: domain extraction, tenant SSO config lookup, SAML AuthnRequest URL generation
- `ProcessCallback` method: SAML response parsing, attribute extraction, user resolution (JIT/linking), token issuance
- `SSOService` interface definition
- SAML request/response handling (using crewjam/saml library)
- Error handling for all failure modes
- Unit tests with mocked dependencies

**Out of scope:**
- HTTP handler layer that calls the service (handled by ST-8)
- Keycloak Admin API operations (handled by ST-6)
- User provisioning/linking logic (handled by ST-5 — service calls into user service)
- Tenant SSO config persistence (handled by ST-4 — service calls into tenant repository)
- Route registration (handled by ST-9)

#### Context

**Related design decisions:**
- DD-2: Automatic email-based account linking — `ProcessCallback` uses `UserService.FindOrCreateBySSO` which automatically links by email.
- DD-3: Dual-auth — the SSO service doesn't disable password auth. It's an additive authentication path.
- DD-4: Single Keycloak realm — `InitiateSSO` constructs the SAML AuthnRequest URL targeting the single realm with an IdP hint derived from the tenant's IdP alias.
- DD-5: Keycloak Admin REST API — the SSO service may call `KeycloakClient.CreateIdP` during tenant SSO enablement (or this may be triggered separately by an admin operation).

**Applicable constraints:**
- SAML response signature validation is critical for security
- Must handle missing SAML attributes gracefully (reject with clear error)
- Token issuance should reuse the existing `AuthService.IssueToken` mechanism
- Error messages should not leak internal details (security constraint)
- SP-initiated flow only — no IdP-initiated handling

**Key scenarios covered:**
- Primary scenario: SP-initiated SSO login end-to-end
- Edge case: unknown email domain → error (caller falls back to password)
- Edge case: Keycloak/IdP unavailable → graceful error
- Edge case: missing SAML attributes → reject with clear error
- Edge case: existing user auto-linking
- Edge case: new user JIT provisioning

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| SSO configuration settings | blocking | ST-2 | `SSOConfig` struct available in `internal/config/config.go`. |
| Tenant SSO config repository | blocking | ST-4 | `TenantRepository.GetSSOConfigByDomain` method exists and works. |
| User service SSO methods | blocking | ST-5 | `UserService.FindOrCreateBySSO` and `LinkKeycloakID` methods exist. |
| Keycloak client | blocking | ST-6 | `KeycloakClient` interface available for dependency injection. |

#### Completion Criteria
- [ ] `internal/auth/sso_service.go` exists with `SSOService` struct and constructor
- [ ] `InitiateSSO` correctly extracts domain, looks up SSO config, and generates SAML AuthnRequest redirect URL
- [ ] `InitiateSSO` returns appropriate error for unknown/disabled domains
- [ ] `ProcessCallback` correctly parses SAML response and extracts user attributes
- [ ] `ProcessCallback` calls `UserService.FindOrCreateBySSO` for JIT provisioning/linking
- [ ] `ProcessCallback` issues a JWT token via existing auth token mechanism
- [ ] `ProcessCallback` rejects invalid SAML responses and missing attributes with clear errors
- [ ] `SSOService` interface is defined for mockability
- [ ] Unit tests cover all primary and edge case scenarios
- [ ] All tests pass

---

### ST-8: SSO HTTP Handlers

**ID:** ST-8
**Type:** implementation
**Wave:** 3
**Priority:** high
**Estimated scope:** medium

#### Purpose
Adds two HTTP handler methods to the existing auth handler: `HandleSSOInitiate` (accepts email, calls SSO service, returns redirect URL) and `HandleSSOCallback` (receives SAML response, calls SSO service, redirects to application with token). These handlers translate HTTP requests/responses into SSO service calls and follow the existing handler pattern in the codebase.

#### Goal
`HandleSSOInitiate` and `HandleSSOCallback` methods exist on the auth handler struct, correctly handle HTTP request/response, delegate to the SSO service, and have appropriate error handling.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Auth | `internal/auth/handler.go` | modify | Add `HandleSSOInitiate(w http.ResponseWriter, r *http.Request)` — accepts POST with `{"email": "..."}`, calls `SSOService.InitiateSSO`, returns JSON `{"redirect_url": "..."}` or redirect. Add `HandleSSOCallback(w http.ResponseWriter, r *http.Request)` — accepts GET with `SAMLResponse` query param, calls `SSOService.ProcessCallback`, redirects to application with token. |

#### Boundaries

**In scope:**
- `HandleSSOInitiate` handler method on existing auth handler struct
- `HandleSSOCallback` handler method on existing auth handler struct
- Request parsing (JSON body for initiate, query params for callback)
- Response formatting (JSON for initiate, redirect for callback)
- HTTP error responses for invalid requests
- Adding `SSOService` dependency to the auth handler struct (or using a separate handler struct if that's the pattern)

**Out of scope:**
- SSO business logic (handled by ST-7 — handlers delegate to service)
- Route registration in main.go (handled by ST-9)
- SAML processing (handled by ST-7 via the SSO service)

#### Context

**Related design decisions:**
- DD-3: Dual-auth — the existing login handler (`POST /api/auth/login`) is NOT modified. SSO handlers are added alongside, not replacing.

**Applicable constraints:**
- Must follow the existing handler pattern in `internal/auth/handler.go`
- `HandleSSOInitiate` endpoint: `POST /auth/sso/initiate`
- `HandleSSOCallback` endpoint: `GET /auth/sso/callback`
- Error responses should not leak internal details

**Key scenarios covered:**
- SSO initiation via HTTP
- SAML callback handling via HTTP
- Invalid request handling (missing email, missing SAMLResponse)

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| SSO service implementation | blocking | ST-7 | `SSOService` interface exists with `InitiateSSO` and `ProcessCallback` methods. Handler can be implemented against the interface. |

#### Completion Criteria
- [ ] `HandleSSOInitiate` method exists on the auth handler and accepts POST requests with email in JSON body
- [ ] `HandleSSOInitiate` calls `SSOService.InitiateSSO` and returns the redirect URL
- [ ] `HandleSSOCallback` method exists on the auth handler and accepts GET requests with SAMLResponse parameter
- [ ] `HandleSSOCallback` calls `SSOService.ProcessCallback` and redirects to the application with the token
- [ ] Both handlers return appropriate HTTP error responses for invalid inputs
- [ ] Handlers follow the existing handler pattern in the codebase

---

### ST-9: Route Registration and Dependency Wiring

**ID:** ST-9
**Type:** integration
**Wave:** 4
**Priority:** high
**Estimated scope:** small

#### Purpose
Wires all SSO components together in `cmd/server/main.go`. Initializes the `KeycloakClient` with `SSOConfig`, initializes the `SSOService` with its dependencies (config, tenant repository, user service, Keycloak client), injects the SSO service into the auth handler, and registers the two SSO routes (`POST /auth/sso/initiate`, `GET /auth/sso/callback`).

#### Goal
The SSO endpoints are accessible and properly wired to the SSO service with all dependencies correctly injected.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Server | `cmd/server/main.go` | modify | Initialize `KeycloakClient` with `config.SSO`. Initialize `SSOService` with config, tenant repo, user service, Keycloak client. Inject SSO service into auth handler (or create new SSO handler if needed). Register `POST /auth/sso/initiate` → `HandleSSOInitiate`. Register `GET /auth/sso/callback` → `HandleSSOCallback`. |

#### Boundaries

**In scope:**
- `KeycloakClient` initialization in main.go
- `SSOService` initialization in main.go
- SSO service injection into the auth handler
- Route registration for both SSO endpoints
- Following existing dependency wiring pattern in main.go

**Out of scope:**
- Implementation of KeycloakClient, SSOService, or handlers (handled by ST-6, ST-7, ST-8)
- Configuration loading (handled by ST-2)
- Any new middleware or authentication for the SSO endpoints

#### Context

**Related design decisions:**
- DD-4: Single Keycloak realm — only one `KeycloakClient` instance is created, configured for the single realm.

**Applicable constraints:**
- Must follow existing wiring pattern in `cmd/server/main.go`
- Routes must use the exact paths: `/auth/sso/initiate` (POST), `/auth/sso/callback` (GET)
- All SSO dependencies should be properly injected (not hardcoded)

**Key scenarios covered:**
- Application startup with SSO endpoints available
- Correct dependency injection chain

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| SSO HTTP handlers | blocking | ST-8 | `HandleSSOInitiate` and `HandleSSOCallback` methods exist on the handler. |
| SSO service | blocking | ST-7 | `SSOService` constructor and interface are available. |
| Keycloak client | blocking | ST-6 | `KeycloakClient` constructor is available. |
| SSO config | blocking | ST-2 | `SSOConfig` is embedded in the main `Config` struct. |

#### Completion Criteria
- [ ] `KeycloakClient` is initialized in main.go with `SSOConfig`
- [ ] `SSOService` is initialized with all required dependencies
- [ ] SSO service is injected into the auth handler
- [ ] `POST /auth/sso/initiate` route is registered and maps to `HandleSSOInitiate`
- [ ] `GET /auth/sso/callback` route is registered and maps to `HandleSSOCallback`
- [ ] Application starts successfully with SSO components wired
- [ ] Routes are accessible (responds to requests, not 404)

---

### ST-10: End-to-End Integration Testing

**ID:** ST-10
**Type:** testing
**Wave:** 4
**Priority:** normal
**Estimated scope:** medium

#### Purpose
Validates that the full SSO flow works end-to-end after all components are wired together. This is convergence testing — verifying that the individual subtasks, when combined, produce a working SSO system. Covers the primary scenario, mandatory edge cases, and acceptance criteria validation.

#### Goal
Integration tests exist that verify the full SSO flow from HTTP request through all layers to response, covering the primary scenario and key edge cases.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Auth | `internal/auth/sso_integration_test.go` (or similar) | create | Integration tests that exercise the full SSO flow: initiate with valid domain → redirect URL returned. Callback with valid SAML response → user provisioned/linked, token returned. Edge cases: unknown domain, disabled SSO, invalid SAML response. Verify password auth still works unchanged. |

#### Boundaries

**In scope:**
- Integration tests for the full SSO flow (initiate → callback → user resolution → token)
- Verification of the primary scenario
- Verification of mandatory edge cases: unknown domain, existing user linking, new user JIT, Keycloak error handling
- Verification that password auth is unchanged (backward compatibility)
- Test setup: database with SSO config, mocked or test Keycloak instance

**Out of scope:**
- Unit tests for individual components (handled by ST-4, ST-5, ST-6, ST-7)
- Performance or load testing
- Testing with real external IdPs

#### Context

**Related design decisions:**
- DD-2: Automatic email-based account linking — integration tests should verify that existing users are automatically linked on first SSO login
- DD-3: Dual-auth — integration tests should verify password auth still works after SSO is enabled

**Applicable constraints:**
- Tests need either a mock Keycloak or a test Keycloak instance
- Tests should be self-contained and not require external SAML IdPs

**Key scenarios covered:**
- Full primary scenario: email → domain lookup → redirect → callback → user created/linked → token issued
- Existing password user + SSO enabled → auto email linking
- SSO user attempts password login → allowed (dual-auth)
- Unknown email domain → normal password login
- Missing SAML attributes → rejection with error

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| Route registration and wiring complete | blocking | ST-9 | All SSO components wired together in main.go. Routes accessible. |

#### Completion Criteria
- [ ] Integration test for full SSO initiate flow: valid email → correct redirect URL
- [ ] Integration test for full SSO callback flow: valid SAML response → user created/linked, token returned
- [ ] Integration test for unknown domain: returns error, no redirect
- [ ] Integration test for existing user linking: existing user found by email, Keycloak ID linked
- [ ] Integration test for password auth unchanged: login endpoint works the same as before
- [ ] All integration tests pass

---

## Critique Review

**Verdict: DECOMPOSITION_APPROVED**

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Task clarity | PASS | Each subtask has a clear Purpose, Goal, and Change Area. An implementor can understand what to do without ambiguity. Completion criteria are specific and verifiable. |
| Boundary quality | PASS | Every subtask has explicit "In scope" and "Out of scope" sections with cross-references to other subtasks by ID. File ownership is unambiguous. |
| Dependency correctness | PASS | Dependencies are typed (blocking/soft), have specific unblock conditions, and the graph is acyclic and consistent. No missing transitive dependencies detected. |
| Parallelizability | PASS | Waves 1 and 2 each have 3 genuinely independent subtasks. No file overlap within parallel groups. Wave 3 is correctly sequential (ST-7 before ST-8). Critical path is well-defined: ST-1/ST-3 -> ST-4/ST-5 -> ST-7 -> ST-8 -> ST-9 -> ST-10. |
| Conflict risk | PASS | Conflict zones are identified and all are low severity with clear resolutions. No parallel subtasks share files. |
| Context completeness | PASS | Each subtask references relevant design decisions, constraints, and scenarios. Implementors have enough context to work independently. |
| Scope discipline | PASS | All subtasks map directly to the implementation design. ST-10 (integration testing) is justified by the design's test requirements and risk mitigations. No extra features or components added. |

**Issues to Address:** None.

**Boundary Overlaps Found:** No problematic overlaps detected. ST-3 (models) and ST-4/ST-5 (repositories) touch different files in the same modules, but they are sequenced across waves with explicit dependencies.

**Missing Dependencies:** No missing dependencies detected.

**Unnecessary Dependencies:** No unnecessary dependencies detected. The ST-8 -> ST-7 blocking dependency could theoretically be a soft dependency (ST-8 could code against the SSOService interface), but since the interface definition is part of ST-7, keeping it as blocking is correct.

**Scope Additions:** No scope additions detected.

**Context Gaps:** No context gaps detected.

**Parallel Execution Risks:** No parallel execution risks detected. All subtasks within the same wave operate on different files.

**Minor Observations:**
- ST-3 bundles tenant model and user model changes into one subtask. These could be split into two smaller subtasks, but given both are small (adding a struct and adding a field), bundling is appropriate to avoid overhead.
- ST-7 is the largest subtask (scope: large). If this proves too large during execution, it could be split into InitiateSSO and ProcessCallback as separate subtasks. The current decomposition is acceptable because both methods are tightly coupled through the SSOService struct.

**Summary:** The decomposition is well-structured with clean boundaries, correct dependencies, and genuine parallelism. The 4-wave structure follows the natural dependency chain from schema to models to repositories/clients to service to handlers to wiring. Each subtask is self-contained with sufficient context for independent execution. The strongest aspect is the dependency correctness and conflict zone analysis. The decomposition is ready for coverage review and user presentation.

## Coverage Review

**Verdict: COVERAGE_OK**
**Confidence: high**

### Coverage Summary

| Source | Total Items | Covered | Partial | Missing |
|--------|------------|---------|---------|---------|
| Agreed task model (requirements) | 7 acceptance criteria + 5 edge cases + 6 constraints | 18 | 0 | 0 |
| Implementation design (changes) | 6 modules, 7 new entities, 5 modified entities | 18 | 0 | 0 |
| Change map (files) | 10 files to modify, 6 files to create | 16 | 0 | 0 |
| Design decisions | 5 decisions + 2 deferred | 7 | 0 | 0 |

### Requirement Traceability (from Agreed Task Model)

| Requirement / Criterion | Covered By | Status |
|------------------------|-----------|--------|
| SP-initiated SAML SSO login completes successfully | ST-7, ST-8, ST-9 | covered |
| Email/password login works identically before and after | ST-3 (read-compatible), ST-10 (verification) | covered |
| New SSO users are JIT-provisioned with correct tenant | ST-5, ST-7 | covered |
| Existing users auto-linked on first SSO login via email | ST-5, ST-7 | covered |
| Per-tenant SSO config stored and retrievable | ST-1, ST-3, ST-4 | covered |
| Keycloak IdP config automated via Admin API | ST-6 | covered |
| JWT tokens from SSO contain all required claims | ST-7 | covered |
| Edge: existing user + SSO auto-linking | ST-5, ST-7, ST-10 | covered |
| Edge: SSO user attempts password login (dual-auth) | ST-10 verifies, no code change needed | covered |
| Edge: unknown email domain | ST-7, ST-10 | covered |
| Edge: Keycloak/IdP unavailable | ST-7 | covered |
| Edge: missing SAML attributes | ST-7, ST-10 | covered |

### File Coverage (from Change Map)

All 16 files (10 modify + 6 create) are explicitly assigned to subtasks. No orphaned files.

### Design Decision Coverage

| Decision | Covered By | Status |
|----------|-----------|--------|
| DD-1: Dedicated tenant_sso_config table | ST-1, ST-3, ST-4 | covered |
| DD-2: Automatic email-based account linking | ST-5, ST-7 | covered |
| DD-3: Dual-auth (SSO + password fallback) | ST-7, ST-8, ST-10 | covered |
| DD-4: Single Keycloak realm with per-tenant IdP | ST-6, ST-7 | covered |
| DD-5: Keycloak Admin REST API | ST-6 | covered |
| Deferred: Multi-domain mapping | excluded (correctly) | covered |
| Deferred: SSO enforcement mode | excluded (correctly) | covered |

### Scope Fidelity
- No over-coverage detected. ST-10 (integration testing) is within scope as the design explicitly lists test requirements.
- No under-coverage detected.

### Done-State Assessment
**If all subtasks complete, is the task done?** Yes.
- **Primary scenario:** Would work end-to-end. The chain email -> domain lookup (ST-4) -> SSO initiate (ST-7, ST-8) -> Keycloak broker -> callback (ST-7, ST-8) -> JIT/link user (ST-5) -> token -> redirect is fully covered.
- **Edge cases:** All five mandatory edge cases are handled across ST-5, ST-7, and ST-10.
- **Acceptance criteria:** All seven criteria are traceable to specific subtask completion criteria.
- **Gaps between subtasks:** No inter-subtask gaps detected.

**Summary:** Coverage is complete with high confidence. Every acceptance criterion, file change, design decision, and edge case from the agreed task model and implementation design maps to at least one subtask with specific completion criteria. The traceability chain from requirement to design element to subtask to completion criterion is unbroken throughout. The strongest aspect is the file-level coverage — all 16 files are accounted for with explicit ownership. No gaps, no over-coverage.

## User Review Log
[Skipped — test run without user review steps]
