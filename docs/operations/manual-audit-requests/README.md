# Manual Audit Requests（phase_1 人工摘要提交）

## 用途

`S-015`（批量空壳社区）与 `S-016`（协调投票）在 `phase_1_manual` 阶段需要人工提供统计摘要，才能触发 LLM 确认层。

本目录用于接收这些摘要，通过 PR 合并触发审计流程，天然留痕（与申诉公开记录同一类机制）。

交叉引用：规则规范 `docs/governance/rule-engine/README.md`；审计事件 schema `docs/spec/audit-event-schema-v1.0.md`。

---

## 提交方式

在本目录下新建文件，命名格式：

```text
{rule_id}-{subject_id}-{YYYYMMDD}.yaml
```

示例：

```text
S-015-community-abc123-20260420.yaml
```

PR 标题格式：

```text
[manual-audit] S-015 community-abc123
```

PR 必须由至少一名维护者 review 后合并。

---

## S-015 摘要格式

```yaml
rule_id: S-015
rule_version: "1.0"
submitted_by: "维护者UID或邮箱"
submitted_at: "2026-04-20T10:00:00Z"

subject:
  type: community
  id: "community-abc123"
  ipo_application_id: "ipo-xyz789"

feature_summary:
  similarity_method: tfidf_cosine
  top_matches:
    - community_id: "community-def456"
      ruleset_id: "ruleset-v1"
      similarity_score: 0.88
    - community_id: "community-ghi789"
      ruleset_id: "ruleset-v2"
      similarity_score: 0.76
  optional_signals:
    creator_uid_overlap: true
    ip_cidr_overlap: false
    registration_burst: true
    registration_burst_detail: "3个社区在同一小时内注册"

notes: "人工观察补充说明（可选）"
```

---

## S-016 摘要格式

```yaml
rule_id: S-016
rule_version: "1.0"
submitted_by: "维护者UID或邮箱"
submitted_at: "2026-04-20T10:00:00Z"

subject:
  type: vote_cluster
  id: "cluster-20260420-001"
  target_community_id: "community-abc123"
  target_ipo_id: "ipo-xyz789"

feature_summary:
  cluster_size: 5
  ipo_count_in_window: 8
  co_vote_alignment_rate: 0.875
  registration_time_span: "同一天内（2026-03-01）"
  ip_cidr_overlap_level: "高度重叠（/24段）"
  optional_signals:
    same_creator_overlap: false
    same_rule_similarity_group: true
    similarity_group_rule_id: "S-015"
    similarity_group_communities:
      - "community-abc123"
      - "community-def456"

notes: "人工观察补充说明（可选）"
```

---

## 合并后处理（目标行为）

合并后建议实现以下目标行为（不要求本周完成）：

- PR 合并即创建 `audit_event`（`triggered_by: manual_review`，`reviewer_uid` 填合并者）。
- 将 `feature_summary` 写入 `audit_event.evidence.feature_summary`，以保证可回放。
- 触发异步 LLM 确认层（`llm_role: confirmation_only`），并将结果更新回同一 `audit_event`。

