# Clarifications Needed

The task "fix the auth" cannot proceed to deep analysis. The following blocking gaps must be resolved first.

## Blocking Gaps

1. **What is broken**: No description of the actual symptom or failure. Is it a login failure? A token issue? A permissions error? Something else entirely?
2. **Where it is broken**: No identification of which service, module, endpoint, or component is involved.
3. **Reproduction path**: No steps to reproduce the issue, no error messages, no logs, no screenshots.
4. **Expected vs. actual behavior**: No description of what should happen versus what does happen.
5. **Affected users/flows**: No information about who encounters this problem or under what conditions.
6. **Severity and timeline**: Unknown whether this is a total outage, intermittent failure, or edge case. Unknown when it started.

## Clarification Questions

1. **What specifically is going wrong with auth?** What error do you see, or what behavior is incorrect? (e.g., "users can't log in," "tokens expire immediately," "admin users can access pages they shouldn't")
2. **Which part of the auth system is affected?** Login/signup, session management, permissions/authorization, integration with an external provider, or something else?
3. **Can you share any error messages, log output, or screenshots** that show the problem?
4. **When did this start happening?** Was it working before, and if so, did anything change recently (deployment, config change, dependency update)?
5. **Who is affected?** All users, specific roles, specific environments (production, staging, local)?
6. **How urgent is this?** Is it blocking users right now, or is it an intermittent/low-severity issue?

## Unsafe Assumptions (flagged for awareness)

- We are assuming a real, reproducible bug exists. If this is a configuration issue, user error, or environment-specific problem, the task framing may need to change.
- We are assuming "auth" refers to this project's authentication/authorization system. If it refers to a third-party provider's issue, scope and ownership change completely.
