---
title: LLM 校准样本与 few-shot 维护
status: 前置（Phase E）
---

# LLM 校准样本（few-shot / 阈值）

本目录用于在 **引入 LLM Oracle 之前** 积累**可回放**的标注样本，与规则 YAML 中的 `few_shot_examples` 互补：

- **规则 YAML**：权威定义（prompt、阈值、审计范围）。  
- **`samples/`**：可按试点批次扩展的「额外正负例、边界例」，便于离线评审与版本化讨论，而不必每次改 50 条主规则文件。

## 布局

| 路径 | 说明 |
|------|------|
| [`samples/`](samples/) | 按规则 ID 分文件的 Markdown 样本（最小字段：input、期望 verdict、备注） |
| [LLM 接入 RFC v0.1](../../engineering/llm-oracle-integration-rfc-v0.1.md) | 工程边界与开关 |

## 维护原则

1. 每条样本应能映射到具体 `rule_id` 与 `audit_scope`。  
2. 至少保持 **2 PASS + 2 BLOCK** 的教学习惯（与 S 类规则规范一致）。  
3. 不在样本中放入真实个人身份信息；使用合成文本。  

## 当前优先规则（建议首批）

- S-001、S-002、S-005（可按社区试点调整；此处仅为落地起点）
