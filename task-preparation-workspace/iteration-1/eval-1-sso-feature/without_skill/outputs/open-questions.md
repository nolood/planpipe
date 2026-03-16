# SSO via SAML - Open Questions

These questions should be resolved during Phase 0 (Discovery & Spike) before implementation begins.

## Architecture

1. **Keycloak realm strategy**: Is the current setup one realm per tenant, or a single shared realm? This determines how SAML IdPs are scoped.
2. **Keycloak version**: Which version of Keycloak is deployed? SAML brokering features vary across versions (especially pre-v17 Wildfly vs. Quarkus).
3. **Database**: What database and ORM does the FastAPI backend use? (Needed for migration planning.)

## Product / UX

4. **Tenant identification at login**: How should the system know which tenant the user belongs to before they authenticate? Options:
   - Subdomain-based (e.g., `acme.app.com`)
   - Email domain lookup (user enters email, system resolves tenant)
   - Explicit tenant selector on login page
   - Dedicated SSO login URL per tenant
5. **Dual auth mode**: Can a tenant have both SSO and email/password enabled simultaneously (e.g., for admin accounts that don't use the corporate IdP)?
6. **Auto-redirect vs. button**: Should SSO-enabled tenants auto-redirect to the IdP, or show a "Sign in with SSO" button alongside the password form?
7. **JIT provisioning scope**: Should JIT-provisioned users get a default role, or should roles be mapped from SAML attributes?

## Security

8. **Certificate rotation**: What is the expected process for IdP certificate rotation? Should the system support multiple active certificates during rotation periods?
9. **SLO requirement**: Is SAML Single Logout (SLO) a hard requirement, or is local-only logout acceptable for the initial release?
10. **Forced SSO**: Should there be an option to enforce SSO-only login for a tenant (disabling password auth entirely)?

## Operations

11. **Monitoring**: Are there existing dashboards/alerting systems where SSO metrics should be integrated?
12. **Onboarding process**: Who configures SSO for a new enterprise tenant -- our internal team, or the tenant admin self-service?
13. **Rollback plan**: If SSO is misconfigured and locks out a tenant's users, what is the recovery mechanism?
