# SSO via SAML - Requirements Document

## 1. Overview

Add SAML-based Single Sign-On (SSO) support so that enterprise client users can authenticate through their corporate Identity Provider (IdP) instead of the standard email/password flow. SSO configuration is per-tenant. The existing login flow for non-SSO users must remain unchanged.

## 2. Context & Constraints

| Item | Detail |
|------|--------|
| **Target** | Q3 delivery |
| **Auth system** | Keycloak (already in use) |
| **Backend** | FastAPI |
| **Frontend** | React |
| **Existing auth code** | `services/auth/` |
| **Multi-tenancy** | SSO config is per-tenant |
| **Backward compatibility** | No changes to current login flow for non-SSO tenants/users |

## 3. Functional Requirements

### FR-1: SAML IdP Integration via Keycloak
- Keycloak must be configured to act as a SAML Service Provider (SP) brokering authentication to external corporate IdPs.
- Each tenant can have zero or one SAML IdP configuration.
- SAML metadata exchange (SP metadata export, IdP metadata import) must be supported.

### FR-2: Per-Tenant SSO Configuration
- Admin users (or a super-admin) must be able to enable/disable SSO for a specific tenant.
- Configuration includes: IdP Entity ID, SSO URL, IdP certificate/metadata XML, attribute mappings (email, name, groups).
- Configuration must be stored persistently and retrievable at login time.

### FR-3: SSO Login Flow
- When a user navigates to the login page and their tenant has SSO enabled, they should be redirected to their corporate IdP.
- After successful IdP authentication, the user is redirected back and a session/token is created as normal.
- If the user does not yet exist in the local system, a Just-In-Time (JIT) provisioning step creates the user account from SAML assertions.
- If the user already exists, their profile attributes are optionally updated from SAML assertions.

### FR-4: Non-SSO Login Unchanged
- Tenants without SSO enabled continue to use the existing email/password flow.
- No UI or backend changes affect the current non-SSO authentication path.

### FR-5: Logout
- SSO users must be able to log out, which should terminate both the local session and (optionally, configurable) trigger SAML Single Logout (SLO) at the IdP.

### FR-6: Error Handling
- Graceful handling of IdP unavailability, invalid SAML responses, expired certificates.
- Clear error messages for users when SSO login fails.

## 4. Non-Functional Requirements

### NFR-1: Security
- All SAML assertions must be validated (signature, audience, timestamps).
- SAML responses must be transmitted over HTTPS.
- IdP certificates must be validated and rotatable without downtime.

### NFR-2: Performance
- SSO login latency should be comparable to standard login (excluding IdP response time).
- Tenant SSO configuration lookup must be fast (cached if necessary).

### NFR-3: Observability
- SSO login attempts (success/failure) must be logged with tenant context.
- Metrics for SSO usage per tenant.

### NFR-4: Testability
- The feature must be testable with a mock IdP in staging/dev environments.

## 5. Out of Scope
- OIDC-based SSO (only SAML for now).
- SCIM provisioning / directory sync.
- SSO for internal admin users (only tenant end-users).
- Changes to existing non-SSO authentication flow.
