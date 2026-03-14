---
name: execution-flow
description: "Stage 6 of the planning pipeline -- executes subtasks with mandatory dual review (task review + code review) after each one, closing tasks only when both reviewers pass. Use this skill whenever you have a set of implementation subtasks ready to execute -- whether from Stage 5's execution backlog or from any other source that provides concrete subtasks with clear completion criteria. Also use when: the user wants controlled, reviewed execution of planned work; you need to orchestrate parallel implementation with review gates and dependency tracking; you're implementing a decomposed task and want to ensure nothing slips through without verification. Triggers on: stage 6, execution flow, execute backlog, implement subtasks, run execution, start implementation, execute plan, execute decomposition, запуск выполнения, исполнение подзадач, execution with review, выполнить план, run the plan, implement the tasks, start the work."
---

# Execution Flow -- Stage 6

You are executing Stage 6 of the planning pipeline. Your job is to take a set of implementation subtasks and execute every one of them -- safely, with review after each -- closing each only after it passes both a task review and a code review.

The planning is done. You execute what was agreed. If the plan turns out to be wrong, you escalate -- you don't silently improvise.

## Input Requirements

This stage needs a set of subtasks to execute. It supports three input formats: structured Stage 5 output, a directory of subtask files, or a raw task list.

### Format A: Stage 5 Output (preferred)

If coming from the planning pipeline, verify:

- `stage-5-handoff.md` -- execution overview with subtask summary, waves, dependency graph, coverage
- `execution-backlog.md` -- complete subtask definitions with context, boundaries, dependencies, completion criteria

Also load (required for full context -- implementers depend on these for detailed constraints and code references):
- `implementation-design.md` -- implementation design from Stage 4 (required)
- `change-map.md` -- file-level change map from Stage 4 (required)
- `system-analysis.md` -- codebase/system analysis from Stage 2 with implicit dependencies, change points, and code-level details (required)
- `constraints-risks-analysis.md` -- detailed constraints and risks from Stage 2 (required)
- `design-decisions.md` -- technical decisions journal from Stage 4
- `agreed-task-model.md` -- confirmed task model from Stage 3
- `coverage-matrix.md` -- requirement-to-subtask traceability

### Format B: `subtasks/` Directory

Subtasks can be provided as individual files in a `subtasks/` directory:

```
subtasks/
├── st-1.md    # Each file is one self-contained subtask
├── st-2.md
├── st-3.md
└── ...
```

Each subtask file should contain at minimum: goal, change area, completion criteria, dependencies. The exact format is flexible -- read each file and extract the structure.

Also look for companion files alongside `subtasks/`:
- `dependency-graph.md` -- how subtasks connect
- `execution-backlog.md` -- optional overview with waves and conflict zones
- `implementation-design.md`, `design-decisions.md` -- design context

When using this format:
1. Read every file in `subtasks/` to build the full picture
2. If `dependency-graph.md` exists, use it; otherwise infer dependencies from the subtask files
3. Group subtasks into execution waves based on dependencies
4. Each implementer receives only its own subtask file content -- not all subtasks

### Format C: Raw Task List

The skill works without any formal structure. You need at minimum:

- **A list of subtasks** in any format (markdown, JSON, plain text, issue tracker export)
- Each subtask must have at least:
  - A clear goal (what to achieve)
  - Enough context to implement independently
  - Some form of completion criteria (even informal)

If subtasks lack structure, normalize them before starting:

1. For each subtask, extract or infer: **goal**, **change area** (what files/modules), **completion criteria** (how to know it's done), **dependencies** (what must come first)
2. Identify which subtasks can run in parallel vs. which are sequential
3. Present the normalized list to the user for confirmation before starting execution

When working without Stage 5, reviews will be lighter -- the Task Reviewer checks against whatever spec exists, the Code Reviewer checks code quality regardless of input format.

---

## Core Rule: No Task Closes Without Dual Review

After an implementer finishes a subtask, two independent reviewers must pass it:

1. **Task Reviewer** -- did we do the right thing? (scope, criteria, boundaries)
2. **Code Reviewer** -- did we do it well? (correctness, quality, tests, safety)

Both must approve before the task moves to `done`. If either rejects, the implementer gets feedback and fixes the issues.

---

## Process

The stage runs as a loop: **load context -> build task registry -> pick ready tasks -> dispatch implementers -> review -> close or rework -> repeat until done**.

---

### Step 1: Load Execution Context

Read all available input artifacts. Build an internal picture of:
- What subtasks exist and what each one requires
- What dependencies connect them
- What execution waves or ordering exists
- What constraints and design decisions apply

**Format A (Stage 5):** Read `stage-5-handoff.md` and `execution-backlog.md`.

**Format B (subtasks/):** Read every file in the `subtasks/` directory. Read `dependency-graph.md` if it exists. Build the dependency graph and execution waves from the individual files.

**Format C (raw list):** Normalize subtasks as described in Input Requirements.

---

### Step 2: Build Task Registry

Assign each subtask an execution status:

| Status | Meaning |
|--------|---------|
| `pending` | Has unsatisfied blocking dependencies |
| `ready` | All dependencies met -- can start |
| `in_progress` | Implementer working on it |
| `in_review` | Done implementing -- reviews running |
| `rework` | Review returned issues -- implementer fixing |
| `done` | Both reviews passed |
| `blocked` | New blocker discovered during execution |

Initialize from the dependency graph: subtasks with no blockers start as `ready`, others as `pending`.

Create `execution-status.md` (template in `references/artifact-templates.md`). Update it after every state change -- this is your live tracking document.

---

### Step 3: Select Execution Mode

For each batch of `ready` subtasks:

**Parallel** -- when multiple subtasks are ready, don't touch the same files, and belong to the same wave. Launch as parallel background subagents. All work happens in the current branch — no worktree isolation. This means subtasks that touch the same files MUST be sequential, not parallel.

**Sequential** -- when only one subtask is ready, subtasks share files, or the dependency chain is strictly linear.

**Plan-first** -- for large or high-risk subtasks. Spawn the implementer with `mode: "plan"` so it proposes an approach before making changes.

Present the execution plan to the user: what runs now, what waits, why.

---

### Step 4: Dispatch Implementer

For each subtask being executed, spawn an implementer subagent.

Build a self-contained prompt from the subtask definition. The implementer has no access to your conversation history, so pack everything it needs.

If the subtask comes from Stage 5's `execution-backlog.md`, it already contains a `Design & System Context` section with relevant excerpts from design and analysis artifacts. **Use it verbatim** — no need to parse `implementation-design.md` or `system-analysis.md` yourself.

Include in the prompt:
- Subtask purpose, goal, change area
- Boundaries (in scope / out of scope)
- Related design decisions with reasoning
- Applicable constraints with concrete details
- Completion criteria (all of them -- the implementer must satisfy every one)
- File paths and modules to modify/create
- **Design & System Context section from the subtask** -- copy it in full. This contains the pre-extracted excerpts from implementation-design.md, system-analysis.md, and constraints-risks-analysis.md scoped to this subtask's change area.
- Rework feedback from prior review cycles (if this is a retry)

If the subtask does NOT have a `Design & System Context` section (e.g., Format B/C input), fall back to reading `implementation-design.md` and `system-analysis.md` directly and extracting the relevant sections for this subtask's change area.

**Implementer rules** (include in the prompt):
- Implement the subtask exactly as specified
- Write or update tests as needed by completion criteria
- Do not modify files outside the declared change area
- Report what was changed and how each completion criterion was met

Choose the subagent type matching the project's language: `go-engineer`, `ts-engineer`, `python-engineer`, `rust-engineer`, or `general-purpose`.

When done, move the subtask to `in_review`.

---

### Step 5: Task Review

Spawn a **Task Reviewer** subagent.

1. Read `agents/task-reviewer.md` from this skill's directory
2. Spawn a **general-purpose** subagent with that prompt
3. Pass: implementer's result + original subtask definition + relevant sections from `implementation-design.md` and `system-analysis.md` for this subtask's change area

The reviewer returns either **TASK_REVIEW_PASSED** or **TASK_REVIEW_CHANGES_REQUESTED** with specific issues.

---

### Step 6: Code Review

If Task Review passed, spawn a **Code Reviewer** subagent.

1. Read `agents/code-reviewer.md` from this skill's directory
2. Spawn a subagent of type **code-reviewer** with that prompt
3. Pass: changed files (diff) + subtask definition + surrounding codebase context

The reviewer returns either **CODE_REVIEW_PASSED** or **CODE_REVIEW_CHANGES_REQUESTED** with findings by severity.

---

### Step 7: Handle Review Results

**Both reviews passed:**
- Move subtask to `done`
- Update `execution-status.md`
- Unblock any `pending` subtasks whose dependencies are now all `done`
- Log completion in `execution-status.md` (Recently Completed section)

**Either review failed:**
- Move subtask to `rework`
- Pass reviewer's feedback to the implementer
- Re-dispatch (Step 4) with original context + review feedback
- After rework: re-run Task Review. If Task Review passes, re-run Code Review.

**Rework limit:** 3 failed review cycles on the same subtask -> stop and escalate to the user. Present: what the subtask requires, what was produced, what reviewers rejected and why, your recommendation.

---

### Step 8: Repeat Until Done

Continue the loop: find `ready` subtasks -> dispatch -> review -> close/rework -> unblock dependents.

After each wave completion, report to the user:
- Which subtasks completed
- Which are now unblocked
- Issues encountered
- What's next

---

### Step 9: Handle Escalations

| Discovery | Action |
|-----------|--------|
| Missing subtask -- something needs doing that no subtask covers | Escalate to user. Do NOT create subtasks unilaterally. |
| Wrong design decision -- agreed approach doesn't work in practice | Escalate. Flag which decision and why. Initiate rollback to Stage 4 if user agrees. |
| New blocking constraint | Move subtask to `blocked`. Try workarounds. If none, escalate. |
| Conflict between subtasks | Stop both. Escalate. May need rollback to Stage 5. |
| Scope gap -- completing all subtasks won't complete the task | Escalate. May need rollback to Stage 5 or Stage 3. |

Do NOT "heroically push through" fundamental problems. Escalate early with evidence.

#### Rollback Procedure

When escalation requires returning to an earlier stage, follow this procedure. **The user decides which stage to roll back to** -- you present the options with impact analysis.

**Step 1: Stop all active work**
- Move all `in_progress` subtasks to `blocked` with reason: "rollback initiated"
- Do NOT discard `done` subtasks -- their code changes remain in the codebase
- Update `execution-status.md`

**Step 2: Present rollback options to the user**

Show the user what each rollback level means:

| Rollback To | When | What Gets Invalidated | What Survives |
|-------------|------|----------------------|---------------|
| **Stage 5** (re-decompose) | Subtask boundaries wrong, missing subtasks, wrong dependency order | `execution-backlog.md`, `stage-5-handoff.md`, all non-done subtask definitions | `done` subtasks (code stays), all Stage 4 artifacts, all Stage 3/2 artifacts |
| **Stage 4** (re-design) | Design decision wrong, approach doesn't work, change map incorrect | `implementation-design.md`, `change-map.md`, `design-decisions.md`, `stage-4-handoff.md`, all Stage 5 artifacts, all non-done subtasks | `done` subtasks IF they don't touch the redesigned area, all Stage 3/2 artifacts |
| **Stage 3** (re-synthesize) | Task understanding wrong, scope wrong, constraints missed | All Stage 3, 4, 5 artifacts, all subtasks | Stage 2 analyses (they're still valid observations), Stage 1 |

Ask: "Which level of rollback do you want? Here's what we'd redo and what we'd keep."

**Step 3: Execute the rollback**

After user confirms the target stage:

1. **Document the rollback** -- add a `## Rollback Log` entry to `execution-status.md`:
   - What triggered the rollback (which subtask, which discovery)
   - Which stage we're rolling back to
   - Which `done` subtasks are preserved vs. invalidated
   - What the user decided

2. **Assess done subtasks** -- for each `done` subtask, determine:
   - Does it touch the area affected by the rollback? → mark as `invalidated` (code may need to be reverted or updated after re-design)
   - Is it independent of the rollback area? → mark as `preserved` (code stays, subtask remains `done`)

3. **Invoke the target stage** with updated context:
   - Include the rollback reason as explicit input ("Stage 6 discovered that DD-3 is not implementable because...")
   - Include list of preserved subtasks and their code changes (the target stage must account for existing changes)
   - Include all evidence from the failed subtask (implementer output, review feedback, specific code that doesn't work)

4. **After the target stage completes** -- re-run all downstream stages:
   - Stage 4 rollback → re-run Stage 5 → re-run Stage 6 (resume with preserved subtasks)
   - Stage 5 rollback → re-run Stage 6 (resume with preserved subtasks)
   - Stage 3 rollback → re-run Stage 4 → Stage 5 → Stage 6

5. **When resuming Stage 6** after rollback:
   - Load the new execution backlog
   - Carry over `preserved` subtasks as already `done`
   - Start fresh on new/changed subtasks
   - Run the full process (dispatch → review → close) for all non-done subtasks

---

### Step 10: Final Smoke Test & Verification

When all subtasks are `done`:

1. **Run integration smoke test** — verify the combined result actually works:
   - Run the project's build command (e.g., `go build ./...`, `npm run build`, `cargo build`). If it fails, fix compilation errors before proceeding.
   - Run the project's test suite (e.g., `go test ./...`, `npm test`, `cargo test`). Report failures.
   - If the project has a linter configured, run it.
   - If any of these fail, identify which subtask's changes caused the issue and dispatch a fix (back to Step 4 for that subtask).
2. Verify all acceptance criteria from the agreed task model are covered
3. Verify no subtasks remain in non-done states
4. Create `execution-summary.md` (template in `references/artifact-templates.md`)
5. Report to user: completion status, smoke test results, review cycles, escalations, follow-up items

---

## Artifact Templates

All templates for this stage's output files are in `references/artifact-templates.md`. **Read that file before creating any artifact.** Every artifact must follow its template exactly -- the same sections, the same structure, the same field names.

| Artifact | When Created | Template In |
|----------|-------------|-------------|
| `execution-status.md` | Step 2, updated after every state change | `references/artifact-templates.md` section 1 |
| `execution-summary.md` | Step 10 when all subtasks done | `references/artifact-templates.md` section 2 |

Templates are not optional. If your output doesn't match the template structure, fix it before proceeding.

---

## Done Criteria

Execution flow is complete when **all** of these hold:

- All subtasks are in `done` status
- Every subtask passed both task review and code review
- No subtasks remain in `blocked`, `rework`, `in_review`, or `in_progress`
- All dependency chains exhausted
- **Final smoke test passed** (project builds, tests pass)
- All acceptance criteria verified (if agreed task model exists)
- `execution-summary.md` created

## Failure Criteria

Execution flow is NOT complete if **any** of these hold:

- Subtasks remain unclosed
- Any subtask closed without both reviews passing
- Unresolved blocking comments exist
- Subtasks stuck due to incorrect dependencies or conflicts
- Scope gap requiring return to planning

---

## Notes

- **Execution, not planning.** The design is an input. If you catch yourself redesigning, stop -- either execute or escalate.
- **Reviews are not optional.** Every subtask, no matter how small, goes through dual review.
- **Self-contained implementer prompts.** The implementer has no conversation history. Pack everything into the prompt. If subtasks come from Stage 5, they already contain a `Design & System Context` section with pre-extracted excerpts — use it verbatim.
- **No worktree isolation.** All implementers work in the current branch. Parallel execution is only for subtasks that touch completely different files. If files overlap, run sequentially.
- **Progress reports at wave boundaries.** Don't dump every status change. Report at wave completion, on blocks, or on escalations.
- **Escalate early.** A subtask that can't be completed as designed is valuable signal. Surface it before wasting effort.
- **Rework is normal; infinite rework is not.** 1-2 cycles happen. 3 cycles means the problem is upstream.
- **Flexible input.** This skill works with Stage 5 output, but also with any set of subtasks that have clear goals and completion criteria.
