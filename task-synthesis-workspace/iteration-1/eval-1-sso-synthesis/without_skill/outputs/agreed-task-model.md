# Agreed Task Model: SAML 2.0 SSO Feature

**Status:** Confirmed (user accepted all sections of the agreement package)

---

## 1. Task Definition

**Task:** Add SAML 2.0 Single Sign-On support to the multi-tenant Go backend platform enabling enterprise tenants to authenticate employees through corporate identity providers via Keycloak SAML brokering, while preserving the existing email/password login flow for non-SSO tenants.

**Type:** New feature
**Complexity:** High
**Timeline:** Q3 delivery (MVO)
**Primary Risk Area:** Integration

---

## 2. Agreed Decisions

### D1. Password Fallback Policy: BLOCK (Option A)

When a tenant has SSO enabled, password login via `POST /api/auth/login` is blocked for that tenant. The endpoint returns an error response directing the user to the SSO flow. Rationale: enterprises require enforcement of corporate authentication policies; allowing password fallback defeats the purpose of SSO.

**Implementation impact:**
- `auth.Service.Login()` must check tenant SSO status and reject password login with a specific error code/message
- Error response should include a hint or redirect URL for the SSO flow
- Non-SSO tenants are completely unaffected

### D2. Account Linking Strategy: AUTOMATIC EMAIL-BASED (Option A)

When an existing password user logs in via SSO for the first time, the system matches by email and updates their user record with the Keycloak brokered subject ID (`keycloak_id`). No duplicate user is created.

**Implementation impact:**
- `user.Service.GetOrCreateByEmail()` enhanced to accept and store `KeycloakID`
- If user exists by email but has no `keycloak_id`, update the record with the brokered subject ID
- If user exists by email AND has a different `keycloak_id`, this is an error condition (log and reject)
- Edge case: email mismatch between IdP assertion and existing account -- reject with clear error

### D3. Per-Tenant Config Storage: NEW TABLE (Option A)

SSO configuration is stored in a dedicated `tenant_sso_config` table with a foreign key to the tenants table. This establishes the pattern for all future per-tenant configuration.

**Implementation impact:**
- New migration: `migrations/002_sso_config.sql`
- Schema: `tenant_sso_config(id UUID PK, tenant_id UUID FK UNIQUE, sso_enabled BOOL, provider_alias TEXT, metadata_url TEXT, metadata_xml TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ)`
- `UNIQUE` constraint on `tenant_id` enforces one SSO config per tenant
- New repository, service methods for SSO config CRUD
- Follows existing repository-service-handler pattern

### Pre-Existing: Fix RequireRole Bug WITH SSO Work

The variable shadowing bug in `internal/auth/middleware.go:99-104` will be fixed as part of the SSO implementation effort. Low effort, reduces overall risk in the auth module.

---

## 3. Agreed Scope

### In Scope (MVO)

| # | Deliverable | Description |
|---|------------|-------------|
| S1 | SSO initiation endpoint | New endpoint (e.g., `POST /api/auth/sso/initiate`) that accepts email, detects SSO tenant, returns Keycloak SAML broker redirect URL |
| S2 | SSO callback endpoint | New endpoint (e.g., `GET /api/auth/sso/callback`) that receives authorization code from Keycloak, exchanges for JWT, provisions user, returns tokens |
| S3 | Per-tenant SSO config storage | `tenant_sso_config` table, repository, service, and admin API endpoint for SSO configuration CRUD |
| S4 | SSO tenant detection in login flow | `auth.Service.Login()` detects SSO-enabled tenants and blocks password login with an appropriate error |
| S5 | Keycloak authorization code flow | `KeycloakClient` methods for generating SAML broker redirect URL and exchanging authorization code for tokens |
| S6 | JIT user provisioning for SSO | Enhanced `GetOrCreateByEmail` that accepts and stores `KeycloakID` for SSO-provisioned users |
| S7 | Automatic account linking | Email-based matching for existing password users logging in via SSO for the first time |
| S8 | Database migration | `migrations/002_sso_config.sql` for SSO configuration table |
| S9 | Keycloak SAML IdP configuration automation | Admin API or utility for programmatic SAML IdP broker setup in Keycloak per tenant |
| S10 | Integration tests | Tests for existing login flow (safety net) and new SSO flow |
| S11 | RequireRole bug fix | Fix variable shadowing in `internal/auth/middleware.go:99-104` |

### Out of Scope (Deferred)

| # | Item | Rationale |
|---|------|-----------|
| X1 | Self-service SSO configuration UI | API-only is sufficient for MVO; UI is a follow-up |
| X2 | IdP-initiated SSO | SP-initiated covers the primary use case; IdP-initiated adds complexity |
| X3 | Single Logout (SLO) | Not required for MVO; adds significant complexity |
| X4 | SCIM provisioning | JIT provisioning is sufficient for MVO |
| X5 | Multiple IdPs per tenant | One IdP per tenant covers initial enterprise requirements |
| X6 | Multiple email domains per tenant | Most enterprises have a primary domain; can extend later |
| X7 | SSO monitoring/alerting dashboard | Operational tooling deferred to post-MVO |
| X8 | OIDC/OAuth SSO | SAML 2.0 only per requirements |

---

## 4. Agreed Architecture

### New Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/auth/sso/initiate` | Public | Accept email, detect SSO tenant, return redirect URL to Keycloak SAML broker |
| GET | `/api/auth/sso/callback` | Public | Receive authorization code from Keycloak, exchange for JWT, return tokens |
| POST | `/api/admin/tenants/{id}/sso` | Protected (admin) | Create/update SSO configuration for a tenant |
| GET | `/api/admin/tenants/{id}/sso` | Protected (admin) | Get SSO configuration for a tenant |
| DELETE | `/api/admin/tenants/{id}/sso` | Protected (admin) | Disable/remove SSO configuration for a tenant |

### Data Model

```
tenant_sso_config
- id: UUID (PK)
- tenant_id: UUID (FK -> tenants.id, UNIQUE)
- sso_enabled: BOOLEAN (DEFAULT false)
- provider_alias: TEXT (Keycloak IdP alias, e.g., "saml-acmecorp")
- metadata_url: TEXT (IdP metadata URL, nullable)
- metadata_xml: TEXT (IdP metadata XML, nullable -- one of url or xml required)
- created_at: TIMESTAMPTZ
- updated_at: TIMESTAMPTZ
```

### Flow: SSO Login

```
User -> POST /api/auth/sso/initiate {email}
  -> auth.Handler.SSOInitiate()
    -> auth.Service.InitiateSSO(email)
      -> tenant.Service.GetByEmailDomain(domain)
      -> tenant.Service.GetSSOConfig(tenantID)
      -> [if SSO not enabled: error]
      -> auth.KeycloakClient.GetSAMLBrokerRedirectURL(providerAlias, callbackURL)
    <- redirect URL
  <- {redirect_url}

User -> Browser redirect to Keycloak -> IdP -> back to Keycloak -> redirect to callback

Keycloak -> GET /api/auth/sso/callback?code=XXX&state=YYY
  -> auth.Handler.SSOCallback()
    -> auth.Service.ProcessSSOCallback(code, state)
      -> auth.KeycloakClient.ExchangeCode(code)
      -> [extract claims from JWT: email, sub, tenant_id]
      -> user.Service.GetOrCreateByEmail(email, tenantID, keycloakID)
      -> [if existing user: update keycloak_id]
    <- TokenResponse {access_token, refresh_token, expires_in, token_type}
  <- TokenResponse (same format as password login)
```

### Flow: Password Login (Modified)

```
User -> POST /api/auth/login {email, password, tenant_id?}
  -> auth.Handler.Login()
    -> auth.Service.Login(email, password, tenantID)
      -> tenant.Service.GetByEmailDomain(domain) [existing]
      -> tenant.Service.GetSSOConfig(tenantID) [NEW CHECK]
      -> [if SSO enabled: REJECT with error "SSO required for this tenant"]
      -> [if SSO not enabled: proceed with existing password flow unchanged]
    <- TokenResponse
  <- TokenResponse
```

---

## 5. Agreed Constraints

### Hard Constraints

| # | Constraint | Source |
|---|-----------|--------|
| C1 | `POST /api/auth/login` unchanged for non-SSO tenants (same contract, behavior, errors) | Business requirement |
| C2 | SSO-issued JWTs contain `sub`, `email`, `tenant_id`, `realm_access.roles` | Middleware compatibility |
| C3 | Context keys (`ContextKeyUserID`, `ContextKeyTenantID`, `ContextKeyRoles`) produce same values for SSO requests | Handler compatibility |
| C4 | All SAML IdP configs in single `platform` Keycloak realm with unique aliases | Existing architecture |
| C5 | Repository-service-handler layering for all new code | Codebase convention |
| C6 | SAML 2.0 protocol only (no OIDC/OAuth SSO) | Requirements |
| C7 | Q3 delivery (MVO scope) | Business timeline |
| C8 | Password login blocked for SSO-enabled tenants | Decision D1 |
| C9 | Automatic email-based account linking | Decision D2 |
| C10 | Separate `tenant_sso_config` table for SSO configuration | Decision D3 |

---

## 6. Agreed Risk Mitigations

| # | Risk | Agreed Mitigation | When |
|---|------|-------------------|------|
| R1 | Zero test coverage + auth changes | Write integration tests for existing login flow before modifications | First implementation phase |
| R2 | Keycloak SAML config complexity | Build Admin API automation layer with reusable IdP templates | During Keycloak integration phase |
| R3 | Account linking edge cases | Automatic email match with error on keycloak_id conflict; clear error messages | During user provisioning implementation |
| R4 | Keycloak client needs Standard Flow | Test in staging; both flows coexist on same client | During Keycloak setup phase |
| R5 | gocloak SAML Admin API gaps | Check source early; prepare direct HTTP fallback client | Early in implementation |
| R6 | IdP attribute mapping variance | Standard mapper template + documented required attributes | During Keycloak integration phase |
| R7 | RequireRole bug | Fix as part of SSO work | Early in implementation |

---

## 7. Open Items (Non-Blocking for Planning)

These items can be resolved during implementation without affecting the plan structure:

| # | Item | Resolution Approach |
|---|------|-------------------|
| O1 | Verify gocloak SAML Admin API coverage | Check source code during implementation; fallback to direct HTTP |
| O2 | Verify Keycloak `platform-app` client Standard Flow status | Check during Keycloak setup; enable if needed |
| O3 | Determine required vs optional SAML attributes | Default to email-only required; name as optional with email fallback |
| O4 | SSO callback URL structure | Use `/api/auth/sso/callback` (standard pattern) |
| O5 | Keycloak hot-reload of IdP configs | Verify during integration testing |
| O6 | SSO at scale (50+ tenants) | Defer to post-MVO operational planning |
| O7 | SSO-specific monitoring | Defer to post-MVO; use standard logging for MVO |
