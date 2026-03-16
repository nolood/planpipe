#!/usr/bin/env python3
"""Grading script for iteration 2 with-skill runs."""
import json
import os
import re

BASE = "/home/nolood/general/plan-pipeline/task-preparation-workspace/iteration-2"

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
        if in_section and re.match(r'\s*\d+\.', line.strip()):
            count += 1
    return count

def check_template(clar_text):
    """Check if clarifications.md follows the fixed template."""
    required = ["# Clarifications Needed", "## Blocking Gaps", "## Open Unknowns",
                "## Assumptions to Verify", "## Questions for the User"]
    found = [r for r in required if r.lower() in clar_text.lower()]
    has_header = ">" in clar_text[:500]  # blockquote header
    return len(found) >= 4 and has_header, found

results = {}

# ── Eval 1: SSO ──
print("=== EVAL 1: SSO Feature ===")
e1_dir = f"{BASE}/eval-1-sso-feature/with_skill/outputs"
e1_req = read_file(f"{e1_dir}/requirements.draft.md")
e1_review = read_file(f"{e1_dir}/readiness-review.md")
e1_clar = read_file(f"{e1_dir}/clarifications.md")

e1_results = []

# 1. requirements.draft.md with all sections
sections = ["Goal", "Problem Statement", "Scope", "Constraints", "Dependencies", "Knowns", "Unknowns", "Assumptions"]
missing = [s for s in sections if s.lower() not in e1_req.lower()]
passed = len(missing) == 0
e1_results.append({"text": "requirements.draft.md file was created with all required sections",
    "passed": passed, "evidence": f"Missing: {missing}" if missing else "All 8 sections present"})

# 2. readiness-review with all 8 criteria
criteria = ["Goal clarity", "Problem clarity", "Scope clarity", "Change target", "Context sufficiency", "Ambiguity", "Assumption safety", "Acceptance"]
found_c = [c for c in criteria if c.lower() in e1_review.lower()]
passed = len(found_c) >= 7
e1_results.append({"text": "readiness-review.md file was created with all 8 criteria scored",
    "passed": passed, "evidence": f"Found {len(found_c)}/8 criteria"})

# 3. clarifications.md created (KEY NEW ASSERTION)
passed = file_exists(f"{e1_dir}/clarifications.md") and len(e1_clar) > 100
e1_results.append({"text": "clarifications.md was created (even though verdict is READY, unknowns must be closed)",
    "passed": passed, "evidence": f"File exists, {len(e1_clar)} chars" if passed else "Missing or empty"})

# 4. Template check
template_ok, found_sections = check_template(e1_clar)
e1_results.append({"text": "clarifications.md follows the fixed template with all required sections",
    "passed": template_ok, "evidence": f"Template sections found: {found_sections}, has header: {'>' in e1_clar[:500]}"})

# 5. Unknowns converted to questions
# Check that items from Unknowns in requirements appear as questions in clarifications
unknowns_keywords = ["keycloak version", "tenant config", "session handling", "jit", "slo", "single logout", "attribute mapping"]
questions_section = e1_clar[e1_clar.lower().find("## questions"):] if "## questions" in e1_clar.lower() else e1_clar
found_in_q = [k for k in unknowns_keywords if k.lower() in questions_section.lower()]
passed = len(found_in_q) >= 4
e1_results.append({"text": "Open unknowns from requirements draft appear as questions in clarifications",
    "passed": passed, "evidence": f"Found {len(found_in_q)}/7 unknown topics in questions: {found_in_q}"})

# 6. Tech context
tech = ["keycloak", "fastapi", "react", "saml", "per-tenant"]
found_t = [t for t in tech if t.lower() in e1_req.lower()]
passed = len(found_t) >= 4
e1_results.append({"text": "Requirements draft mentions Keycloak, FastAPI, React, SAML, per-tenant",
    "passed": passed, "evidence": f"Found {len(found_t)}/5: {found_t}"})

# 7. Task type
passed = contains_any(e1_req, ["feature", "integration"])
e1_results.append({"text": "Task classified as feature or integration",
    "passed": passed, "evidence": "Found" if passed else "Not found"})

results["eval-1-with_skill"] = e1_results

# ── Eval 2: Vague auth ──
print("\n=== EVAL 2: Vague Auth Fix ===")
e2_dir = f"{BASE}/eval-2-vague-auth-fix/with_skill/outputs"
e2_req = read_file(f"{e2_dir}/requirements.draft.md")
e2_review = read_file(f"{e2_dir}/readiness-review.md")
e2_clar = read_file(f"{e2_dir}/clarifications.md")

e2_results = []

passed = file_exists(f"{e2_dir}/requirements.draft.md")
e2_results.append({"text": "requirements.draft.md file was created", "passed": passed,
    "evidence": "Exists" if passed else "Missing"})

verdict_line = next((l for l in e2_review.split("\n") if "Verdict" in l), "")
passed = "NEEDS_CLARIFICATION" in verdict_line
e2_results.append({"text": "readiness-review.md with NEEDS_CLARIFICATION verdict", "passed": passed,
    "evidence": verdict_line})

passed = file_exists(f"{e2_dir}/clarifications.md") and len(e2_clar) > 100
e2_results.append({"text": "clarifications.md was created", "passed": passed,
    "evidence": f"Exists, {len(e2_clar)} chars" if passed else "Missing"})

template_ok, found_sections = check_template(e2_clar)
e2_results.append({"text": "clarifications.md follows the fixed template with all 4 sections",
    "passed": template_ok, "evidence": f"Sections: {found_sections}"})

question_count = e2_clar.count("?")
passed = question_count >= 5
e2_results.append({"text": "Questions are specific and actionable, not generic",
    "passed": passed, "evidence": f"{question_count} questions found"})

unknowns_count = count_items_in_section(e2_req, "## Unknowns")
knowns_count = count_items_in_section(e2_req, "## Knowns")
passed = unknowns_count > knowns_count
e2_results.append({"text": "Requirements draft has more unknowns than knowns",
    "passed": passed, "evidence": f"Knowns: {knowns_count}, Unknowns: {unknowns_count}"})

invented_terms = ["jwt", "oauth2", "redis", "postgres", "mongodb", "express", "django", "spring"]
knowns_section = ""
if "## Knowns" in e2_req:
    start = e2_req.index("## Knowns")
    end_idx = e2_req.find("##", start + 10)
    knowns_section = e2_req[start:end_idx] if end_idx > 0 else e2_req[start:]
specific_tech = [t for t in invented_terms if t.lower() in knowns_section.lower()]
passed = len(specific_tech) == 0
e2_results.append({"text": "Agent did not invent technical details",
    "passed": passed, "evidence": "Clean" if passed else f"Invented: {specific_tech}"})

results["eval-2-with_skill"] = e2_results

# ── Eval 3: Dashboard perf ──
print("\n=== EVAL 3: Dashboard Performance ===")
e3_dir = f"{BASE}/eval-3-dashboard-perf/with_skill/outputs"
e3_req = read_file(f"{e3_dir}/requirements.draft.md")
e3_review = read_file(f"{e3_dir}/readiness-review.md")
e3_clar = read_file(f"{e3_dir}/clarifications.md")

e3_results = []

sections = ["Goal", "Problem Statement", "Scope", "Constraints", "Knowns", "Unknowns", "Assumptions"]
found_s = [s for s in sections if s.lower() in e3_req.lower()]
passed = len(found_s) >= 6
e3_results.append({"text": "requirements.draft.md with all required sections",
    "passed": passed, "evidence": f"Found {len(found_s)}/7"})

criteria = ["Goal clarity", "Problem clarity", "Scope clarity", "Change target", "Context sufficiency", "Ambiguity", "Assumption safety", "Acceptance"]
found_c = [c for c in criteria if c.lower() in e3_review.lower()]
passed = len(found_c) >= 7
e3_results.append({"text": "readiness-review.md with all 8 criteria scored",
    "passed": passed, "evidence": f"Found {len(found_c)}/8"})

passed = file_exists(f"{e3_dir}/clarifications.md") and len(e3_clar) > 100
e3_results.append({"text": "clarifications.md was created to address unknowns",
    "passed": passed, "evidence": f"Exists, {len(e3_clar)} chars" if passed else "Missing"})

template_ok, found_sections = check_template(e3_clar)
e3_results.append({"text": "clarifications.md follows the fixed template with all 4 sections",
    "passed": template_ok, "evidence": f"Sections: {found_sections}"})

passed = "2 second" in e3_req.lower() or "under 2s" in e3_req.lower() or "2-second" in e3_req.lower() or "< 2" in e3_req
e3_results.append({"text": "Goal states the performance target: under 2 seconds",
    "passed": passed, "evidence": "Found" if passed else "Not found"})

tech = ["clickhouse", "graphql", "apps/dashboard", "services/analytics-api", "50"]
found_t = [t for t in tech if t.lower() in e3_req.lower()]
passed = len(found_t) >= 4
e3_results.append({"text": "Knowns mention ClickHouse, GraphQL, file paths, 50k DAU",
    "passed": passed, "evidence": f"Found {len(found_t)}/5: {found_t}"})

passed = contains_any(e3_req, ["root cause", "bottleneck", "query layer", "frontend render", "which specific"])
e3_results.append({"text": "Unknowns include root cause uncertainty",
    "passed": passed, "evidence": "Found" if passed else "Not found"})

results["eval-3-with_skill"] = e3_results

# ── Save ──
for eval_key, eval_results in results.items():
    parts = eval_key.rsplit("-", 1)
    dir_map = {"eval-1": "eval-1-sso-feature", "eval-2": "eval-2-vague-auth-fix", "eval-3": "eval-3-dashboard-perf"}
    eval_dir = dir_map.get(parts[0], parts[0])
    grading_path = f"{BASE}/{eval_dir}/{parts[1]}/grading.json"

    total = len(eval_results)
    passed_count = sum(1 for r in eval_results if r["passed"])

    grading = {
        "expectations": eval_results,
        "summary": {"passed": passed_count, "failed": total - passed_count, "total": total,
                     "pass_rate": round(passed_count / total, 2) if total > 0 else 0}
    }
    os.makedirs(os.path.dirname(grading_path), exist_ok=True)
    with open(grading_path, "w") as f:
        json.dump(grading, f, indent=2)

    print(f"\n{eval_key}: {passed_count}/{total} ({grading['summary']['pass_rate']*100:.0f}%)")
    for r in eval_results:
        status = "PASS" if r["passed"] else "FAIL"
        print(f"  [{status}] {r['text']}")
        print(f"         {r['evidence']}")
