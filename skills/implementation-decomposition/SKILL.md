---
name: implementation-decomposition
description: "Stage 5 of the planning pipeline — decomposes the agreed implementation design into execution-ready subtasks with full context, dependencies, parallel execution optimization, and coverage validation. Use this skill when: Stage 4 solution design is complete and you need to break the implementation into concrete, self-contained work units; you have implementation-design.md, change-map.md, and stage-4-handoff.md ready for decomposition; you need to plan execution waves, identify conflict zones, and validate that all requirements are covered before handing off to implementation. Triggers on: stage 5, decompose implementation, break into subtasks, execution backlog, execution planning, task decomposition, work breakdown, parallel execution plan, декомпозиция реализации, разбиение на подзадачи, план выполнения, бэклог выполнения."
---

# Implementation Decomposition — Stage 5

You are executing Stage 5 of the planning pipeline. Your job is to take the agreed implementation design from Stage 4 and decompose it into a set of concrete, self-contained, execution-ready subtasks — with full context, explicit dependencies, and validated coverage.

You do NOT write code, run tests, or execute changes. You decompose and validate — nothing more. Stage 4 determined what to build and where. Stage 5 determines how to break that work into executable units that can be safely handed off to implementors (human or agent).

Think of it this way: Stage 4 produced a confirmed implementation design with change maps, technical decisions, and an implementation sequence. Stage 5 takes that design and slices it into subtasks that are isolated enough to work on independently, yet connected enough that their completion guarantees the original task is fully done.

If Stage 4 answers "how exactly to implement the solution?", Stage 5 answers "what executable subtasks to break the implementation into so that nothing is lost, nothing extra is added, and the work can be safely handed off?"

## Input Requirements

This stage requires Stage 4 output with a confirmed implementation design.

Before doing anything else, verify you have one of these input sets:

**Preferred (Stage 4 handoff):**
- `stage-4-handoff.md` — self-contained handoff with implementation approach, change summary, sequence, and decisions. This is the single entry point.

**Full context (recommended to also load):**
- `implementation-design.md` — the complete implementation design with change details per module
- `change-map.md` — detailed file-level map of all changes
- `design-decisions.md` — journal of key technical decisions with reasoning

**Supporting references (load if available):**
- `agreed-task-model.md` — the user-confirmed task model from Stage 3 (for coverage validation)
- `constraints-risks-analysis.md` — detailed constraints and risks (Stage 2)
- `design-review-package.md` — user approval points and scope confirmation
- Memobank / memory directory — search for relevant execution patterns, past decomposition decisions

If `stage-4-handoff.md` is missing or `implementation-design.md` doesn't exist, stop immediately and tell the user — send it back to Stage 4. When rejecting, also audit all other expected inputs and report what else is missing and what the impact of each missing input would be. Specifically:
- If `agreed-task-model.md` is also missing, note that coverage validation would not be possible — the Coverage Reviewer needs the agreed task model to verify requirement traceability
- If `change-map.md` is missing, note that file-level decomposition cannot be done
- If `design-decisions.md` is missing, note that subtasks won't have design decision context

This full audit helps the user understand the total gap, not just the first missing file.

If `stage-4-handoff.md` and `implementation-design.md` exist but `agreed-task-model.md` is missing, warn the user that coverage validation will be limited — the decomposition can proceed, but the Coverage Reviewer won't be able to fully verify requirement traceability.

---

## Process

The stage runs in a cycle: **decompose → critique → coverage review → refine → user review → finalize** — repeating until the decomposition is confirmed.

---

### Step 1: Load and Prepare Decomposition Context

Read all Stage 4 artifacts and supporting references. Build a comprehensive picture:

1. Read `stage-4-handoff.md` for the implementation overview
2. Read `implementation-design.md` for the full change details per module
3. Read `change-map.md` for the file-level map with dependency order
4. Read `design-decisions.md` for the technical decisions and their reasoning
5. Read `agreed-task-model.md` for the original requirements, scenarios, and acceptance criteria
6. If a memobank exists, search for: execution patterns, past decomposition decisions on similar tasks, known bottlenecks

Prepare a **decomposition brief** (~300 words) covering:
- Implementation approach and scope
- Change map summary: modules, files, dependency order
- Key technical decisions that affect how work should be split
- Constraints that affect decomposition (shared files, migration ordering, API contract stability)
- Risk zones that require careful sequencing

---

### Step 2: Identify Work Units

Based on the implementation design, identify the natural work units.

Each work unit must be:
- **Logically cohesive** — does one thing, not three unrelated changes bundled together
- **Executable independently** — can be picked up and worked on without waiting for everything else (respecting declared dependencies)
- **Right-sized** — not so small that overhead exceeds value, not so large that it's hard to review or test
- **Tied to a concrete result** — produces something verifiable when done

Use these categories to guide the breakdown:

| Category | Examples |
|----------|---------|
| **Foundation** | Shared types, interfaces, database schemas, configuration |
| **Implementation** | Service logic, API endpoints, data processing, UI components |
| **Integration** | Connecting modules, wiring dependencies, registering routes |
| **Migration** | Data migrations, schema changes, backward-compatible transitions |
| **Testing** | Test coverage for new/changed functionality |

The implementation sequence from Stage 4 is a starting point, but it optimizes for "what order to implement." Decomposition optimizes for "what units to assign" — a different concern. One implementation step might split into multiple subtasks, or multiple steps might merge into one subtask.

---

### Step 3: Define Boundaries and Attach Context

For each subtask, define clear boundaries and provide full context so that the subtask is self-contained.

**Boundaries** determine what's inside and outside the subtask:
- What files/modules this subtask touches (and what it does NOT touch)
- What interfaces this subtask creates or modifies (and what it leaves unchanged)
- What the expected output of this subtask is
- How this subtask relates to the overall implementation

**Context** ensures the implementor can work without re-reading the entire planning history:
- Why the subtask exists (which part of the implementation it covers)
- Its goal (what it achieves when complete)
- The change area (modules, files, change types)
- Applicable constraints from the design — **with concrete details, not just names.** For each constraint, include: the specific code, file paths, data formats, or interfaces that must be preserved. Pull these details from `system-analysis.md` (implicit dependencies, change points) and `implementation-design.md` (change specifications). A constraint like "preserve billing webhook compatibility" is useless — instead write: "preserve billing webhook compatibility: `internal/billing/webhook.go` listens for `OrderCompleted` events with fields `{order_id, amount, currency, timestamp}` — do not change this event schema"
- Related design decisions (with enough reasoning to understand them)
- Dependencies on other subtasks
- Completion criteria (how to know it's done)

**Design & System Context** — for each subtask, copy the relevant excerpts from `implementation-design.md`, `system-analysis.md`, and `constraints-risks-analysis.md` directly into the subtask. This is the most important part of context attachment: Stage 6 will use these excerpts verbatim in the implementer's prompt without any further parsing. Scope each excerpt to the modules/files in the subtask's change area — don't dump entire documents.

---

### Step 4: Map Dependencies

For each subtask, explicitly declare dependencies.

**Dependency types:**

| Type | Meaning | Example |
|------|---------|---------|
| **Blocking** | Cannot start this subtask until the dependency is complete | "Shared types must exist before service implementation" |
| **Soft** | Can partially overlap, but full completion requires the dependency | "API endpoint work can start with mocks, but integration needs the service" |
| **Sequencing** | Order matters for correctness, but no hard technical block | "Migration must run before integration tests" |
| **Shared** | Multiple subtasks depend on the same thing | "Both auth and tenant services depend on the config subtask" |

For each dependency, specify:
- Which subtask it depends on (by ID)
- Why the dependency exists
- What counts as the unblock condition (what must be true for the dependent subtask to start)

The result is a dependency graph — not just a flat list.

---

### Step 5: Optimize for Parallel Execution

After building the initial structure, explicitly optimize for parallel work.

Goals:
- **Maximize parallelism** — identify subtasks that can run simultaneously
- **Minimize file overlap** — reduce risk of merge conflicts
- **Minimize contract overlap** — avoid two subtasks changing the same interface
- **Isolate foundation** — foundation subtasks should complete first, unblocking everything
- **Remove artificial blockers** — dependencies that exist by accident, not necessity

Organize subtasks into **execution waves** — groups of subtasks that can run in parallel within the wave, where each wave depends on the previous wave completing.

---

### Step 6: Detect Conflict Zones

Separately identify areas where subtasks might conflict:

| Conflict Type | Example |
|---------------|---------|
| **File collision** | Two subtasks modify the same file |
| **Contract collision** | Multiple subtasks change the same API or interface |
| **Semantic collision** | Different subtasks interpret the same design decision differently |
| **Migration collision** | Several subtasks depend on the same migration |
| **Hidden prerequisite** | One subtask creates a precondition for another, but it's not in the dependency graph |

For each conflict zone:
- Identify which subtasks are involved
- Assess the severity (can they still be parallel? do they need sequencing? do they need merging?)
- Recommend resolution (sequence them, merge them, add coordination protocol)

---

### Step 7: Build Initial Execution Structure

Assemble the complete execution structure:

1. **All subtasks** with full context (using the Subtask Template from Artifact Templates)
2. **Dependency graph** showing all connections
3. **Execution waves** grouping parallel work
4. **Conflict zones** with resolution recommendations
5. **Foundation subtasks** (must complete first)
6. **Integration subtasks** (wire everything together)
7. **Convergence subtasks** (final verification, testing)

Build `execution-backlog.md` using the template from the **Artifact Templates** section below.

---

### Step 8: Critique the Decomposition

Once the initial structure is built, spawn a **Decomposition Critic** subagent.

1. Read `agents/decomposition-critic.md` from this skill's directory
2. Use the **Agent tool** to spawn a **general-purpose** subagent with that prompt
3. Pass it: the execution backlog + Stage 4 implementation design + change map

The critic independently reviews:
- Task clarity — is each subtask understandable?
- Boundary quality — are boundaries clean or chaotically overlapping?
- Dependency correctness — are dependencies real and complete?
- Parallelizability — does the structure actually enable parallel work?
- Conflict risk — are conflict zones identified and resolved?
- Context completeness — can each subtask be worked on independently?
- Scope discipline — did decomposition add work not in the agreed design?

Save the critic's feedback.

---

### Step 9: Handle Critique Results

**If DECOMPOSITION_APPROVED:**
- Incorporate any minor observations
- Proceed to Step 10 (Coverage Review)

**If NEEDS_REFINEMENT:**
- For each issue the critic flagged, determine if it requires:
  - Subtask restructuring → adjust boundaries, merge or split subtasks
  - Dependency correction → fix the dependency graph
  - Context enrichment → add missing context to subtasks
  - Conflict resolution → adjust execution waves or merge subtasks
- After refinement, re-run the critic
- **Max one refinement round.** If issues remain, document them in the conflict zones section and proceed — the coverage review and user review are additional quality gates

---

### Step 10: Run Coverage Review

After the decomposition passes critique, spawn a **Coverage Reviewer** subagent.

1. Read `agents/coverage-reviewer.md` from this skill's directory
2. Use the **Agent tool** to spawn a **general-purpose** subagent with that prompt
3. Pass it: the execution backlog + agreed task model + implementation design + change map + design decisions

The Coverage Reviewer checks:
- **Coverage completeness** — all required parts of the task are covered by at least one subtask
- **Scope fidelity** — no subtasks extend beyond the agreed scope
- **Requirement traceability** — every requirement, scenario, or constraint maps to a subtask
- **Design alignment** — subtasks match the chosen implementation design
- **Dependency sufficiency** — no missing foundation, migration, setup, or integration subtasks
- **Done-state validity** — completing all subtasks actually completes the original task

The reviewer returns a verdict: **COVERAGE_OK** or **COVERAGE_GAPS_FOUND**, with confidence level (high / medium / low).

---

### Step 11: Handle Coverage Results

**If COVERAGE_OK (high confidence):**
- Proceed to Step 12 (Build Review Package)

**If COVERAGE_OK (medium/low confidence):**
- Review the areas of low confidence
- Add clarifying notes to affected subtasks
- Proceed to Step 12 — surface the low-confidence areas in the review package for user input

**If COVERAGE_GAPS_FOUND:**
- For each gap, determine:
  - Missing coverage → add a subtask or expand an existing one
  - Over-coverage → remove or scope down a subtask
  - Weak mapping → strengthen the traceability (add context, not work)
  - Missing dependency → add the foundation/integration subtask
- After fixes, re-run the Coverage Reviewer
- **Max one coverage revision round.** If gaps remain, document them explicitly and surface in user review

Build `coverage-matrix.md` using the template from the **Artifact Templates** section below.

---

### Step 12: Build Decomposition Review Package

Assemble `decomposition-review-package.md` — a focused document for the user that shows the execution structure, not internal details.

The package surfaces:
- How the implementation was broken down and why
- The execution waves (what runs in parallel, what's sequential)
- Key dependencies the user should understand
- Conflict zones and how they're resolved
- Coverage assessment — what's covered, what's at risk
- Points where the user's input is needed before the decomposition is finalized

Structure the package around clear sections the user can review and approve.

---

### Step 13: Present Decomposition Review to the User

Present the decomposition review package to the user.

Show:
1. The subtask list with brief descriptions (not full context — that's in `execution-backlog.md`)
2. The execution waves — which subtasks are parallel, which are sequential
3. The dependency graph in a readable format
4. Conflict zones and proposed resolutions
5. Coverage assessment — how confident we are that everything is covered
6. Any points requiring user input

Ask: "Does this decomposition look right? Any subtasks that should be split, merged, reordered, or removed? Anything missing?"

**Do NOT dump the full execution backlog on the user.** Show the structure and key decisions. The full details live in `execution-backlog.md` if they want to dive deeper.

---

### Step 14: Refine and Finalize

After all user feedback:

1. Update the decomposition with every correction, addition, and priority change
2. Resolve any conflicts introduced by the user's changes
3. Update execution waves if the user's choices affect ordering
4. Re-validate coverage if subtasks were added or removed

Build the four decomposition artifacts using the **Artifact Templates** section below:

1. `execution-backlog.md` — the main decomposition artifact
2. `coverage-matrix.md` — requirement-to-subtask traceability
3. `decomposition-review-package.md` — user-facing review document
4. `stage-5-handoff.md` — handoff for execution

---

### Step 15: Report to the User

Present a brief summary:
- How many subtasks were created
- How many execution waves
- Key dependencies and sequencing constraints
- Coverage confidence level
- Identified risks for execution
- Readiness for implementation

---

## Artifact Templates

This stage produces up to four files. **Every artifact must follow its template exactly.** These templates ensure consistency across tasks and enable the execution stage to parse the output reliably.

### 1. `execution-backlog.md`

**When:** Always created. The main decomposition artifact — the complete execution-ready task breakdown.

```markdown
# Execution Backlog

> Task: [one-line summary]
> Implementation approach: [as agreed in Stage 4]
> Total subtasks: [N]
> Execution waves: [M]
> Decomposition status: [draft / user-reviewed / finalized]

## Execution Overview

[2-3 sentences: how the implementation was decomposed, what the execution strategy is, how many waves of parallel work]

## Execution Waves

### Wave 1 — Foundation
[What this wave establishes and why it goes first]

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-1 | [title] | foundation | small/medium/large | ST-2, ST-3 |
| ST-2 | [title] | foundation | small/medium/large | ST-1 |

### Wave 2 — Core Implementation
[What this wave builds and what it depends on from Wave 1]

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-4 | [title] | implementation | small/medium/large | ST-5 |
| ST-5 | [title] | implementation | small/medium/large | ST-4 |

### Wave 3 — Integration
[What this wave connects and verifies]

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-7 | [title] | integration | small/medium/large | — |

### Wave N — Convergence
[Final verification and testing]

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-N | [title] | testing | small/medium/large | — |

## Dependency Graph

```
ST-1 (foundation) ──→ ST-4 (implementation)
                  ──→ ST-5 (implementation)
ST-2 (foundation) ──→ ST-5 (implementation)
                  ──→ ST-6 (implementation)
ST-3 (foundation) ──→ ST-6 (implementation)
ST-4 + ST-5 + ST-6 ──→ ST-7 (integration)
ST-7 ──→ ST-8 (convergence)
```

## Conflict Zones

| # | Zone | Subtasks Involved | Conflict Type | Severity | Resolution |
|---|------|-------------------|---------------|----------|------------|
| 1 | [area] | ST-X, ST-Y | file/contract/semantic/migration/hidden | low/medium/high | [how resolved] |
(or "No conflict zones detected")

---

## Subtasks

### ST-1: [Title]

**ID:** ST-1
**Type:** foundation / implementation / integration / migration / testing
**Wave:** [wave number]
**Priority:** critical-path / high / normal
**Estimated scope:** small / medium / large

#### Purpose
[Why this subtask exists — which part of the implementation it covers. 2-3 sentences.]

#### Goal
[What this subtask achieves when complete — one clear statement]

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| [module] | `path/to/file` | modify/create/delete | [what changes and why] |

#### Boundaries

**In scope:**
- [What's included in this subtask]

**Out of scope:**
- [What is NOT part of this subtask — handled by another subtask, with reference]

#### Context

**Related design decisions:**
- DD-N: [decision title] — [how it affects this subtask]

**Applicable constraints (with concrete details):**
- [Constraint name]: [specific file paths, code references, data formats, interface signatures from `system-analysis.md` and `implementation-design.md` that the implementor must know to respect this constraint. Never write a constraint without the concrete details — if you can't find the details, go back to the source artifacts and extract them.]

**Key scenarios covered:**
- [Which scenarios from the agreed model this subtask supports]

#### Design & System Context

This section contains **actual excerpts** (not references) from the design and analysis artifacts, scoped to this subtask's change area. Stage 6 uses this section verbatim in the implementer's prompt — no further parsing needed.

**From `implementation-design.md` — Change Details for this subtask's modules:**
[Copy the relevant `### Module: [Name]` section(s) from implementation-design.md that cover the files/modules this subtask touches. Include: file table, interfaces affected, tests needed.]

**From `system-analysis.md` — relevant modules:**
[Copy the relevant module sections from system-analysis.md. Include: key files with descriptions, change points, implicit dependencies, existing patterns that the implementor must follow or be aware of.]

**From `constraints-risks-analysis.md` — applicable items:**
[Copy any constraints or risks that specifically apply to this subtask's change area. Include the full constraint/risk entry, not just the name.]

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| [what is needed] | blocking/soft/sequencing/shared | ST-M | [what must be true to start] |
(or "No dependencies — can start immediately")

#### Completion Criteria
- [ ] [Criterion 1 — specific, verifiable]
- [ ] [Criterion 2]
- [ ] [Criterion 3]

---

### ST-2: [Title]
...

---

## Critique Review
[Summary of the decomposition critic's findings. What was flagged. What was revised. What remains as accepted limitations.]

## Coverage Review
[Summary of the coverage reviewer's findings. Verdict: COVERAGE_OK / COVERAGE_GAPS_FOUND. Confidence: high / medium / low. What was validated. What gaps were found and fixed.]

## User Review Log
[Changes the user made during decomposition review]
- **[Subtask/Topic]:** [What was proposed -> What the user said -> How the decomposition was updated]
```

---

### 2. `coverage-matrix.md`

**When:** Always created. Traceability matrix showing how requirements map to subtasks.

```markdown
# Coverage Matrix

> Task: [one-line summary]
> Coverage verdict: [COVERAGE_OK / COVERAGE_GAPS_FOUND]
> Confidence: [high / medium / low]

## Requirement Traceability

### From Agreed Task Model

| Requirement / Scenario | Source | Covered By | Status |
|------------------------|--------|-----------|--------|
| [requirement or scenario] | agreed-task-model.md | ST-1, ST-3 | covered / partial / missing |
| [acceptance criterion] | agreed-task-model.md | ST-5 | covered / partial / missing |

### From Implementation Design

| Design Element | Source | Covered By | Status |
|----------------|--------|-----------|--------|
| [module change] | implementation-design.md | ST-2 | covered / partial / missing |
| [new entity] | implementation-design.md | ST-4 | covered / partial / missing |

### From Change Map

| File / Change | Source | Covered By | Status |
|---------------|--------|-----------|--------|
| `path/to/file` — [change] | change-map.md | ST-1 | covered / partial / missing |

### From Design Decisions

| Decision | Source | Covered By | Status |
|----------|--------|-----------|--------|
| DD-N: [decision] | design-decisions.md | ST-3 | covered / partial / missing |

## Coverage Gaps
[What's not fully covered and why]
- [Gap: what, why, recommended action]
(or "No coverage gaps detected")

## Over-Coverage
[Subtasks that go beyond the agreed scope]
- [Subtask: what it adds beyond scope, whether it's justified]
(or "No over-coverage detected")

## Done-State Validation
[If all subtasks are completed, is the original task complete?]
- **Answer:** yes / no / conditional
- **Reasoning:** [Why — what's the evidence]
- **Conditions (if conditional):** [What else is needed beyond these subtasks]
```

---

### 3. `decomposition-review-package.md`

**When:** Always created. The user-facing review document.

```markdown
# Decomposition Review Package

> Task: [one-line summary]
> Total subtasks: [N]
> Execution waves: [M]
> Estimated parallel efficiency: [X subtasks can run simultaneously at peak]

## Decomposition Summary

[3-5 sentences: how the implementation was broken down, what the execution strategy is, key design choices in the decomposition]

## Subtask Overview

| # | Subtask | Type | Wave | Scope | Key Dependencies |
|---|---------|------|------|-------|-----------------|
| ST-1 | [title] | foundation | 1 | small | none |
| ST-2 | [title] | foundation | 1 | medium | none |
| ST-3 | [title] | implementation | 2 | large | ST-1 |
| ... | ... | ... | ... | ... | ... |

## Execution Waves

### Wave 1: [Name]
**Subtasks:** ST-1, ST-2
**Parallel:** all subtasks in this wave can run simultaneously
**Goal:** [what this wave establishes]

### Wave 2: [Name]
**Subtasks:** ST-3, ST-4, ST-5
**Parallel:** ST-3 and ST-4 can run simultaneously; ST-5 starts after ST-3
**Goal:** [what this wave builds]

### Wave N: [Name]
**Subtasks:** ST-N
**Goal:** [final verification]

## Dependency Highlights
[Only the dependencies the user needs to understand — not every internal link]
- **[Key dependency]:** [Why it matters, what it means for execution order]

## Conflict Zones
[Conflicts the user should be aware of]
- **[Zone]:** [What conflicts, how it's resolved, any risk]
(or "No significant conflict zones")

## Coverage Assessment
- **Coverage:** [COVERAGE_OK / COVERAGE_GAPS_FOUND]
- **Confidence:** [high / medium / low]
- **Key finding:** [1-2 sentences about coverage quality]

## Review Points

### Point 1: [Topic]
**Context:** [Why this needs the user's input]
**Current approach:** [What we did]
**Question:** [Clear, specific question for the user]

---

### Point 2: [Topic]
...

## Scope Confirmation

**All agreed requirements covered:**
- [Requirement 1] -> ST-X
- [Requirement 2] -> ST-Y

**Question:** Does this decomposition cover everything you need? Any subtasks that should be split, merged, reordered, or removed?
```

---

### 4. `stage-5-handoff.md`

**When:** Created only when Stage 5 is fully complete — decomposition is reviewed, user has approved, coverage is validated. This is the **primary input for the execution stage**.

```markdown
# Stage 5 Handoff — Implementation Decomposition Complete

## Task Summary
[Agreed task statement — 2-3 sentences]

## Classification
- **Type:** [feature / bug / refactor / integration / research / other]
- **Complexity:** [low / medium / high]
- **Total subtasks:** [N]
- **Execution waves:** [M]
- **Max parallel subtasks:** [P]
- **Solution direction:** [minimal / safe / systematic — as agreed]

## Implementation Approach
[2-3 sentences: what approach was chosen in Stage 4 and how it was decomposed]

## Execution Strategy
[How the work is organized — waves, parallelism, sequencing rationale]

## Subtask Summary

| ID | Title | Type | Wave | Scope | Blocking Dependencies | Completion Criteria Summary |
|----|-------|------|------|-------|-----------------------|---------------------------|
| ST-1 | [title] | foundation | 1 | small | none | [1-line summary] |
| ST-2 | [title] | foundation | 1 | medium | none | [1-line summary] |
| ST-3 | [title] | implementation | 2 | large | ST-1 | [1-line summary] |
| ... | ... | ... | ... | ... | ... | ... |

## Execution Waves

### Wave 1 — [Name]
**Parallel group:** ST-1, ST-2
**Establishes:** [what this wave produces for downstream work]

### Wave 2 — [Name]
**Parallel group:** ST-3 || ST-4; ST-5 after ST-3
**Builds:** [what this wave implements]

### Wave N — [Name]
**Sequential:** ST-N
**Validates:** [what this wave verifies]

## Dependency Graph

```
[ASCII representation of the dependency graph]
```

## Conflict Zones
| Zone | Subtasks | Resolution |
|------|----------|------------|
| [area] | ST-X, ST-Y | [how resolved] |
(or "No conflict zones")

## Coverage Verification
- **Verdict:** COVERAGE_OK
- **Confidence:** high / medium / low
- **All acceptance criteria mapped:** yes / no
- **All change map files covered:** yes / no
- **All design decisions traceable:** yes / no

## Constraints Respected
- [Constraint: how the decomposition respects it]

## Risks for Execution
| Risk | Affected Subtasks | Mitigation | Severity |
|------|-------------------|------------|----------|
| [risk] | ST-X, ST-Y | [mitigation] | low/medium/high |

## User Decisions Log
[Key decisions the user made during decomposition review]
- [Decision 1: what the user chose and why]

## Acceptance Criteria
[From agreed task model — carried forward]
- [Criterion 1]
- [Criterion 2]

## Detailed References
[Files with full decomposition details:]
- `execution-backlog.md` — complete execution backlog with all subtasks
- `coverage-matrix.md` — requirement-to-subtask traceability
- `decomposition-review-package.md` — user review document
- `implementation-design.md` — implementation design (Stage 4)
- `change-map.md` — file-level change map (Stage 4)
- `design-decisions.md` — decision journal (Stage 4)
- `agreed-task-model.md` — agreed task model (Stage 3)
```

---

## Artifact Summary

| # | Artifact | When | Purpose |
|---|----------|------|---------|
| 1 | `execution-backlog.md` | Always | Complete execution backlog with all subtasks — the main artifact |
| 2 | `coverage-matrix.md` | Always | Requirement-to-subtask traceability matrix |
| 3 | `decomposition-review-package.md` | Always | User-facing review document with execution structure |
| 4 | `stage-5-handoff.md` | On completion | **Primary input for execution** — clean, final, self-contained |

Save all artifacts to the working directory (or a designated output path if the user specifies one).

---

## Done Criteria

Stage 5 is complete when **all** of these hold:

- Implementation design is decomposed into clearly defined subtasks
- Each subtask has explicit boundaries, full context, and completion criteria
- Dependencies between subtasks are mapped with types and unblock conditions
- Subtasks are organized into execution waves optimized for parallel work
- Conflict zones are identified and resolved
- Decomposition critic has reviewed and found the structure DECOMPOSITION_APPROVED (or issues were addressed)
- Coverage reviewer has verified COVERAGE_OK with at least medium confidence (or gaps were addressed)
- Decomposition review has been conducted with the user
- Decomposition is updated based on user feedback
- All four artifacts are created following their templates
- `stage-5-handoff.md` has been created

## Failure Criteria

Stage 5 is NOT complete if **any** of these hold:

- Subtasks are vague or overlap chaotically — it's unclear what each one does
- Dependencies between subtasks are missing or incorrect
- Subtasks don't cover the full implementation — parts of the design are orphaned
- Subtasks add work not in the agreed implementation design
- Coverage reviewer found gaps that were not addressed
- No execution structure exists — subtasks are a flat list without waves or ordering
- Completion criteria are missing or untestable
- User was not presented with the decomposition for review
- `stage-5-handoff.md` has not been created

---

## Notes

- **Decomposition, not implementation.** If you catch yourself writing actual code or designing solutions, stop. Your job is to break the agreed design into executable units, not to redesign or implement.
- **The design is already decided.** The implementation design, change map, and technical decisions are inputs, not suggestions. Decompose what was agreed, don't redesign it.
- **Self-contained subtasks are the goal.** An implementor should be able to take a single subtask and work on it without re-reading the entire planning history. Context completeness matters.
- **Dependencies must be explicit.** Implicit dependencies are the primary source of execution failures. If subtask A must finish before subtask B can start, that must be declared — not assumed.
- **Parallel execution is a design goal, not an accident.** Actively optimize for it. Don't just list which subtasks "could" be parallel — organize waves, minimize file overlap, and resolve conflicts.
- **Coverage validation is not optional.** The Coverage Reviewer exists because it's easy to accidentally lose requirements during decomposition. Every acceptance criterion from the agreed model must map to at least one subtask.
- **The critic catches structural flaws.** You decomposed it — you can't objectively review it. The critic can. When it flags something, investigate before dismissing.
- **The review package is for the user, not for you.** Write it in terms they care about. Full subtask details go in `execution-backlog.md`. The review package surfaces structure, dependencies, and decisions.
- **Templates are not optional.** Consistent structure enables the execution stage to parse the output reliably.
- **Memobank check.** If the project has a memobank or knowledge store, check it for: execution patterns, decomposition precedents on similar tasks, known integration bottlenecks. Opportunistic — skip if nothing exists.
