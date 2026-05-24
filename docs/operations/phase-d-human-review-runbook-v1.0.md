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

### 3.1 在 GitHub 网页里哪里配置（Secrets / Variables）

**不要**在左侧 **Actions** 里找——那里只有工作流运行记录。请按下面路径操作（需对本仓库有 **Settings** 权限，一般为 Owner / Admin）：

1. 打开仓库首页：`https://github.com/YvaineHe/LinkRSP`
2. 点顶部菜单 **Settings**（在 **Insights** 旁边；若看不到，说明当前账号无权改仓库设置）
3. 左侧栏点开 **Secrets and variables**，再点 **Actions**
4. 你会看到两个子页签：
   - **Secrets**（敏感值，写入后不可再查看，只能删改）：点 **New repository secret**
   - **Variables**（非敏感、可在 PR 里引用策略允许时可见）：点 **Variables** 页签 → **New repository variable**

**看门狗需要的 Secrets（名称须完全一致）**

| Name | 填什么 |
|------|--------|
| `LINKRSP_API_BASE_URL` | 你的 linkrsp 服务根 URL，**不要**末尾 `/`，例如 `https://你的域名` 或 `http://主机:9090` |
| `LINKRSP_API_SHARED_SECRET` | 与线上/试点环境里的 **`API_SHARED_SECRET`** 完全一致 |

未配置 `LINKRSP_API_BASE_URL` 时，脚本认为 `BASE_URL` 为空并 **直接退出 0**，工作流显示成功但不会真正检查队列（适合尚未暴露 API 的阶段）。

**阈值（可选，在 Variables 页签）**

| Name | 含义 | 不设时脚本默认 |
|------|------|----------------|
| `SEMANTIC_QUEUE_MAX_PENDING` | `pending` 条数超过则失败 | 200 |
| `SEMANTIC_QUEUE_MAX_HUMAN_REVIEW` | `human_review` 条数超过则失败 | 200 |
| `SEMANTIC_QUEUE_OLDEST_PENDING_MAX_HOURS` | 最老 `pending` 超过该小时数则失败 | 72 |
| `SEMANTIC_QUEUE_OLDEST_HUMAN_MAX_HOURS` | 最老 `human_review` 超过该小时数则失败 | 72 |

**说明**：`LINKRSP_API_BASE_URL` / `LINKRSP_API_SHARED_SECRET` 是 **地址和密钥**，不是阈值；阈值只用上面四个 **Variables** 调。

### 3.2 「仓库里没有 API_SHARED_SECRET」是正常的

`API_SHARED_SECRET` **不会**作为文件提交到 Git 仓库（本地只在 [`.env`](../../.env.example) 里配置，且 `.env` 被忽略）。若你还没为**正在运行的 linkrsp 进程**设过密钥，可以按下面二选一：

**方案 A（推荐，对外可访问的 API）**

1. 自己生成一段足够长的随机字符串（例如 32+ 字符），当作共享密钥。  
2. 在 **部署 linkrsp 的环境**里设置环境变量 `API_SHARED_SECRET=<该字符串>`（与 [`.env.example`](../../.env.example) 一致），重启服务。  
3. 在 GitHub **Actions → Secrets**（不是 Variables）里新增 **`LINKRSP_API_SHARED_SECRET`**，值与上一步 **完全相同**。  
4. 同时配置 **`LINKRSP_API_BASE_URL`** 指向该服务的根地址。

**方案 B（仅本地开发、没有公网 API）**

- 可以不设 `API_SHARED_SECRET`（服务端为空则写接口不校验密钥，仅适合本机）。  
- 此时 **不要**配置 `LINKRSP_API_BASE_URL`（或留空），看门狗脚本会跳过检查；等有了公网部署再按方案 A 补齐。

**切勿**把真实密钥写进 **Repository variables**（对他人可见）；密钥一律用 **Secrets** 页签。

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
