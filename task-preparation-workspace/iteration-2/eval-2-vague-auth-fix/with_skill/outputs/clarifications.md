# Clarifications Needed

> Task: Fix broken behavior in the authentication system
> Verdict: NEEDS_CLARIFICATION
> Open items: 5 blocking gaps, 10 unknowns, 3 assumptions to verify

## Blocking Gaps

1. **Problem definition:** We do not know what is broken. Without understanding the symptoms (error messages, incorrect behavior, crashes, security issues), we cannot reason about the fix or even begin investigation.
2. **System identification:** We do not know which system, service, or codebase contains the authentication logic that needs to be fixed. Without this, we cannot identify the affected area or assess dependencies.
3. **Reproduction path:** There are no steps to reproduce the issue, no error messages, no logs, and no user reports. Without at least one of these, investigation would be blind guessing.
4. **Expected behavior:** There is no description of what correct authentication behavior looks like, making it impossible to define what "fixed" means or how to verify the fix.
5. **Scope boundaries:** We cannot determine whether this involves one broken endpoint, an entire auth flow, a token system, a third-party integration, or something else entirely.

## Open Unknowns

1. **Broken behavior:** What specific auth behavior is broken? (login fails, tokens expire too early, sessions lost, wrong credentials accepted, etc.)
2. **Manifestation:** How does the bug manifest for users? (error messages, blank screens, redirects, silent failures, security exposure)
3. **Codebase location:** Which codebase, service, or module contains the auth logic?
4. **Auth mechanism:** What auth mechanism is in use? (session cookies, JWT, OAuth 2.0, SSO, API keys, etc.)
5. **Layer affected:** Is this a frontend issue, backend issue, or infrastructure issue?
6. **Reproduction steps:** What steps reproduce the problem?
7. **Onset timing:** When did the problem start? (recent deploy, configuration change, gradual degradation)
8. **Severity:** What is the severity and urgency? (blocking all users vs. minor edge case)
9. **Success definition:** What does "working correctly" look like — the expected behavior?
10. **Auth vs authz:** Does "auth" mean authentication, authorization, or both?

## Assumptions to Verify

1. **Existing system:** We are assuming there is an existing authentication system that was previously working correctly. Is that accurate, or is this about building something new?
2. **Bug, not feature:** We are assuming this is a fix for broken existing functionality, not a request to add new auth features or redesign the auth system. Is that correct?
3. **Authentication, not authorization:** We are assuming "auth" refers primarily to authentication (verifying identity), not authorization (checking permissions). Does it involve both, or just one?

## Questions for the User

1. What specific behavior is broken? What do you see happening that should not be happening, or what is not happening that should be?
2. Which system, service, or codebase contains the authentication that needs fixing?
3. What authentication mechanism is in use (e.g., JWT tokens, session cookies, OAuth, SSO, username/password)?
4. Can you provide steps to reproduce the problem, or share any error messages or logs?
5. When did this start happening? Was there a recent change — a deploy, config update, or dependency upgrade?
6. Who is affected — all users, specific user groups, or specific environments (staging, production)?
7. How urgent is this — is it blocking production users right now?
8. When you say "auth," do you mean login/authentication, permissions/authorization, or both?
9. Is this about fixing existing auth that broke, or about building/redesigning authentication?
