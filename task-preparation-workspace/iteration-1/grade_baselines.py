#!/usr/bin/env python3
"""Grading script for baseline (without_skill) runs."""
import json
import os

BASE = "/home/nolood/general/plan-pipeline/task-preparation-workspace/iteration-1"

def file_exists(path):
    return os.path.isfile(path)

def read_file(path):
    if not os.path.isfile(path):
        return ""
    with open(path) as f:
        return f.read()

def contains_any(text, terms):
    text_lower = text.lower()
    return any(t.lower() in text_lower for t in terms)

def count_items_in_section(text, section_header):
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

# ── Eval 1: SSO feature (without_skill) ──
print("=== EVAL 1: SSO Feature (without_skill) ===")
e1_dir = f"{BASE}/eval-1-sso-feature/without_skill/outputs"
# Baseline used requirements.md not requirements.draft.md
e1_req = read_file(f"{e1_dir}/requirements.md") or read_file(f"{e1_dir}/requirements.draft.md")
e1_review = read_file(f"{e1_dir}/readiness-review.md")

e1_results = []

# 1. requirements doc exists (any name)
passed = file_exists(f"{e1_dir}/requirements.md") or file_exists(f"{e1_dir}/requirements.draft.md")
e1_results.append({"text": "requirements.draft.md file was created", "passed": passed,
    "evidence": f"Found requirements doc: {'requirements.md' if file_exists(f'{e1_dir}/requirements.md') else 'requirements.draft.md' if file_exists(f'{e1_dir}/requirements.draft.md') else 'MISSING'}"})

# 2. All required sections
sections = ["Goal", "Problem Statement", "Scope", "Constraints", "Dependencies", "Knowns", "Unknowns", "Assumptions"]
# Baseline may use different section names
alt_sections = {"Goal": ["Overview", "Objective"], "Problem Statement": ["Context", "Background"],
                "Dependencies": ["Dependencies & Context", "Context & Constraints"],
                "Constraints": ["Constraints", "Non-Functional"]}
found_sections = []
for s in sections:
    if f"## {s}" in e1_req or s.lower() in e1_req.lower():
        found_sections.append(s)
    else:
        alts = alt_sections.get(s, [])
        for alt in alts:
            if alt.lower() in e1_req.lower():
                found_sections.append(s)
                break
missing = [s for s in sections if s not in found_sections]
passed = len(missing) <= 2  # lenient for baseline
e1_results.append({"text": "Requirements draft contains all required sections",
    "passed": len(missing) == 0,
    "evidence": f"Missing sections: {missing}" if missing else "All sections present"})

# 3. readiness-review.md exists
passed = file_exists(f"{e1_dir}/readiness-review.md")
e1_results.append({"text": "readiness-review.md file was created with a verdict", "passed": passed,
    "evidence": "File missing — baseline did not create a readiness review" if not passed else "File exists"})

# 4. Verdict is READY_FOR_DEEP_ANALYSIS
passed = "READY_FOR_DEEP_ANALYSIS" in e1_review
e1_results.append({"text": "Readiness critic returned READY_FOR_DEEP_ANALYSIS",
    "passed": passed,
    "evidence": "No readiness review file — no verdict" if not e1_review else "Verdict found" if passed else "Wrong verdict"})

# 5. Task type is feature/integration
passed = contains_any(e1_req, ["feature", "integration"])
e1_results.append({"text": "Task correctly classified as feature or integration",
    "passed": passed,
    "evidence": "Found" if passed else "Not explicitly classified"})

# 6. Knowns mention tech stack
tech_terms = ["keycloak", "fastapi", "react", "per-tenant", "saml"]
found = [t for t in tech_terms if t.lower() in e1_req.lower()]
passed = len(found) >= 4
e1_results.append({"text": "Knowns section mentions key technical context",
    "passed": passed,
    "evidence": f"Found {len(found)}/5 terms: {found}"})

# 7. No clarifications.md (for READY verdict)
passed = not file_exists(f"{e1_dir}/clarifications.md")
e1_results.append({"text": "No clarifications.md was created",
    "passed": passed,
    "evidence": "Absent" if passed else "Present"})

results["eval-1-without_skill"] = e1_results

# ── Eval 2: Vague auth (without_skill) ──
print("\n=== EVAL 2: Vague Auth Fix (without_skill) ===")
e2_dir = f"{BASE}/eval-2-vague-auth-fix/without_skill/outputs"
e2_req = read_file(f"{e2_dir}/requirements.draft.md") or read_file(f"{e2_dir}/requirements.md")
e2_review = read_file(f"{e2_dir}/readiness-review.md")
e2_clar = read_file(f"{e2_dir}/clarifications.md") or read_file(f"{e2_dir}/questions.md")

e2_results = []

passed = file_exists(f"{e2_dir}/requirements.draft.md") or file_exists(f"{e2_dir}/requirements.md")
e2_results.append({"text": "requirements.draft.md file was created (even if sparse)", "passed": passed,
    "evidence": "File exists" if passed else "Missing"})

passed = file_exists(f"{e2_dir}/readiness-review.md")
e2_results.append({"text": "readiness-review.md file was created with a verdict", "passed": passed,
    "evidence": "File exists" if passed else "Missing"})

verdict_line = next((l for l in e2_review.split("\n") if "Verdict" in l), "") if e2_review else ""
passed = "NEEDS_CLARIFICATION" in verdict_line
e2_results.append({"text": "Readiness critic returned NEEDS_CLARIFICATION",
    "passed": passed,
    "evidence": f"Verdict line: {verdict_line}" if verdict_line else "No readiness review"})

passed = file_exists(f"{e2_dir}/clarifications.md") or file_exists(f"{e2_dir}/questions.md")
e2_results.append({"text": "clarifications.md was created with specific questions", "passed": passed,
    "evidence": "File exists" if passed else "Missing"})

question_count = e2_clar.count("?") if e2_clar else 0
has_specific = contains_any(e2_clar, ["error", "log", "which", "what", "who", "when", "how"]) if e2_clar else False
passed = question_count >= 3 and has_specific
e2_results.append({"text": "Clarification questions are specific and actionable",
    "passed": passed,
    "evidence": f"{question_count} questions, specific terms: {has_specific}"})

unknowns_count = count_items_in_section(e2_req, "## Unknowns")
knowns_count = count_items_in_section(e2_req, "## Knowns")
passed = unknowns_count > knowns_count
e2_results.append({"text": "Requirements draft has more unknowns than knowns",
    "passed": passed,
    "evidence": f"Knowns: {knowns_count}, Unknowns: {unknowns_count}"})

invented_terms = ["jwt", "oauth2", "redis", "postgres", "mongodb", "express", "django", "spring"]
knowns_section = ""
if "## Knowns" in e2_req:
    start = e2_req.index("## Knowns")
    end_idx = e2_req.find("##", start + 10)
    knowns_section = e2_req[start:end_idx] if end_idx > 0 else e2_req[start:]
specific_tech = [t for t in invented_terms if t.lower() in knowns_section.lower()]
passed = len(specific_tech) == 0
e2_results.append({"text": "Agent did NOT invent specific technical details without evidence",
    "passed": passed,
    "evidence": f"Invented in Knowns: {specific_tech}" if specific_tech else "Clean"})

results["eval-2-without_skill"] = e2_results

# ── Eval 3: Dashboard perf (without_skill) ──
print("\n=== EVAL 3: Dashboard Performance (without_skill) ===")
e3_dir = f"{BASE}/eval-3-dashboard-perf/without_skill/outputs"
e3_req = read_file(f"{e3_dir}/requirements.md") or read_file(f"{e3_dir}/requirements.draft.md")
e3_review = read_file(f"{e3_dir}/readiness-review.md")

e3_results = []

passed = (file_exists(f"{e3_dir}/requirements.md") or file_exists(f"{e3_dir}/requirements.draft.md"))
sections_check = ["Objective", "Scope", "Constraints", "Success"]  # baseline uses different headers
found = sum(1 for s in sections_check if s.lower() in e3_req.lower())
passed = passed and found >= 3
e3_results.append({"text": "requirements.draft.md file was created with all required sections",
    "passed": passed,
    "evidence": f"Found {found}/4 section variants"})

# Baseline has no readiness review
passed = file_exists(f"{e3_dir}/readiness-review.md")
e3_results.append({"text": "readiness-review.md with all 8 criteria scored",
    "passed": passed,
    "evidence": "Missing — baseline did not create readiness review" if not passed else "Exists"})

passed = contains_any(e3_req, ["performance", "optimization", "slow", "latency"])
e3_results.append({"text": "Task correctly classified as bug or performance issue",
    "passed": passed,
    "evidence": "Performance terms found" if passed else "Not found"})

passed = "2 second" in e3_req.lower() or "under 2s" in e3_req.lower() or "< 2" in e3_req or "2-second" in e3_req.lower()
e3_results.append({"text": "Goal states the performance target: under 2 seconds",
    "passed": passed,
    "evidence": "Target found" if passed else "Not found"})

tech = ["clickhouse", "graphql", "apps/dashboard", "services/analytics-api", "50"]
found_tech = [t for t in tech if t.lower() in e3_req.lower()]
passed = len(found_tech) >= 4
e3_results.append({"text": "Knowns mention ClickHouse, GraphQL, file paths, 50k DAU",
    "passed": passed,
    "evidence": f"Found {len(found_tech)}/5: {found_tech}"})

passed = contains_any(e3_req, ["query", "rendering", "bottleneck", "root cause"])
e3_results.append({"text": "Unknowns include root cause uncertainty",
    "passed": passed,
    "evidence": "Root cause terms found" if passed else "Not found"})

has_verdict = "READY_FOR_DEEP_ANALYSIS" in (e3_review or "") or "NEEDS_CLARIFICATION" in (e3_review or "")
passed = has_verdict
e3_results.append({"text": "Verdict is reasonable for the detail level provided",
    "passed": passed,
    "evidence": "No readiness review — no verdict" if not passed else "Verdict present"})

results["eval-3-without_skill"] = e3_results

# ── Save grading results ──
for eval_key, eval_results in results.items():
    parts = eval_key.rsplit("-", 1)
    eval_name = parts[0]
    config = parts[1]

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

print("\n=== BASELINE GRADING COMPLETE ===")
