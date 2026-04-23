Cross-check consistency across the key LinkRSP specification documents. Run this whenever spec files are edited or when writing new rules that reference time windows or formula parameters.

**Steps:**

1. Read the following files (use cached content if recently read in session):
   - `docs/spec/linkrsp-core-algorithm-spec-v1.0.md`
   - `docs/spec/time-and-window-conventions-v1.0.md`
   - `docs/spec/audit-event-schema-v1.0.md`
   - `docs/spec/openapi-v1.0-draft.md`

2. Check **formula parameter consistency** across all rule YAMLs and spec docs:
   - `D_base` = 1.0 everywhere (never configurable in LRS-1.0)
   - `Clip` bounds = (0.8, 3.0) — lower bound = dignity floor, upper = anti-arbitrage cap
   - `V_bit` values = V0:0.1, V1:0.5, V2:1.0 only
   - `K_global` genesis lock = 1.0 until min(1000 S-tasks, 90 days)
   - Any rule that uses `K_global` references the genesis period correctly

3. Check **time window consistency** — compare rule YAMLs against `time-and-window-conventions-v1.0.md`:
   - R-001 overlap: closed interval `[start, end]` inclusive
   - R-002 speed: 200 km/h threshold via Haversine
   - R-005 daily hours: 24h rolling from task `start_time`
   - R-008 D-value: rolling 30d, min sample 50 S-level tasks
   - R-010 post-genesis: K_global genesis endpoint + 7d (168h, not calendar days)
   - S-015 comparison window: 90d
   - S-016 voting window: 180d
   - Insufficient sample fallback (N<50 for R-008, N<5 for S-015, N<3 for S-016): `human_review_queue`, not auto-block

4. Check **audit event schema consistency**:
   - Any rule YAML that specifies `action_taken` values uses only valid values from audit-event-schema
   - S-class rules specify `confidence` field in their audit output
   - Cross-community rules include `feature_summary` with comparison_window snapshot

5. Check **credits-not-currency language** across all spec docs:
   - No spec document describes LRS credits as "currency", "coin", "token", or "guaranteed fiat claim"

6. Report:
   - `[INCONSISTENCY] description — found in: file1 vs file2`
   - `[OK] check_name — consistent`
   - Final summary: `Spec cross-references consistent ✓` or `N inconsistencies found — review before proceeding`
