# Artifact Templates -- Execution Flow

Every artifact must follow its template exactly. Consistent structure makes execution auditable and status trackable.

---

## 1. `execution-status.md`

**When:** Created at Step 2. Updated after every state change throughout execution.

**Purpose:** Live tracking document. The single source of truth for what's happening.

```markdown
# Execution Status

> Task: [one-line summary]
> Total subtasks: [N]
> Started: [timestamp]
> Last updated: [timestamp]

## Current State

| ID | Title | Status | Wave | Review Cycles | Notes |
|----|-------|--------|------|---------------|-------|
| ST-1 | [title] | done/ready/pending/in_progress/in_review/rework/blocked | [N] | [0-N] | [brief note if any] |

## Status Summary

| Status | Count | Subtasks |
|--------|-------|----------|
| done | [N] | ST-1, ST-2 |
| ready | [N] | ST-5 |
| in_progress | [N] | ST-4 |
| in_review | [N] | — |
| rework | [N] | — |
| pending | [N] | ST-7, ST-8 |
| blocked | [N] | — |

## Active Wave: [N] — [Name]
[What this wave is doing, which subtasks are active]

## Recently Completed
- ST-1: [title] — completed [timestamp], unblocked: ST-4, ST-5
- ST-2: [title] — completed [timestamp], unblocked: ST-5, ST-6

## Blocked Items
[Any blocked subtasks with reasons and what would unblock them]
(or "None")

## Escalations
[Any issues escalated to the user with current status]
(or "None")
```

---

## 2. `execution-summary.md`

**When:** Created at Step 10 when all subtasks are done. This is the final output of the execution flow.

**Purpose:** Complete record of execution. What happened, how it went, what to watch for.

```markdown
# Execution Summary

> Task: [one-line summary]
> Total subtasks: [N]
> All completed: yes
> Total review cycles: [N across all subtasks]
> Total rework rounds: [N]
> Escalations: [N]
> Duration: [from first dispatch to last completion]

## Execution Overview
[2-3 sentences: how execution went, notable patterns, any themes from review feedback]

## Final Smoke Test
- **Build:** [pass / fail — command used, errors if any]
- **Tests:** [pass / fail — N passed, M failed, command used]
- **Linter:** [pass / fail / skipped — command used, issues if any]
- **Fix rounds:** [0 / N — what was fixed after smoke test]

## Subtask Results

| ID | Title | Review Cycles | Rework Rounds | Outcome |
|----|-------|---------------|---------------|---------|
| ST-1 | [title] | [N] | [N] | done |

## Acceptance Criteria Verification

| Criterion | Status | Verified By |
|-----------|--------|-------------|
| [criterion from agreed task model] | met / not met | ST-N review |

(If no agreed task model exists: "No formal acceptance criteria — verified through individual subtask completion criteria")

## Wave Execution Log

### Wave 1 — [Name]
- **Subtasks:** ST-1, ST-2, ST-3
- **Execution mode:** parallel / sequential
- **Duration:** [time]
- **Issues:** [any, or "none"]

### Wave 2 — [Name]
...

## Issues Encountered
- [Issue: what happened, how it was resolved]
(or "No issues encountered")

## Escalations
- [Escalation: what, why, how resolved]
(or "No escalations")

## Review Feedback Themes
[Patterns from review feedback — common issues, things that consistently passed/failed]
(or "No recurring themes")

## Follow-up Items
[Technical debt, deferred work, observations for future work]
(or "None")

## Review Quality Summary
- **First-pass approval rate:** [N%] of subtasks passed both reviews on first attempt
- **Most common review feedback:** [pattern if any]
- **Rework distribution:** [which subtasks needed rework — concentrated or spread]
```
