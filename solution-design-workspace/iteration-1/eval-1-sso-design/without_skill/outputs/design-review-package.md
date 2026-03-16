# Design Review Package: SAML SSO for Multi-Tenant Platform

## Executive Summary

This design adds SAML 2.0 Single Sign-On to the multi-tenant Go backend platform. Enterprise tenants can authenticate users through their corporate identity providers (Okta, Azure AD, ADFS, etc.) via Keycloak acting as a SAML broker. The existing email/password login remains fully functional for all tenants.

**Scope:** SP-initiated SAML SSO, per-tenant configuration, JIT user provisioning, automatic account linking by email.

**Not in scope:** IdP-initiated SSO, Single Logout, SCIM provisioning, self-service SSO UI, OIDC/OAuth SSO.

**Estimated change footprint:** 3 new files, 8 modified files, 1 new database table. Zero modifications to existing API endpoints or auth middleware behavior.

---

## How It Works

### User Experience

1. User enters their email on the login page.
2. Frontend calls `GET /api/auth/sso/check?email=...` to detect SSO availability.
3. If SSO-enabled, frontend shows "Sign in with [Company] SSO" button.
4. Clicking the button navigates to `GET /api/auth/sso/initiate?email=...`.
5. Backend redirects to Keycloak, which redirects to the corporate IdP.
6. User authenticates at their corporate IdP (using whatever their company requires -- password, MFA, etc.).
7. User is redirected back through Keycloak to the backend callback.
8. Backend provisions/links the user, returns JWT tokens to the frontend.
9. User is logged in.

### What Stays the Same

- `POST /api/auth/login` works identically for all tenants, SSO-enabled or not.
- Auth middleware validates JWTs the same way regardless of how they were issued.
- All protected endpoints work the same for SSO and password users.
- Keycloak continues to operate in a single realm.
- No existing database tables are modified.

---

## Architecture Decisions

### Key Choices

| Decision | Choice | Why |
|----------|--------|-----|
| SSO endpoints | New routes, not modifying `/api/auth/login` | SSO is a redirect flow, fundamentally different from password API call. Zero risk to existing login. |
| SSO detection | Three-step: check -> initiate -> callback | Frontend stays simple; backend is authoritative about SSO config. |
| IdP routing | Keycloak `kc_idp_hint` parameter | Standard Keycloak feature. Users go directly to their corporate IdP, no intermediate selection page. |
| CSRF protection | Signed cookie (not server-side store) | Stateless, HA-compatible, no Redis or DB dependency for state management. |
| SSO config storage | Dedicated `tenant_sso_config` table | User's explicit choice from Stage 3. Supports clean queries, establishes pattern for future per-tenant config. |
| Account linking | Automatic by email match | User's explicit choice from Stage 3. When existing password user first logs in via SSO, their account is linked automatically. |
| Password fallback | Dual-auth (both SSO and password allowed) | User's explicit choice from Stage 3. Available as fallback during transition. |
| User provisioning | New `GetOrCreateByEmailWithKeycloakID` method | Existing `GetOrCreateByEmail` untouched. SSO has separate path with KeycloakID handling. |
| Keycloak admin API | gocloak with direct HTTP fallback | Try gocloak first; if SAML IdP fields are not supported, fall back to raw HTTP. |
| JWT tenant resolution | Email-domain fallback (existing middleware logic) | Already implemented. No Keycloak custom mapper needed for MVO. |

### User Decisions Honored

All three user corrections from Stage 3 task synthesis are implemented as specified:
- **Dual-auth:** Login endpoint unchanged, password always available.
- **Dedicated table:** `tenant_sso_config` with typed columns, not JSONB.
- **Automatic linking:** Email-based account linking in `GetOrCreateByEmailWithKeycloakID`.

---

## Change Footprint

### New Files (3)

| File | Purpose |
|------|---------|
| `migrations/002_sso_config.sql` | SSO configuration table |
| `internal/tenant/sso_config.go` | SSOConfig data model struct |
| `internal/tenant/sso_repository.go` | SSO config persistence layer |

### Modified Files (8)

| File | What Changes | Risk |
|------|-------------|------|
| `internal/tenant/service.go` | Add SSO config methods, accept SSORepository | Medium -- constructor signature change |
| `internal/user/repository.go` | Add `UpdateKeycloakID` method | Low -- new method only |
| `internal/user/service.go` | Add `GetOrCreateByEmailWithKeycloakID` method | Low -- new method only |
| `internal/auth/keycloak.go` | Add code exchange, URL builder, admin API methods | Medium -- gocloak API gaps possible |
| `internal/auth/service.go` | Add CheckSSO, InitiateSSO, HandleSSOCallback | Medium -- constructor change, core logic |
| `internal/auth/handler.go` | Add SSO endpoint handlers | Low -- new methods only |
| `internal/config/config.go` | Add SSOConfig struct | Low -- additive fields |
| `cmd/server/main.go` | Wire new repos/services, register SSO routes | Medium -- multiple constructor updates |

### Unchanged Files (10)

The auth middleware, auth models, tenant handler/repository/models, user handler/models, httputil, existing migration, and go.mod are all explicitly unchanged.

---

## Database Schema

One new table, no modifications to existing tables:

```sql
tenant_sso_config
├── id              UUID PK
├── tenant_id       UUID FK -> tenants(id) ON DELETE CASCADE, UNIQUE
├── enabled         BOOLEAN
├── idp_alias       TEXT (e.g., "saml-acme")
├── idp_display_name TEXT
├── metadata_url    TEXT (nullable)
├── metadata_xml    TEXT (nullable)
├── entity_id       TEXT
├── sso_url         TEXT
├── certificate     TEXT
├── name_id_format  TEXT
├── sign_requests   BOOLEAN
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ
```

---

## API Surface

### New Public Endpoints

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/api/auth/sso/check?email=...` | Check if email's tenant has SSO | None |
| GET | `/api/auth/sso/initiate?email=...` | Start SSO flow (302 redirect) | None |
| GET | `/api/auth/sso/callback?code=...&state=...` | Handle Keycloak callback | None |

### New Admin Endpoints

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/api/admin/tenants/{tenantID}/sso` | Get tenant SSO config | Admin |
| PUT | `/api/admin/tenants/{tenantID}/sso` | Create/update SSO config | Admin |
| DELETE | `/api/admin/tenants/{tenantID}/sso` | Remove SSO config | Admin |

### Unchanged Endpoints

| Method | Path | Status |
|--------|------|--------|
| POST | `/api/auth/login` | Unchanged |
| POST | `/api/auth/logout` | Unchanged |
| POST | `/api/auth/refresh` | Unchanged |
| GET | `/api/users/me` | Unchanged |
| GET | `/api/tenants/{tenantID}` | Unchanged |
| GET | `/api/admin/users` | Unchanged |
| PUT | `/api/admin/tenants/{tenantID}` | Unchanged |

---

## Risk Mitigation

### Risk: Zero Test Coverage + Auth Changes

**Mitigation:** All SSO code is additive. No existing methods are modified. The existing `Login()` path is untouched. New methods have isolated, testable logic. Implementation plan includes writing integration tests for existing login flow before deploying SSO changes.

### Risk: Keycloak SAML Broker Complexity

**Mitigation:** IdP configuration is automated via Keycloak Admin REST API. The admin endpoint handles both DB and Keycloak atomically. Template-based IdP configuration reduces per-tenant setup errors.

### Risk: gocloak May Not Support SAML IdP Admin API

**Mitigation:** Design includes a direct HTTP fallback. The `ExchangeCode` method (authorization code grant) is well-supported by gocloak's `GetToken`. Only the admin IdP management methods may need fallback. This is a containable risk -- if fallback is needed, it is a single additional file.

### Risk: Account Linking Edge Cases

**Mitigation:** `GetOrCreateByEmailWithKeycloakID` handles all cases explicitly:
- New user: create with KeycloakID
- Existing user without KeycloakID: link (set KeycloakID)
- Existing user with matching KeycloakID: proceed
- Existing user with mismatched KeycloakID: reject with error
- Existing user with mismatched TenantID: reject with error

### Risk: Keycloak Client Standard Flow Not Enabled

**Mitigation:** Documented as a configuration prerequisite. The implementation checklist includes verifying and enabling Standard Flow on the `platform-app` client. Both Standard Flow and Direct Access Grants can coexist on the same client.

---

## Keycloak Configuration Prerequisites

Before deploying SSO code, the Keycloak instance must be configured:

1. **Enable Standard Flow** on the `platform-app` client (Keycloak Admin Console -> Clients -> platform-app -> Settings -> Standard Flow Enabled = ON)
2. **Add Valid Redirect URI:** `{app_base_url}/api/auth/sso/callback`
3. **Add Web Origins:** `{frontend_origin}` (for CORS on token responses)

These are one-time manual steps in Keycloak admin console. Per-tenant SAML IdP configuration is handled by the admin API endpoints.

---

## Environment Variables

New environment variables required:

| Variable | Purpose | Default | Required |
|----------|---------|---------|----------|
| `SSO_CALLBACK_URL` | Full URL for SSO callback endpoint | `http://localhost:8080/api/auth/sso/callback` | Production: yes |
| `SSO_FRONTEND_URL` | Frontend origin for post-auth redirect | `http://localhost:3000` | Production: yes |
| `SSO_STATE_SECRET` | HMAC key for CSRF state cookie signing | (none) | Yes |

---

## Implementation Phases

### Phase 1: Foundation (Low Risk)
- Database migration
- SSO config model, repository, service methods
- User service KeycloakID methods
- Config extension

### Phase 2: Keycloak Integration (Medium Risk)
- Authorization code exchange method
- SSO redirect URL builder
- Keycloak Admin API methods (with gocloak verification)

### Phase 3: SSO Flow (Medium Risk)
- Auth service SSO methods (check, initiate, callback)
- Auth handler SSO endpoints
- SSO admin handler

### Phase 4: Wiring (Medium Risk)
- Route registration in main.go
- Constructor updates
- Docker compose env vars

### Phase 5: Validation
- Integration tests for existing login (safety net)
- SSO flow tests
- Keycloak IdP management tests

---

## Pre-Existing Issue

The `RequireRole` middleware at `internal/auth/middleware.go:99-104` has a variable shadowing bug where loop variable `r` (role string) shadows the `r` parameter (http.Request). This causes a compilation error on the line `r.WithContext(r.Context())`. This bug must be fixed before the SSO admin endpoints work, since they are behind `RequireRole("admin")`. Recommended: fix in a preparatory commit before SSO work begins.

---

## Acceptance Criteria Traceability

| Acceptance Criterion | Design Element |
|---------------------|----------------|
| SP-initiated SAML SSO login completes successfully | SSO initiate -> Keycloak redirect with kc_idp_hint -> SAML broker -> callback -> token exchange |
| Email/password login works identically | POST /api/auth/login completely unchanged |
| New SSO users are JIT-provisioned with correct tenant | GetOrCreateByEmailWithKeycloakID creates user with TenantID from email domain lookup |
| Existing users auto-linked on first SSO login | GetOrCreateByEmailWithKeycloakID sets KeycloakID on existing user if empty |
| Per-tenant SSO config stored and retrievable | tenant_sso_config table + SSORepository + admin GET endpoint |
| Keycloak IdP config automated via Admin API | CreateSAMLIdentityProvider method + admin PUT endpoint |
| JWT tokens from SSO contain all required claims | Keycloak issues JWTs with sub, email; tenant_id resolved via email-domain fallback in existing middleware |

---

## Open Items for Implementation

1. **Verify gocloak SAML IdP admin API support** -- test during Phase 2, implement HTTP fallback if needed
2. **Fix RequireRole bug** -- prerequisite for SSO admin endpoints
3. **Keycloak Standard Flow enablement** -- operational prerequisite, not code
4. **Frontend SSO integration** -- not in scope for backend implementation, but frontend team needs the API contract documented above
5. **SSO monitoring** -- log structured events for SSO initiation, callback success/failure, account linking events using zerolog

---

## Artifacts

| Document | Purpose |
|----------|---------|
| `implementation-design.md` | Full technical design with code-level specifications |
| `change-map.md` | File-by-file change inventory with dependencies and risk |
| `design-decisions.md` | 15 technical decisions with alternatives and rationale |
| `design-review-package.md` | This document -- stakeholder-ready summary |
