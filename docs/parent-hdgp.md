# 与 HDGP 开源基线的关系（简述）

公开对照以 **[HDGP-Core](https://github.com/HumanDignityGuardian/HDGP-Core)** 为准：该仓库提供 **Meta-only** 的伦理与治理材料（如 `spec/HDGP_ETHICS_BASELINE.md`、`spec/HDGP_META_VS_JUDGE_SCOPE.md` 等），**不默认包含**可运行的 Engine/Judge 参考实现与运维门禁；与闭源主线的边界见 Core 仓库自述。

LinkRSP 在本仓库内自行定义 **LRS 劳务协议、社区与积分语义**；在工程上通过 **适配层**对接「策略判定 / 审计」能力——实现可以来自你方主仓、自研模块或兼容接口，**不要求**与任一 HDGP 仓库做代码级同步（与 HDGP 主线的独立性政策一致，仅作采用方理解）。

**概念对齐（摘录）**：尊严底价与反诱导、判定可追溯、规范与叙事分层——与 Core 中伦理基线方向一致；具体规则 ID 与集成契约以你方实际部署的 Engine/策略包版本为准。
