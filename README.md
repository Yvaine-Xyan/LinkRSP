# LinkRSP

**Link Reshuffling & Survival Protocol**  
*Decentralized protocol for labor rebalancing and survival resilience.*

> Exchange time, link surplus.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Governance: HDGP](https://img.shields.io/badge/Governance-HDGP-blue)](https://hdgp-protocol.com)
[![Open meta baseline](https://img.shields.io/badge/Open%20meta-HDGP--Core-informational)](https://github.com/HumanDignityGuardian/HDGP-Core)
[![Status: Architecture phase](https://img.shields.io/badge/Status-Architecture%20phase-orange)]()

---

## What this is

LinkRSP is a **decentralized labor-mutual-aid protocol** and supporting specification set. It is not positioned as a consumer app or a platform company. The design frames it as **backup civic infrastructure for labor coordination** when formal employment, social insurance, and public assistance become less reliable.

**Working premise:** automation displaces standardized production roles at scale. In non-standard, high-touch, and physically embodied work (e.g., remote care escort, elder care, complex in-person collaboration), **human physical presence remains scarce**. LinkRSP aims to anchor credibility and exchange semantics for that scarcity using auditable physical work and attestation.

**Credits are not currency.** In-repo specifications treat LRS credits as **rights / priority credentials** tied to evidence-backed physical work, not securities or guaranteed fiat claims. Cash settlement is out of scope for protocol custody (see governance draft).

Canonical documents: [`docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md`](docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md), [`docs/spec/linkrsp-core-algorithm-spec-v1.0.md`](docs/spec/linkrsp-core-algorithm-spec-v1.0.md), [`docs/reports/feasibility-assessment-v1.0-alpha.md`](docs/reports/feasibility-assessment-v1.0-alpha.md), [`docs/security/threat-model-scope-v1.0.md`](docs/security/threat-model-scope-v1.0.md), [`docs/governance/appeals-and-public-record.md`](docs/governance/appeals-and-public-record.md), [`SECURITY.md`](SECURITY.md), full index [`docs/README.md`](docs/README.md).

---

## Core mechanisms

### LRS credits (LRS-1.0)

Each credit increment must map to **observable physical duration and attestation strength** (thermodynamic metaphor used in docs; see [`docs/spec/thermodynamic-economic-model-research-v1.0.md`](docs/spec/thermodynamic-economic-model-research-v1.0.md)).

```
Credits_delta = (T_phy × V_bit) × Clip((D_base + ΣW_risk) / K_global, 0.8, 3.0)
```

| Symbol | Meaning |
|--------|---------|
| `T_phy` | Physical duration in minutes; primary non-fictional input |
| `V_bit` | Attestation tier weight (0.1 / 0.5 / 1.0) for V0 / V1 / V2 paths |
| `D_base` | Baseline difficulty; **frozen at 1.0** for LRS-1.0 (physical baseline; any future change is a governed spec revision, not a per-tenant knob) |
| `ΣW_risk` | Risk weights summed from community governance; must be selected from the **HDGP atomic labor attribute library** and audited |
| `K_global` | Network-wide normalization constant; genesis lock rules apply (see algorithm spec) |
| `Clip(x, 0.8, 3.0)` | Lower bound implements the documented **dignity floor** on the normalized ratio; upper bound caps high-risk premia |

### Dual settlement

- **Cash layer (external):** parties may agree fiat compensation off-protocol; the protocol does not intermediate custody.
- **Protocol layer (internal):** physical work must be recorded with attestation; credits are ledgered as **non-custodial rights** (shadow-account semantics per feasibility §3.4).

### S/N mesh communities

- **N-type (incubation):** default tier; internal “soft” reputation semantics possible; must remain auditable.
- **S-type (operational):** reached via **IPO-style** transition (compliance scan + **Judge** review), enabling higher-assurance tasks.

**1+1 rule:** one UID may be active in **one S-type** and **one N-type** community per period (see [`docs/governance/management-regulations-draft-v1.0-alpha.md`](docs/governance/management-regulations-draft-v1.0-alpha.md)).

---

## Governance and HDGP

Ethical and policy semantics align with **HDGP** as a **meta / audit layer** ([project site](https://hdgp-protocol.com)). The open **Meta baseline** is published separately as **[HDGP-Core](https://github.com/HumanDignityGuardian/HDGP-Core)** (Apache-2.0). LinkRSP keeps domain logic in this repository and integrates via **adapters** (see [`docs/parent-hdgp.md`](docs/parent-hdgp.md), [`docs/engineering/architecture-v0.1.md`](docs/engineering/architecture-v0.1.md)).

Typical separation:

- **Input path:** community rules and task descriptions evaluated before publication.
- **Output path:** settlement narratives and anomalies reviewed; circuit-breaking where required.

---

## Open source

Licensed under **MIT** ([`LICENSE`](LICENSE)). Contributions are welcome via PR; scope includes **HDGP RuleEngine extensions for LinkRSP** (see feasibility §6: **32** enforcement tracks — 12 R + 16 S + 4 hybrid-class in §6.6; **28** unique rule IDs; target **30–50** engine rules with splits), red-team reports, **Handshake v2** (BLE / QR UX), and translations.

**License note (risk):** MIT permits downstream forks to ship proprietary derivatives without upstream reciprocity. **AGPL** (or other copyleft) would require derivative networked services to publish corresponding source, at the cost of compatibility and adoption friction. **No license change is proposed now**; reassess after the first production cohort and partner constraints.

---

## Technology direction (architecture phase)

Backend language is **locked to Go** for LinkRSP core services (see [`docs/engineering/technology-strategy-v1.0.md`](docs/engineering/technology-strategy-v1.0.md) §3.1): aligns with typical **HDGP Engine** implementations and reduces cross-language adapter cost. Other layers remain as documented. Full direction: [`docs/engineering/technology-strategy-v1.0.md`](docs/engineering/technology-strategy-v1.0.md). Summary:

| Layer | Direction | Notes |
|-------|-----------|--------|
| Policy / HDGP | Versioned **Rule Engine** via HTTP/gRPC **adapter**; **Go** service core | Deterministic rules first; HDGP-Core remains Meta-first; runnable Engine is deployer-specific |
| Attestation | BLE + GPS + NTP; QR-assisted UX for V2 | See algorithm spec §4 |
| Ledger | Append-only / event-sourced **LRS ledger** design | Avoid mutable balance-only tables without audit trail |
| Semantic audit | Pluggable **LLM Oracle**, batch queue | May degrade to rules + manual sampling early |
| Settlement | Dual-track: fiat adapters external; credits internal | No protocol custody of fiat |

---

## Document and delivery status

- [x] Whitepaper v1.0 Alpha — [`docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md`](docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md)
- [x] LRS core algorithm specification v1.0 — [`docs/spec/linkrsp-core-algorithm-spec-v1.0.md`](docs/spec/linkrsp-core-algorithm-spec-v1.0.md)
- [x] Thermodynamic economic model research v1.0 — [`docs/spec/thermodynamic-economic-model-research-v1.0.md`](docs/spec/thermodynamic-economic-model-research-v1.0.md)
- [x] Management regulations draft v1.0 Alpha — [`docs/governance/management-regulations-draft-v1.0-alpha.md`](docs/governance/management-regulations-draft-v1.0-alpha.md)
- [x] Feasibility assessment v1.0 Alpha — [`docs/reports/feasibility-assessment-v1.0-alpha.md`](docs/reports/feasibility-assessment-v1.0-alpha.md)
- [x] Engineering architecture draft v0.1 — [`docs/engineering/architecture-v0.1.md`](docs/engineering/architecture-v0.1.md)
- [x] Technology strategy v1.0 — [`docs/engineering/technology-strategy-v1.0.md`](docs/engineering/technology-strategy-v1.0.md)
- [x] HDGP integration narrative (within feasibility + architecture)
- [x] Threat model scope v1.0 — [`docs/security/threat-model-scope-v1.0.md`](docs/security/threat-model-scope-v1.0.md)
- [ ] RuleEngine LinkRSP extensions to **30–50** rules (baseline: **32** tracks / **28** IDs in feasibility §6)
- [x] **Appeals channel (pre-code)** — [`docs/governance/appeals-and-public-record.md`](docs/governance/appeals-and-public-record.md) · public log [`docs/operations/appeal-log.md`](docs/operations/appeal-log.md)
- [ ] Handshake v2 implementation
- [ ] LRS ledger prototype
- [ ] First minimal closed loop (two real participants, one fully traced task)

---

## Contact

Yvaine He — Founder & Architect  
yvaine.he83@gmail.com · [hdgp-protocol.com](https://hdgp-protocol.com)

---

*When automation displaces occupations, LinkRSP is framed as one possible anchor for humans to verify their coordinates in the physical world.*

---

# LinkRSP（中文）

**劳动力再平衡与生存韧性协议**  
*Link Reshuffling & Survival Protocol（项目代号 linkrsp）*

> 生存协议，织网自救。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![治理：HDGP](https://img.shields.io/badge/治理-HDGP-blue)](https://hdgp-protocol.com)
[![开源 Meta：HDGP-Core](https://img.shields.io/badge/开源%20Meta-HDGP--Core-informational)](https://github.com/HumanDignityGuardian/HDGP-Core)
[![状态：架构阶段](https://img.shields.io/badge/状态-架构阶段-orange)]()

---

## 这是什么

LinkRSP 是一套**去中心化劳动力互助协议**及其规范材料集合。它不定位为消费级 App 或平台公司；设计取向是：在正规就业、社会保险与公共救助承压时，为**可审计的物理劳务协作**提供一套**备用型基础设施语义**。

**工作前提：** 自动化系统性替代标准化生产岗位；在非标准化、高情感密度、强物理在场场景（如异地陪诊、高龄照护、复杂环境协作）中，**人类物理在场仍稀缺**。LinkRSP 尝试为该稀缺性建立**可举证**的价值锚定与互助调度语义。

**积分不是货币。** 仓库内规范将 LRS 积分表述为与**可审计物理工时**绑定的**权益 / 优先权凭证**，不构成证券或法币兑付承诺；法币结算不由协议托管（见管理制度草案）。

权威文档入口：[`docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md`](docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md)、[`docs/spec/linkrsp-core-algorithm-spec-v1.0.md`](docs/spec/linkrsp-core-algorithm-spec-v1.0.md)、[`docs/reports/feasibility-assessment-v1.0-alpha.md`](docs/reports/feasibility-assessment-v1.0-alpha.md)、[`docs/security/threat-model-scope-v1.0.md`](docs/security/threat-model-scope-v1.0.md)、[`docs/governance/appeals-and-public-record.md`](docs/governance/appeals-and-public-record.md)、[`SECURITY.md`](SECURITY.md)，完整索引见 [`docs/README.md`](docs/README.md)。

---

## 核心机制

### LRS 积分（LRS-1.0）

积分增量须对应**可观测物理时长与核验强度**（热力学隐喻见 [`docs/spec/thermodynamic-economic-model-research-v1.0.md`](docs/spec/thermodynamic-economic-model-research-v1.0.md)）。

```
Credits_delta = (T_phy × V_bit) × Clip((D_base + ΣW_risk) / K_global, 0.8, 3.0)
```

| 变量 | 含义 |
|------|------|
| `T_phy` | 物理时长（分钟），主要不可虚构输入 |
| `V_bit` | 核验强度权重（0.1 / 0.5 / 1.0），对应 V0 / V1 / V2 |
| `D_base` | 基准难度；**LRS-1.0 规格冻结为 1.0**（物理基石；若未来变更须走治理下的 spec 修订，不作随意租户级开关） |
| `ΣW_risk` | 风险系数累加，由社区自治定义，须从 **HDGP 原子劳务库**勾选并接受审计 |
| `K_global` | 全网归一化常数；含创世期锁定等规则（见算法 spec） |
| `Clip(x, 0.8, 3.0)` | 下界实现已落盘文档中的**尊严底价**（作用于归一化比值）；上界限制高风险溢价 |

### 双轨结算

- **现金层（外结算）：** 双方可在协议外自行商定法币报酬；协议不做法币托管。
- **协议层（内结算）：** 强制记录物理工时与存证；积分以**非托管权益**入账（影子账户语义见可行性评估 §3.4）。

### 双轨社区（S/N Mesh）

- **N 级（孵化期）：** 默认层级；可有「软声望」语义，仍须可审计。
- **S 级（运行期）：** 经 **IPO 式**跃迁（合规扫描 + **Judge** 评估）后，可承载更高保证任务。

**1+1 Rule：** 同一 UID 每周期仅可关联 **1 个 S 级**与 **1 个 N 级**社区（见 [`docs/governance/management-regulations-draft-v1.0-alpha.md`](docs/governance/management-regulations-draft-v1.0-alpha.md)）。

---

## 治理与 HDGP

伦理与策略语义与 **HDGP** 的 **meta / 审计层**对齐（[项目站点](https://hdgp-protocol.com)）。开源 **Meta 基线**见 **[HDGP-Core](https://github.com/HumanDignityGuardian/HDGP-Core)**（Apache-2.0）。LinkRSP 业务逻辑在本仓库维护，通过**适配器**接入外部引擎（见 [`docs/parent-hdgp.md`](docs/parent-hdgp.md)、[`docs/engineering/architecture-v0.1.md`](docs/engineering/architecture-v0.1.md)）。

常见分工：

- **输入侧：** 社区规则与任务描述在发布前评估。
- **输出侧：** 结算叙事与异常检测；必要时熔断。

---

## 开源策略

以 **MIT** 开源（[`LICENSE`](LICENSE)）。欢迎 PR，范围包括 **HDGP RuleEngine 的 LinkRSP 场景扩展**（可行性 §6：**32** 条执行轨 = R 12 + S 16 + §6.6 混合类 4；**28** 个唯一规则 ID；引擎目标 **30–50** 条含拆分）、红队与压测报告、**Handshake v2**（BLE / 二维码 UX）、多语言文档等。

**许可说明（风险）：** MIT 允许下游在较少义务下 fork 并闭源衍生；**AGPL** 等 copyleft 可要求网络衍生服务公开对应源码，但会带来兼容性与采用面成本。**现阶段不改许可证**；待首批真实用户与合作伙伴约束明确后再评估。

---

## 技术栈（架构阶段）

**LinkRSP 核心后端语言锁定为 Go**（见 [`docs/engineering/technology-strategy-v1.0.md`](docs/engineering/technology-strategy-v1.0.md) §3.1），与常见 **HDGP Engine（Go）** 对齐，降低跨语言边界与运维门槛。其余层仍按该文档执行。摘要：

| 层 | 方向 | 说明 |
|----|------|------|
| 策略 / HDGP | **Rule Engine** 经 HTTP/gRPC **适配器**；**Go** 核心服务 | 规则确定性优先；HDGP-Core 以 Meta 为主；可运行 Engine 由部署方选择 |
| 物理存证 | BLE + GPS + NTP；V2 侧 QR 等 UX | 见算法 spec §4 |
| 账本 | **只追加 / 事件化** LRS 账本设计 | 避免无审计链路的可变余额表 |
| 语义审计 | 可插拔 **LLM Oracle**、批处理队列 | 早期可降级为规则 + 人工抽样 |
| 结算 | 双轨：法币走外部适配；积分走内部 | 协议不托管法币 |

---

## 当前状态

- [x] 白皮书 v1.0 Alpha — [`docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md`](docs/whitepaper/linkrsp-whitepaper-v1.0-alpha.md)
- [x] LRS 核心算法规格说明书 v1.0 — [`docs/spec/linkrsp-core-algorithm-spec-v1.0.md`](docs/spec/linkrsp-core-algorithm-spec-v1.0.md)
- [x] 热力学经济模型深度调研 v1.0 — [`docs/spec/thermodynamic-economic-model-research-v1.0.md`](docs/spec/thermodynamic-economic-model-research-v1.0.md)
- [x] 管理制度草案 v1.0 Alpha — [`docs/governance/management-regulations-draft-v1.0-alpha.md`](docs/governance/management-regulations-draft-v1.0-alpha.md)
- [x] 工程侧架构与技术策略草案 — [`docs/engineering/architecture-v0.1.md`](docs/engineering/architecture-v0.1.md) · [`docs/engineering/technology-strategy-v1.0.md`](docs/engineering/technology-strategy-v1.0.md)
- [x] 整体可行性评估 v1.0 Alpha — [`docs/reports/feasibility-assessment-v1.0-alpha.md`](docs/reports/feasibility-assessment-v1.0-alpha.md)
- [x] HDGP 集成叙事（含于可行性评估与工程架构）
- [x] 威胁模型范围 v1.0 — [`docs/security/threat-model-scope-v1.0.md`](docs/security/threat-model-scope-v1.0.md)
- [ ] RuleEngine LinkRSP 专属扩展至 **30–50** 条（基线 **32** 执行轨 / **28** 唯一 ID，见可行性 §6）
- [x] **申诉入口（代码前）** — [`docs/governance/appeals-and-public-record.md`](docs/governance/appeals-and-public-record.md) · 公开记录 [`docs/operations/appeal-log.md`](docs/operations/appeal-log.md)
- [ ] Handshake v2 实现
- [ ] LRS Ledger 原型
- [ ] 第一个最小闭环（两名真实参与者、一次完整可追溯任务）

---

## 联系

Yvaine He · Founder & Architect  
yvaine.he83@gmail.com · [hdgp-protocol.com](https://hdgp-protocol.com)

---

*当 AI 替代了职业，LinkRSP 被表述为人类在物理世界中确认自身坐标的一种可能锚点。*
