# Requirements Draft

## Goal

Add SAML-based Single Sign-On (SSO) support so that enterprise tenant users can authenticate through their corporate identity provider, integrated with the existing Keycloak authentication system.

## Problem Statement

Enterprise clients need their employees to log in using their organization's identity provider (via SAML) rather than the application's standard email/password flow. This is a common enterprise requirement for security compliance, centralized user management, and reduced credential fatigue. Without SSO, enterprise adoption is blocked — users must maintain separate credentials, and IT departments cannot enforce their organization's authentication policies on our application. The product team has prioritized this for Q3 delivery.

## Scope

What is included in this task:

- SAML 2.0 SP (Service Provider) integration with Keycloak acting as the identity broker
- Per-tenant SSO configuration — each tenant can independently enable/disable SSO and configure their IdP connection
- Backend changes in the Go `internal/auth/` package to support SAML authentication flow
- Frontend changes to detect SSO-enabled tenants and redirect to the appropriate IdP login
- Keycloak realm/client configuration for SAML identity provider brokering
- Coexistence with the existing email/password login flow — non-SSO tenants are unaffected

## Out of Scope

- Changes to the existing email/password login flow for non-SSO tenants
- OIDC/OAuth-based SSO (only SAML is in scope)
- User provisioning/deprovisioning via SCIM
- Self-service SSO configuration UI for tenant admins (TBD)
- IdP-initiated SSO (TBD — SP-initiated is the baseline)
- Migration of existing users to SSO — account linking strategy is an open question

## Constraints

- **Timeline**: Q3 delivery target
- **Technical stack**: Must integrate with existing Keycloak instance, Go backend (chi router), and frontend
- **Backward compatibility**: Current email/password login flow must remain fully functional
- **Protocol**: SAML 2.0 specifically (not OIDC)
- **Multi-tenancy**: SSO configuration must be per-tenant, not system-wide

## Dependencies & Context

- **Keycloak**: Already used for authentication — has built-in SAML IdP brokering support
- **`internal/auth/` package**: Auth middleware, Keycloak client, token validation — the primary backend change target
- **`internal/tenant/` package**: Tenant model and service — tenant resolution during login
- **`internal/user/` package**: User model with keycloak_id mapping
- **Database schema**: tenants and users tables in PostgreSQL

## Knowns

- The application uses Keycloak for authentication (gocloak library)
- The backend is Go with chi router, auth middleware in `internal/auth/`
- SAML 2.0 is the required SSO protocol
- SSO configuration must be per-tenant
- The existing email/password flow must remain unchanged for non-SSO tenants
- The product team wants Q3 delivery
- Keycloak natively supports SAML identity provider brokering
- Tenants are identified by email domain during login

## Unknowns

- Current tenant model has no per-tenant configuration storage beyond basic columns — where to store SSO settings?
- Whether Keycloak is configured with one realm or multiple — this affects SAML IdP setup
- Whether SP-initiated SSO alone is sufficient or if IdP-initiated SSO is also required
- How existing users should be handled when a tenant enables SSO — account linking strategy
- Whether session management requirements exist for SSO (single logout / SLO)
- What SAML attribute mapping requirements are needed
- Whether JIT user provisioning is expected on first SSO login

## Assumptions

- Keycloak acts as the SAML SP/broker — the Go app itself doesn't implement SAML protocol
- The existing tenant model can be extended to include SSO settings (new table or JSONB column)
- SP-initiated SSO is the primary flow needed
- JIT provisioning of new SSO users is expected
- Single Logout (SLO) is not required for initial release
- Enterprise clients will provide IdP metadata XML for configuration
