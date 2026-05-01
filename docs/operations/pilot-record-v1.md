---
title: Phase C 最小试点记录 v1
status: 已完成
---

# Phase C 最小试点记录 v1

## 记录目的

为 Phase C 收口提供**可回放证据**：同一条端到端试点任务在 DB 中形成 `tasks` / `attestations` / `ledger_entries` / `audit_events` 的一致写入，并可通过内部查询接口回放投影余额。

## 匿名化说明

- 参与者 UID 使用试点约定字符串（不绑定真实身份）。
- 存证引用为占位哈希（可替换为真实照片哈希）。

## 试点结果（本次收口凭证）

- **完成时间**：2026-05-02（UTC+8）
- **环境**：Supabase PostgreSQL + LinkRSP REST API（共享密钥门禁可选；本次以本地兼容模式跑通）
- **任务 ID**：`340d2b99-4702-4657-bfb9-cb7527a493e5`
- **存证 ID**：`c39e23e1-e35f-4439-9809-a16f21d6df87`
- **账本分录 ID（commit）**：`f2370fbd-45fc-4ce1-a3d4-29fe07986f58`
- **credits_delta**：15（30 分钟 × V=1 权重 0.5 × Clip(1.0/1.0, 0.8, 3.0)）
- **commit 幂等性**：同一任务重复提交 settlement commit 返回相同 `ledger_entry_id`
- **审计事件完整性（task 维度）**：至少包含 `R-001 R-002 R-003 R-004 R-005 R-006 R-009 R-010`

## 交叉引用

- Runbook：`docs/operations/pilot-runbook-v1.0.md`
- 实施计划：`docs/engineering/implementation-plan-v1.0.md`

