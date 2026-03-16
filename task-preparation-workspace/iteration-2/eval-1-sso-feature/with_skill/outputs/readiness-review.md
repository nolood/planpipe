# Readiness Review

## Verdict: READY_FOR_DEEP_ANALYSIS

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | PASS | The goal is unambiguous: add SAML SSO so enterprise tenant users can log in via their corporate IdP. "Done" means enterprise users on SSO-configured tenants can authenticate through SAML. |
| Problem clarity | PASS | The problem is well-articulated — enterprise clients need IdP-based authentication for security/compliance, and this blocks enterprise adoption. The "why" is clear. |
| Scope clarity | WEAK | The core scope (SAML via Keycloak, per-tenant config, backend/frontend changes) is defined, but several edges are blurry: whether SLO is included, whether self-service IdP config is in scope, whether JIT provisioning vs pre-creation is in scope, and what happens with dual-auth users. These are flagged as unknowns, which is honest, but they materially affect scope size. |
| Change target clarity | PASS | The change targets are clearly identified: Keycloak configuration, `services/auth/` middleware, the React login flow, and the tenant configuration model. Enough specificity to begin analysis. |
| Context sufficiency | WEAK | We know the tech stack and general architecture, but lack concrete details about the current auth middleware implementation, session handling mechanism, tenant config storage, and Keycloak deployment state. Analysis is possible but will require assumptions or exploration of the codebase. |
| Ambiguity level | PASS | No critical ambiguities remain. The unknowns are clearly surfaced and labeled. The core ask is unambiguous. The open questions (SLO, JIT provisioning, dual-auth) are important but do not make the fundamental analysis unreliable. |
| Assumption safety | WEAK | The assumption that Keycloak acts as SAML broker (application never touches raw SAML) is architecturally significant — if wrong, the entire integration approach changes. The assumption about existing multi-tenant architecture being extendable is also important. These are flagged, which is good, but they carry real risk if wrong. |
| Acceptance possibility | PASS | Success criteria are inferrable: an enterprise user on an SSO-configured tenant can authenticate via SAML through their corporate IdP, non-SSO tenants are unaffected, configuration is per-tenant. Verification is straightforward. |

## Summary
The task preparation is solid in its core: the goal, problem, and change targets are clear and well-articulated. The draft earns READY_FOR_DEEP_ANALYSIS because the foundation is strong enough for meaningful analysis to proceed. However, three areas scored WEAK — scope edges (SLO, self-service config, JIT provisioning), context gaps (auth middleware internals, session handling, tenant storage), and assumption risk (Keycloak-as-broker pattern, multi-tenant extendability). These WEAK scores don't block analysis but represent areas where the user's input would significantly improve confidence. The unknowns are honestly labeled, not hidden, which is a mark of good preparation.

## Unsafe Assumptions
- **Keycloak as SAML broker (app never handles raw SAML):** If the team intends direct SAML processing in FastAPI instead, the entire architecture changes. This must be confirmed.
- **Multi-tenant architecture is extendable:** If the tenant model is rigid or tenant config is baked into deployment (env vars, config files per instance), adding per-tenant SSO config may be harder than expected.
- **JIT user provisioning is needed:** If enterprise clients expect pre-provisioned users (e.g., via SCIM or manual import), the scope changes significantly.

## Acceptable Assumptions
- **SP-initiated SSO flow:** This is the standard enterprise pattern and a safe default.
- **Same session mechanism post-SSO:** Keycloak typically issues the same tokens regardless of upstream auth method, so this is architecturally sound.
- **Existing auth middleware can be extended:** Given that it already handles auth, adding an SSO path is a reasonable expectation for well-structured middleware.

## Recommended Clarification Questions
1. Does Keycloak currently have SAML identity brokering configured, or will this be a new Keycloak configuration? What version of Keycloak is deployed?
2. Is the intended architecture that Keycloak handles all SAML protocol details (acting as broker), with the FastAPI backend only talking to Keycloak via OIDC/tokens — or should the backend process SAML assertions directly?
3. How is tenant configuration currently stored and managed — database, config files, admin API?
4. Should SSO users be auto-provisioned on first login (JIT), or must they exist in the system before their first SSO login?
5. Is Single Logout (SLO) in scope for Q3, or only sign-on?
6. Should tenant admins be able to configure their own IdP settings (self-service), or is this an internal admin operation?
7. What should happen if a user already has an email/password account and then logs in via SSO — account linking, or separate accounts?
