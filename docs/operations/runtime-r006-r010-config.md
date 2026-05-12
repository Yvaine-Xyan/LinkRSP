---
title: R-006 / R-010 运行时参数说明
status: 现行
applies_to: Phase B 结算路径、自 Phase C 起持续有效
---

# R-006 / R-010 运行时参数说明

> **性质**：以下为**工程可调**的运行时开关，用于 Phase B/C 过渡与试点；**不改变** LRS-1.0 冻结常数（如 `D_base = 1.0`、Clip 边界）。  
> **实现**：[`internal/config/config.go`](../../internal/config/config.go) · 规则实现 `internal/rules/r006` · `internal/rules/r010`。

## 环境变量

| 变量 | 默认值 | 含义 |
|------|--------|------|
| `R006_ACCELERATION_THRESHOLD` | `1000` | R-006 判定「积分加速度异常」时使用的阈值系数（与规则实现内二阶差分统计量对比）。试点期可保持默认；上线前应按真实分布或治理决策调参并**记录变更原因**。 |
| `R010_GENESIS_END_TIME` | `2099-01-01T00:00:00Z` | 创世期结束时刻（RFC3339，UTC）。在此之前 R-010 的「创世后离群」分支可按规则设计处于保守/未激活语义；达到或超过该时刻后启用 post-genesis 统计。**不应长期依赖 2099 占位**——应在网络进入真实运营前，由维护者设为经治理或数据评审后的切点时间。 |

## 推荐策略（非强制）

1. **试点 / 研发**：可保留默认，专注打通链路与审计回放。  
2. **准备对外部协作者开放写接口前**：将 `R010_GENESIS_END_TIME` 设为真实「创世结束」或「统计启用」时间点，并在变更日志或治理记录中留痕。  
3. **R-006**：在积累足够日粒度积分序列后，用离线分析或监控分位数校准 `R006_ACCELERATION_THRESHOLD`，避免误杀或漏报。

## 交叉引用

- 算法规格：[核心算法规格 v1.0](../spec/linkrsp-core-algorithm-spec-v1.0.md)  
- 实施阶段：[实施计划 v1.0](../engineering/implementation-plan-v1.0.md) Phase B  
