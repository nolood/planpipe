# Requirements Draft

## Goal
Add SAML-based Single Sign-On (SSO) support so that enterprise tenant users can authenticate through their corporate identity provider instead of the standard email/password flow.

## Problem Statement
Enterprise clients need their employees to log in via their organization's identity provider (IdP) using SAML, rather than maintaining separate credentials in our system. This is a common enterprise requirement driven by security policy, compliance, and user experience — employees expect to use their corporate credentials for all work tools. Without SSO, enterprise adoption is blocked or friction-heavy, as IT departments cannot enforce their authentication policies (MFA, password rotation, session management) on our platform. The product team has targeted Q3 for delivery.

## Scope
- SAML 2.0 SP-initiated SSO flow integration with Keycloak as the service provider / identity broker
- Per-tenant SSO configuration: each tenant can have its own SAML IdP settings (metadata URL, entity ID, certificate, attribute mappings)
- Admin interface or configuration mechanism for setting up per-tenant SAML IdP connections
- Backend changes in `services/auth/` to handle SAML assertion processing, session creation, and user provisioning/matching
- Frontend changes to the login flow: detect tenant SSO configuration and redirect SSO-enabled users to SAML flow
- Preservation of the existing email/password login flow for non-SSO tenants (no regressions)

## Out of Scope
- Other SSO protocols (OIDC, OAuth2-only, LDAP) — SAML only for this task
- Changes to the existing email/password authentication flow
- Migration of existing users to SSO (unless clarified otherwise)
- SSO for internal/admin users (unless clarified otherwise)
- Self-service IdP configuration by tenant admins (unclear — may be in scope, needs clarification)
- Single Logout (SLO) support (unclear — needs clarification)
- SCIM provisioning / directory sync (separate concern)

## Constraints
- **Timeline:** Q3 target (product team requirement)
- **Technology:** Must use Keycloak as the SAML SP / identity broker — not a custom SAML implementation
- **Backend:** FastAPI — all new auth endpoints and middleware must be in Python/FastAPI
- **Frontend:** React — login flow changes must integrate with the existing React application
- **Existing auth:** Auth middleware lives in `services/auth/` — changes must extend, not replace, current auth infrastructure
- **Multi-tenancy:** SSO configuration must be per-tenant; the system already supports multi-tenancy (assumed)
- **Non-regression:** Current email/password login flow must remain unchanged for non-SSO tenants

## Dependencies & Context
- **Keycloak:** The existing Keycloak instance must support SAML identity brokering. Keycloak natively supports SAML 2.0, but the current deployment's configuration and version need to be confirmed.
- **`services/auth/` middleware:** The existing auth middleware will need to be extended. Its current architecture, session handling, and token format will influence the integration approach.
- **Tenant model:** The system's tenant model and how tenant configuration is currently stored will determine where SSO config lives.
- **Enterprise client IdPs:** Real enterprise IdPs (Okta, Azure AD, ADFS, etc.) will be the counterparties. Their SAML metadata and attribute schemas vary.

## Knowns
- The system uses Keycloak for authentication
- The backend is FastAPI with auth middleware in `services/auth/`
- The frontend is React
- SAML is the required SSO protocol
- SSO configuration must be per-tenant
- The existing email/password flow must not change for non-SSO users
- Q3 is the target delivery timeframe
- Enterprise client users are the target audience for SSO

## Unknowns
- What is the current Keycloak version and deployment model (self-hosted, managed, containerized)?
- Does Keycloak currently have SAML identity brokering configured, or is this net-new?
- How is tenant configuration currently stored and managed (database table, config file, admin API)?
- How does the current auth middleware handle sessions — JWT tokens, server-side sessions, or something else?
- Is there an admin UI for tenant management, or is tenant config managed through code/config?
- Should SSO users be auto-provisioned (JIT provisioning) on first SAML login, or must they be pre-created?
- What SAML attributes should be mapped to user profile fields (email, name, groups/roles)?
- Should tenant admins be able to self-service configure their IdP, or is this an internal/admin operation?
- Is Single Logout (SLO) required, or only Single Sign-On?
- Are there specific enterprise clients already lined up to test with, and which IdPs do they use?
- What happens if a user exists in both email/password and SSO — can they use both, or does SSO override?
- Does the system have an existing concept of "authentication method" per user or per tenant?

## Assumptions
- Keycloak will act as the SAML Service Provider (SP) / identity broker, meaning the application talks to Keycloak and Keycloak talks SAML to the external IdPs — the FastAPI backend does not process raw SAML assertions directly
- The system already has a multi-tenant architecture with a tenant model/table that can be extended with SSO configuration fields
- Keycloak supports the necessary SAML features in its current deployed version (2.x+ supports SAML brokering)
- JIT (Just-In-Time) user provisioning will be needed — enterprise users won't be pre-created in the system
- The login flow change is SP-initiated: user goes to our login page, tenant is detected, and user is redirected to their IdP
- Session handling after SSO login will use the same mechanism (tokens/sessions) as the current email/password flow
- The existing `services/auth/` middleware can be extended without major refactoring
