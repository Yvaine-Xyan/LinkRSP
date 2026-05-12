---
title: Phase D 语义审计人工复核操作手册 v1.0
status: 现行
applies_to: semantic_audit_jobs 队列、内部 API、semreview CLI
---

# Phase D 语义审计人工复核操作手册

> **范围**：仅描述**系统网关**侧的最小动作——如何列出待办、拉取回放包、写回状态与 verdict，并产生可回放的 `audit_event`。  
> **非范围**：不定义社区 SOP、不规定复核排班与争议裁决流程。

**交叉引用**：[试点 API 手册](pilot-runbook-v1.0.md) · [运行时参数（R-006 / R-010）](runtime-r006-r010-config.md) · [LLM 接入 RFC（前置）](../engineering/llm-oracle-integration-rfc-v0.1.md)

---

## 1. 前置条件

| 项 | 说明 |
|----|------|
| 后端 | `linkrsp` 已部署，可访问 `BASE_URL` |
| 密钥 | **生产环境必须**设置 `API_SHARED_SECRET`；请求头 `X-API-Shared-Secret` 与之一致（空密钥仅用于本地开发，见 [`.env.example`](../../.env.example)） |
| 工具 | `curl` 或 `semreview`（仓库 [`cmd/semreview`](../../cmd/semreview/main.go)） |

```bash
export BASE_URL="https://<host>"   # 或 http://localhost:9090
export SECRET="<API_SHARED_SECRET>"
```

---

## 2. 推荐流程：list → replay → patch

### 2.1 列出待复核任务

**CLI**（默认筛 `pending`）：

```bash
go run ./cmd/semreview list --base-url "$BASE_URL" --secret "$SECRET" --status human_review --limit 50
```

也可查看 `pending`：

```bash
go run ./cmd/semreview list --base-url "$BASE_URL" --secret "$SECRET" --status pending --limit 50
```

**HTTP**：

```bash
curl -sS "$BASE_URL/api/v1/internal/semantic-audit-jobs?status=human_review&limit=20" \
  -H "X-API-Shared-Secret: $SECRET" | python3 -m json.tool
```

### 2.2 拉取回放包（replay）

回放包含：`job`、`writeback_audit_events`；若 `subject_type=task`，另含任务快照、`subject_audit_events`、`ledger_entries`、`attestations`。

**CLI**：

```bash
go run ./cmd/semreview replay --base-url "$BASE_URL" --secret "$SECRET" --job-id "<uuid>"
```

**HTTP**：

```bash
curl -sS "$BASE_URL/api/v1/internal/semantic-audit-jobs/<job_id>/replay" \
  -H "X-API-Shared-Secret: $SECRET" | python3 -m json.tool
```

**阅读顺序建议**：`job`（rule_id、text_ref、trigger_words）→ `subject` / `subject_audit_events` → `writeback_audit_events`（历次写回）→ 需要时再看 ledger / attestations。

### 2.3 写回（PATCH）

至少提供 **`status`、`verdict`、`confidence` 之一**。

| 字段 | 说明 |
|------|------|
| `status` | `pending` / `processing` / `human_review` / `done` / `skipped` |
| `verdict` | `PASS` / `BLOCK` / `SKIP`（CLI 大小写不敏感，服务端存大写） |
| `confidence` | `0`–`1` 浮点；人工判定时建议如实填写，便于后续 Phase E 阈值校准 |

**典型收尾**（人工判定通过并结案）：

```bash
go run ./cmd/semreview patch --base-url "$BASE_URL" --secret "$SECRET" \
  --job-id "<uuid>" --status done --verdict PASS --confidence 0.85
```

**HTTP**：

```bash
curl -sS -X PATCH "$BASE_URL/api/v1/internal/semantic-audit-jobs/<job_id>" \
  -H "X-API-Shared-Secret: $SECRET" \
  -H "Content-Type: application/json" \
  -d '{"status":"done","verdict":"PASS","confidence":0.85}' | python3 -m json.tool
```

写回成功后，服务会在**同一事务**内追加一条 `audit_event`（`subject_type=semantic_audit_job`），供回放与合规追溯。

---

## 3. 队列观测

```bash
curl -sS "$BASE_URL/api/v1/internal/semantic-audit-jobs/stats" \
  -H "X-API-Shared-Secret: $SECRET" | python3 -m json.tool
```

字段说明：`counts_by_status`、`oldest_pending_utc`、`oldest_human_review_utc`。  
定时检查可参考仓库 [`.github/workflows/semantic-queue-watchdog.yml`](../../.github/workflows/semantic-queue-watchdog.yml) 与 [`scripts/ops/check_semantic_queue_stats.py`](../../scripts/ops/check_semantic_queue_stats.py)。

**GitHub 配置**：在仓库 **Settings → Secrets and variables → Actions** 中新增 `LINKRSP_API_BASE_URL`（无尾斜杠）、`LINKRSP_API_SHARED_SECRET`。未配置时脚本对 `BASE_URL` 为空直接退出 0（不阻塞 CI）。阈值可通过 **Variables** 覆盖：`SEMANTIC_QUEUE_MAX_PENDING`、`SEMANTIC_QUEUE_MAX_HUMAN_REVIEW`、`SEMANTIC_QUEUE_OLDEST_PENDING_MAX_HOURS`、`SEMANTIC_QUEUE_OLDEST_HUMAN_MAX_HOURS`（均为可选；未设则用脚本内默认值）。

---

## 4. 入队（enqueue）与滥用面

- 入队接口：`POST /api/v1/internal/semantic-audit-jobs/enqueue`（需共享密钥）。  
- **幂等**：同一 `(rule_id, subject_type, subject_id)` 在同一 UTC **分钟**内重复入队会被去重（见 [`internal/queue/semantic_queue.go`](../../internal/queue/semantic_queue.go)）。  
- **可选速率限制**：环境变量 `SEMANTIC_AUDIT_ENQUEUE_MAX_PER_MINUTE`（`0` 表示关闭），限制全局每分钟入队次数，防止脚本误刷。

---

## 5. 何时标记「需二次讨论」

本手册不定义治理流程。若复核者认为当前证据不足以定论，可：

- 将 `status` 保持为 `human_review`，并 **暂不** 将 `status` 设为 `done`；或  
- 使用 `verdict`=`SKIP` 与说明性置信度，并在组织内部按自有流程跟踪（系统仅保证审计链落库）。

---

## 6. 自检清单（D-ops-1）

- [ ] 能用 `list` 看到队列条目  
- [ ] 能对单条 `job_id` 成功执行 `replay`  
- [ ] 能 `patch` 写回且随后可在 `replay` 的 `writeback_audit_events` 中看到新事件  
- [ ] 生产环境已启用 `API_SHARED_SECRET`  
