---
title: LinkRSP 实施计划（工程阶段排期）
version: v1.0（文稿）
status: 开发前决策集（可迭代）
---

# LinkRSP 实施计划 v1.0（工程排期与开启条件）

> 本文档将 LinkRSP 的工程实现拆为可分期推进的基础设施工作流，并将 **备案 / 国内上线 / 付费资源 / LLM 调用** 等高成本项尽量后置。  
> 交叉引用：技术栈策略 `docs/engineering/technology-strategy-v1.0.md`；规则集 `docs/governance/rule-engine/`；审计事件 `docs/spec/audit-event-schema-v1.0.md`；时间口径 `docs/spec/time-and-window-conventions-v1.0.md`；申诉与公开记录 `docs/governance/appeals-and-public-record.md`。

---

## 0. 运行环境与数据流边界（当前约束）

### 0.1 海外免费资源（允许，用于研发与验证）

- **代码与文档**：GitHub（开源仓库）
- **前端**：Vercel（境外托管）
- **数据库**：Supabase（境外托管；与国内备案/公司主体无绑定）

上述资源以个人/开源名义存在，不等同于国内对公网服务的“正式上线”，也不构成国内备案义务的触发条件。

### 0.2 国内上线（后置，不在近期计划中）

“国内上线”指：以国内公司主体 + 国内服务器/数据库 + 国内公网持续运营形态交付。该阶段需要备案/合规与运营能力，**计划上晚于 HDGP**。

### 0.3 不互通原则（高层约束）

- 本仓库与 HDGP 的不同路径在资源与数据流上原则上独立；若未来需要对接，仅通过**明确的接口契约**实现，避免隐式数据互通。

---

## 1. 平台性质与费用策略（工程约束口径）

- LinkRSP 的工程目标是构建**协议与基础设施**，而非短期商业化平台。
- 当前取向：**不抽成、不做资金托管、不做拉新宣发**（详见白皮书与管理制度草案）。
- 重要说明：上述为**当前取向与边界**，不写成不可变的法律承诺；若未来出现版本化治理调整，应通过公开流程变更并留痕。

---

## 2. 阶段划分（从“可审计基础设施”开始）

> 术语说明：以下阶段以“工程可交付物”为里程碑，并给出**启动条件**。条件未满足时，阶段可停留在文档/模拟层。

### Phase A（已完成）：规范与治理基线

**完成日期：2026-04-25**

- 规则 YAML（R/S/混合/跨社区）共 **50 条**（R-001–R-020、S-001–S-029、S-009b、混合 R-011/R-012/S-009b）
- 规则规范（few-shot 注入、prompt_field_mapping、audit_scope、implementation_phase 等）
- `audit_event` schema、时间窗口口径、申诉与公开记录、manual-audit-requests 格式
- 规则类别覆盖：物理悖论、女巫防御、数据完整性、信用套利、治理操纵、诱骗切换、隐藏附加条件、虚假紧迫、弱势定向、监控越界、资质虚假宣称、未成年人保护、债务陷阱、章程破坏、骚扰诽谤、违法活动、数据留存滥用、虚构紧急剥削

**启动条件**：无（纯文档与仓库维护即可）。

---

### Phase B：R 类规则最小可运行原型（无 LLM、无 HDGP 依赖）

**目标**：在不引入付费资源、不触发国内备案的前提下，实现 R-001—R-005 的可运行闭环，产出可回放审计事件。

**实际进度（截至 2026-04-25，进行中）**：

| 交付物 | 状态 | 说明 |
|--------|------|------|
| 最小后端服务（Go 单体） | ✅ 完成 | `cmd/linkrsp/main.go`，端口 9090，优雅关闭 |
| 最小 Postgres schema | ✅ 完成 | `db/migrations/001_init.sql`，Supabase Session Pooler 接入 |
| 最小 REST API | ✅ 完成 | `internal/api/`，7 条路由，符合 openapi-v1.0-draft.md |
| `audit_event` 落库 | ✅ 完成 | `internal/audit/event.go`，幂等 ON CONFLICT DO NOTHING |
| R-001 物理悖论（并发位置） | ✅ 完成 | `internal/rules/r001/` |
| R-002 V=2 握手速度 | ✅ 完成 | `internal/rules/r002/`，Haversine 球面距离 |
| R-003 任务时长上限 24h | ✅ 完成 | `internal/rules/r003/`，纯算术 |
| R-004 时间戳倒置 | ✅ 完成 | `internal/rules/r004/`，纯比较 |
| R-005 每日工时上限 16h | ✅ 完成 | `internal/rules/r005/`，24h 滚动窗口聚合 |
| LRS-1.0 积分计算公式 | ✅ 完成 | `internal/api/settlement.go`，D_base=1.0、K_global=1.0 创世期 |

**API 路由（已上线）**：

- `POST /api/v1/tasks` — 创建任务（R-004/R-003 pre_execution 检查）
- `GET /api/v1/tasks/{task_id}` — 查询任务
- `POST /api/v1/tasks/{task_id}/attestations` — 提交存证（R-002 post check）
- `POST /api/v1/tasks/{task_id}/settlement/preview` — 结算预览（R-001/R-005）
- `POST /api/v1/tasks/{task_id}/settlement/commit` — 结算落账（ledger_entries 只追加）
- `GET /api/v1/audit-events` — 审计事件列表（含过滤）
- `GET /api/v1/audit-events/{event_id}` — 单条审计事件

**剩余工作**（进入 Phase C 前）：

- R-006—R-010 实现（可在 Phase B 延伸期内完成，不阻塞 Phase C 启动）
- 集成测试（带真实 DB 的 `_integration_test.go`）
- `GET /api/v1/healthz` 扩展（添加 DB ping 状态）

**启动条件**：

- 至少 1 名工程维护者可持续投入；✅ 已满足
- 可使用境外 Supabase 或本地/内网 Postgres；✅ 已接入 Supabase

**不做**：

- 不做用户增长、支付托管、法币撮合；
- 不做 LLM；
- 不强制接入 HDGP 可运行 Engine（适配器保持接口预留即可）。

---

### Phase C：Ledger 雏形 + 事件溯源（仍无 LLM）

**目标**：把积分分录与审计事件做成“只追加”的可回放结构，为 S-009b、S-015/16 等依赖统计/查询的规则铺底。

**交付物**：

- `ledger_entries`（append-only）+ 投影视图（余额为投影，不为唯一真相）
- 结算预览与提交 API（preview/commit 分离）
- S-009b 需要的 `lrs_ledger_query` / `attestation_index_query` 查询接口（可先内部 API）

**启动条件**：

- Phase B 稳定；
- 有至少一组真实试点愿意跑“两个真实的人 + 一次完整任务记录”的最小闭环。

---

### Phase D：S 类审计“无 LLM”降级运行（人工为主）

**目标**：在无资金/无 LLM 的条件下，让 S 类规则不空转：预筛 → 人工复核 → 可回放审计。

**交付物**：

- `semantic_audit_jobs`（Postgres 轮询队列表；幂等键与审计字段齐全）
- 预筛（trigger_words）→ 路由 `human_review_queue`
- 人工复核界面或最小 CLI（可后置；也可先用 PR/Issue 流程承载）

**启动条件**：

- 至少每周可稳定处理一定量人工复核（例如 50–200 条）。

---

### Phase E：引入 LLM（批处理，可插拔）

**目标**：在可控成本下，让 S 类（含 confirmation_only）自动化运转：灰区队列批处理、置信度阈值与 few-shot 注入一致。

**队列实现选择**：本阶段仍建议优先用 **Postgres 轮询**起步；当日处理量持续超过 1 万条或轮询延迟/锁争用成为瓶颈时，再迁移 NATS/RabbitMQ（以真实数据支撑决策）。

**启动条件（建议量化）**：

- 有稳定的 LLM 额度/算力来源（资方或贡献者进入）；
- 有足够的人工复核样本用于 few-shot 与阈值校准；
- 有清晰的隐私与数据最小化策略（参考威胁模型范围）。

---

### Phase F：HDGP 可运行 Engine 接入（后置）

**目标**：将策略判定通过适配器与 HDGP 可运行 Engine/策略包对接（若采用）。

**启动条件**：

- LinkRSP 自身的 `audit_event` 与规则执行链路稳定；
- HDGP Engine/策略包版本稳定，且双方接口契约冻结；
- 通过明确的数据流边界评审（不隐式互通）。

---

### Phase G：国内上线（最后阶段，可能长期不触发）

**目标**：国内公司主体 + 国内服务器/数据库 + 备案 + 合规运营。

**启动条件**：

- 明确的组织与资金来源；
- 明确的数据治理与合规评估；
- 对外持续运营的需求成立。

---

## 3. 仓库/治理迁移（可选）

本仓库为开源协议与基础设施材料集合。未来若需要更清晰的社区治理，可选择：

- 迁移到更适合的组织名下（例如与 HDGP 社区基线不同路径的独立组织）；或
- 维持现状但完善贡献与维护者流程。

迁移应以最小化中断（issues/PR/发布物可追溯）为原则。


