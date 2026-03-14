---
name: task-synthesis
description: "Stage 3 of the planning pipeline — synthesizes deep analysis results and agrees on the task understanding with the user before generating planning artifacts. Use this skill when: Stage 2 deep analysis is complete and you need to consolidate findings into a unified task model; the user wants to review and confirm the understanding before planning begins; you have stage-2-handoff.md or individual analysis files ready for synthesis. Triggers on: stage 3, synthesis, synthesize analysis, agree on task, task agreement, consolidate findings, align on understanding, confirm task model, pre-planning alignment, готово к синтезу, согласование задачи."
---

# Task Synthesis & Agreement — Stage 3

You are executing Stage 3 of the planning pipeline. Your job is to synthesize the multi-stream analysis from Stage 2 into a single coherent task model and resolve contradictions.

You do NOT write plans, design solutions, or write code. You synthesize — nothing more. Stage 2 investigated. Stage 3 consolidates. Stage 4 will design the solution AND present the combined review (understanding + design) to the user.

Think of it this way: Stage 2 produced three independent views of the same task. They overlap, they might contradict, they each emphasize different things. Your job is to merge them into one honest picture. User confirmation happens later — jointly with the design review in Stage 4 — to reduce review fatigue and let the user evaluate understanding and design as a coherent whole.

## Input Requirements

This stage requires Stage 2 output.

Before doing anything else, verify you have one of these input sets:

**Preferred (new Stage 2 format):**
- `stage-2-handoff.md` — self-contained handoff document with consolidated findings from all three analyses. This is the single entry point.

**Full context (recommended to also load):**
- `product-analysis.md` — detailed product/business analysis
- `system-analysis.md` — detailed codebase/system analysis
- `constraints-risks-analysis.md` — detailed constraints/risks analysis

**Also check for:**
- `stage-1-handoff.md` or `requirements.draft.md` — original task statement for reference
- `clarifications.md` — any prior Q&A that shaped the understanding
- Memobank / memory directory — search for related context, past decisions, known patterns

If `stage-2-handoff.md` is missing or doesn't reference completed analyses, stop and tell the user — send it back to Stage 2.

---

## Process

The stage runs as: **synthesize → critique → refine → package for combined review in Stage 4**. User confirmation does NOT happen here — it happens jointly with the design review in Stage 4.

---

### Step 1: Load and Cross-Reference All Inputs

Read all Stage 2 artifacts. Build an internal picture:

1. Read `stage-2-handoff.md` for the consolidated view
2. Read each detailed analysis for the full depth
3. Cross-reference: where do analyses agree? Where do they diverge? Where does one stream flag something the others missed?

Create a working list of:
- **Agreements** — things all three analyses align on
- **Tensions** — places where analyses say different things or emphasize conflicting priorities
- **Gaps** — things that none of the analyses covered well enough
- **Open questions** — unresolved from Stage 2

---

### Step 2: Synthesize Unified Task Model

Merge everything into a single structure. This is not copy-pasting sections together — it's actual synthesis: resolving overlaps, choosing between contradicting views (with reasoning), and filling in the gaps.

Build `analysis.md` using the template from the **Artifact Templates** section below.

The synthesis covers:
- **Task goal** — one clear formulation drawn from product analysis + original requirements
- **Key scenarios** — main flow and mandatory edge cases, cross-checked against system change points
- **System scope** — which modules/areas are involved, validated against the system analysis
- **Constraints** — merged from all sources, deduplicated, prioritized
- **Risks** — consolidated, recalibrated where analyses disagreed on likelihood/impact
- **Candidate solution directions** — not solutions, but general directions (minimal vs. systematic, refactor vs. extend, etc.) informed by what the analyses revealed

When resolving contradictions:
- State what each source says
- Explain which view you chose and why
- If you can't resolve it — mark it as an explicit open question for the user

---

### Step 3: Critique the Synthesis

Spawn a **Synthesis Critic** subagent to independently review the unified model.

1. Read `agents/synthesis-critic.md` from this skill's directory
2. Use the **Agent tool** to spawn a **general-purpose** subagent with that prompt
3. Pass it: the synthesized `analysis.md` content + all Stage 2 analyses + Stage 1 content

The critic checks:
- Internal consistency of the synthesized model
- Whether contradictions from analyses were actually resolved (not just hidden)
- Whether assumptions were promoted to facts without evidence
- Whether the model accurately reflects what the analyses found
- Whether anything important was lost during synthesis

Save the critic's feedback. If the critic finds significant problems, fix them before presenting to the user.

---

### Step 4: Build Agreement Package

Once the synthesis passes critique (or has been refined), split the model into **agreement blocks** — discrete, reviewable chunks for the user.

Build `agreement-package.md` using the template from the **Artifact Templates** section below.

The blocks are:

**Block 1 — Goal & Problem Understanding**
- Did we understand the task correctly?
- Are we solving the right problem?
- Is the expected outcome what the user needs?

**Block 2 — Scope**
- What's included?
- What's excluded?
- Was anything added that shouldn't be there?

**Block 3 — Key Scenarios**
- Which scenario is primary?
- Which edge cases are mandatory?
- Which scenarios can be deferred?

**Block 4 — Constraints**
- Can the API change?
- Is backward compatibility required?
- Are there technical/business/process constraints?
- Are there deadlines?

**Block 5 — Candidate Solution Directions**
- Which direction should planning take?
- Preference: minimal, safe, or systematic?

Each block should be:
- Short enough to review in 30 seconds
- Oriented toward confirmation or correction — not education
- Ending with a clear question: "Is this correct? What would you change?"

---

### Step 5: Package for Combined Review

**Do NOT present blocks to the user here.** User review happens in Stage 4, jointly with the design review.

Build `agreement-package.md` using the template from the **Artifact Templates** section. This package will be included in Stage 4's combined review.

Build a **draft** `agreed-task-model.md` using the template from the **Artifact Templates** section. Mark it as `status: draft — pending user confirmation in Stage 4`. This draft reflects the best synthesis validated by the critic, but has not yet been confirmed by the user.

**Why no user review here:** Presenting understanding separately from design creates review fatigue (8-12 review points before any code). The user thinks holistically — they can't confirm Scope without thinking about Scenarios, and can't confirm Solution Direction without seeing the actual design. Combining these reviews in Stage 4 gives the user one serious review of "here's what we understood AND here's what we'll build" instead of two fragmented ones.

---

### Step 6: Build Handoff

Once the draft task model and agreement package are ready, build the handoff document.

`stage-3-handoff.md` is the **single entry point for Stage 4** (solution design). It packages the synthesized model with all context the design stage needs. Stage 4 should be able to read this file alone and have everything it needs to begin design AND to present the combined review to the user.

**Important:** The handoff must clearly indicate that the task model is a **draft pending user confirmation**. Stage 4 is responsible for presenting the combined review (understanding + design) and finalizing the model based on user feedback.

Save `stage-3-handoff.md` and tell the user that Stage 3 is complete — synthesis is done, and the combined review will happen after design in Stage 4.

---

### Step 7: Report to the User

Present a brief summary:
- What was synthesized
- Key findings and resolved contradictions
- What the draft model contains
- Note: user review will happen jointly with design review in Stage 4

---

## Artifact Templates

This stage produces up to four files. **Every artifact must follow its template exactly.** These templates are not optional — they ensure consistency across tasks and enable Stage 4 to parse the output reliably.

### 1. `analysis.md`

**When:** Always created. The synthesized analytical model of the task — before user agreement.

```markdown
# Synthesized Task Analysis

## Task Goal
[One clear formulation of what needs to be achieved — synthesized from product analysis and original requirements]

## Problem Statement
[Why this task exists — the deeper motivation, synthesized from business intent and original problem statement]

## Key Scenarios

### Primary Scenario
1. [Trigger: what starts the flow]
2. [Step: what happens]
3. [Step: ...]
4. [End state: what the actor sees/has when done]

### Mandatory Edge Cases
- **[Edge case]:** [What happens and why it must be handled in this task]
- **[Edge case]:** [...]

### Deferred Scenarios
- **[Scenario]:** [Why it can be deferred — what the risk of deferring is]

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| [name] | `path/to/module` | [what changes and why] | small/medium/large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `path/to/file:Function` | [what] | [why this change is needed] |

### Dependencies
- **[Dependency]:** [What it is, how it constrains the task]

## Constraints
- **[Constraint]:** [What it is, source, impact on planning]
- **[Constraint]:** [...]

## Risks

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| [risk] | low/medium/high | low/medium/high | [brief idea] |

## Candidate Solution Directions
- **[Direction name]:** [What it means, when it's appropriate, trade-offs]
- **[Direction name]:** [...]

## Resolved Contradictions
[Where analyses disagreed and how it was resolved]
- **[Topic]:** [Analysis A said X, Analysis B said Y. Resolution: Z because...]

## Remaining Open Questions
[Things that could not be resolved from analysis alone — need user input or will surface during planning]
- [Question 1]
- [Question 2]

## Critique Review
[Summary of synthesis critic's findings. What was flagged. What was fixed.]
```

---

### 2. `agreement-package.md`

**When:** Always created. The structured package for block-by-block user review.

```markdown
# Agreement Package

> Task: [one-line task summary]
> Based on: Stage 2 analyses (product, system, constraints/risks)
> Purpose: Confirm or correct the synthesized understanding before planning

---

## Block 1 — Goal & Problem Understanding

**Our understanding:**
[2-3 sentences: what the task achieves and why it matters]

**Expected outcome:**
[What "done" looks like]

**Confirm:** Is this the right goal? Are we solving the right problem? Is this the outcome you need?

---

## Block 2 — Scope

**Included:**
- [What's in scope — specific items]

**Excluded:**
- [What's explicitly out of scope]

**Confirm:** Is the scope correct? Anything missing? Anything that shouldn't be here?

---

## Block 3 — Key Scenarios

**Primary scenario:**
[Brief description of the main flow]

**Mandatory edge cases:**
- [Edge case 1]
- [Edge case 2]

**Deferred (not in this task):**
- [Scenario that can wait]

**Confirm:** Is the primary scenario correct? Are the mandatory edge cases right? Can the deferred items really wait?

---

## Block 4 — Constraints

- [Constraint 1]
- [Constraint 2]
- [Constraint 3]

**Confirm:** Are these constraints accurate? Are there constraints we missed? Can any of these be relaxed?

---

## Block 5 — Candidate Solution Directions

Based on the analysis, we see these possible directions:

- **[Direction A]:** [Brief description — trade-offs]
- **[Direction B]:** [Brief description — trade-offs]

**Confirm:** Which direction do you prefer? Minimal and safe, or systematic and thorough? Any direction we should avoid?
```

---

### 3. `agreed-task-model.md`

**When:** Created as a draft during Stage 3 (pending user confirmation in Stage 4's combined review). Finalized by Stage 4 after user confirms.

```markdown
# Agreed Task Model

> Status: [draft — pending user confirmation / confirmed]
> Agreed on: [date — filled when confirmed in Stage 4]
> Based on: Stage 2 analyses + synthesis critique

## Task Goal
[Final, user-confirmed goal statement]

## Problem Statement
[Final, user-confirmed problem description]

## Scope

### Included
- [Confirmed scope item 1]
- [Confirmed scope item 2]

### Excluded
- [Confirmed exclusion 1]
- [Confirmed exclusion 2]

## Key Scenarios

### Primary Scenario
1. [Step 1]
2. [Step 2]
3. [...]

### Mandatory Edge Cases
- **[Edge case]:** [Description]

### Explicitly Deferred
- **[Scenario]:** [Why deferred, user confirmed]

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| [name] | `path/to/module` | [role] | small/medium/large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `path/to/file:Function` | [what] | [why] |

### Dependencies
- **[Dependency]:** [Impact on task]

## Confirmed Constraints
- **[Constraint]:** [Description — confirmed by user]

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| [risk] | low/medium/high | low/medium/high | [direction] |

## Solution Direction
[User's confirmed preference — which direction, why, trade-offs accepted]

## Accepted Assumptions
[Assumptions the user explicitly accepted as safe to proceed with]
- [Assumption 1: why it's accepted]

## Deferred Decisions
[Decisions that were explicitly pushed to the planning stage]
- [Decision 1: why it's deferred]

## User Corrections Log
[What the user changed from the original synthesis — preserves the decision trail]
- **[Block/Topic]:** [What was proposed → What the user said → How the model was updated]

## Acceptance Criteria
[How to know the task is done correctly — derived from goal + user confirmations]
- [Criterion 1]
- [Criterion 2]
```

---

### 4. `stage-3-handoff.md`

**When:** Created when Stage 3 synthesis is complete — critic has validated, artifacts are ready. This is the **primary input for Stage 4**. User confirmation has NOT happened yet — it happens in Stage 4's combined review.

```markdown
# Stage 3 Handoff — Task Synthesis Complete

> Status: draft — pending user confirmation in Stage 4

## Task Summary
[Synthesized task statement — 2-3 sentences a new team member could read and immediately understand]

## Classification
- **Type:** [feature / bug / refactor / integration / research / other]
- **Complexity:** [low / medium / high]
- **Primary risk area:** [technical / integration / scope / knowledge]
- **Solution direction:** [minimal / safe / systematic — synthesized from analyses]

## Synthesized Goal
[One clear sentence — from synthesis, pending user confirmation]

## Synthesized Problem Statement
[Why this matters — from synthesis, pending user confirmation]

## Synthesized Scope

### Included
- [Item 1]
- [Item 2]

### Excluded
- [Item 1]
- [Item 2]

## Key Scenarios for Planning

### Primary Scenario
1. [Step 1]
2. [Step 2]
3. [...]

### Mandatory Edge Cases
- [Edge case 1]
- [Edge case 2]

## System Map for Planning

### Modules to Change
| Module | Path | What Changes | Scope |
|--------|------|-------------|-------|
| [name] | `path/to/module` | [what and why] | small/medium/large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `path/to/file:Function` | [what] | [why] |

### Critical Dependencies
- **[Dependency]:** [What it is, how it constrains planning]

## Constraints for Planning
- [Constraint 1: what and why — from analyses]
- [Constraint 2: ...]

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| [risk] | low/medium/high | low/medium/high | [direction from analyses] |

## Product Requirements for Planning
- **Primary scenario:** [1-sentence summary]
- **Success signals:** [what to measure]
- **Minimum viable outcome:** [smallest valuable delivery]
- **Backward compatibility:** [what must not break]

## Solution Direction
[Synthesized approach — minimal/safe/systematic, with rationale from analyses. Pending user confirmation in Stage 4.]

## Assumptions (pending confirmation)
- [Assumption: why it seems safe — to be confirmed by user in Stage 4]

## Deferred Items
- [Item: what and why it's deferred]

## Acceptance Criteria
- [Criterion 1]
- [Criterion 2]

## Detailed References
[These files contain the full analysis and draft model:]
- `analysis.md` — synthesized task analysis
- `agreement-package.md` — agreement blocks for Stage 4's combined review
- `agreed-task-model.md` — draft task model (pending user confirmation)
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
```

---

## Artifact Summary

| # | Artifact | When | Purpose |
|---|----------|------|---------|
| 1 | `analysis.md` | Always | Synthesized analytical model |
| 2 | `agreement-package.md` | Always | Agreement blocks for Stage 4's combined review |
| 3 | `agreed-task-model.md` | Always (as draft) | Draft task model — finalized by Stage 4 after user confirms |
| 4 | `stage-3-handoff.md` | On completion | **Primary input for Stage 4** — draft, pending user confirmation |

Save all artifacts to the working directory (or a designated output path if the user specifies one).

---

## Done Criteria

Stage 3 is complete when **all** of these hold:

- Findings from Stage 2 analyses are synthesized into a unified model
- Contradictions between analyses are resolved or explicitly flagged
- Agreement package has been built (for use in Stage 4's combined review)
- Draft `agreed-task-model.md` has been created (pending user confirmation in Stage 4)
- Synthesis critic has confirmed the model is consistent enough for design
- `stage-3-handoff.md` has been created

**Note:** User confirmation does NOT happen in Stage 3. It happens in Stage 4's combined review (understanding + design together).

## Failure Criteria

Stage 3 is NOT complete if **any** of these hold:

- Results from different Stage 2 analyses still conflict without resolution
- Assumptions were promoted to facts without evidence
- Synthesis critic found significant problems that were not addressed
- Blocking contradictions remain that would affect design
- Draft `agreed-task-model.md` has not been created
- `agreement-package.md` has not been created (needed for Stage 4's combined review)
- `stage-3-handoff.md` has not been created

---

## Notes

- **Synthesis, not planning.** If you catch yourself sequencing work or designing architecture, stop — that's Stage 4. Your job is to consolidate understanding.
- **No user review here.** User review happens in Stage 4, jointly with the design. This reduces review fatigue — two serious reviews (after design, after decomposition) instead of five fragmented ones.
- **Don't hide contradictions.** If two analyses say different things, show both views and explain your resolution. Hiding a contradiction is worse than surfacing an uncomfortable truth.
- **The critic catches your blind spots.** You wrote the synthesis — you can't objectively review it. The critic can. When it flags something, investigate before dismissing.
- **The draft model is the product.** Everything else is working material. Draft `agreed-task-model.md` is what Stage 4 builds the design on and then presents to the user for combined confirmation.
- **Memobank check.** If the project has a memobank or knowledge store, check it for relevant context — past decisions on similar tasks, known patterns, prior agreements. Opportunistic — skip if nothing exists.
- **Templates are not optional.** Stage 4 depends on consistent structure. An artifact that doesn't follow the template is incomplete.
