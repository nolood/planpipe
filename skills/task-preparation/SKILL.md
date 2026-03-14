---
name: task-preparation
description: "Stage 1 of the planning pipeline — normalizes raw task input into a validated, structured task statement ready for deep analysis. Use this skill whenever you receive a new task, ticket, issue, feature request, bug report, or any work item that needs to be understood and structured before planning or implementation begins. Also use when requirements seem unclear, when you need to validate whether a task is ready for deeper work, or when preparing input for any planning, analysis, or design process. Triggers on: new task, prepare task, normalize requirements, task intake, stage 1, prepare for planning, validate task, structure requirements, raw ticket, unclear scope, what needs to be done, break down this task, understand the task."
---

# Task Preparation — Stage 1

You are executing Stage 1 of the planning pipeline. Your job is to turn raw task input into a normalized, validated task statement that can safely move to deep analysis.

You do NOT design solutions, write code, or build plans here. You prepare the task — nothing more. The temptation to jump ahead is strong; resist it. A well-prepared task saves hours downstream. A poorly prepared one poisons everything that follows.

## Process

The stage runs in a loop: **prepare → critique → refine** — repeating until the task passes the readiness gate.

---

### Step 1: Intake Context

Gather all available starting context into a single picture before doing anything else.

Collect:
- **Original task statement** — the raw text exactly as received, preserved verbatim
- **Source and identifier** — where the task came from (ticket ID, issue URL, Slack thread, verbal request)
- **Related links** — documents, PRs, discussions, designs, API specs, wiki pages
- **Mentioned entities** — services, modules, APIs, domain objects, teams, people
- **Memobank context** — check if the project has a memobank, memory directory, or similar knowledge store. If it exists, search for similar past tasks, known patterns, prior decisions, and relevant context. If nothing is found or no memobank exists, move on — this is opportunistic, not required.

If the user provided links to external resources (tracker tickets, wiki pages, documents), fetch and read them now. Don't just note that they exist — actually pull the content.

---

### Step 2: Normalize Task Statement

Translate the raw input into a clear, workable form. Determine:

| Question | Answer to find |
|----------|---------------|
| **Core ask** | One sentence: what needs to happen |
| **Task type** | `feature` / `bug` / `refactor` / `integration` / `research` / `other` |
| **Actor** | Who or what triggers/needs this: end user, system, internal process, team |
| **Expected outcome** | What "done" looks like at a high level |
| **Affected area** | Which part of the system, process, or domain is involved |

Write the normalized statement as a short paragraph — something a new team member could read and understand in 30 seconds. Not a list, not a wall of text.

---

### Step 3: Extract Knowns / Unknowns / Assumptions

Split everything you have into three honest buckets.

**Knowns** — things you're confident about:
- The goal and why it matters
- Hard constraints (deadlines, compatibility, performance requirements)
- Which parts of the system are involved
- Available context, documentation, and dependencies

**Unknowns** — things that are genuinely unclear:
- Exact scope boundaries
- Expected behavior in edge cases
- Detailed acceptance criteria
- Impact on adjacent systems or processes

**Assumptions** — things you're treating as true but haven't verified:
- Which service or module owns this
- Whether an API contract can or can't change
- Whether existing behavior must be preserved
- Whether a specific pattern or approach should be reused

The distinction matters. If something feels like a known but you're actually guessing, it's an assumption. Being wrong about this classification is one of the most common ways preparation fails — you carry a guess into planning as if it were a fact, and the whole analysis is built on sand.

---

### Step 4: Draft Requirements

Create `requirements.draft.md` using the template from the **Artifact Templates** section below.

Don't aim for perfection — aim for clarity and honesty about what you know and what you don't. This document will evolve during clarification rounds.

---

### Step 5: Readiness Critique

Spawn a **Readiness Critic** subagent to independently evaluate whether the task preparation is good enough.

1. Read `agents/readiness-critic.md` from this skill's directory
2. Use the **Agent tool** to spawn a subagent with that prompt
3. Pass it the full requirements draft content and the normalized task statement
4. The subagent will evaluate 8 criteria and return a verdict

The critic is deliberately strict — that's the point. It catches weak preparation before it wastes time in later stages.

Save the critic's full response as `readiness-review.md`.

---

### Step 6: Close All Open Gaps

This stage's job is not just to *identify* gaps — it's to *close* them. Every unknown and every weak area must be resolved before the task moves forward. A task with open unknowns is not a prepared task, no matter what the critic says.

**If NEEDS_CLARIFICATION:**
- Build `clarifications.md` using the template below
- Present it to the user: blocking gaps first, then open unknowns, then assumptions to verify
- **Wait for the user's answers**
- Once answers arrive, go back to Step 2 and refine — don't start from scratch, update what you have
- Re-run the critique (Step 5) on the updated draft
- Repeat until the critic returns READY_FOR_DEEP_ANALYSIS *and* all gaps are closed

**If READY_FOR_DEEP_ANALYSIS but with WEAK scores or remaining unknowns:**
- Do NOT declare the stage complete yet
- Check the requirements draft for any items listed under Unknowns or Assumptions
- Check the readiness review for any WEAK criteria
- Convert each open unknown and each WEAK area into a specific clarification question
- Build `clarifications.md` using the template below
- Present it to the user and **wait for answers**
- Update the requirements draft with the answers, re-run critique if needed
- Only declare complete when unknowns are resolved and no WEAK scores remain

**If READY_FOR_DEEP_ANALYSIS with all PASS and no remaining unknowns:**
- Proceed to Step 7: Build Handoff

The clarification questions should be specific and actionable. "What's the scope?" is useless. "Does this change need to cover the mobile API endpoints, or only the web dashboard?" is useful.

**Convergence check:** If you're on the third clarification round, something is wrong — either the task is genuinely too vague to proceed at all, or the critic is being too strict for the available information. Flag this to the user and discuss whether to proceed with documented risks or abandon the task.

---

### Step 7: Build Handoff

Once all unknowns are resolved, all assumptions verified, and the critic returns READY_FOR_DEEP_ANALYSIS with no WEAK scores — build the handoff document.

`stage-1-handoff.md` is the **single entry point for Stage 2**. It is a clean, self-contained document that packages the final state of the task after all clarification rounds. Stage 2 should be able to read this file alone and have everything it needs to begin deep analysis.

Build it from the finalized requirements draft, incorporating all answers from clarification rounds. Do not include the iteration history — only the final resolved state.

Save `stage-1-handoff.md` and tell the user that Stage 1 is complete.

---

## Artifact Templates

This stage produces up to four files. **Every artifact must follow its template exactly.** These templates are not optional — they ensure consistency across tasks and enable Stage 2 to parse the output reliably.

### 1. `requirements.draft.md`

**When:** Always created. Updated after each clarification round.

```markdown
# Requirements Draft

## Goal
[One clear sentence — what needs to be achieved]

## Problem Statement
[What problem does this solve? Why does it matter? Who is affected?]

## Scope
[What's included in this task — be as specific as current knowledge allows]

## Out of Scope
[What's explicitly excluded — write "To be determined" if genuinely unclear at this stage]

## Constraints
[Hard limits: technical, business, time, compatibility, regulatory]

## Dependencies & Context
[What this task depends on or connects to — other tasks, services, teams, timelines]

## Knowns
- [Fact 1]
- [Fact 2]

## Unknowns
- [Unknown 1]
- [Unknown 2]

## Assumptions
- [Assumption 1]
- [Assumption 2]
```

---

### 2. `readiness-review.md`

**When:** Always created. Produced by the Readiness Critic subagent.

```markdown
# Readiness Review

## Verdict: [READY_FOR_DEEP_ANALYSIS | NEEDS_CLARIFICATION]

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Problem clarity | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Scope clarity | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Change target clarity | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Context sufficiency | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Ambiguity level | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Assumption safety | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Acceptance possibility | [PASS/WEAK/FAIL] | [1-2 sentences] |

## Summary
[2-3 sentences: why this verdict]

## Blocking Gaps
[Only if NEEDS_CLARIFICATION]
- [Gap: what's missing and why it matters]

## Unsafe Assumptions
[Only if any assumptions carry risk]
- [Assumption: why it's dangerous if wrong]

## Acceptable Assumptions
[Only if READY_FOR_DEEP_ANALYSIS]
- [Assumption: why it's safe to carry forward]

## Recommended Clarification Questions
[Only if NEEDS_CLARIFICATION]
1. [Specific, actionable question]
```

---

### 3. `clarifications.md`

**When:** Created whenever there are open unknowns, WEAK scores, or NEEDS_CLARIFICATION verdict. Updated after each clarification round until all gaps are closed.

```markdown
# Clarifications Needed

> Task: [one-line task summary]
> Verdict: [READY_FOR_DEEP_ANALYSIS | NEEDS_CLARIFICATION]
> Open items: [N blocking gaps, M unknowns, K assumptions to verify]

## Blocking Gaps

[Only if verdict is NEEDS_CLARIFICATION. Each gap explains what's missing and why it blocks progress.]

1. **[Gap name]:** [What is missing and why it matters for the next stage]
2. **[Gap name]:** [...]

## Open Unknowns

[Items from the Unknowns section of the requirements draft that need user input to resolve.]

1. **[Unknown]:** [Specific question to resolve it]
2. **[Unknown]:** [...]

## Assumptions to Verify

[Assumptions that carry risk if wrong. Ask the user to confirm or correct each one.]

1. **[Assumption]:** [Question to verify — e.g. "Is this correct, or does it work differently?"]
2. **[Assumption]:** [...]

## Questions for the User

[Consolidated, prioritized list of all questions. This is what the user should answer.]

1. [Most critical question]
2. [Next most critical]
3. [...]
```

This template is not optional. Every `clarifications.md` must follow this structure regardless of the verdict or the nature of the task.

---

### 4. `stage-1-handoff.md`

**When:** Created only when Stage 1 is fully complete — all unknowns resolved, all assumptions verified, critic returned READY_FOR_DEEP_ANALYSIS with all PASS scores. This is the **primary input for Stage 2**.

```markdown
# Stage 1 Handoff — Task Preparation Complete

## Task Summary
[Normalized task statement — 2-3 sentences that a new team member could read and immediately understand]

## Classification
- **Type:** [feature / bug / refactor / integration / research / other]
- **Actor:** [who triggers or needs this]
- **Affected area:** [system / process / domain area]
- **Source:** [ticket ID, issue URL, or "verbal request"]

## Goal
[One clear sentence]

## Problem Statement
[Why this matters — the business or user problem being solved]

## Scope
[Final, clarified scope — what's in]

## Out of Scope
[What's explicitly excluded]

## Constraints
[All hard limits — technical, business, time, compatibility]

## Dependencies & Context
[Everything this task connects to — services, teams, timelines, prior work]

## Verified Facts
[Everything confirmed as true — merged from original Knowns + resolved Unknowns + verified Assumptions]
- [Fact 1]
- [Fact 2]

## Accepted Risks
[Anything that remains uncertain but was explicitly accepted by the user as OK to proceed with]
- [Risk 1: what it is and why it was accepted]

## Acceptance Criteria
[How to know the task is done correctly — derived from the goal and clarification answers]
- [Criterion 1]
- [Criterion 2]

## Clarification History
[Brief summary of what was asked and resolved — not the full Q&A, just the key decisions]
- [Round 1: N questions asked, key decisions: ...]
- [Round 2: ...]
```

---

## Artifact Summary

| # | Artifact | When | Purpose |
|---|----------|------|---------|
| 1 | `requirements.draft.md` | Always | Working document — evolves during clarification rounds |
| 2 | `readiness-review.md` | Always | Quality gate — critic's evaluation and verdict |
| 3 | `clarifications.md` | When gaps exist | Questions for the user — drives the refinement loop |
| 4 | `stage-1-handoff.md` | On completion | **Primary input for Stage 2** — clean, final, self-contained |

Save all artifacts to the working directory (or a designated output path if the user specifies one).

---

## Done Criteria

Stage 1 is complete when **all** of these hold:
- Goal is clear and stated
- Problem is articulated with its "why"
- Scope has at least a rough boundary
- Affected system area is identified
- Facts are separated from assumptions
- All unknowns are resolved — not just surfaced, but actually answered by the user
- All assumptions are verified or explicitly accepted by the user
- Readiness critic returned **READY_FOR_DEEP_ANALYSIS** with no WEAK scores remaining
- `stage-1-handoff.md` has been created

## Failure Criteria

Stage 1 is NOT complete if **any** of these hold:
- Cannot determine what the output should be
- Cannot even roughly define scope
- Affected system area is unknown
- Critical parts of the task rest on unverified guesses
- Blocking ambiguities remain unresolved
- Readiness critic returned **NEEDS_CLARIFICATION**
- `stage-1-handoff.md` has not been created

---

## Notes

- **Preparation, not analysis.** This stage exists to make sure the right question is being asked, not to answer it. If you catch yourself evaluating technical approaches or designing solutions, stop — that's the next stage's job.
- **Read external resources.** If the user provides links (wiki, tracker, docs), actually fetch and read them. The context they contain often resolves unknowns that would otherwise become blocking gaps.
- **Memobank is optional.** Not every project has one. Check, use if available, skip if not. Don't ask the user about it.
- **The critic is an ally, not an enemy.** Its strictness protects the rest of the pipeline. When it flags something, it's usually right. When it's wrong, override it with explanation — don't just ignore it.
- **The handoff is the product.** Everything else is working material. `stage-1-handoff.md` is what Stage 2 actually reads. Make it clean, complete, and self-contained.
