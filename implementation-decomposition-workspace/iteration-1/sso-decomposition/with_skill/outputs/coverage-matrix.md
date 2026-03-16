# Coverage Matrix

> Task: Enable SAML 2.0 SSO for multi-tenant Go backend via Keycloak
> Coverage verdict: COVERAGE_OK
> Confidence: high

## Requirement Traceability

### From Agreed Task Model

| Requirement / Scenario | Source | Covered By | Status |
|------------------------|--------|-----------|--------|
| SP-initiated SAML SSO login completes successfully through corporate IdP | agreed-task-model.md (AC-1) | ST-7, ST-8, ST-9 | covered |
| Email/password login works identically before and after | agreed-task-model.md (AC-2) | ST-3 (read-compatible model change), ST-10 (verification) | covered |
| New SSO users are JIT-provisioned with correct tenant | agreed-task-model.md (AC-3) | ST-5 (FindOrCreateBySSO), ST-7 (orchestration) | covered |
| Existing users auto-linked on first SSO login via email | agreed-task-model.md (AC-4) | ST-5 (LinkKeycloakID), ST-7 (ProcessCallback) | covered |
| Per-tenant SSO config stored and retrievable | agreed-task-model.md (AC-5) | ST-1 (table), ST-3 (model), ST-4 (repository) | covered |
| Keycloak IdP config automated via Admin API | agreed-task-model.md (AC-6) | ST-6 (KeycloakClient) | covered |
| JWT tokens from SSO contain all required claims | agreed-task-model.md (AC-7) | ST-7 (ProcessCallback issues token) | covered |
| Primary scenario: user enters email -> SSO redirect -> SAML auth -> callback -> token | agreed-task-model.md (scenario) | ST-4, ST-7, ST-8, ST-5, ST-9 | covered |
| Edge: existing password user + SSO enabled -> auto email linking | agreed-task-model.md (edge case) | ST-5 (FindOrCreateBySSO), ST-7, ST-10 | covered |
| Edge: SSO user attempts password login -> allowed (dual-auth) | agreed-task-model.md (edge case) | ST-10 (verification, no code change needed) | covered |
| Edge: unknown email domain -> normal password login | agreed-task-model.md (edge case) | ST-7 (InitiateSSO returns error), ST-10 | covered |
| Edge: Keycloak/IdP unavailable -> graceful error, password fallback | agreed-task-model.md (edge case) | ST-7 (error handling) | covered |
| Edge: missing SAML attributes -> reject with clear error | agreed-task-model.md (edge case) | ST-7 (ProcessCallback validation), ST-10 | covered |
| Constraint: backward compatibility (POST /api/auth/login unchanged) | agreed-task-model.md | All subtasks (additive changes only) | covered |
| Constraint: single Keycloak realm | agreed-task-model.md | ST-2 (single realm config), ST-6 (single realm client) | covered |
| Constraint: repository-service-handler pattern | agreed-task-model.md | All subtasks follow this pattern | covered |
| Constraint: dedicated table (not JSONB) | agreed-task-model.md | ST-1 (table), ST-3 (model), ST-4 (repository) | covered |
| Constraint: Q3 timeline / MVO-scoped | agreed-task-model.md | All subtasks are MVO-scoped, no extras | covered |

### From Implementation Design

| Design Element | Source | Covered By | Status |
|----------------|--------|-----------|--------|
| Auth module: SSO service, Keycloak client, handlers | implementation-design.md | ST-6, ST-7, ST-8 | covered |
| Tenant module: SSO config model, repository methods | implementation-design.md | ST-3, ST-4 | covered |
| User module: KeycloakID field, SSO service/repo methods | implementation-design.md | ST-3, ST-5 | covered |
| Config module: SSOConfig struct | implementation-design.md | ST-2 | covered |
| Migrations module: tenant_sso_config table, keycloak_id column | implementation-design.md | ST-1 | covered |
| Server module: SSO route registration | implementation-design.md | ST-9 | covered |
| New entity: SSOService | implementation-design.md | ST-7 | covered |
| New entity: KeycloakClient | implementation-design.md | ST-6 | covered |
| New entity: TenantSSOConfig | implementation-design.md | ST-3 | covered |
| New entity: SSOConfig | implementation-design.md | ST-2 | covered |
| New entity: HandleSSOInitiate | implementation-design.md | ST-8 | covered |
| New entity: HandleSSOCallback | implementation-design.md | ST-8 | covered |
| New entity: tenant_sso_config table | implementation-design.md | ST-1 | covered |
| Modified entity: User (KeycloakID field) | implementation-design.md | ST-3 | covered |
| Modified entity: UserRepository (new methods) | implementation-design.md | ST-5 | covered |
| Modified entity: UserService (new methods) | implementation-design.md | ST-5 | covered |
| Modified entity: TenantRepository (new methods) | implementation-design.md | ST-4 | covered |
| Modified entity: Router (SSO routes) | implementation-design.md | ST-9 | covered |

### From Change Map

| File / Change | Source | Covered By | Status |
|---------------|--------|-----------|--------|
| `internal/auth/handler.go` — add SSO handler methods | change-map.md | ST-8 | covered |
| `internal/tenant/model.go` — add TenantSSOConfig struct | change-map.md | ST-3 | covered |
| `internal/tenant/repository.go` — add SSO config methods | change-map.md | ST-4 | covered |
| `internal/tenant/repository_test.go` — add SSO config tests | change-map.md | ST-4 | covered |
| `internal/user/model.go` — add KeycloakID field | change-map.md | ST-3 | covered |
| `internal/user/repository.go` — add FindByEmail, UpdateKeycloakID | change-map.md | ST-5 | covered |
| `internal/user/service.go` — add FindOrCreateBySSO, LinkKeycloakID | change-map.md | ST-5 | covered |
| `internal/user/service_test.go` — add SSO service tests | change-map.md | ST-5 | covered |
| `internal/config/config.go` — add SSOConfig struct | change-map.md | ST-2 | covered |
| `cmd/server/main.go` — initialize SSO, register routes | change-map.md | ST-9 | covered |
| `internal/auth/sso_service.go` — create SSO service | change-map.md | ST-7 | covered |
| `internal/auth/keycloak_client.go` — create Keycloak client | change-map.md | ST-6 | covered |
| `internal/auth/sso_service_test.go` — create SSO service tests | change-map.md | ST-7 | covered |
| `internal/auth/keycloak_client_test.go` — create Keycloak client tests | change-map.md | ST-6 | covered |
| `migrations/00X_add_tenant_sso_config.sql` — create SSO config table | change-map.md | ST-1 | covered |
| `migrations/00Y_add_user_keycloak_id.sql` — add keycloak_id column | change-map.md | ST-1 | covered |

### From Design Decisions

| Decision | Source | Covered By | Status |
|----------|--------|-----------|--------|
| DD-1: Dedicated `tenant_sso_config` table (not JSONB) | design-decisions.md | ST-1 (table), ST-3 (model), ST-4 (repository) | covered |
| DD-2: Automatic email-based account linking | design-decisions.md | ST-5 (FindOrCreateBySSO), ST-7 (ProcessCallback) | covered |
| DD-3: Dual-auth (SSO + password fallback) | design-decisions.md | ST-7 (additive path), ST-8 (separate handlers), ST-10 (verification) | covered |
| DD-4: Single Keycloak realm with per-tenant IdP | design-decisions.md | ST-2 (single realm config), ST-6 (IdP alias naming) | covered |
| DD-5: Keycloak Admin REST API (not CLI) | design-decisions.md | ST-6 (KeycloakClient) | covered |
| Deferred: Multi-domain mapping | design-decisions.md | Excluded from subtasks (correctly deferred) | covered |
| Deferred: SSO enforcement mode | design-decisions.md | Excluded from subtasks (correctly deferred) | covered |

## Coverage Gaps
No coverage gaps detected.

## Over-Coverage
No over-coverage detected. ST-10 (integration testing) adds an integration test file not explicitly listed in the change map, but testing is explicitly required by the implementation design and risk analysis. This is justified supporting work, not scope creep.

## Done-State Validation
- **Answer:** yes
- **Reasoning:** Walking through the primary scenario: a user enters their email -> the system looks up the domain in `tenant_sso_config` (ST-4) -> generates a SAML AuthnRequest redirect URL (ST-7) -> the HTTP handler returns the redirect (ST-8) -> after Keycloak authentication, the callback endpoint receives the SAML response (ST-8) -> the SSO service parses it and calls user service for JIT/linking (ST-7, ST-5) -> issues a JWT token (ST-7) -> redirects to the application. All routes are wired (ST-9). Password auth remains unchanged (additive changes only, verified by ST-10). Every acceptance criterion has a traceable path through the subtask chain. Completing all 10 subtasks fully implements the agreed SSO feature.
