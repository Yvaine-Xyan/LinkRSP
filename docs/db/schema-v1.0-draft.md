# LinkRSP Database Schema Draft v1.0

> 本文件定义实现阶段的最小数据库表结构草案（以 Postgres 为基准）。  
> 目标：支持 R-001—R-005 首批落地、append-only 分录、`audit_event` 留痕与可回放。  
> 交叉引用：审计事件 `docs/spec/audit-event-schema-v1.0.md`；时间口径 `docs/spec/time-and-window-conventions-v1.0.md`；规则目录 `docs/governance/rule-engine/`。

---

## 1. 设计原则

- **Append-only**：账务分录与审计事件只追加，不原地覆盖；更正用冲正/补充事件表达。
- **UTC**：所有时间字段存 UTC（`timestamptz`）。
- **幂等**：写路径必须有 `idempotency_key`（至少对结算与审计事件）。
- **最小化敏感信息**：原始文本与证据包用引用（对象存储/外部表），审计事件保存摘要。

---

## 2. 最小表（Phase B/C）

### 2.1 `tasks`

- `task_id uuid primary key`
- `community_id uuid null`
- `uid_submitter text not null`
- `description_text text not null`
- `start_time_utc timestamptz not null`
- `end_time_utc timestamptz not null`
- `location_hash text null`
- `created_at_utc timestamptz not null default now()`

索引建议：

- `(uid_submitter, start_time_utc, end_time_utc)`
- `(community_id, created_at_utc)`

### 2.2 `attestations`

- `attestation_id uuid primary key`
- `task_id uuid not null references tasks(task_id)`
- `verification_level int not null`  -- 0/1/2
- `timestamp_utc timestamptz not null`
- `location_lat double precision null`
- `location_lng double precision null`
- `location_hash text null`
- `evidence_ref text null`           -- 对象存储引用（可选）
- `created_at_utc timestamptz not null default now()`

索引建议：

- `(task_id)`
- `(verification_level, timestamp_utc)`

### 2.3 `ledger_entries`（append-only）

- `entry_id uuid primary key`
- `task_id uuid not null references tasks(task_id)`
- `uid text not null`
- `credits_delta numeric not null`
- `formula_snapshot jsonb null`      -- D_base/K_global/Clip bounds 等
- `created_at_utc timestamptz not null default now()`
- `idempotency_key text not null unique`

索引建议：

- `(uid, created_at_utc)`
- `(task_id)`

### 2.4 `audit_events`（append-only）

- `event_id uuid primary key`
- `schema_version text not null`     -- "1.0"
- `payload jsonb not null`           -- 完整 audit_event JSON
- `rule_id text not null`
- `subject_type text not null`
- `subject_id text not null`
- `trigger text not null`
- `timestamp_utc timestamptz not null`
- `idempotency_key text not null unique`
- `created_at_utc timestamptz not null default now()`

索引建议：

- `(rule_id, timestamp_utc)`
- `(subject_type, subject_id, timestamp_utc)`
- `(trigger, timestamp_utc)`

> 说明：用 `payload` 承载 schema，外加常用列做索引，以兼顾演进与查询性能。

---

## 3. 队列表（Postgres 轮询，Phase D/E）

### 3.1 `semantic_audit_jobs`

- `job_id uuid primary key`
- `rule_id text not null`
- `rule_version text not null`
- `subject_type text not null`
- `subject_id text not null`
- `text_ref text not null`           -- 指向 tasks/ipo_materials 等
- `status text not null`             -- queued|processing|done|failed
- `attempt int not null default 0`
- `idempotency_key text not null unique`
- `created_at_utc timestamptz not null default now()`
- `updated_at_utc timestamptz not null default now()`

并发建议：

- 处理端使用 `SELECT ... FOR UPDATE SKIP LOCKED` 拉取任务，避免双消费。

---

## 4. IPO 相关（可后置，Phase C 后再加）

> 说明：IPO 与跨社区模式审计需要历史材料与投票记录，但可在最小闭环后再实现。

建议后续引入：

- `ipo_applications`：`ipo_application_id`、`community_id`、`materials_ref`、`submitted_at_utc`、`status`
- `ipo_votes`：`ipo_application_id`、`uid`、`vote`、`timestamp_utc`

当 `ipo_votes` 存在后，可为 `S-016` 的统计分析提供数据基座。

