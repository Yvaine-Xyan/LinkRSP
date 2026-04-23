# CLAUDE.md — LinkRSP 开发指南

## 项目身份

**LinkRSP**（Link Reshuffling & Survival Protocol）是去中心化劳动力互助协议。  
当前阶段：**Phase A 完成 / Phase B 启动中**（Go + PostgreSQL 规则引擎实现）。  
网站：`www.linkrsp.com` | 治理层：HDGP | 许可：MIT

---

## 关键约束（永远不要违反）

| # | 规则 | 原因 |
|---|------|------|
| C-1 | LRS 积分不是货币 — 永远不把积分描述为 "money / tokens / payment" | 白皮书定义：积分是权益凭证 |
| C-2 | 协议不托管法币 — 现金结算在协议外由当事方自行约定 | 管理制度草案 §3 |
| C-3 | `D_base` 在 LRS-1.0 中冻结为 `1.0`，不作为可配置参数 | 算法规格 §2 |
| C-4 | `K_global` 创世期 = 1.0，到达 min(1000 S 任务, 90天) 后才开始衰减 | 算法规格 §3 |
| C-5 | 所有时间边界 UTC-only，滚动窗口以任务 `start_time` 为基准 | `docs/spec/time-and-window-conventions-v1.0.md` |
| C-6 | 账本只追加（INSERT only），绝对不能 UPDATE/DELETE 账本表 | 架构草案 §3 |
| C-7 | LLM 输出不能直接生成积分 — 积分只来自合规存证 | 技术策略 §3.4 |
| C-8 | Rule YAML `version` 固定为 `"1.0"` | versioning-policy.md |

---

## 技术栈（已锁定）

| 层 | 技术 | 备注 |
|----|------|------|
| 后端核心 | **Go** | 见 technology-strategy-v1.0.md §3.1 |
| 数据库 | **PostgreSQL** | 事件化追加，不用 Document DB |
| API | **REST + OpenAPI 3** | openapi-v1.0-draft.md |
| 队列 | Postgres polling → NATS (若 >10k/day) | Phase D+ |
| 移动端 | Kotlin/Swift (BLE/GPS) | Handshake v2 |
| 对象存储 | S3 兼容 | |
| 可观测 | OpenTelemetry + 结构化日志 | |

---

## 关键文件路径

```
docs/spec/
  linkrsp-core-algorithm-spec-v1.0.md   # LRS-1.0 公式权威文件
  audit-event-schema-v1.0.md            # 审计事件 JSON 模式
  time-and-window-conventions-v1.0.md   # 所有时间边界
  openapi-v1.0-draft.md                 # API 草案
docs/db/
  schema-v1.0-draft.md                  # 数据库模式草案
docs/governance/rule-engine/
  README.md                             # 规则 YAML 模式说明
  r-class/  R-001.yaml … R-010.yaml     # 确定性规则 (10条)
  s-class/  S-001.yaml … S-016.yaml     # LLM 语义审计规则 (16条)
  mixed/    R-011 R-012 S-009b          # 混合类规则
docs/engineering/
  implementation-plan-v1.0.md           # Phase A-G 路线图
  architecture-v0.1.md
  technology-strategy-v1.0.md
docs/reports/
  feasibility-assessment-v1.0-alpha.md  # 32条执行轨道来源
```

---

## Rule YAML — 必填字段速查

### 所有规则
```yaml
rule_id: "R-001"           # R-\d{3} | S-\d{3}[ab]?
version: "1.0"
category: "r_class"        # r_class | s_class | mixed
trigger: "post_execution"  # pre_publication | post_execution | ipo_scan | registration | periodic_audit
severity: "block"          # block | warn | flag
description: "..."
inputs: [...]
condition: "..."
action: "..."
false_positive_notes: "..."
```

### S 类额外必填
```yaml
audit_method: "llm_oracle"
queue: "semantic_audit_jobs"
batch_threshold: 50
prompt_template: |
  ...{{placeholder}}...
few_shot_examples:          # 至少 4 条：2 PASS + 2 BLOCK
  - input: "..."
    verdict: "PASS"
    rationale: "..."
  - input: "..."
    verdict: "BLOCK"
    rationale: "..."
confidence_threshold:
  flag: 0.6
  block: 0.8
reasonableness_anchor: "..."
```

### 跨社区类（S-015 / S-016）额外必填
```yaml
implementation_phase: "phase_1_manual"
comparison_window: "90d"
feature_extraction: "tfidf_cosine"
requires_features: [...]
```

---

## 审计事件 — 9 节必填结构

```json
{
  "event_id": "UUIDv4",
  "rule": { "id": "R-001", "version": "1.0", "category": "r_class" },
  "scope": { "trigger": "post_execution", "audit_scope": "task" },
  "subject": { "type": "task", "id": "..." },
  "verdict": { "result": "block|flag|pass", "severity": "...", "confidence": null },
  "evidence": { "fields_checked": [], "matched_values": {}, "feature_summary": "..." },
  "trace": { "timestamp": "UTC ISO8601", "trace_id": "..." },
  "action_taken": "..."
}
```

**幂等键**：`(rule_id, subject_type, subject_id, scope.trigger, timestamp_minute)`  
S 类规则：`confidence` 必填，`feature_summary` 必须含 LLM prompt 摘要（供重放）。

---

## Git 提交格式

```
type(scope): 简短描述（英文）

- 详情 1
- 详情 2
```

| type | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 纯文档变更 |
| `refactor` | 重构，不改行为 |
| `test` | 测试 |
| `chore` | 构建/CI/配置 |

scopes: `rule-engine` `spec` `ops` `engineering` `go` `api` `db` `web`

---

## Go 代码规范（Phase B 启动后适用）

1. **错误处理**：所有 error 必须处理，禁止 `_ = err`（除非显式注释说明原因）
2. **Context 传递**：DB 调用函数第一参数为 `context.Context`
3. **账本写入**：只允许 `INSERT`，表名含 `event` / `ledger` 的表禁止 `UPDATE` / `DELETE`
4. **审计事件**：每条规则执行必须生成对应审计事件，不得跳过
5. **幂等性**：规则检查函数对同一输入重复调用结果一致
6. **无硬编码凭据**：所有密钥、DSN 从环境变量读取
7. **格式**：提交前运行 `make fmt`

---

## 自检流程

### 编辑规则 YAML 时
运行 `/rule-lint` → 确认必填字段齐全 → 确认 few-shot 示例 ≥4 条（S 类）

### 编辑规格文档时
运行 `/spec-check` → 确认公式参数、时间窗口与其他文档一致

### 提交前
运行 `/pre-commit` → 检查违反 C-1~C-8 的描述 → 确认提交信息格式

### Go 代码编写后
运行 `/go-check` → `make lint test` → 确认无账本 UPDATE

---

## 开发阶段前置条件速查

| Phase | 前置条件 |
|-------|---------|
| B（R 规则 MVP） | 1+ Go 工程师 |
| C（账本 + 结算） | Phase B 稳定 + 2名真实用户 |
| D（S 规则无 LLM） | 每周 50-200 人工审核容量 |
| E（LLM 集成） | 稳定预算 + few-shot 样本 + 隐私策略 |
| F（HDGP 接入） | 接口合约冻结 |
| G（国内部署） | ICP 备案 + 境内法人实体 |
