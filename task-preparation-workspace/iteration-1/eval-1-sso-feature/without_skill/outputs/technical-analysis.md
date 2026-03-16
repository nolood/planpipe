# SSO via SAML - Technical Analysis

## 1. Architecture Overview

```
[User Browser]
    |
    v
[React Frontend] -- login request --> [FastAPI Backend]
    |                                       |
    |                                       v
    |                              [Auth Middleware (services/auth/)]
    |                                       |
    |                                       v
    |                              [Keycloak] -- SAML --> [Corporate IdP]
    |                                       |
    |<---- session/token ------------------+
```

The approach leverages Keycloak's built-in SAML Identity Brokering capability rather than implementing SAML directly in FastAPI. This is the recommended path because:
- Keycloak already handles token issuance, session management, and user federation.
- Keycloak has mature SAML SP support with metadata exchange, signature validation, and SLO.
- The FastAPI backend continues to validate Keycloak-issued tokens as it does today.

## 2. Component Breakdown

### 2.1 Keycloak Configuration Layer

**What changes:**
- Create a Keycloak Identity Provider (type: SAML) per tenant that has SSO enabled.
- Each IdP is scoped to the tenant's Keycloak realm (if using realm-per-tenant) or uses a tenant-specific IdP alias (if single-realm multi-tenant).
- Mappers on the IdP translate SAML assertions into Keycloak user attributes.

**Key decisions needed:**
- **Realm strategy**: One realm per tenant vs. single realm with tenant discrimination. Single realm is simpler operationally but requires careful IdP alias naming and first-broker-login flow customization.
- **Keycloak Admin API usage**: SSO config CRUD in the FastAPI backend will call Keycloak Admin REST API to create/update/delete IdP configurations.

### 2.2 Backend (FastAPI) - `services/auth/`

**New endpoints:**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/sso/config` | POST | Create SSO config for a tenant |
| `/api/sso/config/{tenant_id}` | GET | Retrieve SSO config for a tenant |
| `/api/sso/config/{tenant_id}` | PUT | Update SSO config |
| `/api/sso/config/{tenant_id}` | DELETE | Disable/remove SSO for a tenant |
| `/api/sso/metadata/{tenant_id}` | GET | Return SP metadata for tenant's IdP setup |
| `/api/auth/sso/login` | GET | Initiate SSO login (redirect to Keycloak, which redirects to IdP) |
| `/api/auth/sso/callback` | POST | Handle SAML callback (Keycloak handles this, but may need a relay) |

**Middleware changes in `services/auth/`:**
- The existing auth middleware must be extended (not modified) to detect SSO-enabled tenants during login routing.
- Token validation logic remains unchanged -- Keycloak still issues the tokens.

**New data model:**

```
TenantSSOConfig:
  - tenant_id: UUID (FK to tenant)
  - enabled: bool
  - idp_entity_id: str
  - idp_sso_url: str
  - idp_certificate: text
  - idp_metadata_xml: text (optional, alternative to individual fields)
  - attribute_mapping: JSON (maps SAML attributes to user fields)
  - slo_enabled: bool
  - slo_url: str (optional)
  - created_at: datetime
  - updated_at: datetime
```

### 2.3 Frontend (React)

**Login flow changes:**
1. Login page checks tenant SSO status (new API call or included in existing tenant config fetch).
2. If SSO is enabled for the tenant:
   - Show "Sign in with SSO" button (or auto-redirect, depending on UX decision).
   - Clicking it redirects to `/api/auth/sso/login?tenant_id=...`.
3. If SSO is not enabled, show the existing email/password form unchanged.

**New UI:**
- SSO administration panel (for tenant admins): form to upload IdP metadata, configure attribute mappings, enable/disable SSO.
- SSO status indicator in tenant settings.

### 2.4 User Provisioning (JIT)

- When a user authenticates via SAML for the first time and does not exist locally, Keycloak's first-broker-login flow handles account creation.
- A Keycloak event listener (or a post-login hook in FastAPI) ensures the user is also created in the application database with correct tenant association.
- Attribute mapping determines which SAML assertion fields populate user profile fields.

## 3. Risk & Unknowns

| Risk | Impact | Mitigation |
|------|--------|------------|
| Keycloak realm strategy unclear | High -- affects entire architecture | Spike: determine current realm setup, decide strategy before implementation |
| IdP certificate rotation | Medium -- expired certs break login | Support multiple certificates, implement monitoring/alerting |
| SAML clock skew | Low -- assertion validation failures | Configure reasonable clock tolerance in Keycloak |
| Tenant identification during login | Medium -- must know tenant before redirect | Use tenant-specific login URLs or email-domain-based discovery |
| JIT provisioning conflicts | Medium -- user may exist from previous non-SSO registration | Define merge/link strategy |

## 4. Dependencies

- Keycloak Admin REST API access from FastAPI backend.
- Network connectivity from Keycloak to enterprise IdPs (firewall rules, DNS).
- Enterprise clients providing their IdP metadata.
- Agreement on tenant identification strategy at login time.
