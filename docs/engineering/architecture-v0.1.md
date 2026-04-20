# LinkRSP 工程架构理解 · 评估与设计草案（v0.1）

> 性质：工程视角的补充说明；不构成法律或合规承诺。  
> 与《可行性评估报告》的关系：下文在**系统边界、数据与审计链、落地顺序**上展开，并标注报告中的工程缺口。  
> **相关文档**：[可行性评估](../reports/feasibility-assessment-v1.0-alpha.md) · [白皮书](../whitepaper/linkrsp-whitepaper-v1.0-alpha.md) · [核心算法规格 v1.0](../spec/linkrsp-core-algorithm-spec-v1.0.md) · [热力学经济模型调研 v1.0](../spec/thermodynamic-economic-model-research-v1.0.md) · [技术栈与长期策略 v1.0](technology-strategy-v1.0.md) · [威胁模型范围 v1.0](../security/threat-model-scope-v1.0.md) · [申诉与公开记录](../governance/appeals-and-public-record.md) · [管理制度草案](../governance/management-regulations-draft-v1.0-alpha.md) · [§6 规则评述](../reports/rule-scenarios-author-assessment.md) · [版本策略](../governance/versioning-policy.md)（发布物 semver 在未达上线标准前 ≤ v1.0）。

---

## 1. 整体架构理解（逻辑视图）

在工程上可将 LinkRSP 拆为五条**可独立演进**的轴线：

| 轴线 | 职责 | 典型组件 |
|------|------|----------|
| **身份与社区** | UID、S/N 社区归属、「1+1」约束、升 S 流程状态机 | Identity / Membership / Lifecycle 服务 |
| **任务与物理存证** | T_phy、V 档位、握手与设备 UX、原始证据对象存储 | Task、Attestation、Device/BLE 适配 |
| **LRS 核算** | Credits 公式、K_global、创世期锁定、Harness 与阻尼 | Ledger / Policy Engine（数值与不变量） |
| **治理与 Judge** | ΣW_risk 选自「原子劳务库」、投票权重、IPO 材料 | Governance、Voting、规则版本 |
| **合规与审计（HDGP 适配）** | 输入/输出侧判定、灰区队列、异步 LLM、存证与申诉索引 | Adapter → 外部或自托管 Engine；Audit Log；Queue Worker |

跨领域横切：**只追加审计日志**、**策略/规则版本号**、**fail-closed 降级**（与 HDGP 集成单页推荐一致：校验失败则不呈现高险输出）。

---

## 2. 对《可行性评估报告》的工程意见

**总体**：概念与公式层可落地；工程风险主要集中在 **证据可信度、K_global 稳定性、审计成本、治理流程的可执行性** 四条。报告对缓解策略已有方向，以下为补充与排序建议。

### 2.1 强项（保持）

- **物理锚 + Clip + Harness** 的分工清晰：不变量（R 类）放实时链，语义灰区放异步，符合成本与延迟约束。
- **影子账户 / 权益凭证定位** 有利于控制产品叙事与合规边界（实现上仍需具体法域评审）。
- **创世期 K_global=1.0** 对工程可测性友好：便于回归测试与早期解释成本。

### 2.2 建议在下一版规格中写死的工程参数

| 主题 | 建议 |
|------|------|
| **升 S 门槛** | 报告已给参考值；需在 `spec` 中固定为可配置项 + 默认值 + 迁移策略。 |
| **申诉** | 最小闭环：工单 ID、关联审计事件 ID、SLA 计时字段、对外状态枚举（比「仅邮箱」更可测）。 |
| **熔断误杀** | 除人工通道外，建议定义「只读解锁预览」与「复核事件」类型，避免重复熔断。 |
| **V=2 UX** | 将「动态二维码对碰」等列为**独立适配备注**（无障碍、弱网、老年模式），否则默认会沉到 V=0。 |

### 2.3 风险与缺口（工程侧）

- **K_global 阻尼与全网中位数**：需明确时间窗口、样本量下限、冷启动与低活跃社区的**回退策略**（避免除零或单点操纵）。
- **d²C/dt²**：需定义采样周期、窗口、群体基线；否则统计上易被噪声触发或滞后。
- **「HDGP 原子劳务库」**：若库尚未以机器可读格式发布，Judge 侧会阻塞；建议 LinkRSP 先自带 **最小只读目录（snapshot）+ 版本号**，再异步对齐上游。
- **LLM 批处理**：队列需 **幂等键**（任务/规则版本/content-hash），避免重复审同一对象。

---

## 3. 目标参考架构（v0.1 草案）

### 3.1 组件

```
[Clients] → [API Gateway] → [Domain Services]
                              ├── Identity / Community
                              ├── Task & Attestation
                              ├── LRS Ledger (Credits, K, damping)
                              └── Governance / Judge
        ↘ [Policy Adapter] → [Rule Engine 接口]（可替换实现）
        ↘ [Semantic Queue] → [LLM Worker]（可关闭）
        ↘ [Append-only Audit Store] → 索引 / 导出 / 申诉关联
```

### 3.2 数据与事件

- **核心业务表**：用户、社区、任务、存证证据引用、积分分录（建议**不可变分录 + 冲正分录**，而非原地改账）。
- **审计事件**：统一 schema（规则 ID、策略包版本、输入摘要、判定、latency、上游 trace）；与 HDGP 侧 `integrity_events` 类字段**对齐字段名子集**，便于对账。
- **创世期开关**：配置中心或链上/合约外置均可，但需 **可观测性**（当前处于创世/后创世）。

### 3.3 与 HDGP-Core 的边界

- **文档**：伦理与 Meta 语义以 [HDGP-Core](https://github.com/HumanDignityGuardian/HDGP-Core) 为公开对照。
- **代码**：Engine 是否采用、采用哪一发行版，由本仓库独立决策；适配层建议**接口稳定、实现可换**，避免与任一上游仓库形成硬耦合。

---

## 4. 分阶段交付（建议）

| 阶段 | 目标 | 验收要点 |
|------|------|----------|
| **P0** | 单社区、手动/半自动 T/V 记录 + 分录 + 审计日志 | 可追溯一条完整任务；无 K 波动争议 |
| **P1** | R 类规则集 + 升 S 状态机 + 申诉最小闭环 | 规则单测；熔断与申诉可演示 |
| **P2** | 灰区队列 + 批处理 LLM + 抽样复核 | 成本曲线可测；幂等与重试 |
| **P3** | K_global 全量 + 阻尼与滥用对抗硬化 | 压测与博弈场景回归 |

---

## 5. 许可策略（工程配合产品）

- **MIT**：简单、兼容性强，对库与文档友好；**不包含**显式专利授权条款（是否构成问题取决于你是否持有相关专利及合作方要求）。
- **Apache-2.0**：与许多基础设施工具链一致，含**专利授权**表述；若希望与 HDGP-Core **许可证家族**更一致、或预期企业贡献者关心专利条款，可评估迁移。
- **结论**：「是否足够」取决于社区与商业伙伴对**专利、商标、GPL 兼容**的期望；常见做法是 **MIT/Apache-2.0 二选一 + SECURITY/CONTRIBUTING 明确责任边界**，复杂合规再另附使用条款或部署指南（非代码许可证替代）。

---

*草案维护：随 spec 与实现迭代更新本文件。*
