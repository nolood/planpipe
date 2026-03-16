# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | FAIL | "Fix the auth" does not specify what is broken or what a successful fix looks like. It is impossible to determine what "done" means. |
| Problem clarity | FAIL | No problem statement was provided. We do not know what symptom, error, or behavior is wrong, nor why it matters or who is affected. |
| Scope clarity | FAIL | "Auth" could encompass login, sessions, tokens, permissions, MFA, password reset, API auth, or SSO. Without knowing what is broken, the scope is completely undefined. |
| Change target clarity | FAIL | No information about which system, service, module, or code area is involved. We do not even know the tech stack. |
| Context sufficiency | FAIL | Zero context was provided — no tech stack, no architecture, no error messages, no logs, no reproduction steps, no environment details. The next stage would be working entirely blind. |
| Ambiguity level | FAIL | The entire task is one critical ambiguity. Every aspect — what, where, why, how to verify — is unknown. |
| Assumption safety | WEAK | The few assumptions we can make (that auth worked before, that this is authentication not authorization) are reasonable guesses but completely unverified. If wrong, any investigation would be misdirected. |
| Acceptance possibility | FAIL | With no definition of the problem, there is no way to determine whether a fix is correct. We cannot describe success criteria. |

## Summary
This task fails on 7 of 8 criteria. The input "fix the auth" contains no actionable information beyond a general subject area. There is no way to begin meaningful analysis — any work done in the next stage would be pure guesswork. This is a textbook case of a task that needs substantial clarification before it can proceed.

## Blocking Gaps
- **What is broken**: No description of the symptom, error, or incorrect behavior. This is the most fundamental gap — without it, nothing else can be determined.
- **Expected behavior**: No description of what "working auth" should look like for this specific issue.
- **Affected system area**: No information about which part of the auth system is involved or which codebase/service to look at.
- **Tech stack and architecture**: No information about what technologies are in use, making it impossible to even suggest investigation approaches.
- **Environment and reproduction**: No information about where the issue occurs or how to reproduce it.
- **Severity and urgency**: No information about how many users are affected or how critical this is.

## Unsafe Assumptions
- Assuming "auth" means authentication rather than authorization could send investigation in the wrong direction entirely.
- Assuming this is a regression (something that used to work) rather than a missing feature or a design flaw would change the approach.

## Recommended Clarification Questions
1. What specific behavior are you seeing that is wrong? (e.g., "users get a 401 when logging in," "sessions expire after 5 minutes," "password reset emails never arrive")
2. What is the expected/correct behavior? What should happen instead of what is happening now?
3. Which auth system or flow is affected? (e.g., login, session management, token refresh, permissions, SSO, API authentication)
4. What is the tech stack? (language, framework, auth library or service — e.g., "FastAPI backend with JWT tokens" or "Next.js with NextAuth")
5. When did this start happening? Was it working before, and if so, what changed? (recent deploy, config change, dependency update)
6. What environment does this occur in? (production, staging, specific browsers or devices)
7. Are there error messages, logs, or stack traces you can share?
8. How many users are affected, and how urgent is this? (e.g., "all users are locked out" vs. "occasional login failures for some users")
