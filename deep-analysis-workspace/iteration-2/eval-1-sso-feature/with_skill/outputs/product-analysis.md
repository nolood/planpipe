# Product / Business Analysis

## Business Intent

Enterprise adoption of the platform is blocked because organizations cannot enforce their corporate authentication policies on the application. Enterprise IT departments require employees to authenticate through their centralized identity provider (via SAML) for security compliance, audit trail continuity, and credential lifecycle management. Without SSO, each enterprise deal requires employees to maintain separate credentials outside their corporate identity governance, which is a non-starter for security-conscious organizations.

The trigger is Q3 prioritization by the product team, strongly suggesting active enterprise pipeline pressure -- likely one or more deals are contingent on SSO support. The business value is revenue enablement (unblocking enterprise sales), retention (meeting enterprise customer expectations), and compliance (enabling customers to satisfy their own security policies through federated authentication).

This is a new capability being added to an existing authentication system, not a fix or improvement to something broken. The existing email/password flow works correctly; the gap is that it is the only authentication method available.

## Actor & Scenario

**Primary Actor:** Enterprise employee (end user at an organization whose tenant has SSO enabled)

**Main Scenario:**
1. **Trigger:** Employee navigates to the application login page and enters their corporate email address (e.g., jane@acmecorp.com)
2. **Tenant detection:** The system extracts the email domain, looks up the tenant via the `email_domain` field, and determines that this tenant has SSO enabled
3. **Redirect to IdP:** The system redirects the employee to Keycloak's SAML broker endpoint, which in turn redirects to the tenant's configured corporate identity provider (e.g., Okta, Azure AD, ADFS)
4. **Corporate authentication:** The employee authenticates at their corporate IdP using their existing corporate credentials (password, MFA, smart card -- whatever the IdP requires)
5. **SAML assertion return:** The IdP sends a SAML assertion back to Keycloak. Keycloak validates the assertion, maps SAML attributes to Keycloak user attributes, and issues a JWT
6. **Application callback:** The application receives the JWT from Keycloak via the authorization code flow callback. If this is the user's first SSO login, a local user record is created (JIT provisioning) with data from the SAML attributes
7. **End state:** The employee is authenticated and lands in the application with their correct tenant context and role. The experience is indistinguishable from a password-authenticated session from this point forward

**Secondary Actors:**
- **Tenant administrator (platform admin):** Configures SSO for their tenant -- provides IdP metadata XML, sets SSO as the authentication method, tests the connection. This actor's workflow is critical but is partially out of scope (no self-service UI in this iteration -- configuration may be API-driven or manual)
- **Enterprise IT administrator (external):** Configures the corporate IdP to trust the platform as a SAML SP. Needs SP metadata from the platform (entity ID, ACS URL, signing certificate). This actor never touches the platform directly
- **Keycloak (system process):** Acts as the SAML SP/broker -- receives SAML assertions, validates signatures, maps attributes, issues JWTs. Critical intermediary

**Secondary Scenarios:**
- **Non-SSO tenant login:** An employee at a tenant without SSO enabled enters their email and password. The system detects no SSO configuration and proceeds with the standard email/password direct grant flow. This scenario MUST remain unchanged
- **Admin configuring SSO for a tenant:** A platform admin enables SSO for a specific tenant by providing IdP metadata, specifying the email domain mapping, and activating the SSO configuration. The admin verifies the configuration works before enabling it for all tenant users
- **SSO user returning after session expiry:** The user's JWT expires, the refresh token exchange is attempted. If the refresh token is also expired, the user is redirected through the SSO flow again (seamless if their IdP session is still active)

## Expected Outcome

**What changes:**
- Enterprise tenants gain the ability to authenticate their employees through SAML SSO via their corporate identity provider
- The login flow becomes tenant-aware: the system detects whether a tenant uses SSO or email/password and routes accordingly
- New users at SSO-enabled tenants are automatically provisioned on first login (JIT provisioning)
- The tenant model gains per-tenant SSO configuration storage
- New API endpoints exist for SSO initiation and callback handling
- Keycloak configuration includes SAML IdP broker setup per tenant

**What stays the same:**
- Email/password login for non-SSO tenants is completely unchanged -- same API contract, same behavior, same error messages
- All protected routes continue to work identically (they validate JWTs regardless of how the JWT was obtained)
- User data model remains compatible -- SSO-provisioned users have the same fields as password-provisioned users
- Token format (JWT from Keycloak) remains the same for downstream consumers
- Admin and user APIs are unaffected

## Edge Cases

- **First SSO login (JIT provisioning):** A user authenticates via SSO for the first time. No local user record exists. The system must create a user record from SAML attributes (email, name) and associate it with the correct tenant. The existing `GetOrCreateByEmail` pattern in `user.Service` provides a precedent, but SSO users also need their `keycloak_id` populated from the brokered identity. If attribute mapping is incomplete (e.g., IdP doesn't send a display name), the system needs sensible defaults
- **Existing password user on newly SSO-enabled tenant:** A tenant enables SSO after users already exist with email/password accounts. When an existing user logs in via SSO for the first time, the system must link the SSO identity to the existing local user record rather than creating a duplicate. Account linking by email match is the most natural approach, but conflicts (email mismatch, multiple accounts) must be handled
- **SSO misconfiguration:** A tenant admin provides incorrect IdP metadata or the IdP is misconfigured. The user attempts SSO login and the SAML assertion fails validation at Keycloak. The system must surface a meaningful error rather than a generic 500, and the error path must not break the application for other tenants
- **IdP outage:** The tenant's corporate IdP is temporarily unavailable. SSO users at that tenant cannot authenticate. The system should fail gracefully with a clear error message. Whether fallback to email/password is allowed for SSO-enabled tenants is a product decision that needs clarification -- enterprise customers typically do NOT want password fallback as it defeats the purpose of enforced SSO
- **Email domain conflict:** Two tenants claim the same email domain, or an email domain is changed after SSO is configured. The current schema enforces `UNIQUE` on `email_domain`, which prevents the first case. Domain changes after SSO setup would break tenant detection and need careful handling
- **Concurrent SSO and password users in same tenant:** During a migration period, some users at a tenant may still use passwords while SSO is being rolled out. The system needs a clear policy: does enabling SSO for a tenant force all users to SSO, or can both methods coexist per-tenant?

## Success Signals

- **Enterprise deal closure rate:** The primary business metric. SSO was blocking enterprise adoption; its availability should directly correlate with enterprise deal progression. Lagging indicator -- measurable over quarters
- **SSO login success rate:** Percentage of SSO login attempts that complete successfully (user reaches authenticated state). Target: >95% after initial configuration. Leading indicator -- measurable from day one
- **Time-to-SSO-configuration:** How long it takes from a tenant admin starting SSO setup to the first successful SSO login. Lower is better. Leading indicator of admin experience quality
- **Support ticket volume for authentication issues:** Enterprise tenants should generate fewer credential-related support tickets after SSO is enabled, since users no longer manage separate passwords. Lagging indicator
- **SSO adoption rate:** Percentage of eligible tenant users who have successfully logged in via SSO within 30 days of SSO enablement for their tenant. Indicates whether the flow is actually working in practice

## Minimum Viable Outcome

The core that cannot be cut: **SP-initiated SAML SSO login for at least one enterprise tenant, with JIT user provisioning, coexisting with the unchanged email/password flow for non-SSO tenants.**

Specifically, the minimum viable outcome requires:
1. A per-tenant SSO configuration mechanism (even if API-only, no UI)
2. Tenant detection at login that routes SSO-enabled tenants to the SAML flow
3. Keycloak configured as SAML SP/broker for the tenant's IdP
4. SAML assertion handling that results in a valid JWT
5. JIT provisioning of new SSO users with correct tenant association
6. Email/password login unaffected for non-SSO tenants

What CAN be cut from the minimum viable outcome:
- Self-service SSO configuration UI (API or manual config is acceptable)
- IdP-initiated SSO
- Single Logout (SLO)
- Account linking for existing password users (can be manual/admin-driven initially)
- SCIM provisioning
- Multiple IdPs per tenant

## Critique Review

The critic found this analysis SUFFICIENT across all criteria. Business intent clearly articulates the enterprise adoption blocker beyond restating requirements. The main scenario walks through a concrete step-by-step SSO login flow with specific detail about each system interaction. Edge cases are task-specific (JIT provisioning, account linking, IdP outage, domain conflicts) rather than generic. Success signals are measurable with leading/lagging distinction. MVO is honest about what can be cut.

Minor observation from the critic: The admin configuration scenario (secondary actor) could benefit from more detail about the specific steps an admin takes to enable SSO, though this is partially mitigated by the fact that self-service admin UI is explicitly out of scope for this iteration.

## Open Questions

- Should password fallback be available for SSO-enabled tenants when the IdP is down, or should SSO enforcement be absolute? Enterprise customers typically want absolute enforcement, but this creates availability risk
- What is the account linking strategy for existing password users when their tenant enables SSO? Options: automatic link by email match, admin-driven linking, or users must re-register via SSO
- Can a tenant support both SSO and email/password simultaneously during a transition period, or is it an all-or-nothing switch?
- Who performs the initial SSO configuration -- platform super-admins via API, or tenant admins via a future UI? This affects the admin endpoint design
- What SAML attributes are required vs. optional from the IdP? At minimum email is needed, but name, role, and group membership mapping requirements are unclear
- Should there be a "test SSO" capability that allows an admin to verify the configuration before enabling it for all tenant users?
