---
name: task-preparation
description: "Stage 1 of the planning pipeline — normalizes raw task input into a validated, structured task statement ready for deep analysis. Use this skill whenever you receive a new task, ticket, issue, feature request, bug report, or any work item that needs to be understood and structured before planning or implementation begins. Also use when requirements seem unclear, when you need to validate whether a task is ready for deeper work, or when preparing input for any planning, analysis, or design process. Triggers on: new task, prepare task, normalize requirements, task intake, stage 1, prepare for planning, validate task, structure requirements, raw ticket, unclear scope, what needs to be done, break down this task, understand the task."
---

# Task Preparation — Stage 1

You are executing Stage 1 of the planning pipeline. Your job is to turn raw task input into a normalized, validated task statement that can safely move to deep analysis.

You do NOT design solutions, write code, or build plans here. You prepare the task — nothing more. The temptation to jump ahead is strong; resist it. A well-prepared task saves hours downstream. A poorly prepared one poisons everything that follows.

## Output Directory Convention

All pipeline artifacts are stored in `.planpipe/{task-id}/stage-N/` relative to the project root.

**Task ID resolution (Stage 1 determines this):**
1. If the user provides a ticket/issue ID (e.g., `CP-269`, `PROJ-123`, `#42`) → use it as-is
2. If no ID provided → generate from project directory name + sequential number:
   - Get the project directory name (e.g., `my-api`)
   - Check `.planpipe/` for existing task directories matching this project name
   - Assign the next number: `my-api-001`, `my-api-002`, etc.
   - If `.planpipe/` doesn't exist yet, start with `001`

**Directory structure:**
```
.planpipe/
├── {task-id}/
│   ├── stage-1/   # Task Preparation
│   ├── stage-2/   # Deep Analysis
│   ├── stage-3/   # Task Synthesis
│   ├── stage-4/   # Solution Design
│   ├── stage-5/   # Implementation Decomposition
│   └── stage-6/   # Execution Flow
```

Each stage creates its own `stage-N/` directory and saves all artifacts there. The task ID and output path are passed forward through handoff documents and continuation prompts.

---

## Process

The stage runs in a loop: **prepare → critique → refine** — repeating until the task passes the readiness gate.

---

### Step 1: Intake Context and Initialize Output

Gather all available starting context into a single picture before doing anything else.

**Determine the task ID** using the resolution rules above. Create the output directory: `.planpipe/{task-id}/stage-1/`.

Collect:
- **Original task statement** — the raw text exactly as received, preserved verbatim
- **Source and identifier** — where the task came from (ticket ID, issue URL, Slack thread, verbal request). Use the identifier as task ID if it exists.
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

### Step 3: Confirm Understanding with the User

Before going deeper, present your normalized understanding to the user and ask for confirmation. This is the cheapest place to catch misunderstanding — before any analysis work begins.

Show the user:
- Your normalized task statement (from Step 2)
- Task type and affected area
- Expected outcome as you understand it

Ask: "Правильно ли я понял задачу? Если нет — поправь, и я обновлю понимание."

**Wait for the user's response.** Do NOT proceed until the user confirms or corrects.

If the user corrects your understanding — update the normalized statement and present it again. Repeat until confirmed.

---

### Step 4: Verify Task Against Reality

After the user confirms their understanding, verify their claims against the actual system. This step has two phases: first explore the context deeply, then run multiple verifiers in parallel to catch errors from different angles.

Users make mistakes — wrong field names, confused terminology, misremembered processes. One verifier might miss what another catches. So we invest in thorough verification here to avoid poisoning the entire pipeline downstream.

**Do NOT skip this step.** A task built on wrong assumptions wastes everyone's time in later stages.

#### Phase 1: Explore Task Context

Spawn **3 Context Scout** subagents **in parallel** to deeply map the system area relevant to the task from different angles. One scout can miss an entire layer — three covering different aspects build a much more complete "ground truth."

1. Read `agents/context-scout.md` — this file contains the scout's complete role, exploration procedure, and output format
2. Spawn 3 scouts in parallel using the **Agent tool**, each with:
   - `name`: `"context-scout-N"` (e.g. `"context-scout-1"`, `"context-scout-2"`, `"context-scout-3"`)
   - `subagent_type`: `"Explore"`
   - `prompt`: the FULL content of `agents/context-scout.md` + the input data below + the scout's assigned focus area — the agent definition file IS the prompt, do not summarize or skip it

**Assign each scout a focus area** (append to the prompt):

| Scout | Focus | What to map |
|-------|-------|-------------|
| `context-scout-1` | **Data & Entities** | Models, database schemas, table structures, field names and types, entity relationships, data migrations — everything about how data is stored and structured in the area relevant to the task |
| `context-scout-2` | **Logic & Processes** | Business rules, workflow implementations, event handlers, calculations, data flows, API endpoints, service methods — how the system actually behaves in the area relevant to the task |
| `context-scout-3` | **UI & Presentation** | Report templates, dashboard components, form definitions, column labels, display logic, configs, feature flags, role-based variations — what the user actually sees and how it maps to the underlying data |

3. Input data for each scout:
   - The normalized task statement (confirmed by the user)
   - All original context (ticket content, wiki pages, linked documents)
   - Specific modules, entities, fields, and processes mentioned in the task
   - The scout's assigned focus area

When all scouts return, merge their findings into a single `context-map.md`. Deduplicate overlapping findings but preserve all unique observations.

#### Phase 2: Parallel Verification

Once all context scouts return and the merged `context-map.md` is ready, spawn **3-4 Task Verifier** subagents **in parallel**. Each verifier gets the full merged context map but focuses on a different verification angle — this maximizes coverage and catches errors that a single verifier would miss.

1. Read `agents/task-verifier.md` — this file contains the verifier's complete role, verification procedure, claim extraction method, and output format
2. Spawn 3-4 verifiers in parallel using the **Agent tool**, each with:
   - `name`: `"task-verifier-N"` (e.g. `"task-verifier-1"`, `"task-verifier-2"`, etc.)
   - `subagent_type`: `"Explore"`
   - `prompt`: the FULL content of `agents/task-verifier.md` + the merged `context-map.md` + the input data below — the agent definition file IS the prompt, do not summarize or skip it

**Assign each verifier a focus area** (append to the prompt):

| Verifier | Focus | What to check |
|----------|-------|---------------|
| `task-verifier-1` | **Data & Structure** | Entity names, field/column names, data types, database schemas, table structures — does what the user named actually exist and work as described? |
| `task-verifier-2` | **Process & Logic** | Workflows, business rules, data flows, calculations, event chains — does the process actually work the way the user described? |
| `task-verifier-3` | **Terminology & Naming** | Term consistency, naming mismatches between business language and code, cases where the same word means different things, cases where the user's term maps to something else in the system |
| `task-verifier-4` | **Asymmetries & Edge Cases** | Cases where the user assumes uniformity but the system differs (e.g., "all departments show deals" but some show applications), missing distinctions, hidden variations |

If the task is simple enough that 4 verifiers would be redundant (e.g., a one-file bugfix), 3 verifiers with combined focus areas are sufficient.

3. Input data for each verifier:
   - The normalized task statement (confirmed by the user)
   - The context scout's full context map
   - All original context (ticket content, wiki pages, linked documents)
   - The verifier's assigned focus area

#### Phase 3: Enrich Task & Challenge the User

When all verifiers return, merge their findings into `task-verification.md` (deduplicate, prioritize by impact). Then **actively enrich the task and challenge the user's understanding** — this is not a mechanical report, it's a conversation.

**Step A: Present what the system actually looks like.**

Show the user key findings from the context map — things they might not know or might have wrong. This is not "here's a report" — it's "here's what I found, and some of it doesn't match what you described":

- **Discoveries** — things the scouts found that the user didn't mention but are relevant to the task: "Я обнаружил, что в системе есть X — ты это учитывал?"
- **Terminology corrections** — where the user's language doesn't match the code: "Ты говоришь 'сделки', но в коде для отдела Y это `application_count`, не `deal_count` — это одно и то же или разные вещи?"
- **Asymmetries** — where the user assumes uniformity but the system differs: "Ты описываешь все подразделения одинаково, но у типа A показываются сделки, а у типа B — заявки. Это правильное поведение или баг?"
- **Missing context** — things the user might take for granted but didn't specify: "В отчёте есть колонка X, которая считается через Y — ты хочешь это менять или оставить как есть?"

**Step B: Ask probing questions.**

For each finding, ask a specific clarifying question. Not generic "is this right?" — but targeted: "Ты уверен, что [конкретное утверждение]? Потому что в коде я вижу [конкретное доказательство]."

Group questions by severity:
1. **Блокирующие** — если юзер ответит "нет", задача меняется кардинально
2. **Важные** — влияют на скоуп или подход
3. **Уточняющие** — дообогащают контекст

**Wait for the user's answers.** This is a conversation, not a report dump.

**Step C: Update the task.**

After the user answers:
- Update the normalized task statement with corrections and enrichments
- If the corrections change the understanding significantly, re-confirm with the user (Step 3)
- If entirely new claims appeared in the corrections, run one more verifier round (single pass — don't loop endlessly)

The task should now be **richer than what the user originally provided** — enriched with verified system context, corrected terminology, and resolved ambiguities.

Save the final merged report as `task-verification.md`.

---

### Step 5: Extract Knowns / Unknowns / Assumptions

Split everything you have into three honest buckets. **Use the context map and verification results** — they contain verified facts that upgrade the quality of this classification far beyond what the user's description alone provides.

**Knowns** — things you're confident about (from user + verification):
- The goal and why it matters
- Hard constraints (deadlines, compatibility, performance requirements)
- Which parts of the system are involved — **verified by scouts, not just stated by user**
- Actual entity names, field names, data structures — **from context map, not user's description**
- Verified terminology mapping — what the user calls X is actually Y in the system
- Available context, documentation, and dependencies

**Unknowns** — things that are genuinely unclear:
- Exact scope boundaries
- Expected behavior in edge cases
- Detailed acceptance criteria
- Impact on adjacent systems or processes
- Things the scouts couldn't find or verify

**Assumptions** — things you're treating as true but haven't verified:
- Whether an API contract can or can't change
- Whether existing behavior must be preserved
- Whether a specific pattern or approach should be reused
- **Anything the user confirmed but the verifiers couldn't cross-check** (user might still be wrong about things the code doesn't directly show)

The distinction matters. If something feels like a known but you're actually guessing, it's an assumption. Being wrong about this classification is one of the most common ways preparation fails — you carry a guess into planning as if it were a fact, and the whole analysis is built on sand.

---

### Step 6: Draft Requirements

Create `requirements.draft.md` using the template from the **Artifact Templates** section below.

Don't aim for perfection — aim for clarity and honesty about what you know and what you don't. This document will evolve during clarification rounds.

---

### Step 7: Readiness Critique

Spawn a **Readiness Critic** subagent to independently evaluate whether the task preparation is good enough.

1. Read `agents/readiness-critic.md` — this file contains the critic's complete role, 8 evaluation criteria, and output format
2. Use the **Agent tool** with:
   - `name`: `"readiness-critic"`
   - `subagent_type`: `"general-purpose"`
   - `prompt`: the FULL content of `agents/readiness-critic.md` combined with the input data below — the agent definition file IS the prompt, do not summarize or skip it
3. Input data to append to the prompt:
   - The full requirements draft content
   - The normalized task statement

**Do NOT launch a generic subagent without the agent definition.** The file defines what the critic checks and how it reports — without it, the subagent doesn't know its role.

The critic is deliberately strict — that's the point. It catches weak preparation before it wastes time in later stages.

Save the critic's full response as `readiness-review.md`.

---

### Step 8: Close All Open Gaps

This stage's job is not just to *identify* gaps — it's to *close* them. Every unknown and every weak area must be resolved before the task moves forward. A task with open unknowns is not a prepared task, no matter what the critic says.

**If NEEDS_CLARIFICATION:**
- Build `clarifications.md` using the template below
- Present it to the user: blocking gaps first, then open unknowns, then assumptions to verify
- **Wait for the user's answers**
- Once answers arrive, go back to Step 3 and refine — don't start from scratch, update what you have
- Re-run the critique (Step 7) on the updated draft
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
- Proceed to Step 9: Build Handoff

The clarification questions should be specific and actionable. "What's the scope?" is useless. "Does this change need to cover the mobile API endpoints, or only the web dashboard?" is useful.

**Convergence check:** If you're on the third clarification round, something is wrong — either the task is genuinely too vague to proceed at all, or the critic is being too strict for the available information. Flag this to the user and discuss whether to proceed with documented risks or abandon the task.

---

### Step 9: Build Handoff

Once all unknowns are resolved, all assumptions verified, and the critic returns READY_FOR_DEEP_ANALYSIS with no WEAK scores — build the handoff document.

`stage-1-handoff.md` is the **single entry point for Stage 2**. It is a clean, self-contained document that packages the final state of the task after all clarification rounds. Stage 2 should be able to read this file alone and have everything it needs to begin deep analysis.

Build it from the finalized requirements draft, incorporating all answers from clarification rounds. Do not include the iteration history — only the final resolved state.

Save `stage-1-handoff.md` and tell the user that Stage 1 is complete.

Then offer the user two options for continuing to Stage 2:

**Option 1 — Continue in this session:**
> "Запустить Stage 2 (Deep Analysis) прямо сейчас в этой сессии?"

If the user agrees, invoke the `/deep-analysis` skill.

**Option 2 — Continue in a new session:**
Provide a ready-to-paste block with the actual paths filled in:
```
Запусти /deep-analysis

Task ID: {task-id}
Артефакты: .planpipe/{task-id}/stage-1/
```

---

## Artifact Templates

This stage produces up to six files. **Every artifact must follow its template exactly.** These templates are not optional — they ensure consistency across tasks and enable Stage 2 to parse the output reliably.

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

### 2. `context-map.md`

**When:** Always created. Merged from 3 parallel context scouts' reports.

```markdown
# Context Map

> Task: [one-line task summary]
> Scouts: 3 (Data & Entities, Logic & Processes, UI & Presentation)

## Entities & Data Structures

### [Entity Name]
- **Location:** `path/to/file`
- **Type:** model / table / class / service
- **Fields:**

| Field | Type | Description | Notes |
|-------|------|-------------|-------|
| `field_name` | string/int/etc | [what it stores] | [any quirks — e.g., "only for department type X"] |

- **Relationships:** [what it connects to]

### [Entity Name]
...

## Terminology Map

| User's Term | Code Term(s) | Location | Match? |
|------------|-------------|----------|--------|
| [what user calls it] | [what code calls it] | `path/to/file` | exact / partial / mismatch |

## Processes & Business Logic

### [Process Name]
- **Entry point:** `path/to/file:function`
- **Actual flow:**
  1. [Step 1 — what actually happens]
  2. [Step 2]
  3. [...]
- **Key business rules:** [filters, conditions, calculations]
- **Relevant configs:** [feature flags, role checks, env vars]

## UI / Report Structure

### [Report/View/Component Name]
- **Location:** `path/to/template`
- **Columns / Fields shown:**

| Column Label | Data Source | Varies By | Notes |
|-------------|-------------|-----------|-------|
| [label] | `entity.field` | [role/department/type/none] | [any conditional logic] |

## Asymmetries & Variations

[Cases where similar things are actually different — this section is critical for catching user errors]

| What | Assumed Uniform? | Actual Variations | Evidence |
|------|-----------------|-------------------|----------|
| [area] | [what user likely assumes] | [how it actually differs] | `path/to/file:line` |

## Raw Observations

[Anything notable that doesn't fit above — from any of the 3 scouts]
- [Observation 1 — source: scout-N]
- [Observation 2 — source: scout-N]
```

---

### 3. `task-verification.md`

**When:** Always created. Merged from 3-4 parallel task verifiers' reports.

```markdown
# Task Verification Report

## Claims Verified: [N total — X verified, Y mismatches, Z not found, W ambiguous]

## Verified Claims
| # | Claim | Source in Code | Status |
|---|-------|---------------|--------|
| 1 | [what the user said] | `path/to/file:line` | VERIFIED |

## Mismatches Found

### Mismatch 1: [short title]
- **User said:** [what the task description claims]
- **System shows:** [what the code/data actually has]
- **Evidence:** `path/to/file:line` — [relevant code snippet or description]
- **Impact:** [how this error would affect the task if not caught]
- **Suggested question for user:** [specific question to clarify]

## Not Found
| # | Claim | What Was Searched | Possible Explanation |
|---|-------|-------------------|---------------------|
| 1 | [what the user referenced] | [where you looked] | [might be: wrong name, doesn't exist, in external system] |

## Ambiguous
| # | Claim | Candidates Found | Question for User |
|---|-------|-----------------|-------------------|
| 1 | [what the user said] | [option A at `path`, option B at `path`] | [which one did you mean?] |

## Hidden Inconsistencies
- **[Inconsistency]:** [what's wrong and why it matters]

## Verdict: [TASK_VERIFIED | DISCREPANCIES_FOUND]

## Questions for the User
1. [Most critical — blocks correctness]
2. [Important — affects scope]
```

---

### 4. `readiness-review.md`

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

### 5. `clarifications.md`

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

### 6. `stage-1-handoff.md`

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
| 2 | `context-map.md` | Always | Ground truth map of the system area — produced by context scout |
| 3 | `task-verification.md` | Always | Merged verification report from parallel verifiers |
| 4 | `readiness-review.md` | Always | Quality gate — critic's evaluation and verdict |
| 5 | `clarifications.md` | When gaps exist | Questions for the user — drives the refinement loop |
| 6 | `stage-1-handoff.md` | On completion | **Primary input for Stage 2** — clean, final, self-contained |

Save all artifacts to `.planpipe/{task-id}/stage-1/`.

---

## Done Criteria

Stage 1 is complete when **all** of these hold:
- Goal is clear and stated
- Problem is articulated with its "why"
- Scope has at least a rough boundary
- Affected system area is identified
- Facts are separated from assumptions
- Task verifier has checked concrete claims against the system — discrepancies resolved with the user
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
- Task verifier found discrepancies that were not presented to the user
- Blocking ambiguities remain unresolved
- Readiness critic returned **NEEDS_CLARIFICATION**
- `stage-1-handoff.md` has not been created

---

## Notes

- **Preparation, not analysis.** This stage exists to make sure the right question is being asked, not to answer it. If you catch yourself evaluating technical approaches or designing solutions, stop — that's the next stage's job.
- **Users make mistakes.** Don't take the task description as ground truth. The task verifier exists because users confuse field names, mix up terminology, describe processes differently from how they actually work. A task built on wrong assumptions wastes everyone's time. Verify first, then proceed.
- **Read external resources.** If the user provides links (wiki, tracker, docs), actually fetch and read them. The context they contain often resolves unknowns that would otherwise become blocking gaps.
- **Memobank is optional.** Not every project has one. Check, use if available, skip if not. Don't ask the user about it.
- **The critic is an ally, not an enemy.** Its strictness protects the rest of the pipeline. When it flags something, it's usually right. When it's wrong, override it with explanation — don't just ignore it.
- **The handoff is the product.** Everything else is working material. `stage-1-handoff.md` is what Stage 2 actually reads. Make it clean, complete, and self-contained.
- **Subagent prompts = agent definition files.** When spawning a subagent, the content of its `agents/*.md` file IS the prompt. Read the file, combine it with input data, and pass as `prompt`. Never launch a subagent without its definition file — the file defines the agent's specialized role and behavior.
