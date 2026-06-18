# Ads Dashboard 详细设计规格 · 方案 A「转化漏斗优先」

> 用于 11agents 平台内的 ads 模块 · 数据源 = Google Ads API + flatkey admin API
> 配套高保真原型：`ads-dashboard-A.html`

---

## 0. 开源参考（借什么）

| 项目 | License | 借鉴点 |
|---|---|---|
| **PostHog**（funnels）| MIT/付费 | 漏斗可视化标准：水平条 + 逐级 conversion%/drop-off% + 按维度 breakdown + hover 看每段绝对值 |
| **OpenPanel** | MIT | self-host 分析的卡片网格 + 自定义图（funnel/retention）+ realtime 视图 |
| **Plausible** | AGPLv3 | 极简 KPI 顶栏 + Goal/Conversion 列表的克制排版 |
| **GoogleCloudPlatform/marketing-analytics-jumpstart** | Apache-2.0 | Google Ads campaign 表 + conversion funnel + ROAS/smart bidding 字段口径 |
| **Tremor**（React）| Apache-2.0 | 直接用其组件落地：`Card / Metric / BarList / AreaChart / Tracker`；funnel 用 BarList 变体 |
| **Matomo** | GPLv3 | goal/conversion funnel 的分步定义与命名 |

**落地组件栈建议**：React + Tremor（KPI/图/表）+ shadcn（弹窗/抽卡卡片）。漏斗用自绘水平条（Tremor 无原生 funnel）。

来源：[PostHog](https://posthog.com/blog/best-open-source-analytics-tools) · [OpenPanel](https://openpanel.dev/compare/funnelio-alternative) · [Plausible](https://github.com/plausible/analytics) · [GCP marketing-analytics-jumpstart](https://github.com/GoogleCloudPlatform/marketing-analytics-jumpstart) · [funnels topic](https://github.com/topics/funnels) · [Improvado 模板集](https://improvado.io/blog/12-best-marketing-dashboard-examples-and-templates)

---

## 1. 整体布局

```
┌─ Header: 标题 · 数据时间 · 环境徽章 · 日期范围选择器(今日/7日/30日/自定义) ──┐
│                                                                            │
│ ── Zone 1 · 投放进度 KPI 带 ─────────────────────────────────────────────  │
│ [花费] [曝光] [点击] [注册] [外部充值]   ← 5 卡，等宽，带同比/趋势 sparkline  │
│                                                                            │
│ ── Zone 2 · 转化漏斗（HERO，占最大面积）─────────────────────────────────   │
│ 注册 ██████████████████ 100%                                               │
│   ▼ 94% 流失(断点标红)                                                       │
│ 使用 ████ 6.2%                                                              │
│   ▼ 100%                                                                    │
│ 绑卡 ▏ 0%                                                                    │
│ 付费 ▏ 0%        [按 campaign/国家/语言 breakdown 切换]                       │
│                                                                            │
│ ── Zone 3 · 今日优化抽卡（3 卡）────────────────────────────────────────    │
│ [建议1 ✓] [建议2] [建议3]   ← 点选采纳，写入决策记录                          │
│                                                                            │
│ ── Zone 4 · 投放明细表（按转化排序）─────────────────────────────────────   │
│ Campaign | 花费 | 点击 | 转化 | 单转化成本 | 漏斗微缩条 | 状态/操作            │
└────────────────────────────────────────────────────────────────────────────┘
```

**栅格**：12 列。Zone1 = 5 等分卡。Zone2 = 满宽 hero。Zone3 = 3 等分卡。Zone4 = 满宽表。
**响应式**：≥1024px 如上；768–1024 KPI 变 3+2、抽卡竖排；<768（移动）全部单列，漏斗条保留、表格转卡片。
**主题**：深色（与原型一致）。强调色：紫→品红渐变（正常）、红（断点/0 转化）、琥珀（选中/警示）、绿（达标）。

---

## 2. Zone 1 · 投放进度 KPI 带（字段）

| 卡 | 主指标 | 数据源 | 公式 | 格式 | 副标(同比/明细) |
|---|---|---|---|---|---|
| 今日花费 | `cost` | Google Ads `metrics.cost_micros`/1e6（当日，按时区） | Σ 当日 | `$1,234.56` | 累计花费 + 较昨日 ±% |
| 今日曝光 | `impressions` | Ads `metrics.impressions` | Σ | `6,579` | CTR = clicks/impr |
| 今日点击 | `clicks` | Ads `metrics.clicks` | Σ | `135` | CPC = cost/clicks |
| 今日注册 | 新注册数 | flatkey `/api/user/` 当日 `created_at` 计数（外部口径） | count | `47` | 累计转化(Ads conversions) |
| 外部充值 | 外部付费额 | flatkey 充值记录，排除内部账号 | Σ topup（外部） | `$0` | CAC=花费/付费 或标"黑洞" |

**口径铁律**：所有"用户/转化/充值"指标一律**外部口径**——排除内部账号（`group∉{default,plg}` 或邮箱域名 ∈ {lockin.com,voc.ai,shulex,solvea,flatkey.ai,quantumnous} 或用户名含 test）。卡上可加 ⓘ 显示"含内部参考值"。
**同比**：默认对比"昨日同时点"；日期范围切换时对比"上一等长周期"。

---

## 3. Zone 2 · 转化漏斗（HERO）

### 阶段定义（4 步 + 可选 5 步）

| # | 阶段 | 计数定义（flatkey 字段） | 颜色 |
|---|---|---|---|
| 1 | 注册 | 外部用户总数（窗口内 `created_at`） | 渐变 |
| 2 | 有使用记录 | `request_count > 0`（等价 `used_quota > 0`） | 渐变 |
| 3 | 绑卡 | `stripe_card_bound = true` | 渐变 |
| 4 | 付费/充值 | 有成功充值（外部） | 渐变 |
| (1.5 可选) | 建过 Key | 该用户有 ≥1 个 token | 浅色细条 |

### 每阶段字段

| 字段 | 公式 | 展示 |
|---|---|---|
| 绝对值 | count | 条内右侧大字 `14` |
| 整体转化率 | 本阶段 / 阶段1 | `6.2%` |
| 段间转化率 | 本阶段 / 上阶段 | hover 显示 |
| 段间 drop-off | 1 − 段间转化率 | 断点行 `▼94%`，>50% 标红 + "最致命断点" |
| 中位耗时（可选） | median(本阶段时间 − 上阶段时间) | hover "平均 X 小时到下一步" |

### 交互（借 PostHog）
- **Breakdown 切换**：按 `campaign / 国家 / 语言 / 设备` 拆分漏斗（下拉切换；拆分后每段堆叠或并排小漏斗）。
- **窗口联动**：跟随 Header 日期范围。
- **点任一阶段** → 下钻该阶段用户列表（侧抽屉，复用 flatkey 用户表，带 ads_attribution）。
- **断点高亮**：drop-off 最大的那一段自动标红 + 文案（如"绝大多数在 default 死组撞过 503"——这条由规则引擎按归因生成）。

### 数据来源
- 单次拉全量外部用户（`/api/user/?p=*&page_size=100` 分页），客户端按上面定义聚合。规模大时后端出聚合接口。

---

## 4. Zone 3 · 每日优化抽卡引擎

每天按当日数据**自动生成 3 条**互斥的优化动作，用户点选「采纳」（写入决策记录 + 可触发动作）。

### 每张卡字段

| 字段 | 说明 |
|---|---|
| `id / date` | 当日唯一 |
| `title` | 动作标题（≤14 字） |
| `rationale` | 为什么（引用当日具体数字） |
| `impact` | 高/中/低（预估对漏斗的提升） |
| `effort` | 0/低/中（落地成本） |
| `tags` | 如「复用召回邮件」「立即生效」「需改配置」 |
| `action` | 采纳后做什么：`link`（跳转）/ `config`（改配置）/ `email`（触发群发）/ `note`（仅记录） |
| `selected` | 用户是否采纳 |

### 规则引擎（生成 3 条的逻辑，按优先级取前 3）

| 触发条件 | 生成建议 | impact/effort |
|---|---|---|
| 某 step drop-off > 80% 且上游有量 | "修/降该步摩擦"（如注册→使用流失大 → 提免费额度 / 推 quickstart） | 高/中 |
| 存在 default/死组存量用户 > N | "召回 N 个老用户（按语言发首充送 credit）" | 高/低 |
| 某 campaign 花费 > $X 且转化=0 | "暂停/砍 {campaign} 预算，挪给 {top 转化 campaign}" | 中/0 |
| 某 campaign 单转化成本 > 2× 账户均值 | "降 {campaign} 出价 / 收紧关键词" | 中/低 |
| 绑卡率=0 但有使用量 | "在 wallet 页加绑卡 bonus 引导" | 中/中 |
| 今日新增多但 req=0 | "盯 plg 新用户激活，发 D0 邮件" | 中/低 |

> 规则可配置阈值。生成后按 `impact 降序 → effort 升序` 排，取前 3。当日已采纳的记入历史，第二天重算。

---

## 5. Zone 4 · 投放明细表

| 列 | 源 | 格式/规则 |
|---|---|---|
| Campaign | Ads campaign.name | 链接到该 campaign 下钻 |
| 累计花费 | cost | `$638.92` |
| 点击 | clicks | `423` |
| 转化 | conversions（注册，gtag 归因） | `77`，0 标红 |
| 单转化成本 | cost/conversions | `$8.3`；转化=0 显示 `∞` 标红 |
| 漏斗微缩条 | 该 campaign 注册→使用占比 | mini bar |
| 状态/操作 | campaign.status | 启用/暂停徽章 + 「暂停/调价」快捷操作 |

**默认排序**：转化降序（出量的在上，烧钱零产出的沉底标红）。可切花费/单转化成本排序。

---

## 6. 数据源与刷新

| 模块 | 接口 | 频率 |
|---|---|---|
| 花费/曝光/点击/转化/campaign | Google Ads API（GAQL：campaign + metrics，按日期段） | 每 15–30 分钟快照 |
| 注册/使用/绑卡/充值/用户列表 | flatkey admin API：`/api/user/`、`/api/log/`、充值记录、`/api/option/`（GroupRatio 等） | 每 15 分钟 |
| 聚合落地 | 复用现有 `snapshot.py` → `data.json` 的模式，新增 funnel + suggestions 字段 | 同上 |

> 现有 `snapshot.py`（~/google-ads/dashboard）已产出 today/campaigns/keywords/flatkey 等；本 dashboard 在其基础上增加：① 外部口径漏斗四级计数 ② 规则引擎产出的当日 3 条建议 ③ campaign 单转化成本。

---

## 7. 字段字典（master，给 cc 实现对齐）

**Ads（Google Ads API）**：`campaign.id/name/status`、`metrics.cost_micros`、`impressions`、`clicks`、`conversions`、`conversions_value`、按 `segments.date` 切片。
**flatkey 用户（/api/user/）**：`id`、`email`、`group`、`status`、`created_at`、`last_login_at`、`request_count`、`used_quota`、`quota`、`stripe_card_bound`、`ads_attribution`(JSON: gclid/utm_campaign/utm_term/lng…)、`setting.language`。
**flatkey 配置（/api/option/）**：`GroupRatio`、`QuotaForNewUser`、充值 bonus 配置。
**派生**：CTR、CPC、单转化成本、各 step count、段间/整体转化率、drop-off、CAC、外部口径过滤。

---

## 8. 验收 / 落地顺序（给 cc）

1. 数据层：扩 `snapshot.py` 出 funnel 四级 + campaign 单转化成本 + 规则引擎 3 建议 → `data.json`。
2. 布局骨架：Header + 4 Zone 栅格（Tremor Card 网格）。
3. Zone1 KPI 带 + Zone2 漏斗（自绘水平条 + drop-off 标红 + breakdown 切换 + 阶段下钻抽屉）。
4. Zone3 抽卡（shadcn 卡片 + 选中态 + 采纳写决策记录）。
5. Zone4 明细表（排序/标红/快捷操作）。
6. 响应式 + 外部口径过滤全链路校验。
- 验收：外部口径数字与 admin API 手算一致；漏斗 drop-off 计算正确；3 建议每日按当日数据重算；移动端单列可读。
