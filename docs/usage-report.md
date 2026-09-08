# Usage Report —— 用户侧日报表（新板块）

> 独立于现有「运营日报」(controller/ops_report.go)。新板块**不替换、不修改**旧报表，
> 只新增自己的聚合表 / 接口 / 页面。旧报表保持原样运行。

## 背景与目标

- Campaign 拉了 72 万流量，需要一套**每天能打开、秒级返回**的用户侧报表来回答：
  用户每天在用什么模型、调用多少次/多少 token（成本可另按外部价目乘出）、
  漏斗（注册→建 Key→首付→付费金额）是变好还是变坏。
- 系统暂未记录上游成本/供应商价目，因此本板块**只做用户侧事实统计**，不含毛利/资损($)。
- 现有 ops_report 是「全量重算 + 节点内存缓存」，不宜让它承接高频的“今日趋势”。
- 新板块用 **UTC+0 自然日聚合 + 落表 + 今日少量刷新** 的设计，读接口不扫历史。

## 指标口径（与评审通过的设计一致）

| 指标 | 口径 |
| --- | --- |
| 注册 | 当日 UTC+0 新增，`users.status = enabled(1)` **且** `email_verified_at > 0`（人） |
| 激活(建 Key) | 用户**首次**创建 API Key 落在当日；**按人去重**（1 人多 Key 只算 1 人） |
| 首次付费 | 用户**首次**成功 top-up 落在当日（人） |
| 付费金额 | 当日全部成功 top-up 的 `money` 合计（含老客复购；一次性套餐已由订阅同步管线镜像进 top_up） |
| 调用次数 / Tokens | 当日 `logs`(消费日志，`type=consume`) 的行数与 prompt+completion tokens 合计，按模型分组 |
| 转化率（队列，人） | `注册→激活率(7日)` = 该日注册队列中注册后 7 日内建 Key 人数 / 当日注册人数；`激活→首付率(14日)` = 当日建 Key 队列中 14 日内首付人数 / 当日建 Key 人数。分子 ⊆ 分母 → 恒 ≤100%；近 7/14 天队列未到期不展示 |

- 日切 = **UTC+0**（与现运营报表的美西日切不同，属新板块自己的口径，已在接口/页面标注）。
- 时间桶统一取 `created_at / created_time / 完成时间` 的 unix 秒落入 UTC 日。

### 口径假设与待校准点（评审 / 上真实库后核）
1. 付费金额按 `money`（美元字段）汇总；若某供应商 `money=0` 仅记 `payment_amount_minor`，
   该行会计入“首次付费人数”但不计金额——需与支付侧确认 `money` 是否总能回填。
2. “激活”含 signup 自动发放的 key（未按旧报表的 <120s 自动 Key 规则剔除）。
   如需与旧报表完全对齐可加 `exclude_auto` 参数（后续）。
3. 消费日志按 `logs` 主表聚合；`logs_company`（特殊 codex 路由表）本期未并入，若启用
   需在此处补第二条合并查询。
4. 注册未按 `group='plg'` 过滤（与旧报表默认不同）。如需同口径，可加 `group` 参数。

## 表结构（AutoMigrate 注册于 model/main.go orderedMigrationModels）

```sql
CREATE TABLE usage_report_daily (
  date             CHAR(10) PRIMARY KEY,          -- UTC+0 yyyy-mm-dd
  registered       INT NOT NULL DEFAULT 0,
  activated_key    INT NOT NULL DEFAULT 0,
  first_paid       INT NOT NULL DEFAULT 0,
  paid_usd         DECIMAL(14,2) NOT NULL DEFAULT 0,
  calls            BIGINT NOT NULL DEFAULT 0,
  prompt_tokens    BIGINT NOT NULL DEFAULT 0,
  completion_tokens BIGINT NOT NULL DEFAULT 0,
  built_at         BIGINT NOT NULL DEFAULT 0      -- 最近一次计算时间(unix)
);
CREATE TABLE usage_report_daily_model (
  date       CHAR(10) NOT NULL,
  model_name VARCHAR(191) NOT NULL,
  calls      BIGINT NOT NULL DEFAULT 0,
  prompt_tokens BIGINT NOT NULL DEFAULT 0,
  completion_tokens BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (date, model_name)
);
```

## 计算策略（对线上零影响）

1. **历史日**：首次被读到时一次性按 UTC 日窗口查询（SQL 全部带
   `created_at >= start AND created_at < end` 的索引范围，不扫全表），结果幂等
   delete+insert 落 `usage_report_daily(_model)`。
2. **今日**：以 `built_at` 判断，距上次计算 ≥ 5 分钟才重算一次；管理端并发由
   进程内 mutex 串行化。即使运营 1 分钟点一次，也只有 ~1 次/5min 的当日窗口查询。
3. 请求热路径**零改动**：本版不修改 `RecordConsumeLog` 与计费链路。
4. （规划，未接入）Redis 分钟级计数：在 `model.RecordConsumeLog` 成功后追加
   `HINCRBY usage:today:<UTC日期>:model <model> <delta>` 与计数 key，
   “今日”改为 Redis + 已落库 T-1 拼接；含开关，失败静默。先出本版再评估加。

## 接口

- `GET /api/data/usage_report?days=30[&format=csv[&dim=daily|models]]`
  - 管理员鉴权（`middleware.AdminAuth`），与 ops_report 同组。
  - JSON：`{success, data:{ days:[{date,registered,activated_key,first_paid,paid_usd,calls,prompt_tokens,completion_tokens}], models:[{date,model_name,calls,prompt_tokens,completion_tokens}]}}`
  - `format=csv`：`dim=daily`（默认，日漏斗+用量）或 `dim=models`（按日×模型，
    供“用量 × 外部价目”离线核算成本）。
- days 上限 180、默认 30。日期按 UTC+0 今天往前取 N 天。

## 前端（评审稿 v3 布局）

- 新管理页面「用量报表」（区块顺序：漏斗第一屏 → 用量总览 → 模型用量），
  整屏滚动 + 顶部锚点，复用评审通过的交互稿：
  - 漏斗第一屏：双 Y 轴（柱=注册/激活(Key)/首付，线=付费金额）、每日明细表、
    转化率趋势（队列口径 7 日 / 14 日，人）、区间汇总漏斗；
  - 用量总览：今日 KPI、调用/Tokens 日趋势、模型占比与 Top；
  - 模型用量：各模型堆叠面积（调用/Tokens 切换）、汇总表、CSV 导出。
- 图表库与页面接入方式在 React 版推进时确定（echarts 或本地 SVG）。

## 验证

- `go vet ./model/ ./service/ ./controller/`
- `go test ./service/ -run TestUTC`（日期边界单测）
- 全量 `go build`（见 PR 检查项）
- 说明：不修改既有报表任何行为；新表失败不影响业务写路径（聚合只在读接口触发，
  计算错误直接回 500，不会污染计费数据）。

## 回滚

删除两条路由 + controller/model/service 新文件 + 两条 AutoMigrate 条目即可；
旧功能零改动，无回滚依赖。
