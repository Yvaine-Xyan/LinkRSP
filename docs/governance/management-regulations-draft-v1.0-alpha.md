# Linkrsp 管理制度草案

**项目代号**：linkrsp  
**管理归属**：hdgp（厚德归朴）  
**版本**：v1.0 Alpha

> 交叉引用：白皮书 [`../whitepaper/linkrsp-whitepaper-v1.0-alpha.md`](../whitepaper/linkrsp-whitepaper-v1.0-alpha.md)；核心算法规格 [`../spec/linkrsp-core-algorithm-spec-v1.0.md`](../spec/linkrsp-core-algorithm-spec-v1.0.md)；热力学经济调研 [`../spec/thermodynamic-economic-model-research-v1.0.md`](../spec/thermodynamic-economic-model-research-v1.0.md)；可行性评估（含高频违规场景）[`../reports/feasibility-assessment-v1.0-alpha.md`](../reports/feasibility-assessment-v1.0-alpha.md) §6；工程架构 [`../engineering/architecture-v0.1.md`](../engineering/architecture-v0.1.md)；技术栈与长期策略 [`../engineering/technology-strategy-v1.0.md`](../engineering/technology-strategy-v1.0.md)；申诉与公开记录 [`appeals-and-public-record.md`](appeals-and-public-record.md)。版本策略 [`versioning-policy.md`](versioning-policy.md)。

---

## 一、身份准入与限制管理（Governance of Identity）

为防止精力稀释与信用投机，系统对参与者执行硬性约束。

**「1+1」双轨约束**：每个唯一 UID 在同一周期内，仅允许关联 **1 个强任务社区（S-Type）** 和 **1 个非强任务社区（N-Type）**。  
**目的**：使个体在生存层（S）保持专注与责任，在协作层（N）保持共识孵化与创作的纯粹性。

**身份切换冷却期**：用户退出当前 S 级社区并加入新社区时，系统设置物理冷却期，防止通过频繁切换身份进行恶意洗分。（具体参数由后续 `spec` 固定为可配置项 + 默认值。）

---

## 二、社区生命周期与「升 S」机制（Community Lifecycle & IPO）

linkrsp 不预设社区实质，但通过 **评审板块（Judge）** 管控社区的价值演化。

**N 级阶段（孵化期）**：新建社区默认为 N 级，进行理念讨论、非标协作与互助尝试。此阶段积分可为「软积分」，侧重社区内部声望（与白皮书「影子系统」叙事一致，工程实现上仍须可审计）。

**升 S 跃迁（IPO 机制）**

- **申请条件**：发起者提交自治规则，并满足成员基数、存证任务量等硬性门槛（参考值见可行性评估报告）。  
- **伦理合规扫描**：须通过 **hdgp 网关**审计，确保规则不含奴役、欺诈或剥夺尊严的逻辑。  
- **全民评估**：在 Judge 板块进行自然语言公示，接受共识校验与逻辑修正。

**S 级阶段（运行期）**：获「升 S」认证后，社区可发布具有刚性对价的劳务任务；积分为具备跨社区承兑潜力的「硬积分」（语义上仍为**权益与优先权**，非货币化承诺）。

---

## 三、经济与结算管理（Economic & Settlement Policy）

**金钱交易不干预原则**：系统不干预用户间法币交易，允许双方自主商定报酬以解决当下温饱。  
**声明**：linkrsp **不提供**资金托管与担保；现金层风险由用户自行承担。

**积分生成锚定**：积分生成严格挂钩 **T（时间）、V（核验）、D（难度）** 三维指标；禁止脱离物理工时的算法奖励，使积分成为劳动力资源的可审计备份。

---

## 四、风险监控与熔断机制（Risk Control & Circuit Breaker）

**动态熔断逻辑**

- **异常拦截**：网关监测到虚假任务或数据通胀时，自动锁定相关账户。  
- **红线惩罚**：若 S 级社区触碰 hdgp 伦理红线，系统执行「降级熔断」，强制退回 N 级，并公开标记发起人信用记录。

**审计追溯权**：linkrsp 保留对存证记录的底层审计权；在严重合规风险下，协议发起方可依 **hdgp 准则**进行人工干预。（须配套申诉与复核事件类型，见工程架构草案。）

---

## 五、社区自治与版本化管理

**规则透明度**：自治规则在 Judge 板块通过后，即成为该社区「宪章」；重大修订须重新公示与评审，保证成员对生存环境的可预期性。

---

## 架构师声明

linkrsp 的管理不以行政控制为目的，而是为了维护物理规则的公正性。**我们不管理「人」，我们只管理「协议的履行」。**

---

*草案版本：v1.0 Alpha；发布物版本策略见 [`versioning-policy.md`](versioning-policy.md)。*
