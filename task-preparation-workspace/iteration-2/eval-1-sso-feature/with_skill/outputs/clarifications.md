# Clarifications Needed

> Task: Add SAML-based SSO support for enterprise tenants via Keycloak
> Verdict: READY_FOR_DEEP_ANALYSIS
> Open items: 0 blocking gaps, 12 unknowns, 3 assumptions to verify

## Blocking Gaps

None — verdict is READY_FOR_DEEP_ANALYSIS.

## Open Unknowns

1. **Keycloak version and deployment:** What version of Keycloak is deployed, and how is it hosted (self-hosted, managed, containerized)? This affects which SAML features are available and how configuration is managed.
2. **Keycloak SAML brokering state:** Does Keycloak currently have any SAML identity brokering configured, or is this entirely net-new? This determines how much Keycloak setup work is involved.
3. **Tenant configuration storage:** How is tenant configuration currently stored and managed — database table, config file, admin API? This determines where per-tenant SSO settings will live.
4. **Session handling mechanism:** How does the current auth middleware handle sessions — JWT tokens, server-side sessions, cookies, or a combination? This affects how SSO sessions integrate with the existing flow.
5. **Tenant admin UI existence:** Is there an existing admin UI for tenant management, or is tenant configuration managed through code/config/database directly?
6. **User provisioning strategy:** Should SSO users be auto-provisioned (JIT provisioning) on first SAML login, or must they be pre-created in the system before their first SSO login?
7. **SAML attribute mapping:** What SAML attributes should be mapped to user profile fields? At minimum: which attribute carries the email/username, display name, and any role/group information?
8. **Self-service IdP configuration:** Should tenant admins be able to configure their own SAML IdP settings through a UI, or is IdP configuration an internal/admin-only operation?
9. **Single Logout (SLO):** Is SAML Single Logout required for Q3, or only Single Sign-On?
10. **Dual authentication handling:** What should happen if a user already has an email/password account and then logs in via SSO from the same tenant — should accounts be linked, should SSO take over, or should they be treated as separate accounts?
11. **Existing authentication method concept:** Does the system currently have a concept of "authentication method" per user or per tenant, or would this be a new concept?
12. **Target enterprise clients for testing:** Are there specific enterprise clients already lined up for SSO, and which IdPs do they use (Okta, Azure AD, ADFS, etc.)? This affects testing and attribute mapping priorities.

## Assumptions to Verify

1. **Keycloak as SAML broker (app talks OIDC to Keycloak, Keycloak talks SAML to IdPs):** We are assuming the FastAPI backend never processes raw SAML assertions — Keycloak handles all SAML protocol details and the backend continues to interact with Keycloak via its existing integration (likely OIDC tokens). Is this correct, or does the team intend direct SAML processing in the backend?
2. **Multi-tenant architecture is extendable:** We are assuming the system already has a tenant model/table that can be extended with SSO configuration fields without major refactoring. Is this accurate? How is the tenant model currently structured?
3. **JIT provisioning is the expected pattern:** We are assuming enterprise users will not be pre-created — they will be provisioned automatically on their first SSO login. Is this the intended behavior, or should there be a separate user import/sync process?

## Questions for the User

1. Is the intended architecture that Keycloak handles all SAML protocol details (acting as identity broker), with the FastAPI backend only talking to Keycloak via OIDC/tokens — or should the backend process SAML assertions directly?
2. How is tenant configuration currently stored and managed — database table, config files, admin API, or something else?
3. What version of Keycloak is deployed, and does it currently have any SAML identity brokering configured?
4. How does the current auth middleware in `services/auth/` handle sessions — JWT tokens, server-side sessions, or another mechanism?
5. Should SSO users be auto-provisioned on first login (JIT), or must they exist in the system beforehand?
6. What should happen if a user already has an email/password account and then authenticates via SSO — account linking, SSO takeover, or separate accounts?
7. Is Single Logout (SLO) in scope for Q3, or only sign-on?
8. Should tenant admins be able to configure their own IdP settings (self-service), or is this an internal admin operation?
9. Is there an existing admin UI for tenant management that we would extend, or would SSO config management need a new interface?
10. Are there specific enterprise clients already lined up for testing, and which IdPs do they use?
11. What SAML attributes should map to user profile fields (email, name, roles/groups)?
12. Does the tenant model currently have extensible configuration storage, or would adding per-tenant SSO settings require schema changes?
