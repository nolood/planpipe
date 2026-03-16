# Product / Business Analysis

## Business Intent

Enterprise adoption of the platform is blocked because organizations cannot enforce their corporate authentication policies. IT departments at enterprise clients require employees to authenticate through their organization's identity provider (IdP) -- this is typically a non-negotiable security and compliance requirement for enterprise procurement. Without SAML SSO, every enterprise prospect hits the same wall: their security team will not approve a tool that requires separate credentials outside their IdP.

The trigger is clear: the product team has prioritized this for Q3, which signals existing enterprise pipeline deals are stalled or at risk. This is revenue enablement -- not a nice-to-have feature, but a gate on a customer segment. The business value is unlocking the enterprise segment entirely: deals that are currently blocked can close, and the platform can move upmarket.

This is a new capability layered onto the existing authentication system, not a fix or improvement to something broken. The current email/password flow works fine for non-enterprise tenants and must remain untouched.

## Actor & Scenario

**Primary Actor:** Enterprise employee (end user at an organization whose IT department has configured SAML SSO)

**Main Scenario:**
1. Employee navigates to the platform login page
2. Employee enters their corporate email address (e.g., jane@bigcorp.com)
3. The system detects that bigcorp.com is associated with a tenant that has SSO enabled
4. The system redirects the employee to their corporate IdP login page (e.g., Okta, Azure AD, ADFS)
5. Employee authenticates using their corporate credentials (which may include MFA enforced by their organization)
6. The IdP sends a SAML assertion back to Keycloak (acting as the service provider / broker)
7. Keycloak validates the assertion, creates or maps a session, and issues tokens to the platform
8. The platform receives the tokens, resolves the user's tenant, and creates/updates the local user record if needed (JIT provisioning)
9. Employee lands on their dashboard, fully authenticated, with the correct tenant context

**Secondary Actors:**

- **Tenant administrator (IT admin at the enterprise):** Provides IdP metadata XML, works with platform support to configure the SAML connection for their tenant. In future iterations, may use a self-service UI.
- **Platform operations team:** Configures Keycloak realm/client settings and SAML identity provider brokering. Handles initial tenant SSO setup until self-service is available.
- **Non-SSO users at the same tenant (during transition):** If a tenant enables SSO but has existing users on email/password, their experience must be considered -- can they still log in with password, or are they forced to SSO?

**Secondary Scenarios:**

- **First-time SSO user (JIT provisioning):** User authenticates via IdP for the first time. No local user record exists. The system creates one automatically, mapping the Keycloak/SAML identity to a new local user in the correct tenant.
- **Returning SSO user:** User authenticates via IdP. Local user record already exists. The system updates last_login and proceeds normally.
- **Non-SSO tenant user:** Logs in with email/password as before. Completely unaffected by the SSO feature.
- **Tenant admin requesting SSO setup:** Admin contacts platform support, provides IdP metadata. Ops team configures Keycloak and enables SSO flag on the tenant.

## Expected Outcome

**What changes:**
- Enterprise tenants can enable SAML SSO, allowing their employees to authenticate through their corporate IdP
- The login flow gains an SSO detection step: when a user enters an email from an SSO-enabled tenant, they are redirected to their IdP instead of seeing a password prompt
- New users from SSO-enabled tenants are automatically provisioned on first login
- The platform can close enterprise deals that were previously blocked by authentication requirements

**What stays the same:**
- Non-SSO tenants see zero changes to their login experience
- The email/password authentication flow continues to work for all non-SSO tenants
- Existing user data, sessions, and tenant structures are unaffected
- The API contract for protected routes (Bearer token in Authorization header) does not change -- downstream of authentication, everything works the same regardless of how the user originally authenticated

**How users know it worked:**
- Enterprise employees click one button or enter their email and are redirected to their familiar corporate login page
- After authenticating with their corporate credentials, they land in the platform with the correct tenant and permissions
- IT admins see their employees using the platform without maintaining separate credentials

## Edge Cases

- **Email domain collision:** A user's email domain matches an SSO-enabled tenant, but the user is not actually part of that organization (e.g., personal email on a domain that was later claimed by an enterprise). This matters because the current system uses email_domain as the sole tenant resolution mechanism -- there is no secondary verification.

- **SSO enablement for a tenant with existing password users:** When a tenant enables SSO, existing users who previously logged in with email/password need a transition path. Do they keep password access? Are they forced to SSO on next login? Is there an account linking step where their existing local account is connected to their IdP identity? This is explicitly called out as an unknown in the requirements and is the highest-risk product edge case.

- **IdP unavailability during login:** The enterprise's IdP is down or unreachable. The user cannot authenticate. Unlike password login (which depends only on Keycloak), SSO login depends on an external system outside the platform's control. The user experience during IdP outages needs consideration -- at minimum, a clear error message rather than a generic failure.

- **User exists in IdP but not authorized for this application:** The IdP successfully authenticates the user, but the organization has not granted them access to this specific application. The SAML assertion arrives, but the user should not be provisioned. Whether this is handled by IdP-side application assignment or platform-side checks is an open question.

- **Multiple email domains per tenant:** The current schema has a single email_domain per tenant. Enterprise organizations frequently have multiple email domains (e.g., bigcorp.com, bigcorp.co.uk, acquired-company.com). If SSO is tied to email domain detection, this limitation would need to be addressed.

- **Tenant plan gating:** SSO is typically an enterprise-tier feature. The tenant model has a `plan` field ("free", "pro", "enterprise"). SSO enablement should likely be restricted to enterprise-plan tenants. This gating logic does not exist today.

## Success Signals

- **Enterprise deal conversion rate:** The primary lagging indicator. Deals that were previously blocked by "no SSO" objection should start closing. Measurable by sales pipeline data.

- **SSO login success rate:** Leading indicator. Of SSO login attempts (redirects to IdP), what percentage complete successfully and land the user in the platform? Target should be >95%. Failures indicate configuration or integration problems.

- **Time-to-first-login for SSO users:** How long from IdP redirect to landing on the dashboard? Should be comparable to password login (under 5 seconds for the platform portion, excluding time spent on the IdP's own login page).

- **Zero regression in password login metrics:** Existing email/password login success rate, latency, and error rate should not change after SSO is deployed. Any degradation indicates the SSO changes are affecting the existing flow.

- **Support ticket volume for SSO setup:** How many tickets does it take to get a tenant's SSO configured and working? Lower is better. High volume suggests the setup process is too complex or error-prone.

## Minimum Viable Outcome

The smallest result that still delivers the stated business value:

1. SP-initiated SAML SSO flow works end-to-end for at least one enterprise tenant
2. SSO is configured per-tenant (not system-wide) -- can be enabled for specific tenants while others remain on password login
3. JIT user provisioning creates local user records on first SSO login
4. The existing email/password login flow is completely unaffected for non-SSO tenants
5. Keycloak acts as the SAML broker -- the Go backend does not implement SAML protocol directly

What can be deferred from the MVO:
- IdP-initiated SSO
- Single Logout (SLO)
- Self-service SSO configuration UI for tenant admins
- Account linking for existing password users (can be handled manually or deferred)
- SCIM provisioning/deprovisioning
- Multiple email domains per tenant

## Self-Critique Notes

- **Account linking remains the biggest product gap.** The requirements call it an unknown, and the analysis cannot resolve it without a product decision. The MVO defers it, but this means the first enterprise tenant to enable SSO will have existing users who need manual intervention or will lose access to their old accounts. This needs a product decision before implementation begins.

- **The "multiple email domains" edge case could be a blocker.** The assumption is that enterprises have one domain, but in practice many have several. The current schema (single email_domain column on tenants) may not support real enterprise deployments without modification. This is not just a technical concern -- it affects whether the feature actually works for the target customers.

- **Plan gating is assumed but not specified.** The analysis assumes SSO is enterprise-tier only, based on the plan field existing in the tenant model. But the requirements don't explicitly state this. If SSO should be available to all plans, the product economics change.

- **The IT admin experience is underspecified.** The MVO defers self-service UI, meaning SSO setup requires platform ops involvement. For the first few enterprise clients this is acceptable, but it does not scale. The product analysis cannot fully evaluate the admin experience without knowing the planned setup workflow.

- **Behavioral assumption about email-first login flow:** The main scenario assumes users enter their email first and the system detects SSO. This is a common pattern (used by Google, Microsoft, Slack) but the requirements don't explicitly specify this UX. An alternative is a "Sign in with SSO" button. This is a product decision that affects the technical implementation.
