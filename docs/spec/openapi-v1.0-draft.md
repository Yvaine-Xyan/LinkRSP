# LinkRSP API Contract Draft v1.0（OpenAPI 草案）

> 本文件定义工程实现阶段的最小 API 契约（先写契约再写代码）。  
> **注意**：这不是最终 `openapi.yaml`，而是用于对齐字段、路径与版本策略的草案。  
> 交叉引用：审计事件 schema `docs/spec/audit-event-schema-v1.0.md`；时间口径 `docs/spec/time-and-window-conventions-v1.0.md`；规则目录 `docs/governance/rule-engine/`。

---

## 1. 版本策略

- 对外 API 路径版本在未达上线标准前保持 **≤ v1.0**（见 `docs/governance/versioning-policy.md`）。
- 草案建议路径前缀：`/api/v1/`（最终以实现为准）。

---

## 2. 核心资源与端点（最小集）

### 2.1 Tasks（任务）

- `POST /api/v1/tasks`
  - 创建任务（任务元数据 + 描述文本）
- `GET /api/v1/tasks/{task_id}`
  - 查询任务

### 2.2 Attestations（存证/核验）

- `POST /api/v1/tasks/{task_id}/attestations`
  - 上传存证（V=0/1/2 的证据摘要 + 时间戳/位置摘要）

### 2.3 Settlement（结算）

- `POST /api/v1/tasks/{task_id}/settlement/preview`
  - 仅计算预览（不落账），返回 credits_delta 与会触发的规则摘要
- `POST /api/v1/tasks/{task_id}/settlement/commit`
  - 落账（append-only ledger entry），并生成 audit_event

### 2.4 Audit events（审计事件）

- `GET /api/v1/audit-events?subject_type=...&subject_id=...`
  - 查询审计事件列表（分页）
- `GET /api/v1/audit-events/{event_id}`
  - 查询单条审计事件

### 2.5 Governance / IPO（可后置）

- `POST /api/v1/ipo/applications`
- `GET /api/v1/ipo/applications/{ipo_application_id}`

> 说明：IPO 相关端点可在 Phase C 后再实现；但 `S-009a/b`、`S-015/16` 的 subject 需要预留 `ipo_application_id`。

---

## 3. 数据对象草案（关键字段）

### 3.1 Task

- `task_id`（UUID）
- `community_id`（可选）
- `uid_submitter`
- `description_text`
- `start_time_utc` / `end_time_utc`
- `location_hash`（可选）

### 3.2 Attestation

- `attestation_id`（UUID）
- `task_id`
- `verification_level`（0/1/2）
- `timestamp_utc`
- `location_lat/lng`（可选；可用 hash + 精度标记替代）
- `evidence_ref`（对象存储引用；可选）

### 3.3 Ledger entry（只追加）

- `entry_id`（UUID）
- `task_id`
- `uid`
- `credits_delta`
- `formula_snapshot`（可选：D_base、K_global、Clip bounds）
- `created_at_utc`

### 3.4 Audit event

遵循 `docs/spec/audit-event-schema-v1.0.md`。

---

## 4. 队列（Postgres 轮询表）草案

LLM 与人工复核队列在早期可用数据库表承载：

- `semantic_audit_jobs`
  - `job_id`、`rule_id`、`subject`、`text_ref`、`status`、`created_at_utc`、`idempotency_key`

迁移到消息队列（NATS/RabbitMQ）的条件见实施计划。

