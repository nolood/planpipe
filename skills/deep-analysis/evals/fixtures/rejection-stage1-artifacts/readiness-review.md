# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | FAIL | "Fix the authentication" — cannot determine what "done" means. No specific bug, behavior, or outcome described. |
| Problem clarity | FAIL | No problem statement exists. "Something is wrong" with no specifics. |
| Scope clarity | FAIL | "Auth-related changes" gives no boundary. Could be a typo fix or a complete rewrite. |
| Change target clarity | WEAK | General area (auth) is known but nothing more specific. |
| Context sufficiency | FAIL | Almost no context provided. Would be guessing about everything in the next stage. |
| Ambiguity level | FAIL | Everything is ambiguous. Every aspect of this task is unclear. |
| Assumption safety | WEAK | The only assumption ("auth system exists") is trivially true but useless. |
| Acceptance possibility | FAIL | No way to determine if a fix is correct without knowing what's broken. |

## Summary

This task is not ready for deep analysis. The description "fix the auth" provides almost no information to work with. Five FAIL scores across core criteria. Analysis would be pure guesswork.

## Blocking Gaps

- No description of what is broken or what symptoms exist
- No specific error, behavior, or user report
- No scope boundary — "fix auth" could mean anything
- No way to verify success

## Recommended Clarification Questions

1. What specific behavior is broken? (error message, incorrect behavior, security issue, performance problem?)
2. Who reported this and what did they observe?
3. Which part of the auth system is affected? (login, token validation, permissions, session management?)
4. How should we verify the fix works?
