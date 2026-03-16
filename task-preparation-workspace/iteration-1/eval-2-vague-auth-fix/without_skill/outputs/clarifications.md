# Clarification Questions

The task "fix the auth" cannot proceed to analysis without answers to the following questions. They are ordered by priority — the first few are the most critical blockers.

## Critical (must answer before any work can begin)

1. **What is broken?**
   What specific behavior are you seeing that is wrong? For example: "users get a 401 error when trying to log in," "sessions expire immediately," "OAuth callback fails with an error," "password reset tokens are invalid." A concrete symptom is needed to begin investigation.

2. **What should happen instead?**
   What is the expected correct behavior? What did this look like when it was working?

3. **Which auth system or flow is affected?**
   "Auth" covers a lot of ground. Which part is broken?
   - Login (email/password, OAuth, SSO, magic link)
   - Session management (tokens, cookies, expiration)
   - Authorization / permissions (who can access what)
   - Token refresh or renewal
   - Password reset
   - Multi-factor authentication
   - API authentication (keys, service-to-service)
   - Something else

## Important (needed for effective investigation)

4. **What is the tech stack?**
   What language, framework, and auth library or service is this built on? Where does the auth code live in the codebase?

5. **When did this start?**
   Was this working before? If so, when did it break, and did anything change around that time? (Deploy, config change, dependency upgrade, infrastructure change)

6. **How do you reproduce it?**
   What steps trigger the issue? Does it happen every time or intermittently? Is it specific to certain users, browsers, or devices?

7. **Are there error messages or logs?**
   Any error messages in the UI, server logs, or browser console that relate to this issue?

## Useful (helps prioritize and scope)

8. **How many users are affected and how urgent is this?**
   Is this blocking all users from logging in, or is it a partial/intermittent issue? Is there a workaround?
