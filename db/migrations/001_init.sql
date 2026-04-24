-- LinkRSP Phase B — 初始 Schema
-- 规范参考: docs/db/schema-v1.0-draft.md
-- 约束 C-6: audit_events / ledger_entries 只允许 INSERT，禁止 UPDATE/DELETE
-- 所有时间字段 UTC (timestamptz)，约束 C-5

-- ── tasks ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tasks (
    task_id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id     UUID        NULL,
    uid_submitter    TEXT        NOT NULL,
    description_text TEXT        NOT NULL,
    start_time_utc   TIMESTAMPTZ NOT NULL,
    end_time_utc     TIMESTAMPTZ NOT NULL,
    location_hash    TEXT        NULL,
    created_at_utc   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT tasks_time_order CHECK (end_time_utc > start_time_utc)
);

CREATE INDEX IF NOT EXISTS idx_tasks_uid_time
    ON tasks (uid_submitter, start_time_utc, end_time_utc);
CREATE INDEX IF NOT EXISTS idx_tasks_community
    ON tasks (community_id, created_at_utc);

-- ── attestations ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS attestations (
    attestation_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id            UUID        NOT NULL REFERENCES tasks (task_id),
    verification_level SMALLINT    NOT NULL CHECK (verification_level IN (0, 1, 2)),
    timestamp_utc      TIMESTAMPTZ NOT NULL,
    location_lat       DOUBLE PRECISION NULL,
    location_lng       DOUBLE PRECISION NULL,
    location_hash      TEXT        NULL,
    evidence_ref       TEXT        NULL,
    created_at_utc     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attestations_task
    ON attestations (task_id);
CREATE INDEX IF NOT EXISTS idx_attestations_level_time
    ON attestations (verification_level, timestamp_utc);

-- ── ledger_entries (append-only, 约束 C-6) ───────────────────────────────────
CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id          UUID    NOT NULL REFERENCES tasks (task_id),
    uid              TEXT    NOT NULL,
    credits_delta    NUMERIC NOT NULL,
    formula_snapshot JSONB   NULL,   -- D_base/K_global/Clip bounds 快照
    created_at_utc   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idempotency_key  TEXT    NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_ledger_uid_time
    ON ledger_entries (uid, created_at_utc);
CREATE INDEX IF NOT EXISTS idx_ledger_task
    ON ledger_entries (task_id);

-- ── audit_events (append-only, 约束 C-6) ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_events (
    event_id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_version   TEXT        NOT NULL DEFAULT '1.0',
    payload          JSONB       NOT NULL,  -- 完整 audit_event JSON
    rule_id          TEXT        NOT NULL,
    subject_type     TEXT        NOT NULL,
    subject_id       TEXT        NOT NULL,
    trigger          TEXT        NOT NULL,
    timestamp_utc    TIMESTAMPTZ NOT NULL,
    idempotency_key  TEXT        NOT NULL UNIQUE,
    created_at_utc   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_rule_time
    ON audit_events (rule_id, timestamp_utc);
CREATE INDEX IF NOT EXISTS idx_audit_subject
    ON audit_events (subject_type, subject_id, timestamp_utc);
CREATE INDEX IF NOT EXISTS idx_audit_trigger_time
    ON audit_events (trigger, timestamp_utc);

-- ── Row-level 保护: 禁止对账本/审计表执行 UPDATE/DELETE ──────────────────────
-- （数据库层的 C-6 硬约束；应用层由 Go 代码保证，此处双重防护）
CREATE OR REPLACE RULE no_update_ledger AS
    ON UPDATE TO ledger_entries DO INSTEAD NOTHING;
CREATE OR REPLACE RULE no_delete_ledger AS
    ON DELETE TO ledger_entries DO INSTEAD NOTHING;
CREATE OR REPLACE RULE no_update_audit AS
    ON UPDATE TO audit_events DO INSTEAD NOTHING;
CREATE OR REPLACE RULE no_delete_audit AS
    ON DELETE TO audit_events DO INSTEAD NOTHING;
