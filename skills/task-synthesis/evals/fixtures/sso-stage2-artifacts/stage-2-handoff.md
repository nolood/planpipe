# Stage 2 Handoff — Deep Analysis Complete

## Task Summary

Add SAML 2.0 Single Sign-On support to a multi-tenant Go backend platform so that enterprise tenants can authenticate their employees through corporate identity providers. Keycloak (already in use for email/password authentication) will act as the SAML SP/broker. The existing email/password login flow must remain fully functional for non-SSO tenants. SSO configuration is per-tenant -- each tenant independently enables SSO and provides their IdP's SAML metadata. The backend uses Go with chi router, gocloak library, PostgreSQL via pgx, and follows a clean repository-service-handler pattern across `internal/auth/`, `internal/tenant/`, and `internal/user/` packages.

## Classification
- **Type:** feature
- **Complexity:** high — requires new authentication flow (authorization code vs direct grant), schema changes, Keycloak configuration, per-tenant routing logic, and JIT user provisioning, all while preserving backward compatibility with the existing password flow
- **Primary risk area:** integration — the task depends heavily on Keycloak SAML brokering configuration, enterprise IdP interoperability, and the shift from direct grant to authorization code flow

## Analysis Summary

### Product / Business
Enterprise adoption is blocked because organizations cannot enforce their corporate authentication policies on the platform. SAML SSO is table-stakes for enterprise sales, and Q3 delivery is prioritized to unblock active enterprise pipeline. The main scenario is SP-initiated: a user enters their email, the system detects an SSO-enabled tenant by email domain, redirects to the corporate IdP via Keycloak, and the user lands authenticated in the application after SAML assertion processing. Success looks like enterprise deals closing, SSO login success rate above 95%, and reduced authentication support tickets.

### Codebase / System
The codebase is a clean Go application with four packages: `internal/auth/` (5 files -- handler, service, keycloak client, middleware, models), `internal/tenant/` (4 files), `internal/user/` (4 files), and `internal/config/`. The primary change points are: auth handler (new SSO endpoints), auth service (SSO login branch), keycloak client (authorization code flow methods), tenant model/repository (SSO config storage), and a new database migration. The auth middleware should work unchanged since it validates JWTs regardless of issuance method. There is zero test coverage across the entire codebase. A variable shadowing bug exists in the RequireRole middleware.

### Constraints / Risks
The most important constraints are: single Keycloak realm architecture (all tenant IdPs share one realm), no existing per-tenant config storage mechanism, and the fundamental shift from direct grant to authorization code flow for SSO users. The highest risks are: zero test coverage creating regression danger during auth flow changes (high likelihood, high impact), Keycloak SAML broker configuration complexity at scale (medium, high), and undefined account linking strategy for existing password users (high, medium). The existing login API contract, JWT token format, and context key contract must all be preserved.

## System Map

### Modules Involved
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Auth | `internal/auth/` | Primary change target — new SSO endpoints, service logic, Keycloak integration | large |
| Tenant | `internal/tenant/` | SSO config storage, tenant detection for SSO routing | medium |
| User | `internal/user/` | JIT provisioning enhancement (KeycloakID population) | small |
| Config | `internal/config/` | Possible SAML-related base configuration | small |
| Server | `cmd/server/main.go` | Route registration for new SSO endpoints | small |
| Migrations | `migrations/` | New migration for SSO config schema | medium |

### Key Change Points
| Location | What Changes | Scope |
|----------|-------------|-------|
| `internal/auth/handler.go` | New SSO initiation and callback HTTP endpoints | large |
| `internal/auth/service.go:Login` | SSO tenant detection branch; new SSO callback processing method | medium |
| `internal/auth/keycloak.go` | Authorization code exchange methods; possibly SAML IdP management via Admin API | medium |
| `internal/tenant/models.go` | SSO configuration fields/struct added to tenant model | medium |
| `internal/tenant/repository.go` | New queries for SSO config CRUD | medium |
| `migrations/002_sso_config.sql` | New table or columns for per-tenant SSO configuration | medium |
| `internal/user/service.go:GetOrCreateByEmail` | Accept and store KeycloakID for SSO-provisioned users | small |
| `cmd/server/main.go` | Register SSO routes in public route group | small |

### Critical Dependencies
- **Keycloak v24.0:** Must be configured with SAML IdP broker per tenant, Standard Flow enabled on the client, attribute mappers for tenant_id claim in brokered tokens. Keycloak handles all SAML protocol operations -- the Go app never sees raw SAML assertions
- **gocloak v13.9.0:** Must support authorization code token exchange. SAML IdP management via Admin API may or may not be covered -- may need direct HTTP calls
- **Enterprise IdPs:** External SAML IdPs controlled by customers. Metadata exchange happens during configuration. IdP availability is a runtime dependency for SSO login only

## Constraints the Plan Must Respect

- **Backward compatibility:** The existing `POST /api/auth/login` endpoint with email/password must continue working identically for non-SSO tenants. Source: business requirement + `internal/auth/handler.go:18-29`
- **Single Keycloak realm:** All SAML IdP configurations must coexist in the `platform` realm. Each tenant IdP needs a unique alias. Source: `internal/auth/keycloak.go:18`, `internal/config/config.go:19`
- **Repository-service-handler pattern:** All new code must follow the existing layering convention. Source: consistent architecture across all modules
- **JWT claim compatibility:** SSO-issued JWTs must contain `sub`, `email`, `tenant_id`, and `realm_access.roles` claims for middleware and handler compatibility. Source: `internal/auth/keycloak.go:68-88`, `internal/auth/middleware.go:61-83`
- **No per-tenant config exists yet:** SSO is the first feature that needs per-tenant configuration storage. The schema design for this sets a precedent. Source: `internal/tenant/models.go:18-23`
- **Q3 delivery timeline:** Scope decisions should favor MVO delivery within Q3. Source: requirements draft
- **SAML 2.0 protocol only:** No OIDC/OAuth SSO in scope. Source: requirements draft

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Suggested Mitigation |
|------|-----------|--------|---------------------|
| Zero test coverage makes auth flow changes dangerous | high | high | Establish integration tests for existing login flow before modifying it. All SSO code should include tests |
| Keycloak SAML broker configuration complexity and per-tenant IdP management at scale | medium | high | Build automation layer using Keycloak Admin API. Create reusable IdP configuration templates |
| Account linking undefined for existing password users enabling SSO | high | medium | Define strategy before implementation: match by email, update KeycloakID. Plan for edge cases (email mismatch) |
| Keycloak client may need Standard Flow enabled, risking config error | medium | high | Test Keycloak client changes in staging. Both direct grant and standard flow can coexist |
| gocloak may not cover SAML IdP Admin API | medium | medium | Check gocloak source early. Prepare fallback: direct HTTP client for Keycloak Admin REST API |
| SAML attribute mapping varies across IdP vendors | medium | medium | Design standard attribute mapper template. Document required IdP attributes |

## Product Requirements for Planning

- **Main scenario:** Enterprise user enters email at login, system detects SSO-enabled tenant, redirects to corporate IdP via Keycloak SAML broker, user authenticates at IdP, returns with JWT, lands in the application
- **Success signals:** Enterprise deal closure rate, SSO login success rate (>95%), time-to-SSO-configuration, authentication support ticket volume reduction
- **Minimum viable outcome:** SP-initiated SAML SSO for at least one tenant, with JIT user provisioning, coexisting with unchanged email/password flow. No self-service UI, no IdP-initiated SSO, no SLO, no SCIM
- **Backward compatibility:** Email/password login must be completely unaffected for non-SSO tenants. Same API contract, same behavior, same error messages

## Critique Results

The independent critic reviewed all three analyses and found all three SUFFICIENT. No FAIL scores were issued across any criterion. All analyses demonstrated specificity (concrete code references, real file paths, actual function names) rather than generic statements.

Key strengths identified by the critic:
- Product analysis articulated business intent beyond requirements restatement and provided measurable success signals with leading/lagging distinction
- System analysis verified all claims from actual code reads, with specific file paths and line references throughout. Every module was explored, not just listed
- Constraints analysis provided calibrated risk assessments (not all "high") with evidence and code references for each constraint

Minor observations (did not block SUFFICIENT verdicts):
- Product analysis could elaborate more on the admin configuration secondary scenario
- System analysis found a tangential but real bug (RequireRole variable shadowing) that indicates limited code exercising
- Constraints analysis noted gocloak SAML Admin API coverage needs verification but appropriately flagged it as an open question

No cross-analysis contradictions were found. All three analyses align on the core challenges: no per-tenant config storage, single Keycloak realm, auth flow paradigm shift from direct grant to authorization code, and zero test coverage.

## Open Questions for Planning

1. **Password fallback policy for SSO tenants** — Should SSO-enabled tenants have password login blocked (security enforcement) or available as fallback (availability)? This is a product decision with significant architectural implications. Blocks: login endpoint behavior design
2. **Account linking strategy** — How should existing password users be handled when their tenant enables SSO? Automatic email-based linking, admin-driven linking, or re-registration? Affects: user service logic, data migration, rollout planning
3. **Per-tenant config storage design** — New table (`tenant_sso_config`) vs JSONB column on tenants table? This sets a precedent for all future per-tenant configuration. Affects: schema design, repository pattern, migration complexity
4. **gocloak SAML IdP Admin API coverage** — Does gocloak v13.9.0 support `CreateIdentityProvider` and related SAML IdP management endpoints, or is a custom HTTP client needed? Affects: implementation approach for IdP configuration automation
5. **Keycloak client Standard Flow configuration** — Is the current `platform-app` client already configured for Authorization Code flow, or does it need reconfiguration? Affects: deployment steps, testing strategy
6. **SSO and password coexistence within a tenant** — Can a tenant support both SSO and email/password simultaneously during transition, or is it all-or-nothing? Affects: tenant config model, login routing logic
7. **SAML attribute requirements** — Which SAML attributes are required from IdPs (email, name, groups?) and how should missing optional attributes be handled? Affects: Keycloak mapper configuration, JIT provisioning logic
8. **SSO configuration management at scale** — How will 50+ tenant IdP configurations be managed, monitored, and debugged? Nice to resolve but planning can proceed without it

## Detailed Analyses

These files contain the full analysis and can be consulted for details:
- `product-analysis.md` — full product/business analysis
- `system-analysis.md` — full codebase/system analysis
- `constraints-risks-analysis.md` — full constraints/risks analysis
