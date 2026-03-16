# SSO via SAML - Task Breakdown

## Phase 0: Discovery & Spike (Week 1)

### T-0.1: Audit current Keycloak setup
- Determine realm strategy (single vs. multi-realm).
- Document current authentication flow end-to-end.
- Inventory existing `services/auth/` middleware capabilities.
- **Output**: Architecture decision record on realm strategy.

### T-0.2: Spike - Keycloak SAML Identity Brokering PoC
- Manually configure a SAML IdP in Keycloak (using a test IdP like samltest.id or a local SimpleSAMLphp).
- Verify login flow, JIT provisioning, and token issuance.
- Measure any latency overhead.
- **Output**: Validated PoC, documented configuration steps.

### T-0.3: Define tenant identification strategy
- Decide how the system determines which tenant (and thus which IdP) applies at login time.
- Options: subdomain-based, email-domain-based, explicit tenant selector.
- **Output**: Decision document.

---

## Phase 1: Backend - Data Model & SSO Config API (Weeks 2-3)

### T-1.1: Database migration - TenantSSOConfig table
- Create migration for `tenant_sso_config` table.
- Fields: tenant_id, enabled, idp_entity_id, idp_sso_url, idp_certificate, idp_metadata_xml, attribute_mapping (JSON), slo_enabled, slo_url, timestamps.
- Add indexes on tenant_id.

### T-1.2: SSO Config CRUD endpoints
- `POST /api/sso/config` - Create SSO configuration for a tenant.
- `GET /api/sso/config/{tenant_id}` - Retrieve configuration.
- `PUT /api/sso/config/{tenant_id}` - Update configuration.
- `DELETE /api/sso/config/{tenant_id}` - Remove/disable SSO.
- Input validation: certificate format, URL validation, required fields.
- Authorization: only tenant admins or super-admins.

### T-1.3: Keycloak Admin API integration
- Implement a Keycloak admin client service in FastAPI.
- Methods: create_identity_provider, update_identity_provider, delete_identity_provider, get_identity_provider.
- Sync TenantSSOConfig changes to Keycloak IdP configuration.
- Handle Keycloak API errors gracefully.

### T-1.4: SP metadata endpoint
- `GET /api/sso/metadata/{tenant_id}` - Generate/return SAML SP metadata XML.
- This is what enterprise clients give to their IdP admin to configure trust.

### T-1.5: Unit tests for config CRUD and Keycloak integration
- Mock Keycloak Admin API responses.
- Test validation, error cases, authorization.

---

## Phase 2: Backend - SSO Login Flow (Weeks 3-4)

### T-2.1: SSO login initiation endpoint
- `GET /api/auth/sso/login?tenant_id=...` (or inferred from subdomain/email domain).
- Look up tenant SSO config, verify enabled.
- Redirect user to Keycloak authorization endpoint with the correct IdP hint.

### T-2.2: Extend auth middleware for SSO context
- Add SSO-awareness to existing middleware in `services/auth/`.
- Token validation remains the same (Keycloak tokens), but add tenant SSO metadata to the auth context if applicable.
- Ensure non-SSO paths are completely unaffected (guard with feature flag or config check).

### T-2.3: JIT user provisioning hook
- After first SAML login, ensure user exists in the application database.
- Map SAML assertion attributes to user profile fields using tenant-specific attribute mapping.
- Handle edge case: user already exists (linked via email).

### T-2.4: SSO logout
- Implement local session termination for SSO users.
- Optionally trigger SAML SLO if configured for the tenant.
- Redirect to appropriate post-logout page.

### T-2.5: Error handling and logging
- Handle: IdP unavailable, invalid SAML response, expired certificate, clock skew.
- Structured logging for all SSO events with tenant_id context.
- Return user-friendly error pages/messages.

### T-2.6: Integration tests for SSO login flow
- Test with mock IdP (e.g., SimpleSAMLphp in Docker).
- Test happy path, error cases, JIT provisioning.

---

## Phase 3: Frontend (Weeks 4-5)

### T-3.1: Tenant SSO status check on login page
- Fetch tenant SSO status on login page load (new API call or extend existing tenant config endpoint).
- Cache result to avoid repeated calls.

### T-3.2: Conditional SSO login button
- If tenant has SSO enabled: show "Sign in with SSO" button.
- If SSO is the only auth method for the tenant, optionally auto-redirect.
- If SSO is not enabled: show existing email/password form unchanged.
- No visual or functional changes for non-SSO tenants.

### T-3.3: SSO redirect handling
- Handle redirect to IdP and return from IdP.
- Parse callback tokens, establish frontend session.
- Handle error states (IdP errors, network issues).

### T-3.4: SSO admin configuration UI
- New page/section in tenant admin settings.
- Form fields: enable/disable toggle, IdP metadata upload (file or paste), attribute mapping configuration.
- Display SP metadata download link.
- Validation and feedback on save.

### T-3.5: Frontend tests
- Unit tests for conditional login rendering.
- Integration tests for SSO admin panel.

---

## Phase 4: Testing & Hardening (Week 5-6)

### T-4.1: End-to-end test with mock IdP
- Docker-based test environment with SimpleSAMLphp or Keycloak-as-IdP.
- Full flow: configure SSO -> login -> JIT provision -> logout.

### T-4.2: Security review
- SAML assertion validation thoroughness.
- Certificate pinning and rotation testing.
- Replay attack prevention (one-time assertion use).
- CSRF protection on SSO endpoints.

### T-4.3: Multi-tenant isolation testing
- Verify tenant A's IdP cannot authenticate into tenant B.
- Verify non-SSO tenants are unaffected.

### T-4.4: Performance testing
- SSO login latency benchmarks.
- SSO config lookup performance under load.

### T-4.5: Documentation
- Admin guide: how to configure SSO for a tenant.
- Enterprise client guide: how to provide IdP metadata.
- Developer guide: architecture and troubleshooting.

---

## Estimated Effort

| Phase | Duration | Eng. Effort |
|-------|----------|-------------|
| Phase 0: Discovery & Spike | 1 week | 1 engineer |
| Phase 1: Config API | 2 weeks | 1-2 engineers |
| Phase 2: Login Flow | 2 weeks | 1-2 engineers |
| Phase 3: Frontend | 2 weeks | 1 engineer |
| Phase 4: Testing & Hardening | 1-2 weeks | 1-2 engineers |
| **Total** | **~6-8 weeks** | |

Note: Phases 1-3 can partially overlap. Phase 0 is a prerequisite for all others.
