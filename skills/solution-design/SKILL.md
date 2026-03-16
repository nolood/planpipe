---
name: solution-design
description: "Stage 4 of the planning pipeline — transforms the agreed task model into a concrete implementation design with change maps, technical decisions, and user approval points. Use this skill when: Stage 3 task synthesis is complete and you need to design how the task will be implemented in the system; the user has an agreed-task-model.md and wants to move to implementation design; you need to determine which files, modules, and interfaces to change, what technical decisions to make, and what to confirm with the user before coding begins. Triggers on: stage 4, solution design, design implementation, implementation design, design the solution, how to implement, plan the changes, map the changes, technical design, design review, проектирование решения, дизайн реализации, карта изменений."
---

# Solution Design — Stage 4

You are executing Stage 4 of the planning pipeline. Your job is to transform the synthesized task model from Stage 3 into a concrete, actionable implementation design — and then present a **combined review** to the user covering both the task understanding and the design.

You do NOT write code or execute changes. You design the implementation and get user confirmation — nothing more. Stage 3 synthesized the understanding (but did not get user confirmation). Stage 4 designs the solution AND presents the combined review.

Think of it this way: Stage 3 produced a draft task model (validated by critic, but not yet confirmed by the user). Stage 4 takes that understanding, maps it onto the actual codebase, and then presents the user with one comprehensive review: "here's what we understood AND here's what we'll build." This combined review reduces user fatigue — one serious review instead of two fragmented ones.

## Input Requirements

This stage requires Stage 3 output with a synthesized task model (draft, pending user confirmation).

Before doing anything else, verify you have one of these input sets:

**Preferred (Stage 3 format):**
- `stage-3-handoff.md` — self-contained handoff document with synthesized goal, scope, scenarios, constraints, and solution direction. This is the single entry point.

**Full context (recommended to also load):**
- `agreed-task-model.md` — the draft task model (status: draft — pending user confirmation in this stage's combined review)
- `analysis.md` — synthesized analysis from Stage 3
- `agreement-package.md` — Stage 3's agreement blocks (to include in the combined review)

**Supporting references (load if available):**
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
- Memobank / memory directory — search for relevant patterns, past design decisions, architectural conventions

If `stage-3-handoff.md` is missing or `agreed-task-model.md` doesn't exist, stop immediately and tell the user — send it back to Stage 3. If `agreement-package.md` is missing, warn the user that the combined review will not include the understanding blocks — the user will only review the design.

---

## Process

The stage runs in a cycle: **design → critique → combined user review (understanding + design) → refine** — repeating until both the task understanding and the design are confirmed.

---

### Step 1: Load and Prepare Design Context

Read all Stage 3 artifacts and supporting references. Build a comprehensive picture:

1. Read `stage-3-handoff.md` for the agreed model
2. Read `agreed-task-model.md` for the full confirmed understanding including user corrections
3. Read `analysis.md` for the analytical depth
4. Read Stage 2 system analysis for code-level details (change points, dependencies, patterns)
5. If a memobank exists, search for: architectural conventions, past design decisions on similar tasks, known technical debt, preferred patterns

Prepare a **design brief** (~300 words) for the subagents covering:
- Agreed goal and scope
- Solution direction (as confirmed by the user)
- System map: modules, change points, dependencies
- Constraints the design must respect
- Risks the design must mitigate
- Key scenarios the design must support

---

### Step 2: Launch Design Architect

Spawn a **Design Architect** subagent to build the initial implementation design.

1. Read `agents/design-architect.md` — this file contains the architect's complete role, codebase verification instructions, change specification format, and output structure
2. Use the **Agent tool** with:
   - `name`: `"design-architect"`
   - `subagent_type`: `"Explore"`
   - `prompt`: the FULL content of `agents/design-architect.md` combined with the input data below — the agent definition file IS the prompt, do not summarize or skip it
3. Input data to append to the prompt: the design brief + full Stage 3 content + Stage 2 system analysis + constraints analysis

The architect must actually read the codebase to verify and extend the change map from Stage 2. Stage 2's system analysis provides a starting point, but the architect digs deeper — tracing data flows, checking interfaces, discovering implicit dependencies that surface only when you design the actual changes.

The architect returns:
- Chosen implementation approach with alternatives considered
- Detailed change map (files, modules, interfaces)
- Concrete change specifications (new/modified entities, data flows)
- Key technical decisions with reasoning
- Implementation sequence
- Risk zones

---

### Step 3: Critique the Design

Once the architect returns, spawn a **Design Critic** subagent.

1. Read `agents/design-critic.md` — this file contains the critic's complete role, spot-check procedure, evaluation criteria, and output format
2. Use the **Agent tool** with (the critic needs codebase access to spot-check the change map):
   - `name`: `"design-critic"`
   - `subagent_type`: `"Explore"`
   - `prompt`: the FULL content of `agents/design-critic.md` combined with the input data below — the agent definition file IS the prompt, do not summarize or skip it
3. Input data to append to the prompt: the architect's design output + Stage 3 agreed task model + Stage 2 analyses

The critic independently reviews the design for:
- Feasibility within task constraints
- Scope discipline (no scope creep beyond agreed model)
- Architectural consistency with the existing codebase
- Change impact coverage (are all affected areas identified?)
- Risk awareness (are real risks handled, not just listed?)
- User-facing decision clarity (are approval points clear?)

Save the critic's feedback.

---

### Step 4: Handle Critique Results

**If DESIGN_APPROVED:**
- Incorporate any minor observations into the design
- Proceed to Step 5 (Build Review Package)

**If NEEDS_REVISION:**
- For each issue the critic flagged, determine if it requires:
  - Additional codebase exploration → spawn a targeted Explore subagent
  - Design revision → update the design artifacts directly
  - User input → add to the approval points list
- After revision, re-run the critic (Step 3) on the updated design
- **Max one revision round.** If the design is still insufficient, document remaining gaps in the risk zones section and proceed — the user review in Step 6 is the ultimate quality gate

---

### Step 5: Build Combined Review Package

Assemble `design-review-package.md` — a **combined** review document that covers both the task understanding (from Stage 3) and the implementation design.

This is the user's **first and primary review point** — they haven't seen the synthesized understanding yet. The package must give them the full picture in one coherent document.

**Part 1 — Task Understanding** (from Stage 3's `agreement-package.md`):
- Goal and problem statement — what we're solving and why
- Scope — what's in, what's out
- Key scenarios — primary flow and mandatory edge cases
- Constraints — what the design must respect

**Part 2 — Implementation Design:**
- The chosen approach and why
- Key changes (what's being added, modified, removed)
- Affected areas (modules, APIs, data models)
- Technical decisions that have trade-offs
- Risk zones the user should be aware of

**Part 3 — Key Decisions** (3-5 items max):
Structure the combined review around **key decisions** — the most important choices that affect the outcome. Each decision should be self-contained: context, options (if any), recommendation, and a clear question.

Keep the total to 3-5 decision points. The goal is one focused review session, not a questionnaire. If a decision is obvious from context, don't make it an approval point — just include it in the summary.

---

### Step 6: Present Combined Review to the User

Present the combined review package to the user.

This is a single, coherent review — not two separate reviews stitched together. The user should read it once and come back with all their feedback in one pass.

Show:
1. **Understanding summary** — "Here's what we think the task is" (goal, scope, key scenarios, constraints)
2. **Design summary** — "Here's how we plan to implement it" (approach, key changes, affected areas)
3. **Key decisions** — "Here are 3-5 choices that need your input" (with context, options, recommendations)

Ask: "Does this match your understanding? Do you agree with the approach and the key decisions? Anything to change before we proceed to decomposition?"

**Do NOT split this into multiple rounds.** Present everything, let the user respond once with all their feedback. If they want to discuss specific points, follow up — but don't force sequential block-by-block confirmation.

**If the user rejects the understanding (Part 1):** The synthesis is wrong — roll back to Stage 3. Re-run synthesis with the user's corrections as explicit input. Then re-design in Stage 4.

**If the user rejects the design (Part 2) but accepts understanding:** Re-run Steps 2-6 of this stage with the user's feedback. No need to re-do Stage 3.

**If the user rejects the understanding AND the analysis seems wrong:** Roll back to Stage 2. Re-run deep analysis with the user's corrections, then Stage 3, then Stage 4.

---

### Step 7: Refine and Finalize

After all user feedback:

1. **Finalize the task model:** Update Stage 3's draft `agreed-task-model.md` with user corrections from the combined review. Change status from "draft" to "confirmed". Update the `Agreed on` date. Add user corrections to the `User Corrections Log`. This is now the **agreed foundation** — user-confirmed understanding.
2. **Update the design** with every decision, correction, and priority change
3. Resolve any conflicts introduced by user choices
4. Mark explicitly approved decisions as approved
5. Update the implementation sequence if the user's choices affect order of work

Build the four design artifacts using the **Artifact Templates** section below:

1. `implementation-design.md` — the main design artifact
2. `change-map.md` — detailed map of all changes
3. `design-decisions.md` — journal of key decisions with reasoning
4. `stage-4-handoff.md` — handoff for the next stage

Also finalize `agreed-task-model.md` (from Stage 3) with user confirmations.

---

### Step 8: Report to the User

Present a brief summary:
- What was designed
- Key decisions made and confirmed
- Change scope (how many files/modules, rough size of changes)
- Identified risks and their mitigations
- Readiness for implementation

Then offer the user two options for continuing to Stage 5:

**Option 1 — Continue in this session:**
> "Запустить Stage 5 (Implementation Decomposition) прямо сейчас в этой сессии?"

If the user agrees, invoke the `/implementation-decomposition` skill.

**Option 2 — Continue in a new session:**
Provide a ready-to-paste block with actual paths filled in:
```
Запусти /implementation-decomposition

Task ID: {task-id}
Артефакты: .planpipe/{task-id}/ (stage-1/ через stage-4/)
```

---

## Artifact Templates

This stage produces up to five files. **Every artifact must follow its template exactly.** These templates ensure consistency across tasks and enable the next stage to parse the output reliably.

### 1. `implementation-design.md`

**When:** Always created. The main design artifact — the complete implementation design.

```markdown
# Implementation Design

> Task: [one-line summary]
> Solution direction: [minimal / safe / systematic — as agreed]
> Design status: [draft / user-reviewed / finalized]

## Implementation Approach

### Chosen Approach
[What approach was chosen and why. 2-3 paragraphs covering: the core idea, why this path over alternatives, and whether this is a minimal fix or a more systematic change.]

### Alternatives Considered
- **[Alternative A]:** [What it is, why it was rejected — specific trade-off that made it worse]
- **[Alternative B]:** [...]

### Approach Trade-offs
[Honest assessment: what this approach gives up, what risks it accepts, what it optimizes for]

## Solution Description

### Overview
[How the solution works end-to-end. Walk through the primary scenario showing how data flows through the changed system.]

### Data Flow
[How data moves through the system after changes. Include: entry point → processing → storage → output. Mark which parts are new vs. modified.]

### New Entities
[New types, classes, functions, services, or modules being added]

| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| [name] | class/function/service/type | `path/to/file` | [what it does] |

### Modified Entities
[Existing entities that change]

| Entity | Location | Current Behavior | New Behavior | Breaking? |
|--------|----------|-----------------|-------------|-----------|
| [name] | `path/to/file:line` | [what it does now] | [what it will do] | yes/no |

## Change Details

### Module: [Module Name]

**Path:** `path/to/module/`
**Role in changes:** [what this module does for the task]

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `file.go` | modify | [what changes and why] | small/medium/large |
| `new_file.go` | create | [what it contains and why it's needed] | small/medium/large |

**Interfaces affected:**
- [Interface/function signature change description]

**Tests needed:**
- [What should be tested for this module's changes]

### Module: [Next Module]
...

## Key Technical Decisions

| # | Decision | Reasoning | Alternatives Rejected | User Approved? |
|---|----------|-----------|----------------------|----------------|
| 1 | [What was decided] | [Why — specific, not generic] | [What else was considered] | yes/no/pending |
| 2 | ... | ... | ... | ... |

## Dependencies

### Internal Dependencies
- **[Module A → Module B]:** [What A needs from B, whether B's interface changes]

### External Dependencies
- **[Service/Library/API]:** [What it is, version constraints, whether it needs updates]

### Migration Dependencies
- **[Migration]:** [What data/schema changes are needed, in what order]
(or "No migrations required")

## Implementation Sequence

[Order of implementation that minimizes risk and allows incremental validation]

| Step | What | Why This Order | Validates |
|------|------|----------------|-----------|
| 1 | [First change] | [Why it goes first — usually: foundation, no dependencies] | [What you can test after this step] |
| 2 | [Second change] | [Why it follows step 1] | [What becomes testable] |
| ... | ... | ... | ... |

## Risk Zones

| Risk Zone | Location | What Could Go Wrong | Mitigation | Severity |
|-----------|----------|-------------------|------------|----------|
| [zone] | `path/to/area` | [specific failure mode] | [what to do about it] | low/medium/high |

## Backward Compatibility

### API Changes
[What API contracts change, how consumers are affected, migration path]
(or "No API changes")

### Data Changes
[What data schemas or storage formats change, migration strategy]
(or "No data changes")

### Behavioral Changes
[What behaviors change that existing code or users might depend on]
(or "No behavioral changes")

## Critique Review
[Summary of the design critic's findings. What was flagged. What was revised. What remains as accepted limitations.]

## User Approval Log
[Decisions confirmed by the user during design review]
- **[Decision/Topic]:** [What was proposed → User's choice → How design was updated]
```

---

### 2. `change-map.md`

**When:** Always created. Detailed map of all file-level changes.

```markdown
# Change Map

> Task: [one-line summary]
> Total files affected: [N modified, M new, K deleted]

## Files to Modify

| File | Module | Change Description | Scope | Dependencies |
|------|--------|-------------------|-------|-------------|
| `path/to/file.go` | [module] | [what changes] | small/medium/large | [what must change first] |
| ... | ... | ... | ... | ... |

## Files to Create

| File | Module | Purpose | Template/Pattern |
|------|--------|---------|-----------------|
| `path/to/new_file.go` | [module] | [why this file is needed] | [based on existing pattern at `path/to/example`] |
| ... | ... | ... | ... |

## Files to Delete

| File | Module | Reason | Replaced By |
|------|--------|--------|-------------|
| `path/to/old_file.go` | [module] | [why it's being removed] | [what replaces it, or "not replaced"] |
(or "No files to delete")

## Interfaces Changed

| Interface | Location | Current Signature | New Signature | Consumers |
|-----------|----------|------------------|---------------|-----------|
| [name] | `path/to/file:line` | [current] | [new] | [who calls this] |
| ... | ... | ... | ... | ... |

## Data / Schema Changes

| What | Type | Description | Migration Needed? |
|------|------|-------------|-------------------|
| [table/type/schema] | add/modify/remove | [what changes] | yes/no |
(or "No data/schema changes")

## Configuration Changes

| What | Location | Description |
|------|----------|-------------|
| [config item] | `path/to/config` | [what changes] |
(or "No configuration changes")

## Change Dependency Order

[Which changes depend on which — determines safe implementation order]

```
[change A] → [change B] → [change C]
                         → [change D]
[change E] (independent)
```
```

---

### 3. `design-decisions.md`

**When:** Always created. Journal of key technical decisions.

```markdown
# Design Decisions

> Task: [one-line summary]
> Total decisions: [N]
> User-approved: [M of N]

## Decision 1: [Short Title]

**Decision:** [What was decided — one sentence]

**Context:** [Why this decision was needed — what problem or choice point it addresses]

**Reasoning:** [Why this option was chosen — specific, not generic]

**Alternatives considered:**
- **[Alt A]:** [What it is] → Rejected because: [specific reason]
- **[Alt B]:** [What it is] → Rejected because: [specific reason]

**Trade-offs accepted:**
- [What this decision gives up or risks]

**User approval:** [required / approved / not required]

**Impact:** [What parts of the system this decision affects]

---

## Decision 2: [Short Title]
...

---

## Deferred Decisions

[Decisions that were explicitly pushed to implementation time]

- **[Decision]:** [Why it's deferred, what information is needed to make it, when it should be revisited]
```

---

### 4. `design-review-package.md`

**When:** Always created. The **combined** user-facing review document covering both task understanding and design.

```markdown
# Combined Review: Understanding & Design

> Task: [one-line summary]
> Solution direction: [minimal / safe / systematic]
> Changes: [N files modified, M new, K deleted across L modules]

## Part 1 — Our Understanding of the Task

### Goal
[2-3 sentences: what the task achieves and why it matters]

### Scope
**Included:**
- [What's in scope]

**Excluded:**
- [What's out of scope]

### Key Scenarios
**Primary:** [Brief description of the main flow]
**Mandatory edge cases:** [List]
**Deferred:** [What can wait]

### Constraints
- [Constraint 1]
- [Constraint 2]

## Part 2 — Proposed Implementation

### Solution Summary
[3-5 sentences: what the implementation does, how it works at a high level, what the user should expect]

### Key Changes
- **[Change Area 1]:** [What changes and why — in terms the user cares about]
- **[Change Area 2]:** [...]

### Risk Zones
- **[Risk]:** [What could happen, what we're doing about it]

## Part 3 — Key Decisions (confirm or correct)

### Decision 1: [Title]

**Context:** [Why this needs the user's input]
**Options:**
- **Option A:** [What it means] — [trade-off]
- **Option B:** [What it means] — [trade-off]
**Recommendation:** [Which option and why]

---

### Decision 2: [Title]
...

---

### Decision N: [Title] (max 5 decisions)
...

## Final Questions

1. Does the understanding (Part 1) match what you need?
2. Does the implementation approach (Part 2) look right?
3. Do you agree with the key decisions (Part 3)?
4. Anything to add, change, or flag before we proceed to decomposition?
```

---

### 5. `stage-4-handoff.md`

**When:** Created only when Stage 4 is fully complete — design is reviewed, user has approved all approval points, artifacts are finalized. This is the **primary input for the next stage** (execution planning or implementation).

```markdown
# Stage 4 Handoff — Solution Design Complete

## Task Summary
[Agreed task statement — 2-3 sentences]

## Classification
- **Type:** [feature / bug / refactor / integration / research / other]
- **Complexity:** [low / medium / high]
- **Change scope:** [N files modified, M new, K deleted across L modules]
- **Solution direction:** [minimal / safe / systematic — as agreed]

## Implementation Approach
[2-3 sentences: what approach was chosen and why]

## Solution Overview
[How the solution works end-to-end — 1 paragraph]

## Change Summary

### Modules Affected
| Module | Path | Changes | Scope |
|--------|------|---------|-------|
| [name] | `path/` | [summary of changes] | small/medium/large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `path/to/file:Function` | [what] | [why] |

### New Entities
| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| [name] | [type] | `path/` | [purpose] |

### Interface Changes
| Interface | Change | Consumers Affected |
|-----------|--------|-------------------|
| [name] | [what changes] | [who is affected] |

## Implementation Sequence
| Step | What | Validates |
|------|------|-----------|
| 1 | [change] | [what becomes testable] |
| 2 | [change] | [what becomes testable] |

## Key Technical Decisions
| Decision | Reasoning | User Approved |
|----------|-----------|---------------|
| [decision] | [why] | yes/no |

## Constraints Respected
- [Constraint: how the design respects it]

## Risks and Mitigations
| Risk | Mitigation | Severity |
|------|------------|----------|
| [risk] | [mitigation] | low/medium/high |

## Backward Compatibility
[Summary of what's backward compatible and what isn't. Migration needs if any.]

## User Decisions Log
[Key decisions the user made during design review]
- [Decision 1: what the user chose and why]

## Acceptance Criteria
[From agreed task model — carried forward]
- [Criterion 1]
- [Criterion 2]

## Detailed References
[Files with full design details:]
- `implementation-design.md` — complete implementation design
- `change-map.md` — detailed file-level change map
- `design-decisions.md` — full decision journal
- `design-review-package.md` — user review document
- `agreed-task-model.md` — agreed task model (Stage 3)
- `analysis.md` — synthesized analysis (Stage 3)
```

---

## Artifact Summary

| # | Artifact | When | Purpose |
|---|----------|------|---------|
| 1 | `implementation-design.md` | Always | Complete implementation design — the main artifact |
| 2 | `change-map.md` | Always | Detailed file-level change map |
| 3 | `design-decisions.md` | Always | Journal of key technical decisions with reasoning |
| 4 | `design-review-package.md` | Always | User-facing review document with approval points |
| 5 | `stage-4-handoff.md` | On completion | **Primary input for the next stage** — clean, final, self-contained |

Save all artifacts to `.planpipe/{task-id}/stage-4/`.

---

## Done Criteria

Stage 4 is complete when **all** of these hold:

- Implementation approach is chosen and described with alternatives considered
- All affected system areas are mapped to specific files and modules
- Change map covers every file to modify, create, or delete
- New and modified entities are specified with locations and purposes
- Key technical decisions are documented with reasoning and alternatives
- Design critic has reviewed and found the design DESIGN_APPROVED (or issues were addressed)
- **Combined review** (understanding + design) has been presented to the user
- User has confirmed both the task understanding and the design
- `agreed-task-model.md` has been finalized (status changed from draft to confirmed)
- Design is updated based on user feedback
- All five artifacts are created following their templates
- `stage-4-handoff.md` has been created

## Failure Criteria

Stage 4 is NOT complete if **any** of these hold:

- It's unclear how to implement the task — the design doesn't translate to actionable changes
- Specific files and modules to change are not identified
- Key technical decisions are not justified — no reasoning or alternatives
- Decisions that affect the user (API changes, UX changes, scope changes) were not presented for approval
- Design extends beyond the agreed scope without user confirmation
- Design critic found significant problems that were not addressed
- Change map is missing or incomplete
- `stage-4-handoff.md` has not been created

---

## Notes

- **Design, not implementation.** If you catch yourself writing actual code or running tests, stop. Your job is to specify what changes to make and why, not to make them. The next stage handles execution.
- **The architect must read the codebase.** Generic design that doesn't reference actual code is useless. Specific paths, real interfaces, actual patterns. The system analysis from Stage 2 is a starting point — the architect verifies and extends it.
- **Scope discipline matters.** The agreed task model from Stage 3 defines the boundaries. If the design naturally suggests changes beyond that scope, surface them as approval points — don't silently expand.
- **Combined review = one serious review.** This stage presents both the task understanding (from Stage 3) and the design in one package. The user confirms both at once. Keep it to 3-5 key decisions — not a questionnaire.
- **The user's word is final.** Any decision that affects APIs, UX, data models, or crosses the agreed scope boundary must be explicitly approved.
- **The critic catches design flaws.** You designed it — you can't objectively review it. The critic can. When it flags something, investigate before dismissing.
- **The review package is for the user, not for you.** Write it in terms they care about. Implementation details go in `implementation-design.md`. The review package surfaces understanding, approach, and key decisions.
- **Templates are not optional.** Consistent structure enables the next stage to parse the output reliably.
- **Memobank check.** If the project has a memobank or knowledge store, check it for: architectural conventions, preferred patterns, past design decisions on similar tasks. Opportunistic — skip if nothing exists.
- **Subagent prompts = agent definition files.** When spawning a subagent, the content of its `agents/*.md` file IS the prompt. Read the file, combine it with input data, and pass as `prompt`. Never launch a subagent without its definition file — a generic subagent without the agent definition will not perform the specialized design/critique the pipeline requires.
