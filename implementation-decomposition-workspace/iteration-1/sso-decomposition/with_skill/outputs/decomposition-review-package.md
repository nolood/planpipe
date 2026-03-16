# Decomposition Review Package

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Total subtasks: 10
> Execution waves: 4
> Estimated parallel efficiency: 3 subtasks can run simultaneously at peak (Waves 1 and 2)

## Decomposition Summary

The SAML SSO implementation is decomposed into 10 subtasks organized across 4 execution waves, following the natural dependency chain of the codebase: schema -> models -> repositories/clients -> services -> handlers -> wiring -> testing. The decomposition maximizes parallelism where possible -- Waves 1 and 2 each contain 3 independent subtasks that can run simultaneously, while Waves 3 and 4 are sequential due to genuine dependency constraints (the SSO service orchestrates multiple Wave 2 components, and the handlers depend on the service). The decomposition directly mirrors the implementation design's 6-module structure, with each module's changes assigned to specific subtasks with clear file ownership and no overlapping boundaries.

## Subtask Overview

| # | Subtask | Type | Wave | Scope | Key Dependencies |
|---|---------|------|------|-------|-----------------|
| ST-1 | Database Migrations for SSO | foundation | 1 | small | none |
| ST-2 | SSO Configuration Struct | foundation | 1 | small | none |
| ST-3 | Tenant and User Model Extensions | foundation | 1 | small | none |
| ST-4 | Tenant SSO Config Repository Methods | implementation | 2 | medium | ST-1, ST-3 |
| ST-5 | User Repository and Service SSO Extensions | implementation | 2 | medium | ST-1, ST-3 |
| ST-6 | Keycloak Admin API Client | implementation | 2 | medium | ST-2, ST-3 (soft) |
| ST-7 | SSO Service Implementation | implementation | 3 | large | ST-2, ST-4, ST-5, ST-6 |
| ST-8 | SSO HTTP Handlers | implementation | 3 | medium | ST-7 |
| ST-9 | Route Registration and Dependency Wiring | integration | 4 | small | ST-2, ST-6, ST-7, ST-8 |
| ST-10 | End-to-End Integration Testing | testing | 4 | medium | ST-9 |

## Execution Waves

### Wave 1: Foundation
**Subtasks:** ST-1, ST-2, ST-3
**Parallel:** all 3 subtasks can run simultaneously
**Goal:** Establish the database schema (migrations), application configuration (SSOConfig struct), and data models (TenantSSOConfig struct, User.KeycloakID field) that all subsequent work depends on.

### Wave 2: Core Implementation
**Subtasks:** ST-4, ST-5, ST-6
**Parallel:** all 3 subtasks can run simultaneously
**Goal:** Build the data access layer (tenant SSO config repository, user SSO repository/service) and the Keycloak integration client. Each operates on different files in different modules with no overlap.

### Wave 3: SSO Orchestration
**Subtasks:** ST-7, then ST-8
**Parallel:** Sequential within this wave -- ST-8 (handlers) depends on ST-7 (service) because the handlers delegate to the SSOService interface defined in ST-7.
**Goal:** Build the SSO service that orchestrates the full SAML flow (initiate + callback), then build the HTTP handlers that expose it.

### Wave 4: Integration & Convergence
**Subtasks:** ST-9, then ST-10
**Goal:** Wire all components together in main.go and verify the full SSO flow works end-to-end with integration tests.

## Dependency Highlights

- **ST-7 (SSO Service) is the convergence point:** It depends on 4 prior subtasks (ST-2, ST-4, ST-5, ST-6) and cannot start until all of them are complete. This is the primary bottleneck in the execution plan. Waves 1 and 2 should be completed as quickly as possible to unblock ST-7.
- **ST-1 (Migrations) unblocks repository work:** Both ST-4 and ST-5 need the database schema to exist for their tests. Prioritize ST-1 if only one Wave 1 subtask can start first.
- **ST-3 (Models) unblocks all of Wave 2:** The TenantSSOConfig struct and User.KeycloakID field are used by all three Wave 2 subtasks. If ST-3 is delayed, Wave 2 is fully blocked.

## Conflict Zones

No significant conflict zones. All subtasks within parallel waves operate on different files. The only shared-directory situation (ST-6, ST-7, ST-8 all in `internal/auth/`) is sequenced across waves -- they create/modify different files (`keycloak_client.go`, `sso_service.go`, `handler.go` respectively).

## Coverage Assessment
- **Coverage:** COVERAGE_OK
- **Confidence:** high
- **Key finding:** All 7 acceptance criteria, all 16 files from the change map, all 5 design decisions, and all 5 mandatory edge cases trace to specific subtask completion criteria. The done-state validation confirms that completing all 10 subtasks fully implements the agreed SSO feature with no gaps.

## Review Points

### Point 1: ST-7 Scope (Large)
**Context:** ST-7 (SSO Service Implementation) is the largest subtask, estimated as "large" scope. It creates the core service with two complex methods (InitiateSSO and ProcessCallback) plus comprehensive unit tests.
**Current approach:** Kept as a single subtask because both methods are tightly coupled through the SSOService struct and share dependencies.
**Question:** Should ST-7 be split into two subtasks (InitiateSSO and ProcessCallback), or is the current single-subtask approach acceptable?

### Point 2: ST-3 Bundling (Two Modules)
**Context:** ST-3 bundles model changes from two modules (tenant model.go and user model.go) into one subtask.
**Current approach:** Bundled because both changes are small (adding a struct and adding a field) and have no dependencies on each other.
**Question:** Should ST-3 be split into separate tenant and user model subtasks, or is the bundling acceptable given the small scope?

### Point 3: Integration Testing Approach (ST-10)
**Context:** ST-10 requires either a mock Keycloak or a test Keycloak instance for integration testing. The approach for mocking the external SAML IdP and Keycloak interactions needs to be decided.
**Current approach:** Left flexible -- test setup can use either approach.
**Question:** Should integration tests use a mock Keycloak server (simpler, faster) or a real Keycloak test instance (more realistic, slower)?

## Scope Confirmation

**All agreed requirements covered:**
- SP-initiated SAML SSO login -> ST-7, ST-8, ST-9
- Email/password backward compatibility -> ST-3 (read-compatible), ST-10 (verification)
- JIT user provisioning -> ST-5, ST-7
- Automatic account linking by email -> ST-5, ST-7
- Per-tenant SSO config storage -> ST-1, ST-3, ST-4
- Keycloak IdP automation -> ST-6
- JWT token issuance from SSO -> ST-7

**Question:** Does this decomposition cover everything you need? Any subtasks that should be split, merged, reordered, or removed?
