# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | FAIL | "Fix the auth" does not define what "done" means. We cannot determine what the output or result should be because we don't know what is broken. |
| Problem clarity | FAIL | There is no articulated problem beyond "auth is broken." The nature of the breakage, its symptoms, and its impact are completely unknown. |
| Scope clarity | FAIL | There are no boundaries at all — we don't know which part of the auth system, which service, which codebase, or even whether this is frontend or backend. Cannot roughly determine what's included. |
| Change target clarity | FAIL | We know the general domain is "auth" but have no idea which specific system, module, service, or codebase needs to change. |
| Context sufficiency | FAIL | There is essentially zero context. No error messages, no reproduction steps, no system description, no tech stack information. The next stage would be guessing entirely. |
| Ambiguity level | FAIL | The entire task is one critical ambiguity. Every important dimension — what, where, how, why, when — is undefined. |
| Assumption safety | WEAK | The assumptions made (existing system, bug not feature, auth not authz) are reasonable defaults, but without any context they could all be wrong. The assumption that this is a bug rather than a feature request or redesign carries moderate risk. |
| Acceptance possibility | FAIL | With no definition of the problem, there is no way to determine if any fix is correct. We cannot describe how to verify the task was done. |

## Summary
This task fails 7 of 8 criteria. The raw input "fix the auth" provides essentially no actionable information — it identifies a domain area ("auth") and a desired action ("fix") but nothing else. The next stage cannot do any meaningful work with this level of preparation. This is a textbook case of a task that needs significant clarification before it can proceed.

## Blocking Gaps
- **Problem definition:** We do not know what is broken. Without understanding the symptoms, we cannot even begin to reason about the fix.
- **System identification:** We do not know which system, service, or codebase contains the authentication logic that needs to be fixed.
- **Reproduction path:** No steps to reproduce the issue, no error messages, no logs, no user reports — nothing to guide investigation.
- **Expected behavior:** No description of what correct authentication behavior looks like, making it impossible to define "done."
- **Scope boundaries:** Cannot determine whether this involves one broken endpoint, an entire auth flow, a token system, a third-party integration, or something else entirely.

## Unsafe Assumptions
- **"This is a bug fix, not a feature or redesign":** If the user actually wants to rebuild or replace the auth system, treating this as a simple fix would lead to completely wrong scoping.
- **"Auth means authentication":** If the user means authorization (permissions, roles, access control), the affected system and approach could be entirely different.

## Recommended Clarification Questions
1. What specific behavior is broken? What do you see happening that shouldn't be happening (or not happening that should be)?
2. Which system, service, or codebase contains the authentication that needs fixing?
3. What auth mechanism is in use (e.g., JWT, session cookies, OAuth, SSO)?
4. When did this start happening? Was there a recent change (deploy, config update, dependency upgrade)?
5. Can you provide steps to reproduce the problem, or any error messages/logs?
6. Who is affected — all users, specific user groups, specific environments?
7. How urgent is this — is it blocking production users right now?
8. When you say "auth," do you mean login/authentication, permissions/authorization, or both?
