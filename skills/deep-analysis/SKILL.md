---
name: deep-analysis
description: "Use for deep-analysis — structured, multi-stream investigation of a task that has already been prepared or scoped, done BEFORE plan creation. Invoke when: the user completed task preparation and wants to analyze the codebase, product intent, risks, and constraints; they mention deep analysis, stage 2, READY_FOR_DEEP_ANALYSIS, or analyzing a task before planning; they reference handoff, readiness, or requirements files from preparation. This is NOT brainstorming (exploring ideas), NOT plan-writing, NOT coding, NOT bug-fixing, NOT PR review. Distinct from research skill — this runs a specific 3-stream pipeline (product + system + constraints) with critic validation on prepared tasks."
---

# Deep Task Analysis — Stage 2

You are executing Stage 2 of the planning pipeline. Your job is to build a thorough, multi-faceted knowledge base about the task so that the next stage (plan synthesis) has solid ground to stand on.

You do NOT build plans or write code here. You investigate, map, and document — nothing more. Stage 3 synthesizes. Stage 2 researches.

Think of it this way: Stage 1 made sure we're asking the right question. Stage 2 makes sure we understand the territory well enough to answer it.

## Input Requirements

This stage requires Stage 1 output with a **READY_FOR_DEEP_ANALYSIS** verdict.

Before doing anything else, verify you have one of these input sets:

**Preferred (new Stage 1 format):**
- `stage-1-handoff.md` — self-contained handoff document with all verified facts, accepted risks, and acceptance criteria. This is the single entry point — no need for other files.

**Legacy (old Stage 1 format):**
- `requirements.draft.md` — the structured task statement from Stage 1
- `readiness-review.md` — the critic's evaluation (must contain `READY_FOR_DEEP_ANALYSIS`)
- `clarifications.md` (optional) — any Q&A from Stage 1's clarification rounds

If using legacy format, check `readiness-review.md` for the verdict. If it does NOT show `READY_FOR_DEEP_ANALYSIS`, stop immediately and tell the user — send it back to Stage 1.

---

## Process

### Step 1: Load and Verify Input

Read all Stage 1 artifacts. Confirm:
1. The verdict is `READY_FOR_DEEP_ANALYSIS`
2. Core sections are populated (goal, scope, constraints, knowns/unknowns/assumptions or verified facts)
3. If clarifications exist, integrate the answers into your understanding

Prepare a **task briefing** for the subagents: a concise summary (~200 words) covering the goal, scope, affected system areas, key constraints, and important unknowns. This briefing plus the full Stage 1 content is what each subagent receives.

---

### Step 2: Launch Parallel Analysis Streams

Spawn three subagents **in parallel** using the Agent tool. Each investigates the task from its own angle and produces a **draft analysis**.

The analysts do NOT self-critique. Their job is to produce the most thorough analysis they can. A separate critic reviews their output in the next step.

#### Stream 1: Product / Business Analysis

1. Read `agents/product-analyst.md` from this skill's directory
2. Spawn a **general-purpose** subagent with that prompt
3. Pass it: task briefing + full Stage 1 content + any clarifications

This stream answers: **why does this task exist and what outcome actually matters?**

#### Stream 2: Codebase / System Analysis

1. Read `agents/system-analyst.md` from this skill's directory
2. Spawn an **Explore** subagent (thoroughness: "very thorough") with that prompt
3. Pass it: task briefing + full Stage 1 content + specific file paths and module names

This stream answers: **where in the system does this task live and what gets touched?**

This is the only stream that actively explores the codebase.

#### Stream 3: Constraints / Risks Analysis

1. Read `agents/constraints-analyst.md` from this skill's directory
2. Spawn a **researcher** subagent with that prompt
3. Pass it: task briefing + full Stage 1 content + any clarifications

This stream answers: **what limits, risks, and sensitive areas must the plan account for?**

---

### Step 3: Critique All Analyses

When all three analysts return, spawn an **Analysis Critic** subagent.

1. Read `agents/analysis-critic.md` from this skill's directory
2. Spawn an **Explore** subagent (thoroughness: "very thorough") with that prompt — the critic needs codebase access to spot-check system analysis claims
3. Pass it: all three draft analyses + the original task briefing + Stage 1 content

The critic independently reviews each analysis for:
- Completeness — are required sections present and populated?
- Specificity — are findings concrete or generic filler?
- Accuracy — do claims match evidence? Does system analysis reference real code?
- Cross-consistency — do the three analyses contradict each other?

The critic returns a structured review with a verdict per analysis: **SUFFICIENT** or **NEEDS_REFINEMENT** with specific issues.

---

### Step 4: Handle Critique Results

**If all three analyses are SUFFICIENT:**
- Proceed to Step 5 (Assemble Artifacts)
- Incorporate any minor observations from the critic into the final artifacts

**If any analysis is NEEDS_REFINEMENT:**
- For each analysis that needs refinement, spawn a new subagent with:
  - The original analyst prompt
  - The draft analysis
  - The critic's specific issues for that analysis
  - Instruction: "Revise this analysis to address the critic's issues. Do not start from scratch — fix the identified problems."
- After refinement, proceed to Step 5
- **Max one refinement round.** If the analysis is still insufficient after one refinement, note the remaining gaps in the artifact's Issues section and proceed.

---

### Step 5: Assemble Artifacts

Write the three analysis artifacts to the output directory using the **exact templates** from the Artifact Templates section below. Every artifact must follow its template — these are not optional.

1. `product-analysis.md`
2. `system-analysis.md`
3. `constraints-risks-analysis.md`

Include the critic's review summary in each artifact's **Critique Review** section.

---

### Step 6: Build Handoff

Once all analyses are finalized and the critic's issues addressed, build the handoff document.

`stage-2-handoff.md` is the **single entry point for Stage 3**. It is a clean, self-contained document that consolidates the key findings from all three analyses. Stage 3 should be able to read this file alone and have everything it needs to begin plan synthesis. The three detailed analysis files remain as supporting references.

Build it by synthesizing the finalized analyses — not by copy-pasting entire sections, but by extracting what matters for planning.

Save `stage-2-handoff.md` and tell the user that Stage 2 is complete.

---

### Step 7: Report to the User

Present a brief synthesis covering:

- **Key findings from each stream** — 2-3 sentences per analysis, not a full recap
- **Critique results** — what the critic found, what was refined
- **Cross-analysis observations** — dependencies, tensions, or alignments between the three streams
- **Knowledge base assessment** — sufficient for plan synthesis, or are there critical gaps?
- **Open questions for planning** — things the planning stage should account for

If the knowledge base is sufficient, indicate the task is ready for Stage 3 (plan synthesis).

---

## Artifact Templates

Every artifact must follow its template exactly. These templates ensure consistency across tasks and enable Stage 3 to parse the output reliably.

### 1. `product-analysis.md`

**When:** Always created.

```markdown
# Product / Business Analysis

## Business Intent
[Why the task exists — the real motivation, the trigger, the business value. Not a requirements restatement — the deeper "why".]

## Actor & Scenario

**Primary Actor:** [who — specific role, not just "user"]

**Main Scenario:**
1. [Trigger: what causes the scenario to start]
2. [Step: what happens next]
3. [Step: ...]
4. [End state: what the actor sees/has when done]

**Secondary Actors:** [if any — with their relationship to the task]

**Secondary Scenarios:** [if any — briefly described]

## Expected Outcome
[What "done" looks like from the product perspective. What changes AND what stays the same.]

## Edge Cases
- **[Edge case name]:** [Description — what happens and why it matters for THIS task]
- **[Edge case name]:** [...]

## Success Signals
- **[Signal]:** [What to measure, what direction indicates success]
- **[Signal]:** [...]

## Minimum Viable Outcome
[The core that cannot be cut — the smallest thing that still delivers business value.]

## Critique Review
[Summary of the critic's findings for this analysis. What issues were raised. What was refined. What remains uncertain.]

## Open Questions
[Questions this analysis could not resolve — inputs for the planning stage.]
- [Question 1]
- [Question 2]
```

---

### 2. `system-analysis.md`

**When:** Always created.

```markdown
# Codebase / System Analysis

## Relevant Modules

### [Module/Area Name]
- **Path:** `path/to/module/`
- **Purpose:** [what this module does — from reading the code]
- **Key files:** [`file1.go` — does X], [`file2.go` — does Y]
- **Relevance to task:** [why this module matters]

### [Module/Area Name]
...

## Change Points

| Location | What Changes | Scope | Confidence |
|----------|-------------|-------|------------|
| `path/to/file:Function` | [what needs to change and why] | small/medium/large | high/medium/low |
| ... | ... | ... | ... |

## Dependencies

### Upstream (what affected code depends on)
- **[Dependency]:** [what it is, how it's used, whether it constrains changes]

### Downstream (what depends on affected code)
- **[Consumer]:** [what it is, how it uses the affected code, impact of changes]

### External
- **[Service/DB/API]:** [connection details, contracts, relevant behavior]

### Implicit
- **[Hidden dependency]:** [what it is, how you found it, why it matters]

## Existing Patterns
- **[Pattern name]:** [How the codebase handles this. Examples at: `path/to/example`. Why it matters for this task.]
- ...

## Technical Observations
- **[Observation]:** [What you found and why it's relevant]
- ...

## Test Coverage

| Area | Test Type | Coverage Level | Key Test Files | Notes |
|------|-----------|---------------|----------------|-------|
| [module] | unit/integration/e2e | good/sparse/none | `path/to/tests/` | [details] |

## Critique Review
[Summary of the critic's findings for this analysis. What issues were raised. What was refined.]

## Open Questions
[What you couldn't verify. What needs deeper investigation during planning.]
- [Question 1]
- [Question 2]
```

---

### 3. `constraints-risks-analysis.md`

**When:** Always created.

```markdown
# Constraints / Risks Analysis

## Constraints

### Architectural
- **[Constraint]:** [What it is, why it constrains the task, source/evidence]

### Technical
- **[Constraint]:** [...]

### Business
- **[Constraint]:** [...]

### Compatibility
- **[Constraint]:** [...]

### Regulatory/Compliance
- **[Constraint]:** [...] (or "None identified")

## Risks

| Risk | Category | Likelihood | Impact | Evidence | Mitigation Idea |
|------|----------|-----------|--------|----------|-----------------|
| [Specific risk] | technical/integration/scope/knowledge/regression | low/medium/high | low/medium/high | [What makes you think this] | [Brief idea] |
| ... | ... | ... | ... | ... | ... |

## Integration Dependencies
- **[System/Service/API]:** [Contract type, stability, change flexibility, failure mode]
- ...

## Backward Compatibility

| What Changes | Current Consumers | Migration Needed? | Rollback Safe? | Notes |
|-------------|-------------------|-------------------|----------------|-------|
| [interface/schema/behavior] | [who depends on it] | yes/no/unknown | yes/no/unknown | [details] |

If no interfaces change: "This task does not modify external-facing interfaces, schemas, or behavioral contracts."

## Sensitive Areas
- **[Area/Module]:** [Why it's sensitive — fragile code, no tests, high traffic, incidents. Risk level: low/medium/high]
- ...

## Critique Review
[Summary of the critic's findings for this analysis. What issues were raised. What was refined.]

## Open Questions
[Constraints you couldn't verify. Risks you might be miscalibrating.]
- [Question 1]
- [Question 2]
```

---

### 4. `stage-2-handoff.md`

**When:** Created only when Stage 2 is fully complete — all analyses finalized, critic issues addressed. This is the **primary input for Stage 3**.

```markdown
# Stage 2 Handoff — Deep Analysis Complete

## Task Summary
[From Stage 1 — one paragraph that a new team member could read and immediately understand]

## Classification
- **Type:** [feature / bug / refactor / integration / research / other]
- **Complexity:** [low / medium / high — based on analysis findings]
- **Primary risk area:** [technical / integration / scope / knowledge]

## Analysis Summary

### Product / Business
[2-3 sentences: why the task exists, main scenario, what success looks like]

### Codebase / System
[2-3 sentences: what parts of the system are involved, key change points, notable patterns]

### Constraints / Risks
[2-3 sentences: most important constraints, highest risks, critical compatibility concerns]

## System Map

### Modules Involved
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| [name] | `path/to/module` | [what it does for this task] | small/medium/large |
| ... | ... | ... | ... |

### Key Change Points
| Location | What Changes | Scope |
|----------|-------------|-------|
| `path/to/file:Function` | [what and why] | small/medium/large |
| ... | ... | ... |

### Critical Dependencies
- **[Dependency]:** [what it is, why it matters for planning]
- ...

## Constraints the Plan Must Respect
- [Constraint 1: what and why — with source]
- [Constraint 2: ...]

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Suggested Mitigation |
|------|-----------|--------|---------------------|
| [risk] | low/medium/high | low/medium/high | [idea] |
| ... | ... | ... | ... |

## Product Requirements for Planning
- **Main scenario:** [1-sentence summary of the primary user flow]
- **Success signals:** [what to measure]
- **Minimum viable outcome:** [smallest thing that still delivers value]
- **Backward compatibility:** [what must not break]

## Critique Results
[What the independent critic found. What was refined. What remains as accepted limitations.]

## Open Questions for Planning
[All unresolved questions from all three analyses, consolidated and prioritized]
1. [Most critical question — blocks planning decisions]
2. [Important question — affects approach selection]
3. [Nice to resolve — but planning can proceed without it]

## Detailed Analyses
[These files contain the full analysis and can be consulted for details:]
- `product-analysis.md` — full product/business analysis
- `system-analysis.md` — full codebase/system analysis
- `constraints-risks-analysis.md` — full constraints/risks analysis
```

---

## Artifact Summary

| # | Artifact | When | Purpose |
|---|----------|------|---------|
| 1 | `product-analysis.md` | Always | Detailed product/business analysis |
| 2 | `system-analysis.md` | Always | Detailed codebase/system analysis |
| 3 | `constraints-risks-analysis.md` | Always | Detailed constraints/risks analysis |
| 4 | `stage-2-handoff.md` | On completion | **Primary input for Stage 3** — consolidated, self-contained |

---

## Done Criteria

Stage 2 is complete when **all** of these hold:

- Product/business analysis delivers clear intent, scenario, and success signals
- Codebase/system analysis maps specific modules, change points, and dependencies (with file paths, not vague references)
- Constraints/risks analysis identifies real, specific constraints and risks (not generic lists)
- All three analyses have been reviewed by the analysis critic
- Critic issues have been addressed (or explicitly documented as unresolvable)
- All artifacts follow their templates exactly
- `stage-2-handoff.md` has been created
- The knowledge base is specific enough for a planning stage to construct concrete steps

## Failure Criteria

Stage 2 is NOT complete if **any** of these hold:

- Any core analysis stream was not executed
- Analysis is superficial — generic statements that could apply to any task
- System analysis makes claims about code it didn't actually read
- Key modules, constraints, or risks are obviously missing
- The critic found significant problems that were not addressed or documented
- Artifacts don't follow templates
- Cross-analysis contradictions remain unexplained
- `stage-2-handoff.md` has not been created

---

## Notes

- **Investigation, not planning.** If you catch yourself designing solutions or sequencing work, stop. That's Stage 3.
- **The system analyst must actually read code.** Generic statements are useless. Specific paths, actual interfaces, real patterns.
- **Constraints should be verified, not assumed.** Check the code before asserting a constraint.
- **The critic is independent.** It sees the analysts' output with fresh eyes. This is where quality comes from — not self-review.
- **Templates are not optional.** Stage 3 depends on consistent structure. An analysis that doesn't follow the template is incomplete.
- **Memobank check.** If the project has a memobank or knowledge store, each subagent should search it for relevant context. Opportunistic — skip if nothing exists.
