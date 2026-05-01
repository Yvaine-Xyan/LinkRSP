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
  - 仅计算预览（不落账），返回 `formula_snapshot`、`rules_checked`
  - 当前执行链包含：`R-001`、`R-005`、`R-006`、`R-010`
- `POST /api/v1/tasks/{task_id}/settlement/commit`
  - 落账（append-only ledger entry），并持久化对应 audit_event
  - 通过 `idempotency_key` 保证重复提交返回同一 ledger entry

### 2.4 Audit events（审计事件）

- `GET /api/v1/audit-events?subject_type=...&subject_id=...`
  - 查询审计事件列表（分页）
- `GET /api/v1/audit-events/{event_id}`
  - 查询单条审计事件

### 2.5 Internal queries（内部查询 / 投影视图骨架）

- `GET /api/v1/internal/lrs-ledger-query`
  - 按 `uid + window_start_utc + window_end_utc` 查询账本窗口
  - 返回窗口内分录列表 `entries[]`
  - 返回 `entry_count` 作为窗口内分录计数
  - 返回 `projected_balance` 作为按 `created_at_utc ASC` 回放后的窗口投影余额
  - 每条分录包含 `projected_balance`，用于回放每一步累计结果
- `GET /api/v1/internal/attestation-index-query`
  - 按 `uid + window_start_utc + window_end_utc` 查询存证窗口
  - 返回窗口内存证列表 `items[]`，供规则与统计查询复用
- `GET /api/v1/internal/semantic-audit-jobs`
  - 可选过滤：`status`（默认 `pending`）、`rule_id`、`limit`（默认 50，最大 200）
  - 返回 `{"jobs": [...], "total": N}`，每条 job 含 trigger_words、routed_to、verdict、confidence
- `GET /api/v1/internal/semantic-audit-jobs/stats`
  - 队列最小统计（Phase D 运维视角）：按 status 聚合计数 + 最早 pending/human_review 时间
- `POST /api/v1/internal/semantic-audit-jobs/enqueue`
  - 内部入队入口（供自治工具/脚本/运营触发）
  - 请求体：`rule_id`、`subject_type`、`subject_id`、`text_ref`/`text`、可选 `trigger_words`
  - 返回：accepted + routed_to + trigger_words（预筛命中子集）
- `GET /api/v1/internal/semantic-audit-jobs/{job_id}/replay`
  - 证据回放包（Phase D 运维/人工复核视角）：返回 job、写回审计事件、以及（当 subject_type=task）关联任务的存证/账本/审计事件摘要
- `PATCH /api/v1/internal/semantic-audit-jobs/{job_id}`
  - 人工复核写回：可更新 `status`、`verdict`（PASS/BLOCK/SKIP）、`confidence`（0–1）
  - 幂等安全：重复 PATCH 同字段无副作用；返回更新后完整 job 对象
  - 写回落库：写回同时追加一条 `audit_event`（subject_type=`semantic_audit_job`，便于回放）

> 运维建议：对 Phase D 的最小人工复核通路，可直接使用仓库内 CLI `cmd/semreview`（list/patch），不强制绑定特定社区 SOP。

### 2.6 Health（健康检查）

- `GET /healthz`
- `GET /api/v1/healthz`
  - 返回服务状态与数据库 ping 状态

### 2.7 Governance / IPO（可后置）

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
- `location_hash`（可选）
- `evidence_ref`（对象存储引用；可选）

> 当前实现：存证写路径先执行 `R-002`，插入成功后补写 `R-009` 审计事件。

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

