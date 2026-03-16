# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | FAIL | "Fix the auth" does not define what "fixed" looks like. We cannot determine what the output or result should be because the broken behavior is unidentified. |
| Problem clarity | FAIL | There is no articulated problem beyond "auth is broken." No symptom, no error, no user impact, no reproduction path. The "why" is entirely missing. |
| Scope clarity | FAIL | Cannot even roughly determine what is included. "Auth" could mean login flows, token validation, session management, permissions, SSO integration, or any combination. No boundary is possible without knowing the actual issue. |
| Change target clarity | WEAK | We know the general area is "auth," but that is a broad domain, not a specific system, module, or component. It narrows the universe somewhat but not enough to direct analysis. |
| Context sufficiency | FAIL | Essentially zero context. No error messages, no logs, no reproduction steps, no affected users, no codebase to inspect, no linked tickets or discussions. The next stage would be guessing entirely. |
| Ambiguity level | FAIL | The task is almost entirely ambiguous. Every meaningful dimension — what is broken, where, how, for whom, since when — is unknown. These are not minor ambiguities; they are fundamental. |
| Assumption safety | WEAK | The assumptions listed (that a real bug exists, that "auth" refers to this project's system) are reasonable guesses but completely unverified. If the issue turns out to be a configuration problem or a misunderstanding, the entire framing is wrong. |
| Acceptance possibility | FAIL | There is no way to determine if the result is correct because we do not know what "correct auth behavior" means in this context, nor what is currently failing. |

## Summary
This task has 6 FAIL scores and 2 WEAK scores. The raw input "fix the auth" provides almost no actionable information. Every critical dimension — what is broken, where it is broken, how it manifests, who is affected, and what success looks like — is unknown. This cannot proceed to deep analysis in its current state; doing so would produce fictional plans based on guesses.

## Blocking Gaps
- **What is broken**: No description of the actual symptom or failure. Is it a login failure? A token issue? A permissions error? Something else entirely?
- **Where it is broken**: No identification of which service, module, endpoint, or component is involved.
- **Reproduction path**: No steps to reproduce the issue, no error messages, no logs, no screenshots.
- **Expected vs. actual behavior**: No description of what should happen versus what does happen.
- **Affected users/flows**: No information about who encounters this problem or under what conditions.
- **Severity and timeline**: Unknown whether this is a total outage, intermittent failure, or edge case. Unknown when it started.

## Unsafe Assumptions
- **"A real, reproducible bug exists"**: If this turns out to be a configuration issue, user error, or environment-specific problem, the entire task framing collapses.
- **"Auth refers to this project's auth system"**: If the user means something else (e.g., a third-party auth provider's issue), the scope and ownership change completely.

## Recommended Clarification Questions
1. What specifically is going wrong with auth? What error do you see, or what behavior is incorrect? (e.g., "users can't log in," "tokens expire immediately," "admin users can access pages they shouldn't")
2. Which part of the auth system is affected — login/signup, session management, permissions/authorization, integration with an external provider, or something else?
3. Can you share any error messages, log output, or screenshots that show the problem?
4. When did this start happening? Was it working before, and if so, did anything change recently (deployment, config change, dependency update)?
5. Who is affected — all users, specific roles, specific environments (production, staging, local)?
6. How urgent is this — is it blocking users right now, or is it an intermittent/low-severity issue?
