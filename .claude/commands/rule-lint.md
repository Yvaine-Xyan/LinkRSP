Validate one or more LinkRSP rule YAML files against the project schema. If no file is specified, validate all recently modified rule YAMLs.

**Steps:**

1. Identify the target YAML file(s) from the argument or from recent edits in this session.

2. Read each file and check ALL of the following:

   **Universal required fields** (every rule):
   - `rule_id`: must match pattern `R-\d{3}` or `S-\d{3}[ab]?`
   - `version`: must be exactly `"1.0"`
   - `category`: must be one of `r_class`, `s_class`, `mixed`
   - `trigger`: must be one of `pre_publication`, `post_execution`, `ipo_scan`, `registration`, `periodic_audit`
   - `severity`: must be one of `block`, `warn`, `flag`
   - `description`: non-empty string
   - `inputs`: non-empty list
   - `condition`: non-empty string
   - `action`: non-empty string
   - `false_positive_notes`: non-empty string

   **S-class additional required fields** (if category = s_class):
   - `audit_method`: must be `"llm_oracle"`
   - `queue`: must be `"semantic_audit_jobs"`
   - `prompt_template`: non-empty, must contain at least one `{{placeholder}}`
   - `few_shot_examples`: list with ≥ 4 items; must include ≥ 2 with `verdict: PASS` and ≥ 2 with `verdict: BLOCK`; each item must have `input`, `verdict`, `rationale`
   - `confidence_threshold.flag`: number between 0 and 1
   - `confidence_threshold.block`: number > `confidence_threshold.flag`
   - `reasonableness_anchor`: non-empty string

   **Cross-community additional required fields** (S-015, S-016):
   - `implementation_phase`: must be `phase_1_manual` | `phase_2_automated` | `phase_3_advanced`
   - `comparison_window`: present and non-empty
   - `feature_extraction`: present
   - `requires_features`: non-empty list
   - `llm_role`: present

   **Content sanity checks** (all rules):
   - `description` must not describe credits as "currency", "money", "token", "payment"
   - If rule references a time window (e.g. "24h", "30d", "90d"), verify it matches the frozen values in `docs/spec/time-and-window-conventions-v1.0.md`

3. Report results:
   - For each violation: `[FAIL] field_name — reason` with the rule_id and file path
   - For each passed check: `[OK]`
   - Summary line: `N/M checks passed for rule_id`

4. If all checks pass: output `✓ Rule YAML valid: rule_id`
   If any fail: output `✗ rule_id has N violation(s) — fix before committing`
