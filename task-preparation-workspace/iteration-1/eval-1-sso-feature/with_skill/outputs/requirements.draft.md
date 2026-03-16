# Requirements Draft

## Goal

Add SAML-based Single Sign-On (SSO) support so that enterprise tenant users can authenticate through their corporate identity provider, integrated with the existing Keycloak authentication system.

## Problem Statement

Enterprise clients need their employees to log in using their organization's identity provider (via SAML) rather than the application's standard email/password flow. This is a common enterprise requirement for security compliance, centralized user management, and reduced credential fatigue. Without SSO, enterprise adoption is blocked or friction-heavy — users must maintain separate credentials, and IT departments cannot enforce their organization's authentication policies (MFA, password rotation, session control) on our application. The product team has prioritized this for Q3 delivery.

## Scope

What is included in this task:

- SAML 2.0 SP (Service Provider) integration with Keycloak acting as the identity broker
- Per-tenant SSO configuration — each tenant can independently enable/disable SSO and configure their IdP connection
- Backend changes in the FastAPI `services/auth/` middleware to support SAML authentication flow (SP-initiated SSO at minimum)
- Frontend changes in the React application to detect SSO-enabled tenants and redirect to the appropriate IdP login
- Keycloak realm/client configuration for SAML identity provider brokering
- Coexistence with the existing email/password login flow — non-SSO tenants are unaffected

## Out of Scope

- Changes to the existing email/password login flow for non-SSO tenants
- OIDC/OAuth-based SSO (only SAML is in scope per the request)
- User provisioning/deprovisioning via SCIM or other directory sync protocols
- Self-service SSO configuration UI for tenant admins (to be determined — may be admin-only or API-only initially)
- IdP-initiated SSO (to be determined — SP-initiated is the baseline, IdP-initiated may be added)
- Migration of existing users to SSO — whether existing email/password users on a newly SSO-enabled tenant are automatically linked or must re-register is an open question

## Constraints

- **Timeline**: Q3 delivery target (product team requirement)
- **Technical stack**: Must integrate with existing Keycloak instance, FastAPI backend, and React frontend
- **Backward compatibility**: Current email/password login flow must remain fully functional and unchanged for non-SSO tenants
- **Protocol**: SAML 2.0 specifically (not OIDC)
- **Multi-tenancy**: SSO configuration must be per-tenant, not system-wide

## Dependencies & Context

- **Keycloak**: Already in use for authentication — acts as the identity broker. Keycloak has built-in support for SAML identity provider brokering, which should simplify the integration.
- **`services/auth/` middleware**: Existing auth middleware in the FastAPI backend — this is the primary backend change target. Current structure/capabilities are not fully known.
- **Tenant model**: The application already supports multi-tenancy (implied by "per-tenant SSO config"). The tenant data model and how tenants are identified during login needs to be understood.
- **React frontend**: Existing login flow components will need to be extended to support SSO redirect for SSO-enabled tenants.
- **Enterprise client IdPs**: External SAML IdPs operated by enterprise clients — each will have their own metadata, certificates, and configuration.

## Knowns

- The application uses Keycloak for authentication today
- The backend is FastAPI with auth middleware in `services/auth/`
- The frontend is React
- SAML 2.0 is the required SSO protocol
- SSO configuration must be per-tenant
- The existing email/password flow must remain unchanged for non-SSO tenants
- The product team wants this delivered in Q3
- Enterprise client users are the target audience for SSO
- Keycloak natively supports SAML identity provider brokering

## Unknowns

- Current structure and capabilities of the `services/auth/` middleware — what does it handle today, how is it structured, what extension points exist?
- How tenants are identified during the login flow — is it by subdomain, by email domain, by explicit tenant selection, or something else?
- Whether Keycloak is configured with one realm per tenant or a single shared realm — this affects how SAML IdP connections are set up
- Whether SP-initiated SSO alone is sufficient or if IdP-initiated SSO is also required
- How existing users should be handled when a tenant enables SSO — account linking strategy
- Whether there are session management requirements specific to SSO (e.g., single logout / SLO)
- Whether a self-service SSO configuration UI is needed or if admin/API-based setup is acceptable for initial release
- What the SAML attribute mapping requirements are — which attributes from the IdP assertion need to be mapped to user profile fields
- Whether JIT (Just-In-Time) user provisioning is expected when an SSO user logs in for the first time

## Assumptions

- Keycloak will serve as the SAML SP (Service Provider) / identity broker, and the application will continue to authenticate against Keycloak — the application itself does not need to implement SAML protocol handling directly
- The existing tenant model already has a mechanism for storing tenant-level configuration, which can be extended to include SSO settings
- SP-initiated SSO is the primary flow needed (user starts at our login page, gets redirected to their IdP)
- Keycloak uses a single realm (or a small number of realms), and SAML IdP connections will be configured as identity providers within Keycloak
- The `services/auth/` middleware handles token validation from Keycloak and can be extended to handle SSO-related claims/attributes
- JIT provisioning of new SSO users is expected (create user on first SSO login rather than requiring pre-registration)
- Single Logout (SLO) is not required for the initial release
- Enterprise clients will provide their IdP metadata (XML) for configuration
