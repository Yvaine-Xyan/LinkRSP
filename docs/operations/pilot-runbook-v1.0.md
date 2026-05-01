---
title: Phase C 最小试点操作手册 v1.0
status: 进行中
applies_to: Phase C 收口前的"两人一次任务"端到端验证
---

# Phase C 最小试点操作手册

> 本手册解决一个具体问题：在没有 UI、没有社区注册流程、没有移动端的情况下，
> 两名真实参与者如何通过现有 API 完成一次端到端任务并让数据进入 DB。

---

## 1. 前置认知：哪些东西"现在不需要"

| 缺失项                    | 为什么现在不是阻塞                                                                        |
| ------------------------- | ----------------------------------------------------------------------------------------- |
| 用户注册 / 身份验证       | `uid` 当前为自由字符串；两人约定好各自的 uid 即可（如 `alice-pilot-01` / `bob-pilot-01`） |
| 社区创建 (N/S 级)         | `community_id` 为可选字段；Phase C 试点任务可不绑定社区                                   |
| IPO 跃迁渠道              | IPO 是 Phase D+ 治理动作，与任务积分记录无关                                              |
| 移动端 BLE/GPS 握手 (V=2) | Phase C 使用 V=0（自报）或 V=1（照片摘要），V=2 进 Phase E/F                              |
| 积分提现 / 法币兑换       | 白皮书承诺：积分在链外不可直接兑现；Phase C 只验证分录写入正确                            |

---

## 2. 端到端流程（纯 API，curl 或 Postman 即可）

### 步骤 0：环境准备

```bash
export BASE_URL="https://<your-backend-host>"     # 或 http://localhost:9090
export SECRET="<LINKRSP_API_SHARED_SECRET 值>"
export ALICE="alice-pilot-01"
export BOB="bob-pilot-01"
```

### 步骤 1：甲方创建任务

```bash
TASK=$(curl -sX POST "$BASE_URL/api/v1/tasks" \
  -H "X-API-Shared-Secret: $SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "uid_submitter": "'"$ALICE"'",
    "description_text": "帮我搬 3 箱书到 3 楼，约 30 分钟",
    "start_time_utc": "2026-05-01T09:00:00Z",
    "end_time_utc":   "2026-05-01T09:30:00Z"
  }')
echo "$TASK" | python3 -m json.tool
TASK_ID=$(echo "$TASK" | python3 -c "import sys,json; print(json.load(sys.stdin)['task_id'])")
echo "$TASK_ID" > /tmp/linkrsp_task_id
```

系统自动执行 R-003（任务时长上限）和 R-004（时间戳倒置）检查并写入审计事件。

### 步骤 2：乙方提交存证 (V=1 照片摘要)

任务完成后，乙方提供一个照片/截图的哈希或存储引用（可以是 sha256 字符串，无需真实上传）：

```bash
ATTEST=$(curl -sX POST "$BASE_URL/api/v1/tasks/$TASK_ID/attestations" \
  -H "X-API-Shared-Secret: $SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "verification_level": 1,
    "timestamp_utc": "2026-05-01T09:32:00Z",
    "evidence_ref": "sha256:abc123<实际照片哈希或随机串>",
    "location_hash": "gcj02:39.9042,116.4074"
  }')
echo "$ATTEST" | python3 -m json.tool
echo "$TASK_ID" > /tmp/linkrsp_task_id
export TASK_ID=$(cat /tmp/linkrsp_task_id)
```

系统执行 R-002（握手速度）和 R-009（连续 V=0 衰减警告）检查。

### 步骤 3：结算预览

```bash
curl -sX POST "$BASE_URL/api/v1/tasks/$TASK_ID/settlement/preview" \
  -H "X-API-Shared-Secret: $SECRET" | python3 -m json.tool
```

返回 `credits_delta`（预期 = `T_phy_hours × V_bit × Clip(1.0/1.0, 0.8, 3.0)`）、`rules_checked` 列表，以及各规则的 `verdict`。**预览不写 DB**。

### 步骤 4：结算提交（写 DB）

```bash
curl -sX POST "$BASE_URL/api/v1/tasks/$TASK_ID/settlement/commit" \
  -H "X-API-Shared-Secret: $SECRET" | python3 -m json.tool
```

成功后返回 `ledger_entry_id`，账本与审计事件在同一事务内写入。

### 步骤 5：验证

```bash
# 查账本分录
curl -sG "$BASE_URL/api/v1/internal/lrs-ledger-query" \
  -H "X-API-Shared-Secret: $SECRET" \
  --data-urlencode "uid=$ALICE" \
  --data-urlencode "window_start_utc=2026-05-01T00:00:00Z" \
  --data-urlencode "window_end_utc=2026-05-02T00:00:00Z" | python3 -m json.tool

# 查审计事件（应有 R-003/004/002/009/001/005/006/010 各一条）
curl -sG "$BASE_URL/api/v1/audit-events" \
  -H "X-API-Shared-Secret: $SECRET" \
  --data-urlencode "subject_type=task" \
  --data-urlencode "subject_id=$TASK_ID" | python3 -m json.tool
```

---

## 3. 工程假设与 DB 适当性

### 哪些假设数据可以进入生产 DB？

| 数据类型                                   | 可否入 DB | 理由                                            |
| ------------------------------------------ | --------- | ----------------------------------------------- |
| 真实时间戳（两人真实操作的时间）           | ✅ 可以   | 这就是"真实数据"                                |
| 真实 uid（两人约定的字符串）               | ✅ 可以   | uid 现阶段无中心鉴权，约定即有效                |
| V=1 存证引用（照片哈希，可人工核查）       | ✅ 可以   | evidence_ref 是可溯源的引用                     |
| 位置哈希（精确到街道级别的坐标哈希）       | ✅ 可以   | 不需要精确到米                                  |
| 纯虚构时间/坐标（为了通过规则而捏造）      | ❌ 不可以 | 违反审计可信性；仅限于集成测试（会被 TRUNCATE） |
| 积分结果（由规则引擎计算的 credits_delta） | ✅ 可以   | 这是规则引擎的输出，是系统的"真相"              |

### V=0 自报的特殊情况

V=0 存证（无照片，纯自报）可以进入 DB，但：

- R-009 会检测连续 V=0 条带并写入 `warn` 审计事件
- 对于 Phase C 试点，建议至少提供一次 V=1（手机截图 + 哈希）避免 R-009 触发阻断

---

## 4. 缺失模块与启动时机

```
社区 (N/S 级)
├── 需要：UI + 社区创建 API + 身份鉴权
├── 解锁：community_id 绑定、社区维度规则 (R-008 D 共振抑制)
└── 启动时机：Phase E 前（LLM 引入时开始需要社区维度）

IPO 跃迁 (N→S)
├── 需要：社区已存在 + IPO 申请 API + 投票/人工审核
├── 解锁：S-009a/b、S-015/16 等跨社区规则
└── 启动时机：有社区后自然推进

注册 / 身份鉴权
├── 需要：前端 UI + JWT 或 OAuth2
├── 解锁：uid 与真实身份绑定、防女巫 (R-001 并发位置依赖 uid 唯一性)
└── 启动时机：Phase E 前（对外公测前必须有）

移动端 V=2 握手 (BLE/GPS)
├── 需要：Kotlin/Swift app + BLE 握手协议
├── 解锁：V=2 积分（V_bit=1.0）、高精度物理存证
└── 启动时机：Phase F 前（目前 V=0/1 即可）
```

---

## 5. Phase C 收口最小验收标准

1. **两名真实参与者**（可以是仓库维护者自己）各用一个约定 uid
2. **一次完整任务**（起止时间真实，任务内容真实，存证为 V=1 照片哈希）
3. **结算成功**（`ledger_entry_id` 存在，`credits_delta` 与预览一致）
4. **审计事件完整**（至少 R-003/004/002/009/001/005/006/010 各一条）
5. **账本投影可回放**（`lrs-ledger-query` 返回正确 `projected_balance`）

满足上述 5 条，Phase C 可标记为 ✅ 完成。

---

_维护：试点完成后请将结果（任务 ID、积分结果、参与者匿名化）记录到 `docs/operations/appeal-log.md` 或新建 `docs/operations/pilot-record-v1.md`。_
