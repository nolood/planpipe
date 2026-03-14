# Stage 5 Handoff — Implementation Decomposition Complete

## Task Summary
Enable SAML 2.0 SSO authentication for enterprise tenants in a multi-tenant Go backend, using Keycloak as a SAML broker. The system supports SP-initiated SAML flow, per-tenant SSO configuration via a dedicated database table, JIT user provisioning, automatic email-based account linking, and programmatic Keycloak IdP automation through the Admin REST API. Password authentication is preserved as a fallback for all users (dual-auth).

## Classification
- **Type:** feature
- **Complexity:** high
- **Total subtasks:** 10
- **Execution waves:** 4
- **Max parallel subtasks:** 3
- **Solution direction:** systematic

## Implementation Approach
Stage 4 chose a systematic approach: a dedicated SSO subsystem within the existing repository-service-handler architecture. The auth module gains an SSOService (flow orchestration) and KeycloakClient (Admin API), the tenant module gains SSO config storage via a dedicated table, and the user module is extended with JIT provisioning and email-based account linking. The decomposition breaks this into 10 subtasks following the natural dependency chain from schema through models, repositories, services, handlers, to wiring.

## Execution Strategy
Work is organized into 4 waves. Waves 1 and 2 maximize parallelism (3 subtasks each, all independent). Wave 3 is sequential -- the SSO service must be built before its HTTP handlers. Wave 4 wires everything together and runs integration tests. The critical path runs through: ST-1/ST-3 (foundation) -> ST-4/ST-5 (repositories) -> ST-7 (SSO service) -> ST-8 (handlers) -> ST-9 (wiring) -> ST-10 (testing). ST-7 is the convergence point where 4 dependencies must be met before work can start.

## Subtask Summary

| ID | Title | Type | Wave | Scope | Blocking Dependencies | Completion Criteria Summary |
|----|-------|------|------|-------|-----------------------|---------------------------|
| ST-1 | Database Migrations for SSO | foundation | 1 | small | none | Both migration files exist, run cleanly up and down |
| ST-2 | SSO Configuration Struct | foundation | 1 | small | none | SSOConfig struct exists, loads from env vars |
| ST-3 | Tenant and User Model Extensions | foundation | 1 | small | none | TenantSSOConfig struct and User.KeycloakID field exist |
| ST-4 | Tenant SSO Config Repository Methods | implementation | 2 | medium | ST-1, ST-3 | GetSSOConfigByDomain and UpsertSSOConfig work with tests |
| ST-5 | User Repository and Service SSO Extensions | implementation | 2 | medium | ST-1, ST-3 | FindOrCreateBySSO and LinkKeycloakID work with tests |
| ST-6 | Keycloak Admin API Client | implementation | 2 | medium | ST-2 | CreateIdP, GetIdP, DeleteIdP work with mocked HTTP tests |
| ST-7 | SSO Service Implementation | implementation | 3 | large | ST-2, ST-4, ST-5, ST-6 | InitiateSSO and ProcessCallback orchestrate full SSO flow with tests |
| ST-8 | SSO HTTP Handlers | implementation | 3 | medium | ST-7 | HandleSSOInitiate and HandleSSOCallback work with proper HTTP I/O |
| ST-9 | Route Registration and Dependency Wiring | integration | 4 | small | ST-2, ST-6, ST-7, ST-8 | SSO routes registered, dependencies wired, app starts |
| ST-10 | End-to-End Integration Testing | testing | 4 | medium | ST-9 | Integration tests verify full SSO flow and backward compatibility |

## Execution Waves

### Wave 1 — Foundation
**Parallel group:** ST-1, ST-2, ST-3
**Establishes:** Database schema (tenant_sso_config table, users.keycloak_id column), application configuration (SSOConfig struct with Keycloak settings), and data models (TenantSSOConfig struct, User.KeycloakID field).

### Wave 2 — Core Implementation
**Parallel group:** ST-4 || ST-5 || ST-6
**Builds:** Data access layer for SSO config (tenant repository), user provisioning/linking logic (user repository + service), and Keycloak Admin API integration (KeycloakClient).

### Wave 3 — SSO Orchestration
**Sequential:** ST-7, then ST-8
**Builds:** SSO service that orchestrates the full SAML flow (initiate + callback), then HTTP handlers that expose the service via REST endpoints.

### Wave 4 — Integration & Convergence
**Sequential:** ST-9, then ST-10
**Validates:** All components wired correctly in main.go, SSO endpoints accessible, full flow works end-to-end, backward compatibility preserved.

## Dependency Graph

```
ST-1 (migrations)        ──→ ST-4 (tenant repo)
                          ──→ ST-5 (user repo+service)
ST-2 (config)             ──→ ST-6 (keycloak client)
                          ──→ ST-7 (sso service)
ST-3 (models)             ──→ ST-4 (tenant repo)
                          ──→ ST-5 (user repo+service)
                          ──→ ST-6 (keycloak client) [soft]
ST-4 (tenant repo)        ──→ ST-7 (sso service)
ST-5 (user repo+service)  ──→ ST-7 (sso service)
ST-6 (keycloak client)    ──→ ST-7 (sso service)
ST-7 (sso service)        ──→ ST-8 (handlers)
ST-8 (handlers)           ──→ ST-9 (wiring)
ST-9 (wiring)             ──→ ST-10 (integration testing)
```

## Conflict Zones
| Zone | Subtasks | Resolution |
|------|----------|------------|
| `internal/auth/` directory | ST-6, ST-7, ST-8 | Sequenced across Waves 2-3. Each creates/modifies a different file (keycloak_client.go, sso_service.go, handler.go). No actual file overlap. |

No other conflict zones. All parallel subtasks within waves operate on different files in different modules.

## Coverage Verification
- **Verdict:** COVERAGE_OK
- **Confidence:** high
- **All acceptance criteria mapped:** yes
- **All change map files covered:** yes (16/16)
- **All design decisions traceable:** yes (5/5 + 2 deferred correctly excluded)

## Constraints Respected
- **Backward compatibility:** All changes are additive. Existing password auth unchanged. User model gains optional field (read-compatible).
- **Repository-service-handler pattern:** Every subtask follows this architecture. SSOService -> handlers pattern, repositories for data access.
- **Single Keycloak realm:** Config uses single realm. IdP isolation via alias naming `tenant-{id}-saml`.
- **Dedicated SSO config table:** ST-1 creates normalized table, ST-3 creates typed model, ST-4 creates typed repository methods.
- **MVO scope:** No deferred items included. No admin UI, no IdP-initiated SSO, no SLO, no SCIM.

## Risks for Execution
| Risk | Affected Subtasks | Mitigation | Severity |
|------|-------------------|------------|----------|
| ST-7 convergence bottleneck | ST-7 (blocks ST-8, ST-9, ST-10) | Prioritize Wave 1 and 2 completion. ST-7 is large scope -- ensure adequate time. | medium |
| SAML XML/signature handling complexity | ST-7 | Use crewjam/saml library. Test with real Keycloak instance during ST-10. | medium |
| Account linking race condition | ST-5 | DB UNIQUE constraint on keycloak_id prevents duplicates. Handle constraint violations gracefully in FindOrCreateBySSO. | medium |
| Keycloak Admin API compatibility | ST-6 | Test against Keycloak v24.0+. HTTP mock tests cover API contract. Validate with real Keycloak in ST-10. | low |
| Integration test environment | ST-10 | Requires mock Keycloak or test instance. Decision needed on approach. | low |

## User Decisions Log
[Skipped -- test run without user review steps. All prior user decisions from Stage 4 are preserved and reflected in the decomposition.]

## Acceptance Criteria
- SP-initiated SAML SSO login completes successfully through corporate IdP
- Email/password login works identically before and after
- New SSO users are JIT-provisioned with correct tenant
- Existing users auto-linked on first SSO login via email
- Per-tenant SSO config stored and retrievable
- Keycloak IdP config automated via Admin API
- JWT tokens from SSO contain all required claims

## Detailed References
- `execution-backlog.md` -- complete execution backlog with all 10 subtasks
- `coverage-matrix.md` -- requirement-to-subtask traceability
- `decomposition-review-package.md` -- user review document
- `implementation-design.md` -- implementation design (Stage 4)
- `change-map.md` -- file-level change map (Stage 4)
- `design-decisions.md` -- decision journal (Stage 4)
- `agreed-task-model.md` -- agreed task model (Stage 3)
