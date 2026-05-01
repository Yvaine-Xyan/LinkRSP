-- LinkRSP Ops — keepalive heartbeats (to prevent Supabase project sleep).
-- This table is intentionally simple and append-friendly.

CREATE TABLE IF NOT EXISTS ops_heartbeats (
    heartbeat_id  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp_utc TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source        TEXT        NOT NULL,
    note          TEXT        NULL
);

CREATE INDEX IF NOT EXISTS idx_ops_heartbeats_time
    ON ops_heartbeats (timestamp_utc DESC);

