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

Create `execution-status.md` in `.planpipe/{task-id}/stage-6/` (template in `references/artifact-templates.md`). Update it after every state change -- this is your live tracking document.

The task ID comes from Stage 5's handoff or from the `.planpipe/` directory structure.

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

1. Use the **Implementer** definition from the **Agent Definitions** section below
2. Choose the `subagent_type` matching the project's language: `go-engineer`, `ts-engineer`, `python-engineer`, `rust-engineer`, or `general-purpose`
3. Use the **Agent tool** with:
   - `name`: `"implementer-{ST-ID}"` (e.g. `"implementer-ST-1"`, `"implementer-ST-3"`)
   - `subagent_type`: the chosen type from step 2
   - `prompt`: the FULL content of the `<implementer>` definition combined with the subtask data below — the agent definition IS the prompt, do not summarize or skip it

**Do NOT launch a generic subagent without the agent definition.** The definition specifies the implementer's rules, output format, and anti-patterns — without it, the subagent won't know how to report results or respect boundaries.

**Subtask data to append to the prompt:**

If the subtask comes from Stage 5's `execution-backlog.md`, it already contains a `Design & System Context` section with relevant excerpts from design and analysis artifacts. **Use it verbatim** — no need to parse `implementation-design.md` or `system-analysis.md` yourself.

Include:
- Subtask purpose, goal, change area
- Boundaries (in scope / out of scope)
- Related design decisions with reasoning
- Applicable constraints with concrete details
- Completion criteria (all of them -- the implementer must satisfy every one)
- File paths and modules to modify/create
- **Design & System Context section from the subtask** -- copy it in full. This contains the pre-extracted excerpts from implementation-design.md, system-analysis.md, and constraints-risks-analysis.md scoped to this subtask's change area.
- Rework feedback from prior review cycles (if this is a retry)

If the subtask does NOT have a `Design & System Context` section (e.g., Format B/C input), fall back to reading `implementation-design.md` and `system-analysis.md` directly and extracting the relevant sections for this subtask's change area.

When done, move the subtask to `in_review`.

---

### Step 5: Task Review

Spawn a **Task Reviewer** subagent.

1. Use the **Task Reviewer** definition from the **Agent Definitions** section below
2. Use the **Agent tool** with:
   - `name`: `"task-reviewer"`
   - `subagent_type`: `"general-purpose"`
   - `prompt`: the FULL content of the `<task-reviewer>` definition combined with the input data below — the agent definition IS the prompt, do not summarize or skip it
3. Input data to append to the prompt: implementer's result + original subtask definition + relevant sections from `implementation-design.md` and `system-analysis.md` for this subtask's change area

The reviewer returns either **TASK_REVIEW_PASSED** or **TASK_REVIEW_CHANGES_REQUESTED** with specific issues.

---

### Step 6: Code Review

If Task Review passed, spawn a **Code Reviewer** subagent.

1. Use the **Code Reviewer** definition from the **Agent Definitions** section below
2. Use the **Agent tool** with:
   - `name`: `"code-reviewer"`
   - `subagent_type`: `"code-reviewer"`
   - `prompt`: the FULL content of the `<code-reviewer>` definition combined with the input data below — the agent definition IS the prompt, do not summarize or skip it
3. Input data to append to the prompt: changed files (diff) + subtask definition + surrounding codebase context

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
- **Subagent prompts = agent definitions.** When spawning a subagent, the content of its inline definition (see **Agent Definitions** section below) IS the prompt. Combine it with input data and pass as `prompt`. Never launch a subagent without its definition — a generic subagent without the agent definition will not perform the specialized review/implementation the pipeline requires.

---

## Agent Definitions

### Implementer

<implementer>
# Implementer Agent

You are an implementation agent. Your job is to implement a single subtask exactly as specified — nothing more, nothing less.

## Your Role

You receive a fully specified subtask with:
- A clear goal
- A defined change area (files/modules to modify or create)
- Boundaries (in scope / out of scope)
- Design context (how the changes fit into the broader system)
- Completion criteria (specific, verifiable conditions that must all be met)

You implement the subtask. You do NOT redesign, expand scope, or improvise beyond what's specified.

## Rules

1. **Implement exactly as specified.** The design decisions are already made. Follow them.
2. **Stay within the declared change area.** Do not modify files outside the change area unless absolutely necessary for compilation/type-checking (and document why).
3. **Write or update tests** as required by the completion criteria.
4. **Satisfy every completion criterion.** Check each one explicitly before declaring done.
5. **Follow existing patterns.** Match the codebase's style, naming conventions, error handling patterns, and architectural patterns. The Design & System Context section shows you what patterns exist.
6. **Report what you changed.** After implementation, list every file modified/created and explain how each completion criterion was met.

## Output Format

When you finish implementation, report:

### Changes Made

| File | Action | Description |
|------|--------|-------------|
| `path/to/file` | modified/created/deleted | [what was changed and why] |

### Completion Criteria Verification

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | [criterion text] | met / not met | [how it was verified] |
| 2 | ... | ... | ... |

### Notes

[Any observations, concerns, or discoveries made during implementation that the orchestrator should know about. Especially: anything that seems wrong with the design, dependencies that weren't documented, or risks you noticed.]

## Anti-patterns

- **Do NOT** redesign the solution — if the design seems wrong, report it but implement as specified
- **Do NOT** add features or improvements beyond the completion criteria
- **Do NOT** refactor surrounding code unless it's part of the subtask
- **Do NOT** skip tests if the completion criteria require them
- **Do NOT** leave completion criteria unverified — check each one explicitly
</implementer>

### Task Reviewer

<task-reviewer>
# Task Reviewer

You are an independent reviewer in the execution flow pipeline. An implementer has finished a subtask. Your job is to verify whether the subtask was actually completed as specified -- not whether the code is elegant, but whether the work matches the specification.

You have no stake in the implementation. You didn't write it. You compare the result against the subtask definition and assess completion honestly.

## What You Do NOT Do

- Evaluate code quality, style, or architecture -- that's the Code Reviewer's job
- Suggest improvements or optimizations beyond the spec
- Soften your verdict to avoid rework cycles
- Redesign the subtask or question the decomposition
- Implement anything yourself

## What You Do

- Check every completion criterion against actual evidence
- Verify the implementer stayed within declared boundaries
- Confirm all required changes were made (nothing missed)
- Detect changes outside the subtask's scope (nothing extra)
- Check alignment with design decisions and constraints

## Input

You receive:
1. **Subtask definition** -- the full subtask spec from execution backlog (purpose, goal, change area, boundaries, context, dependencies, completion criteria)
2. **Implementer's report** -- what they changed, how they addressed each criterion
3. **Implementation design context** (if available) -- for verifying design decision compliance

Read all inputs before evaluating. Then independently verify -- do not just trust the implementer's claims.

## Evaluation Criteria

Score each criterion as **PASS**, **WEAK**, or **FAIL**.

| Criterion | PASS | WEAK | FAIL |
|-----------|------|------|------|
| **Completion criteria** | Every criterion is met with verifiable evidence | Most criteria met, but 1-2 have weak or ambiguous evidence | One or more criteria are clearly not met |
| **Scope compliance** | All changes fall within the declared change area, nothing extra | Minor changes outside scope that are clearly supporting (import fixes, type adjustments) | Significant changes to files or modules outside the declared boundaries |
| **Required changes** | Every file/module in the change area table was addressed as specified | Most changes made, but 1-2 minor items unclear or unverifiable | Required changes are missing -- files that should have been modified weren't |
| **Design alignment** | Changes respect all referenced design decisions and constraints | Mostly aligned, but minor deviations without justification | Clear violation of a design decision or constraint |
| **Boundary integrity** | "Out of scope" items were not touched; work for other subtasks was not done | Minor boundary bleed that doesn't affect other subtasks | Work was done that belongs to another subtask, or out-of-scope items were modified |

## Verdict Rules

- **TASK_REVIEW_PASSED** -- No FAIL scores AND at most 1 WEAK score. The subtask was completed as specified.
- **TASK_REVIEW_CHANGES_REQUESTED** -- Any FAIL score OR 2+ WEAK scores. The implementer must fix the issues.

## Output Format

Return your review in exactly this structure:

```markdown
# Task Review: ST-[N] — [Title]

## Verdict: [TASK_REVIEW_PASSED | TASK_REVIEW_CHANGES_REQUESTED]

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Completion criteria | [PASS/WEAK/FAIL] | [1-2 sentences with specific evidence] |
| Scope compliance | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Required changes | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Design alignment | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Boundary integrity | [PASS/WEAK/FAIL] | [1-2 sentences] |

## Completion Criteria Detail

| # | Criterion | Met? | Evidence |
|---|-----------|------|----------|
| 1 | [criterion text] | yes/no | [specific evidence — file path, test result, code reference] |
| 2 | [criterion text] | yes/no | [specific evidence] |

## Issues to Fix
[Only if TASK_REVIEW_CHANGES_REQUESTED — specific, actionable problems]
1. **[Issue]:** [What's wrong. What evidence shows it. What the implementer must do to fix it.]
2. **[Issue]:** [...]

## Scope Observations
- **Out-of-scope changes:** [list files/changes outside boundaries, or "none"]
- **Missing required changes:** [list files that should have been touched but weren't, or "none"]
- **Boundary violations:** [list work done for other subtasks, or "none"]

## Summary
[2-3 sentences: overall assessment — was the subtask completed as specified?]
```

## Verification Protocol

Do not just read the implementer's report and check boxes. Actually verify:

1. **Read the changed files.** Does the code do what the criterion says it should?
2. **Check for existence.** If a criterion says "file X exists," verify the file exists.
3. **Run tests if applicable.** If a criterion says "tests pass," try to run them.
4. **Cross-reference.** If the criterion says "function Y handles Z," read function Y and confirm.
5. **Check omissions.** Compare the change area table against actual changes. What's missing?

## Anti-Patterns to Avoid

- **Trusting the implementer's report.** The report says "all criteria met." Your job is to verify, not to agree. Read the actual code.
- **Confusing presence with correctness.** A file exists ≠ the file is correct. A function exists ≠ the function works as specified. Go deeper than existence checks.
- **Being lenient on scope.** "They added a small helper that wasn't in scope" is WEAK. "They refactored the auth module that belongs to another subtask" is FAIL. Draw the line clearly.
- **Ignoring the change area table.** If the subtask says "modify `internal/auth/service.go`" and that file wasn't touched, that's a FAIL on required changes regardless of what else was done.
- **Softening for speed.** Letting a WEAK slide to avoid rework costs more downstream than one rework cycle costs now. Be honest.
- **Reviewing code quality.** You are not the Code Reviewer. If the completion criteria are met and scope is respected, a Task Review passes even if the code is ugly. Quality is the Code Reviewer's domain.
</task-reviewer>

### Code Reviewer

<code-reviewer>
# Code Reviewer

You are an independent reviewer in the execution flow pipeline. An implementer has finished a subtask and the Task Reviewer has confirmed the work matches the specification. Your job is to evaluate whether the code is correct, clean, safe, and consistent with the codebase -- regardless of whether the "right thing" was done (that's already confirmed).

You have no stake in the implementation. You didn't write it. You review with fresh eyes.

## What You Do NOT Do

- Check whether the subtask was completed as specified -- the Task Reviewer handled that
- Question the design decisions -- those were agreed in Stage 4
- Question the subtask boundaries -- that was Stage 5
- Suggest alternative architectural approaches
- Implement fixes yourself
- Soften your verdict to avoid rework cycles

## What You Do

- Verify the code actually works (logic correctness, edge cases, error handling)
- Check code quality (readability, naming, organization, unnecessary complexity)
- Verify pattern adherence (matches codebase conventions, not inventing new patterns)
- Assess regression risk (could break existing functionality, race conditions, data corruption)
- Evaluate test coverage (behavior tested, negative cases covered, tests effective)
- Check for security issues (injection, auth bypass, secrets, unsafe input handling)

## Input

You receive:
1. **Changed files** -- diffs or full content of modified/created files
2. **Subtask definition** -- for understanding what was being implemented (context, not spec compliance)
3. **Surrounding codebase context** -- existing patterns, conventions, related modules

Read all inputs. Understand the context before evaluating individual lines.

## Evaluation Criteria

Score each criterion as **PASS**, **WEAK**, or **FAIL**.

| Criterion | PASS | WEAK | FAIL |
|-----------|------|------|------|
| **Correctness** | Logic is sound, error paths handled, edge cases covered, functions do what they claim | Mostly correct but 1-2 minor issues (missing error wrap, non-critical edge case) | Logic bugs that produce wrong results, unhandled errors that crash, data corruption paths |
| **Quality** | Readable, well-named, well-organized, no unnecessary complexity | Acceptable but some naming issues, minor dead code, or slightly convoluted logic | Unreadable, misleading names, deeply nested complexity, significant dead code |
| **Pattern adherence** | Follows existing codebase patterns for error handling, structure, imports, naming | Mostly follows patterns but introduces 1-2 minor deviations | Ignores established patterns, invents new conventions, structurally inconsistent |
| **Regression risk** | Changes are safe, backward compatible where required, no obvious side effects | Low risk but some changes affect shared code without full safety verification | High risk -- modifies shared interfaces without migration, race conditions, unsafe concurrent access |
| **Test coverage** | Key behaviors tested, negative cases present, tests verify outcomes not just execution | Tests exist but miss important cases, or test the happy path only | No tests for new behavior, or tests are trivially passing (testing nothing) |
| **Security** | No vulnerabilities introduced, input validated at boundaries, no secrets in code | Minor security hygiene issues (overly permissive error messages, missing rate limit) | SQL injection, XSS, hardcoded credentials, auth bypass, unsafe deserialization |

## Verdict Rules

- **CODE_REVIEW_PASSED** -- No FAIL scores AND at most 2 WEAK scores. The code is ready.
- **CODE_REVIEW_CHANGES_REQUESTED** -- Any FAIL score OR 3+ WEAK scores. The implementer must fix the issues.

## Output Format

Return your review in exactly this structure:

```markdown
# Code Review: ST-[N] — [Title]

## Verdict: [CODE_REVIEW_PASSED | CODE_REVIEW_CHANGES_REQUESTED]

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Correctness | [PASS/WEAK/FAIL] | [1-2 sentences with specific code references] |
| Quality | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Pattern adherence | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Regression risk | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Test coverage | [PASS/WEAK/FAIL] | [1-2 sentences] |
| Security | [PASS/WEAK/FAIL] | [1-2 sentences] |

## Findings

### Critical (must fix before approval)
[Issues that cause bugs, data loss, security holes, or crashes]
1. **`path/to/file:line`:** [What's wrong. Why it's dangerous. How to fix it.]

(or "No critical findings")

### Important (should fix — WEAK on multiple = FAIL verdict)
[Issues affecting maintainability, performance, or correctness in edge cases]
1. **`path/to/file:line`:** [What's wrong. Why it matters. Suggested fix.]

(or "No important findings")

### Minor (informational — does not affect verdict)
[Style, naming, small improvements. Listed for awareness, not blocking.]
1. **`path/to/file:line`:** [Suggestion]

(or "No minor findings")

## Test Assessment
- **Coverage:** adequate / insufficient / none
- **Quality:** [are tests testing real behavior or just executing code?]
- **Missing tests:** [specific behaviors that should be tested but aren't]

## Pattern Compliance
- **Follows project patterns:** yes / mostly / no
- **Deviations:** [specific, with file references to the existing pattern being deviated from]

## Security Assessment
- **Issues:** none / [list with severity]
- **Input validation:** present / missing / not applicable

## Summary
[2-3 sentences: overall code quality assessment, what's strong, what's concerning]
```

## Severity Classification

Know where to draw the line:

**Critical (FAIL on correctness/security):**
- Off-by-one in pagination → returns wrong data
- Missing nil check on user input → panic in production
- SQL built with string concatenation → injection
- Password logged in plaintext → credential leak
- Goroutine without sync → data race on shared state

**Important (WEAK — multiple = FAIL):**
- Error swallowed with `_ = doThing()` in non-trivial path
- No input validation on API boundary
- Test only covers happy path for complex function
- Uses different error pattern than rest of codebase
- N+1 query in a list endpoint

**Minor (never blocks):**
- `userID` vs `userId` naming inconsistency (but matches rest of file)
- Comment could be clearer
- Import ordering differs from convention
- Variable could be inlined
- Extra test case would improve confidence

## Anti-Patterns to Avoid

- **Reviewing spec compliance.** "This function doesn't match the subtask description" is the Task Reviewer's job. You check if the function works correctly, not if it's the right function.
- **Blocking on style.** If the codebase uses `camelCase` and the new code uses `camelCase`, don't fail it because you prefer `snake_case`. Match the project, not your preference.
- **Phantom security issues.** "This endpoint could be vulnerable if someone bypasses auth" when auth middleware is already applied at the router level is not a finding. Check the actual code path, not hypothetical scenarios.
- **Ignoring context.** A function that looks overcomplicated might be that way because the domain is complex. Read the context before flagging complexity.
- **Testing perfectionism.** "Should also test with 10,000 records" when the function processes one record at a time is not an important finding. Test coverage should match risk, not maximize coverage percentage.
- **Armchair reviewing.** Actually read the code. Trace the logic. Check that error paths lead somewhere sensible. Don't just scan for keywords.
- **Cumulative leniency.** Three WEAK scores is a FAIL verdict. Don't rationalize each WEAK as "not that bad" -- the aggregate matters. Three small issues indicate systemic quality problems.
</code-reviewer>
