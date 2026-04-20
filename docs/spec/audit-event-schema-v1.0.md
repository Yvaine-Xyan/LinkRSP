# LinkRSP Audit Event Schema v1.0

> 本文档定义所有规则判定产生的审计事件最小结构。  
> 任何规则实现（R 类 / S 类 / 混合类 / 跨社区模式识别）的输出必须符合本结构，  
> 以保证审计留痕可回放、可申诉、可外部验收。
>
> 交叉引用：规则规范 `docs/governance/rule-engine/README.md`；申诉与公开记录 `docs/governance/appeals-and-public-record.md`。

---

## 1. 最小字段定义

```json
{
  "event_id": "string, UUIDv4, 幂等键",
  "schema_version": "string, 当前为 1.0",

  "rule": {
    "rule_id": "string, 例如 R-001 / S-007 / S-009b",
    "rule_version": "string, 例如 1.0",
    "category": "string, 对应规则 YAML 的 category 字段",
    "policy_bundle_version": "string | null, 未来接入签名策略包时填写"
  },

  "scope": {
    "trigger": "string, pre_execution | post_execution | ipo_scan | registration | periodic_audit | ...",
    "audit_scope": "string, single_document | cross_community_pattern"
  },

  "subject": {
    "type": "string, task | community | ipo_application | uid | vote_cluster | ruleset",
    "id": "string, 对应 subject.type 的唯一标识",
    "secondary_id": "string | null, 例如 R-001 的冲突任务ID / S-015 的匹配社区ID"
  },

  "verdict": {
    "result": "string, PASS | FLAG | BLOCK",
    "severity": "string, block | warn | flag",
    "confidence": "number | null, 0.0-1.0; S 类/LLM 输出必填，R 类为 null",
    "auto_actioned": "boolean, 是否触发自动拦截/熔断/退回"
  },

  "evidence": {
    "matched_pattern": "string | null, 命中的模式描述/矛盾点摘要",
    "trigger_words_hit": "array<string> | null, 预筛命中的触发词",
    "feature_summary": "object | null, S-015/016 相似度/投票统计摘要等",
    "conflicting_ids": "array<string> | null, 冲突对象ID列表（如 R-001）",
    "config_snapshot": "object | null, 触发该事件时使用的 comparison_window/threshold 配置快照"
  },

  "trace": {
    "trace_id": "string, 跨服务追踪ID",
    "timestamp_utc": "string, ISO8601, 判定发生时间（统一 UTC）",
    "triggered_by": "string, system_auto | manual_review",
    "reviewer_uid": "string | null, 人工触发/合并者/复核者标识"
  },

  "action_taken": {
    "notified": "array<string>, 被通知的角色列表（例如 submitter / operator / community_admin）",
    "routed_to": "string | null, human_review_queue 等",
    "appeal_eligible": "boolean, 是否允许申诉"
  }
}
```

---

## 2. 幂等键约定

- `event_id` 使用 UUIDv4，由产生审计事件的服务生成。
- 同一规则对同一 subject 在同一 trigger 时间点内不应产生重复 event。实现侧可用以下键做去重检查（示例）：

```text
(rule.rule_id, subject.type, subject.id, scope.trigger, timestamp_utc 截断到分钟)
```

> 说明：去重键为实现建议；最终以业务链路（结算/发布/IPO）是否允许重复判定为准。

---

## 3. S 类规则的特殊要求（可回放）

- `verdict.confidence` 必填。
- `evidence.feature_summary` 必须包含 **LLM 实际接收的 prompt 摘要**（不含原始全文），以保证审计可回放。
- 原始文本通过 `subject.id` 关联查询，不直接写入 `audit_event`（避免隐私与存储放大）。

---

## 4. cross_community_pattern 的特殊要求（S-015 / S-016）

当 `scope.audit_scope = cross_community_pattern` 时，`evidence.feature_summary` 至少包含：

- `comparison_window` 的快照（source/days/min_sample）
- `feature_extraction` 的方法与阈值
- S-015：相似度方法、Top matches 与分数
- S-016：集群大小、对齐率、注册时间跨度等统计摘要

并建议将 `cross_rule_signals`（若触发）写入 `evidence.feature_summary` 以便复核与复跑。

