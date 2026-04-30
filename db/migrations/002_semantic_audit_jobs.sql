-- LinkRSP Phase D — 语义审计队列
-- 规范参考: docs/engineering/implementation-plan-v1.0.md §Phase D
-- openapi 草案: docs/spec/openapi-v1.0-draft.md §4
-- 约束: 此表通过 status 字段演进状态，不使用 UPDATE/DELETE 清除记录；
--       verdict / confidence 由人工复核或 LLM（Phase E）回填。

CREATE TABLE IF NOT EXISTS semantic_audit_jobs (
    job_id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id          TEXT        NOT NULL,
    subject_type     TEXT        NOT NULL,
    subject_id       TEXT        NOT NULL,
    text_ref         TEXT        NULL,     -- 文本片段或对象存储引用
    status           TEXT        NOT NULL  DEFAULT 'pending'
                         CHECK (status IN ('pending', 'processing', 'human_review', 'done', 'skipped')),
    verdict          TEXT        NULL
                         CHECK (verdict IS NULL OR verdict IN ('PASS', 'BLOCK', 'SKIP')),
    confidence       NUMERIC     NULL
                         CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    trigger_words    TEXT[]      NULL,     -- 预筛命中的关键词列表
    routed_to        TEXT        NULL
                         CHECK (routed_to IS NULL OR routed_to IN ('human_review_queue', 'llm_queue')),
    idempotency_key  TEXT        NOT NULL UNIQUE,
    created_at_utc   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at_utc   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saj_status_time
    ON semantic_audit_jobs (status, created_at_utc);
CREATE INDEX IF NOT EXISTS idx_saj_rule_subject
    ON semantic_audit_jobs (rule_id, subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_saj_routed
    ON semantic_audit_jobs (routed_to, status, created_at_utc)
    WHERE routed_to IS NOT NULL;
