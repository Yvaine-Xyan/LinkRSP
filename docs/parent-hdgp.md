# 与 HDGP-Protocol（上层指导）的对齐说明

**本地路径**：`D:\HDGP\HDGP-Protocol`

## HDGP 在理念上提供什么

HDGP（厚德归朴）将自身定位为**高风险智能系统的工程治理与审计框架**：人类尊严优先、默认安全、输出前可审计的 Rule Engine（`/evaluate` 等），以及版本化、可签名的策略包。在该工作区内请从 `README.md`、`spec/HDGP_ETHICS_BASELINE.md` 等入口阅读。

**与 LinkRSP 的衔接点（概念层）**：

- **尊严底价 / 反诱导**：HDGP 伦理基线中的「保护性拦截」与 LinkRSP 中 Clip 下限、Harness 熔断、语义审计（防剥削、防虚假承诺）同一脉络。
- **审计与可追溯**：HDGP 要求判定可溯源；LinkRSP 将积分产出、社区规则与申诉链存证，依赖同一套「可举证」思路。
- **分层**：HDGP 文档中「规范意图 / 工程假设 / 叙事」分层（见 `docs/HDGP_PHILOSOPHY_ENGINEERING_LAYERING_DRAFT.md`）与 LinkRSP 区分「权益凭证 vs 货币」「基础设施 vs 运营平台」的表述方式一致，避免把愿景写进硬编码规则。

## LinkRSP 在 HDGP 之上解决什么

LinkRSP 聚焦 **AI 时代物理劳务的再平衡与长期韧性**：S/N 社区、LRS 积分、物理存证（T/V）、与法币并行的双轨设计。HDGP 在此项目中主要承担 **meta 层**：输入侧（规则与任务文本）与输出侧（积分与叙事）的合规与语义守门，并通过 **RuleEngine 扩展**覆盖 LinkRSP 高频违规场景（R/S 类规则）。

集成接口的权威说明以 HDGP 侧为准，例如：

- `docs/HDGP_INTEGRATION_ONEPAGER.md` — 最小五步接入
- `spec/HDGP_INTEGRATION_SPEC.md`、`spec/HDGP_ENGINE_API_SPEC.md` — 完整规范

## 本仓库职责边界

| 内容 | 存放位置 |
|------|----------|
| LinkRSP 专属可行性、算法与治理叙事 | 本仓库 `docs/` |
| HDGP Engine、策略包与一致性测试 | `HDGP-Protocol` / HDGP-Core |

开发时建议同时打开上述本地工作区，变更规则 ID 或审计语义时以 HDGP 的 `spec/` 与 `conformance-tests/` 为一致性基准。
