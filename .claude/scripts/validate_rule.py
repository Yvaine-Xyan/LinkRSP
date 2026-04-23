#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import sys, os
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf-8-sig"):
    sys.stdout = open(sys.stdout.fileno(), mode="w", encoding="utf-8", buffering=1)
"""
LinkRSP Rule YAML Validator
Usage: py validate_rule.py <path/to/rule.yaml> [path2.yaml ...]
       py validate_rule.py --all   # validate all rules in docs/governance/rule-engine/
"""

import re
import glob

try:
    import yaml
except ImportError:
    print("[ERROR] PyYAML not installed. Run: pip3 install pyyaml")
    sys.exit(1)

CATEGORY_RE       = re.compile(r'^[a-z][a-z0-9_]*$')
VALID_TRIGGERS    = {"pre_execution", "pre_publication", "post_execution",
                     "ipo_scan", "registration", "periodic_audit"}
VALID_SEVERITIES  = {"block", "warn", "flag", "flag_for_audit", "block_pending_review"}
VALID_IMPL_PHASES = {"phase_1_manual", "phase_2_automated", "phase_3_advanced"}
CURRENCY_WORDS    = {"currency", "money", "token", "payment", "coin", "fiat claim"}

RULE_ID_RE = re.compile(r'^[RS]-\d{3}[ab]?$')

CROSS_COMMUNITY_IDS = {"S-015", "S-016"}


def check(failures, condition, message):
    if not condition:
        failures.append(message)


def validate_rule(path):
    failures = []
    warnings = []

    with open(path, "r", encoding="utf-8") as f:
        try:
            rule = yaml.safe_load(f)
        except yaml.YAMLError as e:
            return [f"YAML parse error: {e}"], []

    if not isinstance(rule, dict):
        return ["File does not contain a YAML mapping at top level"], []

    # ── Universal required fields ──────────────────────────────────
    rule_id = rule.get("rule_id", "")
    check(failures, RULE_ID_RE.match(str(rule_id)),
          f"rule_id '{rule_id}' must match R-\\d{{3}} or S-\\d{{3}}[ab]?")

    check(failures, rule.get("version") == "1.0",
          f"version must be '1.0', got: {rule.get('version')!r}")

    cat = rule.get("category", "")
    check(failures, cat and CATEGORY_RE.match(str(cat)),
          f"category must be a non-empty snake_case string, got: {cat!r}")

    trigger = rule.get("trigger", "")
    check(failures, trigger in VALID_TRIGGERS,
          f"trigger must be one of {VALID_TRIGGERS}, got: {trigger!r}")

    sev = rule.get("severity", "")
    check(failures, sev in VALID_SEVERITIES,
          f"severity must be one of {VALID_SEVERITIES}, got: {sev!r}")

    # R-class requires condition + inputs + action; S-class uses condition_notes instead
    is_s_class = str(rule_id).startswith("S-")

    if not is_s_class:
        for field in ("condition", "action", "false_positive_notes"):
            val = rule.get(field)
            check(failures, val and str(val).strip(),
                  f"R-class: '{field}' is required and must be non-empty")
        inputs = rule.get("inputs")
        check(failures, isinstance(inputs, list) and len(inputs) > 0,
              "R-class: 'inputs' must be a non-empty list")

    # Currency language check
    desc = str(rule.get("description", "")).lower()
    for word in CURRENCY_WORDS:
        if word in desc:
            failures.append(f"description contains forbidden currency language: '{word}'")

    # ── S-class additional fields ─────────────────────────────────
    # Only apply LLM-oracle checks when audit_method == llm_oracle
    # Mixed rules (e.g. S-009b) use rule_engine_with_ledger_query and skip these
    is_llm_oracle = rule.get("audit_method", "") == "llm_oracle"
    if is_s_class and is_llm_oracle:
        check(failures, rule.get("audit_method") == "llm_oracle",
              "S-class: 'audit_method' must be 'llm_oracle'")

        check(failures, bool(rule.get("queue", "").strip()),
              "S-class: 'queue' is required (e.g. semantic_gray_queue)")

        pt = str(rule.get("prompt_template", ""))
        check(failures, pt.strip(), "S-class: 'prompt_template' is required")
        check(failures, "{{" in pt, "S-class: 'prompt_template' must contain at least one {{placeholder}}")

        examples = rule.get("few_shot_examples", [])
        check(failures, isinstance(examples, list) and len(examples) >= 4,
              f"S-class: 'few_shot_examples' requires >=4 items, got {len(examples)}")

        if isinstance(examples, list):
            # Support both label/text/why and input/verdict/rationale schemas
            def get_verdict(e):
                return e.get("verdict") or e.get("label", "")
            pass_count  = sum(1 for e in examples if isinstance(e, dict) and get_verdict(e).upper() == "PASS")
            block_count = sum(1 for e in examples if isinstance(e, dict) and get_verdict(e).upper() == "BLOCK")
            check(failures, pass_count >= 2,
                  f"S-class: few_shot_examples needs >=2 PASS examples, got {pass_count}")
            check(failures, block_count >= 2,
                  f"S-class: few_shot_examples needs >=2 BLOCK examples, got {block_count}")

            for i, ex in enumerate(examples):
                if not isinstance(ex, dict):
                    failures.append(f"few_shot_examples[{i}]: must be a mapping")
                    continue
                has_text    = ex.get("input") or ex.get("text")
                has_verdict = ex.get("verdict") or ex.get("label")
                has_reason  = ex.get("rationale") or ex.get("why")
                check(failures, has_text,    f"few_shot_examples[{i}]: missing text/input field")
                check(failures, has_verdict, f"few_shot_examples[{i}]: missing verdict/label field")
                check(failures, has_reason,  f"few_shot_examples[{i}]: missing rationale/why field")

        # confidence_threshold is optional (may be in condition_notes instead)
        ct = rule.get("confidence_threshold")
        if ct is not None and isinstance(ct, dict):
            flag_t  = ct.get("flag")
            block_t = ct.get("block")
            if flag_t is not None:
                check(failures, isinstance(flag_t, (int, float)) and 0 < flag_t < 1,
                      f"S-class: confidence_threshold.flag must be float in (0,1), got {flag_t!r}")
            if block_t is not None:
                check(failures, isinstance(block_t, (int, float)) and 0 < block_t < 1,
                      f"S-class: confidence_threshold.block must be float in (0,1), got {block_t!r}")
            if isinstance(flag_t, (int, float)) and isinstance(block_t, (int, float)):
                check(failures, block_t > flag_t,
                      f"S-class: confidence_threshold.block ({block_t}) must be > flag ({flag_t})")

        if not rule.get("reasonableness_anchor", ""):
            warnings.append("S-class: 'reasonableness_anchor' missing (recommended)")

    # ── Cross-community additional fields ──────────────────────────
    if str(rule_id) in CROSS_COMMUNITY_IDS:
        impl = rule.get("implementation_phase", "")
        check(failures, impl in VALID_IMPL_PHASES,
              f"Cross-community: 'implementation_phase' must be one of {VALID_IMPL_PHASES}, got: {impl!r}")

        check(failures, rule.get("comparison_window") is not None,
              "Cross-community: 'comparison_window' is required")

        check(failures, rule.get("feature_extraction") is not None,
              "Cross-community: 'feature_extraction' is required")

        feats = rule.get("requires_features", [])
        check(failures, isinstance(feats, list) and len(feats) > 0,
              "Cross-community: 'requires_features' must be a non-empty list")

        check(failures, rule.get("llm_role", "").strip(),
              "Cross-community: 'llm_role' is required")

    return failures, warnings


def find_all_rules(base="docs/governance/rule-engine"):
    return sorted(glob.glob(os.path.join(base, "**", "*.yaml"), recursive=True))


def main():
    args = sys.argv[1:]

    if not args or args == ["--all"]:
        paths = find_all_rules()
        if not paths:
            print("[WARN] No rule YAML files found in docs/governance/rule-engine/")
            sys.exit(0)
    else:
        paths = args

    total_files  = 0
    total_issues = 0

    for path in paths:
        if not os.path.isfile(path):
            print(f"[ERROR] File not found: {path}")
            total_issues += 1
            continue

        total_files += 1
        failures, warnings = validate_rule(path)

        rule_label = os.path.basename(path)
        if not failures and not warnings:
            print(f"OK {rule_label}")
        else:
            for w in warnings:
                print(f"  [WARN]  {rule_label}: {w}")
            for f in failures:
                print(f"  [FAIL]  {rule_label}: {f}")
                total_issues += 1

    print()
    if total_issues == 0:
        print(f"OK All {total_files} rule(s) valid.")
        sys.exit(0)
    else:
        print(f"FAIL {total_issues} issue(s) found across {total_files} file(s).")
        sys.exit(1)


if __name__ == "__main__":
    main()
