# Execution Backlog (Wave 1 Only)

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Implementation approach: Systematic — dedicated SSO subsystem following repository-service-handler pattern
> Total subtasks: 3 (Wave 1 foundation only)
> Execution waves: 1
> Decomposition status: finalized

## Execution Overview

This is Wave 1 only — the foundation layer. Three independent subtasks that establish the database schema, application configuration, and data models. All three can run fully in parallel with no inter-dependencies.

## Execution Waves

### Wave 1 — Foundation

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-1 | Database Migrations for SSO | foundation | small | ST-2, ST-3 |
| ST-2 | SSO Configuration Struct | foundation | small | ST-1, ST-3 |
| ST-3 | Tenant and User Model Extensions | foundation | small | ST-1, ST-2 |

## Dependency Graph

```
ST-1 (migrations) — no dependencies
ST-2 (config) — no dependencies
ST-3 (models) — no dependencies
All three are independent foundation tasks.
```

## Conflict Zones

No conflict zones. All three subtasks operate on different files in different packages.

---

## Subtasks

### ST-1: Database Migrations for SSO

**ID:** ST-1
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Create the database migration files that add SSO-specific schema to the existing database. This establishes the storage layer that all subsequent SSO work depends on.

#### Goal
Two migration files exist (up and down) that add the `tenant_sso_config` table and the `keycloak_id` column to the `users` table.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| migrations | `migrations/002_sso.up.sql` | create | Add tenant_sso_config table and users.keycloak_id column |
| migrations | `migrations/002_sso.down.sql` | create | Reverse the SSO schema changes |

#### Boundaries

**In scope:**
- Creating the `tenant_sso_config` table with columns: id, tenant_id (FK), idp_entity_id, idp_sso_url, idp_certificate, sp_entity_id, enabled, created_at, updated_at
- Adding `keycloak_id` column (TEXT, nullable, UNIQUE) to the `users` table
- Creating appropriate indexes
- Writing the down migration to reverse all changes

**Out of scope:**
- Any Go code changes — handled by ST-3 (models) and ST-4+ (repositories)
- Seed data or test fixtures
- Migration execution tooling

#### Context

**Related design decisions:**
- DD-3: Dedicated tenant_sso_config table (normalized) instead of JSON column on tenants table. Reasoning: allows independent querying, cleaner indexing, easier migration.
- DD-5: keycloak_id on users table as nullable TEXT with UNIQUE constraint. Reasoning: supports gradual migration, dual-auth, account linking.

**Applicable constraints:**
- Backward compatibility: existing queries must not break. All changes are additive (new table + new nullable column).
- Migration must be reversible (down migration required).

**Key scenarios covered:**
- Per-tenant SSO configuration storage
- User-to-Keycloak identity mapping

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] `migrations/002_sso.up.sql` exists with CREATE TABLE tenant_sso_config and ALTER TABLE users ADD COLUMN keycloak_id
- [ ] `migrations/002_sso.down.sql` exists and reverses all changes from the up migration
- [ ] tenant_sso_config table has all required columns: id, tenant_id, idp_entity_id, idp_sso_url, idp_certificate, sp_entity_id, enabled, created_at, updated_at
- [ ] tenant_id has a foreign key reference to tenants(id)
- [ ] users.keycloak_id is TEXT, nullable, with UNIQUE constraint
- [ ] Appropriate indexes are created (at minimum: tenant_sso_config.tenant_id)

---

### ST-2: SSO Configuration Struct

**ID:** ST-2
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Add SSO-related configuration fields to the application config. This provides the Keycloak connection settings that the SSO service and Keycloak client will use.

#### Goal
The Config struct in `config/config.go` includes SSO/Keycloak settings loaded from environment variables.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| config | `config/config.go` | modify | Add SSOConfig struct and integrate it into the main Config |

#### Boundaries

**In scope:**
- Adding an SSOConfig struct with fields: KeycloakURL, KeycloakRealm, KeycloakAdminUser, KeycloakAdminPassword, SAMLCallbackURL, SPEntityID
- Adding SSOConfig field to the main Config struct
- Loading SSOConfig values from environment variables in the Load() function

**Out of scope:**
- Keycloak client implementation — handled by ST-6
- SSO service logic — handled by ST-7
- Validation of config values — can be added later

#### Context

**Related design decisions:**
- DD-1: Single Keycloak realm for all tenants, with per-tenant IdP aliases. The config reflects this by having one set of Keycloak connection settings.

**Applicable constraints:**
- Follow existing config pattern: getEnv() helper with fallback values.
- Environment variable names should follow the existing convention (UPPER_SNAKE_CASE).

**Key scenarios covered:**
- Application configuration for SSO subsystem

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] SSOConfig struct exists in `config/config.go` with fields: KeycloakURL, KeycloakRealm, KeycloakAdminUser, KeycloakAdminPassword, SAMLCallbackURL, SPEntityID
- [ ] Main Config struct includes an SSO field of type SSOConfig
- [ ] Load() function populates SSOConfig from environment variables using the existing getEnv() pattern
- [ ] Environment variable names follow UPPER_SNAKE_CASE convention (e.g., KEYCLOAK_URL, KEYCLOAK_REALM)

---

### ST-3: Tenant and User Model Extensions

**ID:** ST-3
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
Extend the existing data models with SSO-related fields and create the TenantSSOConfig model struct. This establishes the Go types that repository and service layers will use.

#### Goal
A TenantSSOConfig struct exists in the tenant package, and the User struct has a KeycloakID field.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| tenant | `internal/tenant/model.go` | modify | Add TenantSSOConfig struct |
| user | `internal/user/model.go` | modify | Add KeycloakID field to User struct |

#### Boundaries

**In scope:**
- Creating TenantSSOConfig struct with fields matching the database table: ID, TenantID, IdPEntityID, IdPSSOURL, IdPCertificate, SPEntityID, Enabled, CreatedAt, UpdatedAt
- Adding KeycloakID field (*string, nullable) to the User struct with appropriate JSON tag
- Adding JSON tags to TenantSSOConfig fields

**Out of scope:**
- Repository methods — handled by ST-4 (tenant) and ST-5 (user)
- Any business logic
- Database queries

#### Context

**Related design decisions:**
- DD-3: Dedicated tenant_sso_config table → dedicated TenantSSOConfig struct in the tenant package.
- DD-5: keycloak_id as nullable → *string in Go for proper nil handling.

**Applicable constraints:**
- Follow existing model patterns: same JSON tag style, same time.Time usage for timestamps.
- User model change must be backward compatible: KeycloakID is a pointer (nil = not set).

**Key scenarios covered:**
- SSO configuration data model
- User-to-Keycloak identity mapping model

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] TenantSSOConfig struct exists in `internal/tenant/model.go` with all required fields: ID, TenantID, IdPEntityID, IdPSSOURL, IdPCertificate, SPEntityID, Enabled, CreatedAt, UpdatedAt
- [ ] TenantSSOConfig fields have appropriate JSON tags
- [ ] User struct in `internal/user/model.go` has a KeycloakID field of type *string
- [ ] KeycloakID has a JSON tag: `json:"keycloak_id,omitempty"`
- [ ] Existing User fields are unchanged (backward compatible)
