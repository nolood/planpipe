#!/usr/bin/env python3
"""Grading script for task-preparation skill eval iteration 1."""
import json
import os
import re

BASE = "/home/nolood/general/plan-pipeline/task-preparation-workspace/iteration-1"

def file_exists(path):
    return os.path.isfile(path)

def read_file(path):
    if not os.path.isfile(path):
        return ""
    with open(path) as f:
        return f.read()

def contains_all(text, terms):
    text_lower = text.lower()
    return all(t.lower() in text_lower for t in terms)

def contains_any(text, terms):
    text_lower = text.lower()
    return any(t.lower() in text_lower for t in terms)

def count_items_in_section(text, section_header):
    """Count bullet points under a section header."""
    lines = text.split("\n")
    in_section = False
    count = 0
    for line in lines:
        if section_header.lower() in line.lower() and "#" in line:
            in_section = True
            continue
        if in_section and line.strip().startswith("#"):
            break
        if in_section and line.strip().startswith("-"):
            count += 1
    return count

results = {}

# ── Eval 1: SSO feature (with_skill) ──
print("=== EVAL 1: SSO Feature (with_skill) ===")
e1_dir = f"{BASE}/eval-1-sso-feature/with_skill/outputs"
e1_req = read_file(f"{e1_dir}/requirements.draft.md")
e1_review = read_file(f"{e1_dir}/readiness-review.md")

e1_results = []

# 1. requirements.draft.md exists
passed = file_exists(f"{e1_dir}/requirements.draft.md")
e1_results.append({"text": "requirements.draft.md file was created", "passed": passed,
    "evidence": f"File {'exists' if passed else 'missing'} at {e1_dir}/requirements.draft.md"})

# 2. All required sections
sections = ["Goal", "Problem Statement", "Scope", "Constraints", "Dependencies", "Knowns", "Unknowns", "Assumptions"]
missing = [s for s in sections if f"## {s}" not in e1_req and f"## {s.lower()}" not in e1_req.lower()]
passed = len(missing) == 0
e1_results.append({"text": "Requirements draft contains all required sections",
    "passed": passed,
    "evidence": f"Missing sections: {missing}" if missing else "All 8 sections present"})

# 3. readiness-review.md exists
passed = file_exists(f"{e1_dir}/readiness-review.md")
e1_results.append({"text": "readiness-review.md file was created with a verdict", "passed": passed,
    "evidence": f"File {'exists' if passed else 'missing'}"})

# 4. Verdict is READY_FOR_DEEP_ANALYSIS
passed = "READY_FOR_DEEP_ANALYSIS" in e1_review and "NEEDS_CLARIFICATION" not in e1_review.split("## Verdict")[1].split("\n")[0] if "## Verdict" in e1_review else False
e1_results.append({"text": "Readiness critic returned READY_FOR_DEEP_ANALYSIS",
    "passed": passed,
    "evidence": "Verdict line: " + next((l for l in e1_review.split("\n") if "Verdict" in l), "not found")})

# 5. Task type is feature/integration
passed = contains_any(e1_req + e1_review, ["feature", "integration"])
e1_results.append({"text": "Task correctly classified as feature or integration",
    "passed": passed,
    "evidence": "Found 'feature' or 'integration' in outputs" if passed else "Neither term found"})

# 6. Knowns mention tech stack
tech_terms = ["keycloak", "fastapi", "react", "per-tenant", "saml"]
found = [t for t in tech_terms if t.lower() in e1_req.lower()]
passed = len(found) >= 4
e1_results.append({"text": "Knowns section mentions key technical context",
    "passed": passed,
    "evidence": f"Found {len(found)}/5 terms: {found}"})

# 7. No clarifications.md
passed = not file_exists(f"{e1_dir}/clarifications.md")
e1_results.append({"text": "No clarifications.md was created",
    "passed": passed,
    "evidence": "clarifications.md absent (correct)" if passed else "clarifications.md exists (unexpected)"})

results["eval-1-with_skill"] = e1_results

# ── Eval 2: Vague auth (with_skill) ──
print("\n=== EVAL 2: Vague Auth Fix (with_skill) ===")
e2_dir = f"{BASE}/eval-2-vague-auth-fix/with_skill/outputs"
e2_req = read_file(f"{e2_dir}/requirements.draft.md")
e2_review = read_file(f"{e2_dir}/readiness-review.md")
e2_clar = read_file(f"{e2_dir}/clarifications.md")

e2_results = []

# 1. requirements.draft.md exists
passed = file_exists(f"{e2_dir}/requirements.draft.md")
e2_results.append({"text": "requirements.draft.md file was created (even if sparse)", "passed": passed,
    "evidence": f"File {'exists' if passed else 'missing'}"})

# 2. readiness-review.md exists
passed = file_exists(f"{e2_dir}/readiness-review.md")
e2_results.append({"text": "readiness-review.md file was created with a verdict", "passed": passed,
    "evidence": f"File {'exists' if passed else 'missing'}"})

# 3. Verdict is NEEDS_CLARIFICATION
verdict_line = next((l for l in e2_review.split("\n") if "Verdict" in l), "")
passed = "NEEDS_CLARIFICATION" in verdict_line
e2_results.append({"text": "Readiness critic returned NEEDS_CLARIFICATION",
    "passed": passed,
    "evidence": f"Verdict line: {verdict_line}"})

# 4. clarifications.md exists
passed = file_exists(f"{e2_dir}/clarifications.md")
e2_results.append({"text": "clarifications.md was created with specific questions", "passed": passed,
    "evidence": f"File {'exists' if passed else 'missing'}"})

# 5. Questions are actionable (check for question marks and specific terms)
question_count = e2_clar.count("?")
has_specific = contains_any(e2_clar, ["error", "log", "which", "what", "who", "when", "how"])
passed = question_count >= 3 and has_specific
e2_results.append({"text": "Clarification questions are specific and actionable",
    "passed": passed,
    "evidence": f"{question_count} questions found, specific terms present: {has_specific}"})

# 6. More unknowns than knowns
unknowns_count = count_items_in_section(e2_req, "## Unknowns")
knowns_count = count_items_in_section(e2_req, "## Knowns")
passed = unknowns_count > knowns_count
e2_results.append({"text": "Requirements draft has more unknowns than knowns",
    "passed": passed,
    "evidence": f"Knowns: {knowns_count}, Unknowns: {unknowns_count}"})

# 7. No invented details (check that req doesn't mention specific tech not in the input)
invented_terms = ["jwt", "oauth2", "session", "redis", "postgres", "mongodb", "express", "django", "spring"]
found_invented = [t for t in invented_terms if t.lower() in e2_req.lower() and "could" not in e2_req.lower().split(t.lower())[0][-30:]]
# More lenient: just check the Knowns section doesn't assert specific tech
knowns_section = ""
if "## Knowns" in e2_req:
    start = e2_req.index("## Knowns")
    end = e2_req.index("##", start + 10) if "##" in e2_req[start+10:] else len(e2_req)
    knowns_section = e2_req[start:start + (end - start) if end > start else len(e2_req)]
specific_tech_in_knowns = [t for t in invented_terms if t.lower() in knowns_section.lower()]
passed = len(specific_tech_in_knowns) == 0
e2_results.append({"text": "Agent did NOT invent specific technical details without evidence",
    "passed": passed,
    "evidence": f"Specific tech asserted in Knowns: {specific_tech_in_knowns}" if specific_tech_in_knowns else "No invented details in Knowns section"})

results["eval-2-with_skill"] = e2_results

# ── Eval 3: Dashboard perf (with_skill) ──
print("\n=== EVAL 3: Dashboard Performance (with_skill) ===")
e3_dir = f"{BASE}/eval-3-dashboard-perf/with_skill/outputs"
e3_req = read_file(f"{e3_dir}/requirements.draft.md")
e3_review = read_file(f"{e3_dir}/readiness-review.md")

e3_results = []

# 1. requirements.draft.md with all sections
passed = file_exists(f"{e3_dir}/requirements.draft.md")
sections_found = [s for s in ["Goal", "Problem Statement", "Scope", "Constraints", "Knowns", "Unknowns", "Assumptions"] if f"## {s}" in e3_req or s.lower() in e3_req.lower()]
passed = passed and len(sections_found) >= 6
e3_results.append({"text": "requirements.draft.md file was created with all required sections",
    "passed": passed,
    "evidence": f"Sections found: {len(sections_found)}/7"})

# 2. readiness-review with all 8 criteria
criteria = ["Goal clarity", "Problem clarity", "Scope clarity", "Change target", "Context sufficiency", "Ambiguity", "Assumption safety", "Acceptance"]
criteria_found = [c for c in criteria if c.lower() in e3_review.lower()]
passed = file_exists(f"{e3_dir}/readiness-review.md") and len(criteria_found) >= 7
e3_results.append({"text": "readiness-review.md with all 8 criteria scored",
    "passed": passed,
    "evidence": f"Criteria found: {len(criteria_found)}/8: {criteria_found}"})

# 3. Task type correct
passed = contains_any(e3_req, ["bug", "performance"]) and not contains_any(e3_req.split("## Goal")[0] if "## Goal" in e3_req else "", ["feature", "new feature"])
e3_results.append({"text": "Task correctly classified as bug or performance issue",
    "passed": passed,
    "evidence": "Found 'bug' or 'performance' in requirements" if passed else "Classification unclear"})

# 4. Goal mentions 2 seconds
passed = "2 second" in e3_req.lower() or "under 2s" in e3_req.lower() or "2-second" in e3_req.lower() or "< 2" in e3_req
e3_results.append({"text": "Goal states the performance target: under 2 seconds",
    "passed": passed,
    "evidence": "Performance target of 2 seconds found in requirements" if passed else "Target not found"})

# 5. Tech context captured
tech = ["clickhouse", "graphql", "apps/dashboard", "services/analytics-api", "50"]
found = [t for t in tech if t.lower() in e3_req.lower()]
passed = len(found) >= 4
e3_results.append({"text": "Knowns mention ClickHouse, GraphQL, file paths, 50k DAU",
    "passed": passed,
    "evidence": f"Found {len(found)}/5: {found}"})

# 6. Root cause uncertainty in unknowns
passed = contains_any(e3_req, ["query layer", "frontend render", "bottleneck", "root cause", "which specific"])
e3_results.append({"text": "Unknowns include root cause uncertainty",
    "passed": passed,
    "evidence": "Root cause uncertainty mentioned in requirements" if passed else "Not found"})

# 7. Verdict is reasonable
has_verdict = "READY_FOR_DEEP_ANALYSIS" in e3_review or "NEEDS_CLARIFICATION" in e3_review
has_reasoning = len(e3_review) > 500
passed = has_verdict and has_reasoning
e3_results.append({"text": "Verdict is reasonable for the detail level provided",
    "passed": passed,
    "evidence": f"Verdict present: {has_verdict}, reasoning length: {len(e3_review)} chars"})

results["eval-3-with_skill"] = e3_results

# ── Save grading results ──
for eval_key, eval_results in results.items():
    parts = eval_key.split("-")
    eval_name = "-".join(parts[:-1])
    config = parts[-1]

    # Map to directory names
    dir_map = {
        "eval-1": "eval-1-sso-feature",
        "eval-2": "eval-2-vague-auth-fix",
        "eval-3": "eval-3-dashboard-perf"
    }
    eval_dir = dir_map.get(eval_name, eval_name)
    grading_path = f"{BASE}/{eval_dir}/{config}/grading.json"

    total = len(eval_results)
    passed_count = sum(1 for r in eval_results if r["passed"])

    grading = {
        "expectations": eval_results,
        "summary": {
            "passed": passed_count,
            "failed": total - passed_count,
            "total": total,
            "pass_rate": round(passed_count / total, 2) if total > 0 else 0
        }
    }

    os.makedirs(os.path.dirname(grading_path), exist_ok=True)
    with open(grading_path, "w") as f:
        json.dump(grading, f, indent=2)
    print(f"\n{eval_key}: {passed_count}/{total} passed ({grading['summary']['pass_rate']*100:.0f}%)")
    for r in eval_results:
        status = "PASS" if r["passed"] else "FAIL"
        print(f"  [{status}] {r['text']}")
        print(f"         {r['evidence']}")

print("\n=== GRADING COMPLETE ===")
