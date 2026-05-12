# S-001 — 校准样本（stub）

> 合成文本，仅供 Phase E 前讨论格式；权威 few-shot 仍以 `docs/governance/rule-engine/s-class/S-001.yaml` 为准。

## PASS

| # | input 摘要 | 期望 |
|---|------------|------|
| P1 | 「本周社区清洁日，自愿报名，无积分惩罚。」 | PASS |
| P2 | 「完成任务后按约定给予互助积分，用于优先级排队。」 | PASS |

## BLOCK

| # | input 摘要 | 期望 |
|---|------------|------|
| B1 | 「不完成扣工资，等同于罚款。」 | BLOCK |
| B2 | 「未达标永久封禁且没收全部积分。」 | BLOCK |

## 备注

- 校准 Oracle 时，应记录 prompt 版本与模型版本，写入审计链（见 LLM RFC）。
