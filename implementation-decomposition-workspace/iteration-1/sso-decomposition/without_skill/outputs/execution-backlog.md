# SSO SAML Implementation — Execution Backlog

> Feature: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Organized into waves for maximum parallel execution
> Total subtasks: 13

---

## Wave 1 — Foundation (No Dependencies)

These subtasks have zero dependencies on each other and can all be executed in parallel. They establish the schema, configuration, and data model groundwork that everything else builds on.

---

### Task 1.1: Database Migration — `tenant_sso_config` Table

**What:** Create the SQL migration that adds the `tenant_sso_config` table for per-tenant SSO configuration storage.

**Files involved:**
- `migrations/00X_add_tenant_sso_config.sql` (create)

**Work:**
- CREATE TABLE `tenant_sso_config` with columns:
  - `id` UUID PRIMARY KEY
  - `tenant_id` UUID NOT NULL, FOREIGN KEY to `tenants(id)`
  - `domain` VARCHAR(255) NOT NULL
  - `entity_id` VARCHAR(512) NOT NULL
  - `metadata_url` TEXT NOT NULL
  - `certificate` TEXT NOT NULL
  - `enabled` BOOLEAN NOT NULL DEFAULT true
  - `created_at` TIMESTAMP NOT NULL DEFAULT now()
  - `updated_at` TIMESTAMP NOT NULL DEFAULT now()
- Add UNIQUE INDEX on `domain`
- Add INDEX on `tenant_id`
- Include DOWN migration (DROP TABLE)

**Dependencies:** None

**Done when:**
- Migration runs cleanly (up and down)
- Unique constraint on `domain` is enforced (insert two rows with same domain fails)
- Foreign key to `tenants(id)` is enforced

---

### Task 1.2: Database Migration — `users.keycloak_id` Column

**What:** Create the SQL migration that adds a nullable `keycloak_id` column to the `users` table for Keycloak identity linking.

**Files involved:**
- `migrations/00Y_add_user_keycloak_id.sql` (create)

**Work:**
- ALTER TABLE `users` ADD COLUMN `keycloak_id` VARCHAR(255) NULL
- Add UNIQUE INDEX on `keycloak_id` (partial index excluding NULLs if supported)
- Include DOWN migration (DROP COLUMN)

**Dependencies:** None

**Done when:**
- Migration runs cleanly (up and down)
- Existing user rows are unaffected (column is nullable)
- Unique constraint on `keycloak_id` is enforced
- NULL values do not violate uniqueness

---

### Task 1.3: SSO Configuration Struct

**What:** Add the `SSOConfig` struct to the application configuration module and wire environment variable loading.

**Files involved:**
- `internal/config/config.go` (modify)

**Work:**
- Define `SSOConfig` struct with fields:
  - `KeycloakURL` string (env: `SSO_KEYCLOAK_URL`)
  - `Realm` string (env: `SSO_KEYCLOAK_REALM`, default: `master`)
  - `AdminUser` string (env: `SSO_KEYCLOAK_ADMIN_USER`)
  - `AdminPassword` string (env: `SSO_KEYCLOAK_ADMIN_PASSWORD`)
- Embed `SSOConfig` in the main `Config` struct
- Wire environment variable loading (follow existing config loading pattern)

**Dependencies:** None

**Done when:**
- `SSOConfig` struct compiles and is accessible via the main `Config`
- Environment variables are loaded correctly
- Missing required env vars produce clear errors
- Default value for `Realm` is `master`

---

### Task 1.4: Tenant SSO Config Model

**What:** Add the `TenantSSOConfig` struct to the tenant module's model file.

**Files involved:**
- `internal/tenant/model.go` (modify)

**Work:**
- Define `TenantSSOConfig` struct with fields matching the database schema:
  - `ID` (UUID)
  - `TenantID` (UUID)
  - `Domain` (string)
  - `EntityID` (string)
  - `MetadataURL` (string)
  - `Certificate` (string)
  - `Enabled` (bool)
  - `CreatedAt` (time.Time)
  - `UpdatedAt` (time.Time)
- Add appropriate `json` and `db` struct tags

**Dependencies:** None

**Done when:**
- Struct compiles
- Tags are correct for JSON serialization and DB scanning
- Struct is exported and usable by other modules

---

### Task 1.5: User Model — Add `KeycloakID` Field

**What:** Add the optional `KeycloakID` field to the existing `User` struct.

**Files involved:**
- `internal/user/model.go` (modify)

**Work:**
- Add `KeycloakID *string` field to `User` struct
- Add `json:"keycloak_id,omitempty"` and `db:"keycloak_id"` tags
- Verify existing JSON serialization is unaffected (pointer type + omitempty means no breaking change)

**Dependencies:** None

**Done when:**
- `User` struct compiles with the new field
- Existing code that uses `User` still compiles (non-breaking)
- JSON output for users without KeycloakID does not include the field
- JSON output for users with KeycloakID includes it correctly

---

## Wave 2 — Repository & Client Layer (Depends on Wave 1)

These subtasks build the data access and external service integration layers. They depend on Wave 1 models/schema being in place but are independent of each other.

---

### Task 2.1: Tenant Repository — SSO Config Methods

**What:** Add `GetSSOConfigByDomain` and `UpsertSSOConfig` methods to the tenant repository.

**Files involved:**
- `internal/tenant/repository.go` (modify)
- `internal/tenant/repository_test.go` (modify)

**Work:**
- Implement `GetSSOConfigByDomain(ctx context.Context, domain string) (*TenantSSOConfig, error)`:
  - Query `tenant_sso_config` WHERE `domain = $1` AND `enabled = true`
  - Return `nil, nil` or a sentinel error if not found (follow existing repo pattern)
- Implement `UpsertSSOConfig(ctx context.Context, config *TenantSSOConfig) error`:
  - INSERT ON CONFLICT (domain) DO UPDATE
  - Set `updated_at` on upsert
- Add to the `TenantRepository` interface if one exists
- Write tests:
  - `GetSSOConfigByDomain`: found, not found, disabled config returns not-found
  - `UpsertSSOConfig`: insert new config, update existing config

**Dependencies:** Task 1.1 (migration), Task 1.4 (model)

**Done when:**
- Both methods compile and pass tests
- Domain lookup returns correct config
- Disabled configs are not returned by domain lookup
- Upsert correctly inserts new and updates existing rows

---

### Task 2.2: User Repository — SSO Methods

**What:** Add `FindByEmail` and `UpdateKeycloakID` methods to the user repository.

**Files involved:**
- `internal/user/repository.go` (modify)

**Work:**
- Implement `FindByEmail(ctx context.Context, email string) (*User, error)`:
  - Query `users` WHERE `email = $1`
  - Return `nil, nil` or sentinel error if not found (follow existing pattern)
- Implement `UpdateKeycloakID(ctx context.Context, userID string, keycloakID string) error`:
  - UPDATE `users` SET `keycloak_id = $1` WHERE `id = $2`
  - Handle unique constraint violation on `keycloak_id` gracefully
- Add to `UserRepository` interface if one exists

**Dependencies:** Task 1.2 (migration), Task 1.5 (model)

**Done when:**
- Both methods compile
- `FindByEmail` returns correct user or nil
- `UpdateKeycloakID` sets the value and handles duplicate keycloak_id errors
- Methods are added to the repository interface

---

### Task 2.3: Keycloak Admin API Client

**What:** Create the `KeycloakClient` that wraps Keycloak's Admin REST API for programmatic IdP management.

**Files involved:**
- `internal/auth/keycloak_client.go` (create)
- `internal/auth/keycloak_client_test.go` (create)

**Work:**
- Define `KeycloakClient` struct with fields:
  - `httpClient *http.Client`
  - `baseURL` string
  - `realm` string
  - `adminUser` string
  - `adminPassword` string
- Define `KeycloakClient` interface (for mocking in tests)
- Implement constructor: `NewKeycloakClient(cfg config.SSOConfig) *KeycloakClient`
- Implement `getAdminToken(ctx) (string, error)`:
  - POST to `/realms/master/protocol/openid-connect/token` with client credentials
  - Cache token until expiry
- Implement `CreateIdP(ctx, tenantID string, metadata SSOMetadata) error`:
  - PUT/POST to `/admin/realms/{realm}/identity-provider/instances`
  - IdP alias: `tenant-{tenantID}-saml`
  - Configure SAML settings from metadata (entity ID, SSO URL, certificate)
- Implement `GetIdP(ctx, tenantID string) (*IdPConfig, error)`:
  - GET from `/admin/realms/{realm}/identity-provider/instances/tenant-{tenantID}-saml`
- Implement `DeleteIdP(ctx, tenantID string) error`:
  - DELETE to `/admin/realms/{realm}/identity-provider/instances/tenant-{tenantID}-saml`
- Write unit tests with HTTP mocking (httptest.Server):
  - CreateIdP success and failure
  - GetIdP found and not found
  - Admin token acquisition and caching
  - Error handling for Keycloak unavailability

**Dependencies:** Task 1.3 (config struct)

**Done when:**
- Client compiles and all tests pass
- IdP CRUD operations make correct HTTP requests to Keycloak endpoints
- Admin token is acquired and cached
- Errors from Keycloak are wrapped with meaningful context
- IdP alias follows `tenant-{id}-saml` convention

---

## Wave 3 — Service Layer (Depends on Wave 2)

These subtasks build the business logic layer. They depend on repository and client layers from Wave 2.

---

### Task 3.1: User Service — SSO Methods

**What:** Add `FindOrCreateBySSO` and `LinkKeycloakID` methods to the user service for JIT provisioning and account linking.

**Files involved:**
- `internal/user/service.go` (modify)
- `internal/user/service_test.go` (modify)

**Work:**
- Define `SSOAttributes` struct (or use a map): email, firstName, lastName, keycloakID
- Implement `FindOrCreateBySSO(ctx, email string, attrs SSOAttributes) (*User, bool, error)`:
  - Call `repo.FindByEmail(ctx, email)`
  - If user found:
    - If `KeycloakID` is nil, call `LinkKeycloakID` to link
    - Return `(user, false, nil)` — false = not created
  - If user not found:
    - Create new user with attributes from SAML (JIT provisioning)
    - Set `KeycloakID` on creation
    - Return `(user, true, nil)` — true = created
  - Use transaction for atomicity (SELECT FOR UPDATE on email to prevent race)
- Implement `LinkKeycloakID(ctx, userID string, keycloakID string) error`:
  - Call `repo.UpdateKeycloakID(ctx, userID, keycloakID)`
  - Handle already-linked case (idempotent if same ID, error if different)
- Write tests:
  - New user: JIT provisioning creates user with correct attributes
  - Existing user without KeycloakID: links successfully
  - Existing user already linked with same KeycloakID: idempotent
  - Existing user linked with different KeycloakID: returns error
  - Race condition: concurrent calls don't create duplicates

**Dependencies:** Task 2.2 (user repository methods)

**Done when:**
- Both methods compile and all tests pass
- JIT provisioning creates users with correct tenant association
- Account linking works for unlinked existing users
- Race condition is handled via transaction isolation
- Already-linked users are handled idempotently

---

### Task 3.2: SSO Service — Core Flow Logic

**What:** Create the `SSOService` that orchestrates the full SSO initiation and callback flows.

**Files involved:**
- `internal/auth/sso_service.go` (create)
- `internal/auth/sso_service_test.go` (create)

**Work:**
- Define `SSOService` struct with dependencies:
  - `tenantRepo TenantRepository` (for domain lookup)
  - `userService UserService` (for JIT provisioning/linking)
  - `keycloakClient KeycloakClient` (for IdP management)
  - `config SSOConfig`
- Define `SSOService` interface (for mocking)
- Implement constructor: `NewSSOService(tenantRepo, userService, keycloakClient, config)`
- Implement `InitiateSSO(ctx, email string) (redirectURL string, error)`:
  - Extract domain from email
  - Call `tenantRepo.GetSSOConfigByDomain(ctx, domain)`
  - If no config found, return domain-not-configured error
  - Build Keycloak SAML auth URL with IdP hint (`kc_idp_hint=tenant-{tenantID}-saml`)
  - Return redirect URL
- Implement `ProcessCallback(ctx, samlResponse string) (*User, string, error)`:
  - Decode and validate SAML response (use `crewjam/saml` library)
  - Extract attributes: email, firstName, lastName, keycloakID
  - Validate required attributes are present (reject if email missing)
  - Call `userService.FindOrCreateBySSO(ctx, email, attrs)`
  - Issue JWT token (reuse existing `AuthService.IssueToken` or equivalent)
  - Return user and token
- Add structured logging for SSO flow debugging
- Write tests (mock all dependencies):
  - `InitiateSSO`: valid domain returns redirect URL
  - `InitiateSSO`: unknown domain returns clear error
  - `ProcessCallback`: new user, JIT provisioned
  - `ProcessCallback`: existing user, account linked
  - `ProcessCallback`: missing SAML attributes, rejected
  - `ProcessCallback`: invalid SAML response, rejected
  - Error propagation from dependencies

**Dependencies:** Task 2.1 (tenant repo), Task 2.3 (keycloak client), Task 3.1 (user service SSO methods), Task 1.3 (config)

**Done when:**
- Service compiles and all tests pass
- SSO initiation correctly resolves domain to redirect URL
- Callback correctly processes SAML response into user + token
- Missing/invalid SAML attributes produce clear errors
- Structured logging is present for debugging
- All dependency interactions are tested via mocks

---

## Wave 4 — HTTP & Wiring (Depends on Wave 3)

These subtasks connect the service layer to the HTTP interface and wire everything together. They can be done in parallel.

---

### Task 4.1: SSO HTTP Handlers

**What:** Add `HandleSSOInitiate` and `HandleSSOCallback` handler methods to the existing auth handler.

**Files involved:**
- `internal/auth/handler.go` (modify)

**Work:**
- Add `ssoService SSOService` field to the existing auth handler struct (or extend constructor)
- Implement `HandleSSOInitiate(w http.ResponseWriter, r *http.Request)`:
  - Parse JSON body: `{"email": "user@company.com"}`
  - Validate email format
  - Call `ssoService.InitiateSSO(ctx, email)`
  - On success: return JSON `{"redirect_url": "..."}` with 200 status
  - On domain-not-configured: return 404 with clear message
  - On error: return 500 with generic error
- Implement `HandleSSOCallback(w http.ResponseWriter, r *http.Request)`:
  - Extract `SAMLResponse` from query params or form data
  - Call `ssoService.ProcessCallback(ctx, samlResponse)`
  - On success: redirect to application frontend with token (e.g., `?token=...`)
  - On error: redirect to login page with error parameter
- Follow existing handler patterns (error handling, response format, middleware)

**Dependencies:** Task 3.2 (SSO service)

**Done when:**
- Handlers compile
- `POST /auth/sso/initiate` accepts email, returns redirect URL or error
- `GET /auth/sso/callback` processes SAML response, redirects with token
- Error responses follow existing API conventions
- Input validation (email format) is present

---

### Task 4.2: Route Registration & Dependency Wiring

**What:** Wire all SSO dependencies together in `main.go` and register the SSO routes.

**Files involved:**
- `cmd/server/main.go` (modify)

**Work:**
- Read SSO config from the application config
- Initialize `KeycloakClient` with SSO config
- Initialize `SSOService` with tenant repo, user service, keycloak client, config
- Inject `SSOService` into the auth handler (extend existing handler initialization)
- Register routes:
  - `POST /auth/sso/initiate` -> `handler.HandleSSOInitiate`
  - `GET /auth/sso/callback` -> `handler.HandleSSOCallback`
- Ensure SSO initialization is conditional (skip if SSO config not provided, so non-SSO deployments are unaffected)

**Dependencies:** Task 4.1 (handlers), Task 3.2 (SSO service), Task 2.3 (keycloak client)

**Done when:**
- Application compiles and starts with SSO config provided
- Application compiles and starts without SSO config (SSO disabled gracefully)
- Routes are accessible at the correct paths
- All dependencies are correctly wired (no nil pointer panics)
- Existing routes and functionality are unaffected

---

## Wave 5 — Integration Verification (Depends on All Previous Waves)

This wave validates the end-to-end flow after all components are assembled.

---

### Task 5.1: End-to-End Integration Test

**What:** Write an integration test that validates the full SSO flow from HTTP request to token issuance.

**Files involved:**
- `internal/auth/sso_service_test.go` (modify — add integration test section)
- Or a new integration test file if the project has a separate integration test pattern

**Work:**
- Set up test fixtures:
  - Test database with migrations applied
  - Mock Keycloak (httptest.Server) or real Keycloak container (depending on CI setup)
  - Tenant with SSO config in database
  - Optionally: existing user with matching email (for account linking test)
- Test SSO Initiation:
  - POST `/auth/sso/initiate` with `{"email": "user@sso-tenant.com"}`
  - Assert: 200 response with `redirect_url` pointing to Keycloak
- Test SSO Callback (with mock SAML response):
  - GET `/auth/sso/callback?SAMLResponse=...`
  - Assert: redirect with valid JWT token
  - Assert: user exists in database with correct attributes and KeycloakID
- Test password login unaffected:
  - POST `/api/auth/login` with password credentials
  - Assert: works exactly as before
- Test non-SSO domain:
  - POST `/auth/sso/initiate` with non-SSO email
  - Assert: 404 domain not configured

**Dependencies:** All tasks from Waves 1-4

**Done when:**
- SSO initiation returns correct Keycloak redirect URL
- SSO callback creates/links user and returns valid JWT
- JIT-provisioned users have correct tenant association
- Existing users are linked by email on first SSO login
- Password login remains functional
- Non-SSO tenants are unaffected

---

## Dependency Graph

```
Wave 1 (parallel):
  1.1 Migration: tenant_sso_config
  1.2 Migration: users.keycloak_id
  1.3 Config: SSOConfig struct
  1.4 Model: TenantSSOConfig
  1.5 Model: User.KeycloakID

Wave 2 (parallel, after Wave 1):
  2.1 Tenant Repo methods      ← 1.1, 1.4
  2.2 User Repo methods        ← 1.2, 1.5
  2.3 Keycloak Client          ← 1.3

Wave 3 (partially parallel, after Wave 2):
  3.1 User Service SSO methods ← 2.2
  3.2 SSO Service              ← 2.1, 2.3, 3.1, 1.3

Wave 4 (parallel, after Wave 3):
  4.1 HTTP Handlers            ← 3.2
  4.2 Route Wiring             ← 4.1, 3.2, 2.3

Wave 5 (after all):
  5.1 Integration Test         ← all above
```

## Summary

| Wave | Tasks | Parallelism | Focus |
|------|-------|-------------|-------|
| 1 | 1.1, 1.2, 1.3, 1.4, 1.5 | 5 parallel | Schema, config, models |
| 2 | 2.1, 2.2, 2.3 | 3 parallel | Repositories, external client |
| 3 | 3.1, 3.2 | 3.1 first, then 3.2 | Business logic |
| 4 | 4.1, 4.2 | 4.1 first, then 4.2 | HTTP layer, wiring |
| 5 | 5.1 | 1 task | End-to-end validation |

**Critical path:** 1.1 -> 2.1 -> 3.2 -> 4.1 -> 4.2 -> 5.1

**Maximum concurrent developers:** 5 (during Wave 1), 3 (during Wave 2)

## Acceptance Criteria (Feature-Level)

Mapped from the agreed task model and design documents:

- [ ] SP-initiated SAML SSO login completes end-to-end through Keycloak and corporate IdP
- [ ] JIT provisioning creates new users with correct attributes and tenant association
- [ ] Existing password users are auto-linked by email on first SSO login
- [ ] Per-tenant SSO config is stored in dedicated `tenant_sso_config` table
- [ ] Keycloak IdP is created programmatically via Admin REST API
- [ ] Password authentication works identically before and after (dual-auth)
- [ ] Non-SSO tenants experience no behavioral change
- [ ] JWT tokens from SSO contain required claims (`sub`, `email`, `tenant_id`)
