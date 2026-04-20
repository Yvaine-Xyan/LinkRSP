# LinkRSP Rule Engine 扩展（HDGP 集成）— 规则编写规范（v1.0）

本目录用于承载 LinkRSP 场景的规则扩展草案，作为 **HDGP Rule Engine / AI-Oracle** 的起草输入。

> 交叉引用：可行性评估 §6（规则清单与阈值口径）见 `docs/reports/feasibility-assessment-v1.0-alpha.md`。  
> 规则条数口径：**32 条执行轨**（12 R + 16 S + 4 混合类编排），**28 个唯一规则 ID**（R-001—R-012，S-001—S-016）。

---

## 目录结构

- `r-class/`：R 类（确定性判断，Rule Engine 直接执行）
- `s-class/`：S 类（LLM 语义审计，灰区队列）
- `mixed/`：混合类（来自可行性评估 §6.6；在实现中通常体现为 **R 类的确定性门禁 + S 类的统计/相似性审计** 的组合编排）

---

## 规则 YAML 最小字段

R 类（确定性）建议至少包含：

- `rule_id`：例如 `R-001`
- `version`：固定字符串（当前为 `"1.0"` 文稿）
- `category`：分类枚举（建议小写蛇形）
- `trigger`：`pre_execution` | `post_execution` | `ipo_scan` | `registration` | `periodic_audit`（可扩展）
- `severity`：`block` | `warn` | `flag`
- `description`
- `inputs`：字段列表
- `condition`：可执行的伪查询/表达式（实现侧需映射为具体查询）
- `action`：`verdict`、`log_fields`、`notify`、`audit_trail`
- `false_positive_notes`

S 类（语义）建议至少包含：

- `audit_method: llm_oracle`
- `queue`、`batch_threshold`
- `prompt_template`（必须版本化）
- `few_shot_examples`（强烈建议；用于降低误报率）
- `reasonableness_anchor`（可选但推荐；为“合理性”判断提供显式锚点，避免模型自造标准）
- `condition_notes`（阈值与降级策略）
- `action`（FLAG/BLOCK 路由）

---

## few-shot 注入约定（渲染规则）

为避免不同实现者对 prompt 注入方式不一致，约定如下：

- `few_shot_examples` 为有序列表；实现侧按出现顺序分组为：
  - `PASS` 前两条依次注入 `{{few_shot_pass_1}}`、`{{few_shot_pass_2}}`
  - `BLOCK` 前两条依次注入 `{{few_shot_block_1}}`、`{{few_shot_block_2}}`
- 若某类不足 2 条：
  - 缺失项以空字符串注入，并将 `confidence` 阈值策略自动收紧为 **只 FLAG 不自动 BLOCK**；
  - 同时在审核流程中标记“few-shot 不完整”，优先补齐样本后再开启自动拦截。

> 备注：该约定仅定义「如何把 YAML 的 few-shot 文本注入 prompt」；不限制你方在实现侧进一步增加系统提示、上下文元信息或审计字段。

---

## 触发词预筛（可选优化）

对 S 类规则中包含明确触发词表的情况（例如 `S-011` 的 `dehumanization_trigger_words`），建议实现侧在调用 LLM 前做一次**关键词预筛**：

- 命中触发词：进入灰区队列（LLM 审计）
- 未命中触发词：可跳过本条规则以节省 token（仍可由其他规则覆盖）

该优化不改变规则语义，只改变审计成本结构；需保证预筛逻辑与词表版本可追溯。

---

## 版本与冻结策略

- 本目录规则文件的 `version: "1.0"` 属于**文稿版本**，不代表对外发布物 semver 变化（见 `docs/governance/versioning-policy.md`）。
- 任何阈值（如 200km/h、16h、24h、0.8/3.0）必须与可行性评估正文一致；如需调整，先改可行性评估或补充 `spec`，再改规则。

---

## 审核与合并流程（建议）

1. 规则新增/修改必须通过 PR。
2. PR 必须包含：
   - 规则 YAML 变更
   - 至少 1 个测试用例描述（可先以文字用例形式，后续再落地为自动化）
3. 对 S 类规则，必须同时更新 prompt 的版本与误判说明。

