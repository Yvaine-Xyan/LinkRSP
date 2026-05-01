#!/usr/bin/env bash
# Phase C 最小试点种子脚本
# 用途：本地/staging 验证两人一次任务完整链路
# 用法：BASE_URL=http://localhost:9090 SECRET=xxx bash scripts/seed/pilot_seed.sh
#
# 注意：此脚本使用真实格式数据（规则引擎会正常执行），但时间戳为过去固定值。
# 若需要测试真实当前时间，手动替换 START_TIME / END_TIME / ATTEST_TIME。

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:9090}"
SECRET="${SECRET:-}"
ALICE="${ALICE:-alice-pilot-01}"

if [ -z "$SECRET" ]; then
  echo "ERROR: SECRET environment variable is required (LINKRSP_API_SHARED_SECRET)"
  exit 1
fi

H_AUTH="X-API-Shared-Secret: $SECRET"
H_CT="Content-Type: application/json"

START_TIME="2026-05-01T09:00:00Z"
END_TIME="2026-05-01T09:30:00Z"
ATTEST_TIME="2026-05-01T09:32:00Z"
LOCATION_HASH="gcj02:39.9042_116.4074"

log() { echo "[pilot_seed] $*"; }

# ── Step 1: Create task ────────────────────────────────────────────────────────
log "Creating task for uid=$ALICE ..."
TASK_RESP=$(curl -sf -X POST "$BASE_URL/api/v1/tasks" \
  -H "$H_AUTH" -H "$H_CT" \
  -d "{
    \"uid_submitter\": \"$ALICE\",
    \"description_text\": \"搬书 3 箱，3 楼，约 30 分钟 [Phase C pilot]\",
    \"start_time_utc\": \"$START_TIME\",
    \"end_time_utc\":   \"$END_TIME\"
  }")
TASK_ID=$(echo "$TASK_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['task_id'])")
log "  task_id = $TASK_ID"

# ── Step 2: Submit attestation (V=1) ──────────────────────────────────────────
log "Submitting V=1 attestation ..."
ATTEST_RESP=$(curl -sf -X POST "$BASE_URL/api/v1/tasks/$TASK_ID/attestations" \
  -H "$H_AUTH" -H "$H_CT" \
  -d "{
    \"verification_level\": 1,
    \"timestamp_utc\": \"$ATTEST_TIME\",
    \"location_hash\": \"$LOCATION_HASH\",
    \"evidence_ref\": \"sha256:pilot-seed-placeholder-0000000000000001\"
  }")
ATTEST_ID=$(echo "$ATTEST_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['attestation_id'])")
log "  attestation_id = $ATTEST_ID"

# ── Step 3: Settlement preview ────────────────────────────────────────────────
log "Previewing settlement ..."
PREVIEW=$(curl -sf -X POST "$BASE_URL/api/v1/tasks/$TASK_ID/settlement/preview" \
  -H "$H_AUTH" -H "$H_CT")
CREDITS=$(echo "$PREVIEW" | python3 -c "import sys,json; print(json.load(sys.stdin)['formula']['credits_delta'])")
RULES=$(echo "$PREVIEW" | python3 -c "import sys,json; print(json.load(sys.stdin)['rules_checked'])")
log "  credits_delta (preview) = $CREDITS"
log "  rules_checked           = $RULES"

# ── Step 4: Settlement commit ─────────────────────────────────────────────────
log "Committing settlement ..."
COMMIT=$(curl -sf -X POST "$BASE_URL/api/v1/tasks/$TASK_ID/settlement/commit" \
  -H "$H_AUTH" -H "$H_CT")
ENTRY_ID=$(echo "$COMMIT" | python3 -c "import sys,json; print(json.load(sys.stdin)['ledger_entry_id'])")
log "  ledger_entry_id = $ENTRY_ID"

# Idempotency check: commit again should return same entry
COMMIT2=$(curl -sf -X POST "$BASE_URL/api/v1/tasks/$TASK_ID/settlement/commit" \
  -H "$H_AUTH" -H "$H_CT")
ENTRY_ID2=$(echo "$COMMIT2" | python3 -c "import sys,json; print(json.load(sys.stdin)['ledger_entry_id'])")
if [ "$ENTRY_ID" != "$ENTRY_ID2" ]; then
  echo "FAIL: idempotent commit returned different entry: $ENTRY_ID vs $ENTRY_ID2"
  exit 1
fi
log "  idempotency OK (same ledger_entry_id on repeat commit)"

# ── Step 5: Verify ledger projection ─────────────────────────────────────────
log "Verifying ledger projection ..."
LEDGER=$(curl -sfG "$BASE_URL/api/v1/internal/lrs-ledger-query" \
  -H "$H_AUTH" \
  --data-urlencode "uid=$ALICE" \
  --data-urlencode "window_start_utc=2026-05-01T00:00:00Z" \
  --data-urlencode "window_end_utc=2026-05-02T00:00:00Z")
PROJ_BAL=$(echo "$LEDGER" | python3 -c "import sys,json; print(json.load(sys.stdin)['projected_balance'])")
ENTRY_COUNT=$(echo "$LEDGER" | python3 -c "import sys,json; print(json.load(sys.stdin)['entry_count'])")
log "  entry_count       = $ENTRY_COUNT"
log "  projected_balance = $PROJ_BAL"
if [ "$PROJ_BAL" != "$CREDITS" ]; then
  echo "FAIL: projected_balance ($PROJ_BAL) != preview credits_delta ($CREDITS)"
  exit 1
fi
log "  ledger projection matches preview ✓"

# ── Step 6: Verify audit events ───────────────────────────────────────────────
log "Verifying audit events ..."
AUDIT=$(curl -sfG "$BASE_URL/api/v1/audit-events" \
  -H "$H_AUTH" \
  --data-urlencode "subject_type=task" \
  --data-urlencode "subject_id=$TASK_ID")
TOTAL=$(echo "$AUDIT" | python3 -c "import sys,json; print(json.load(sys.stdin)['total'])")
log "  audit event total = $TOTAL (expected >= 6)"
if [ "$TOTAL" -lt 6 ]; then
  echo "FAIL: expected at least 6 audit events, got $TOTAL"
  exit 1
fi
log "  audit events OK ✓"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════"
echo "  Phase C pilot seed: ALL CHECKS PASSED ✓"
echo "  task_id         = $TASK_ID"
echo "  attestation_id  = $ATTEST_ID"
echo "  ledger_entry_id = $ENTRY_ID"
echo "  credits_delta   = $CREDITS"
echo "  audit_events    = $TOTAL"
echo "══════════════════════════════════════════════"
