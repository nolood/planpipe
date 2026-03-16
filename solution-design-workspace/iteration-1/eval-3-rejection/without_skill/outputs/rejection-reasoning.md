# Stage 4 Rejection: Insufficient Stage 3 Inputs

## Decision

**Cannot proceed with solution design.** The Stage 3 artifacts do not contain enough information to design an implementation. This task must be sent back to Stage 3 for completion.

## Reasons for Rejection

### 1. No Agreed Task Model Exists

The `incomplete-handoff.md` explicitly states:

> "This handoff was generated before user agreement was obtained. The agreed-task-model.md does not exist."

Stage 4 requires a confirmed, user-agreed task model as its primary input. Without user agreement on what the task is, any implementation design would be speculative and likely wrong. The handoff document itself marks the solution direction as "not agreed."

### 2. Critical Fields Are Unknown

The handoff lists:

- **Complexity:** unknown
- **Primary risk area:** unknown
- **Solution direction:** not agreed

These are not optional fields. Complexity determines the depth of the design. Risk areas determine where the design needs mitigation strategies. The solution direction determines the entire approach (minimal fix vs. systematic change). Without any of these, there is no basis for design decisions.

### 3. Analysis Is Too Vague to Act On

The `analysis.md` contains placeholder-level content:

- **Task Goal:** "Improve the notification system" -- which notification system? What platform? What technology?
- **Problem Statement:** "Users don't get notifications reliably" -- no specifics on failure modes, frequency, affected user segments, or root cause hypotheses
- **System Scope:** "Not yet determined" -- cannot design changes without knowing what system we are changing
- **Constraints:** "Unknown" -- cannot make design decisions without knowing what constraints to respect
- **Risks:** "None identified yet" -- a system with unreliable notifications has risks; this indicates analysis was not performed
- **Candidate Solution Directions:** "Fix the notification system" -- this is a restatement of the problem, not a solution direction

### 4. No Codebase or System Context

There is no information about:

- What programming language or framework the notification system uses
- Where the notification system lives in the codebase
- What notification channels exist (email, push, SMS, in-app)
- What the current architecture looks like
- What has been tried before
- What "reliably" means in measurable terms

Without any of this, there is nothing to map an implementation design onto.

## What Stage 3 Needs to Produce

Before Stage 4 can proceed, Stage 3 must deliver:

1. **`agreed-task-model.md`** -- a user-confirmed task model with:
   - Specific, measurable problem statement
   - Defined scope (what is in/out)
   - Agreed solution direction
   - Acceptance criteria

2. **A complete `analysis.md`** with:
   - Identified system components involved
   - Known constraints (performance, compatibility, timeline)
   - Identified risks with preliminary assessment
   - Concrete solution directions with trade-offs

3. **A valid `stage-3-handoff.md`** with:
   - Known complexity classification
   - Identified risk areas
   - Agreed solution direction
   - Confirmation that user agreement was obtained

## Summary

The provided Stage 3 artifacts amount to: "Something is wrong with notifications; fix it." This is a problem report, not an analyzed, scoped, and agreed task. Implementation design requires knowing what to build, where to build it, and what constraints to respect. None of these are present. Returning to Stage 3.
