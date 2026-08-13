# 视频模型计费配置指南

> 面向 AI 助手和运维人员。目标：看完这份文档就能独立判断「某个视频模型该怎么配价」，不需要读代码。

## 先搞清楚：系统里有两套按秒计费机制

这是最容易搞混的地方。**两套都是按秒，但配置方式完全不同。**

### 机制 A：`ModelPrice` 直接当每秒价

| | |
|---|---|
| 配在哪 | 系统设置 → 模型定价 →「按次计费」那栏的 JSON（key = `ModelPrice`）|
| 长什么样 | `"grok-imagine-video": 0.09` |
| 含义 | **每秒 $0.09** |
| 谁在用 | `grok-imagine-video*`、`sora-2*`、`veo-*`、`wan2.*`、`happyhorse-*` |

秒数由适配器代码从请求里解析，乘进去。分辨率倍率（如果有）也硬编码在适配器里。

**适用条件**：模型只有一个价，或者分辨率差异可以用一个固定倍率表达。

### 机制 B：规则表按秒

| | |
|---|---|
| 配在哪 | 数据库 option key `billing_setting_video.video_price_rules`（PR #710 后控制台有界面）|
| 长什么样 | 见下方规则格式 |
| 谁在用 | `seedance-*` 系、`doubao-seedance-*`、`MiniMax-H3` |

**适用条件**：不同分辨率 / 有无参考视频 需要**各自独立的每秒价**。

⚠️ **机制 B 仍然需要配 `ModelPrice`** —— 见下方「为什么 B 也要 ModelPrice」。

---

## 判断某个模型该用哪套

```
这个模型的每秒价，在不同分辨率下是不同的数字吗？
├─ 否（一个价走天下）        → 机制 A，配 ModelPrice 即可
├─ 是，但只是固定倍率关系      → 机制 A 够用（倍率在适配器代码里）
└─ 是，且各档价格互不成比例    → 机制 B，配规则表
```

Seedance 属于最后一种：480p `$0.140`、720p `$0.314`，且带参考视频时价格和计费基准都变，所以用 B。

Grok 没有分辨率参数，用 A。

---

## 机制 A 怎么配

在「模型定价」的 `ModelPrice` JSON 里加一行：

```json
{
  "grok-imagine-video": 0.09,
  "sora-2": 0.3
}
```

数字就是**每秒美元价**。改价直接改这个数字。

**注意**：`ModelPrice` 里也混着非视频模型的按次价（图片、音频等）。**别整体覆盖这个 JSON** —— 生产上有 100+ 条，覆盖会全清。只改你要改的那一行。

---

## 机制 B 怎么配

### 规则格式

```json
{
  "model": "doubao-seedance-2-5-260628",
  "match": { "resolution": "720p", "has_video": "true" },
  "price_per_second": 0.188,
  "basis": "total_duration",
  "fallback_seconds": 30
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | **客户端调用时用的模型名**，不是上游名 |
| `match` | 是 | 匹配维度。只写要约束的，没写的= 通配 |
| `price_per_second` | 是 | 每秒美元价，必须 > 0 |
| `basis` | 是 | `output_duration` 或 `total_duration` |
| `fallback_seconds` | `total_duration` 时必填 | 兜底秒数 |
| `source_rate_per_1m_tokens` | 否 | 仅备注：这个价是从哪个 token 价换算来的 |
| `assumed_fps` | 否 | 仅备注：换算时假设的帧率 |

### `match` 支持的维度

| 维度 | 取值 | 哪些渠道用 |
|---|---|---|
| `resolution` | `480p` `512p` `720p` `768p` `1080p` `2k` `4k` | 大多数 |
| `has_video` | `true` `false` | 所有 |
| `mode` | `std` `pro` | **仅 kling**（它没有分辨率参数）|

⚠️ **取值必须是上面这些**。写 `1440p` 会被拒绝保存；写 `4K` 会被自动折叠成 `4k`（可以，但建议直接写小写）。

⚠️ **给 kling 配规则必须用 `mode`**，写 `resolution` 永远匹配不上 —— 而对已配价的模型，匹配不上 = 该请求被拒绝。

### 匹配规则

- 一条规则「命中」的条件：`model` 完全相等，且 `match` 里**每个**键都和请求解析出的维度相等
- 多条命中时：**约束条数多的赢**
- 约束条数相同且可能同时命中的两条 → **保存时被拒绝**（歧义）

### `basis` 怎么选

| 值 | 乘的是什么 | 什么时候用 |
|---|---|---|
| `output_duration` | 生成出来的视频时长 | 默认情况 |
| `total_duration` | **输入 + 输出**总时长 | 带参考视频时，上游按总时长收费 |

选 `total_duration` 必须填 `fallback_seconds` —— 因为本地无法探测用户上传的参考视频有多长（要下载才知道，有 SSRF 和延迟风险），所以按这个秒数顶格预扣。

---

## 为什么机制 B 也要配 `ModelPrice`

计算公式是：

```
billable_units = price_per_second × 秒数 ÷ ModelPrice
最终额度      = ModelPrice × QuotaPerUnit × GroupRatio × billable_units
```

`ModelPrice` 先被除掉、后被乘回来，**约掉了**。所以：

- **它的值不影响客户实付多少** —— 填 1 和填 100 结果一样
- **但它必须存在且 > 0**，否则计算直接报错，该模型所有请求被拒
- **它的存在是开关** —— 配了 `ModelPrice` 才会走按秒路径；没配就回退到 token 计费

**建议填 `1`**，这样日志里 `video_billing_units` 的数值直接等于美元金额，一眼能看懂。

**已有值的别改**：`doubao-seedance-2-5-260628`=0.14、`MiniMax-H3`=0.08 是历史值，改了会让日志里的历史数据不连续。

---

## 常见问题排查

### 症状：配了价但没生效，日志里 `model_price: -1`

`-1` 是「没找到 ModelPrice，回退 token 计费」的标记。

**最常见原因：模型名不匹配。** 定价查的是**客户端调用时用的名字**（`OriginModelName`），不是上游名。

排查方法：
1. 在日志里找一条该模型的记录，看 `model_name` 字段
2. 对比 `ModelPrice` 里的 key 是否**完全一致**（大小写、连字符都算）

**真实案例**：`seedance2.0-pro`（无连字符）配了，但客户端实际调用 `seedance-2.0-pro`（有连字符），导致 28 次调用全部走 token 计费。同一模型存在三种拼写：`seedance2.0-pro`、`seedance-2.0-pro`、`Seedance2.0-pro`，三个都得配。

### 症状：日志里 `model_price` 有值但金额不随时长变化

说明走的是**纯按次**计费，秒数没乘进去。检查该渠道的适配器有没有实现按秒逻辑。

### 症状：该模型所有请求都被拒绝

模型在规则表里（触发严格模式），但当前请求的维度匹配不上任何规则。检查：
- `match` 的取值是否在合法词表里
- kling 是不是错用了 `resolution` 而非 `mode`
- 该渠道是否真的支持你配的那个分辨率档位

---

## 怎么确认配置真的生效了

**别只看配置存进去了就完事。** 发一个真实请求，然后查日志：

```
日志 → 找到该模型的记录 → 看 other 字段
```

| 看到什么 | 说明 |
|---|---|
| `"model_price": -1` | ❌ 没生效，走 token 计费 |
| `"model_price": 1` + `video_billing_units` | ✅ 按秒生效（机制 B）|
| `"model_price": 0.09`，且不同时长金额不同 | ✅ 按秒生效（机制 A）|

反推实付金额：

```
美元 = quota ÷ group_ratio ÷ 500000
```

例：`quota=706500, group_ratio=0.9` → `$1.57` = `0.314 × 5秒` ✓

---

## 当前生产配置状态（2026-08-13）

### 机制 B（规则表，60 条规则）

| 模型 | 规则数 | 备注 |
|---|---|---|
| `seedance-2.0` | 8 | 480p/720p/1080p/4k × 有无视频 |
| `seedance2.0-pro` | 8 | |
| `seedance-2.0-pro` | 8 | 别名，同价 |
| `Seedance2.0-pro` | 8 | 别名，同价 |
| `seedance-2.0-fast` | 4 | 仅 480p/720p |
| `seedance-2.0-mini` | 4 | 仅 480p/720p |
| `doubao-seedance-2-0-260128` | 8 | |
| `doubao-seedance-2-0-fast-260128` | 4 | |
| `doubao-seedance-2-5-260628` | 4 | ModelPrice 保留 0.14 |
| `MiniMax-H3` | 4 | ModelPrice 保留 0.08 |

### 机制 A（ModelPrice 直接按秒）

| 模型 | 每秒价 |
|---|---|
| `sora-2` / `sora-2-pro` | $0.3 / $0.5 |
| `veo-3.0-generate-001` | $0.4 |
| `veo-3.0-fast-generate-001` | $0.15 |
| `grok-imagine-video` | $0.09 |
| `grok-imagine-video-1.5` | $0.11 |
| `wan2.*` / `happyhorse-*` | 各自不同 |

### 未配置（第三方转售，按官方价配会亏本）

`jimeng-video-*`、`bytedance/seedance-*`(blockrun)、`techmobi` 的 seedance、`kuaizi-lizhen-*`

这些渠道的进价高于原厂官网价（含转售商利润），要配得先拿到实际进价。**不配 = 维持原有计费，不影响使用。**

---

## 价格换算：token 价 → 每秒价

上游公布 `$/百万 token` 时（如 BytePlus），换算公式：

```
tokens/秒 = 宽 × 高 × 帧率 ÷ 1024
$/秒      = tokens/秒 ÷ 1,000,000 × ($/百万token)
```

**例**：720p @ 24fps @ $7.0/1M
```
1280 × 720 × 24 ÷ 1024 = 21,600 tokens/秒
21,600 ÷ 1e6 × 7.0     = $0.1512/秒
```

⚠️ **必须逐档算，不能按 token 单价的大小推断**。4K 的 token 单价最低（$4.0 vs $7.0），但每秒价最高 —— 因为像素数是 720p 的 9 倍。

⚠️ **换算假设了帧率**。如果上游把某模型从 24fps 改成 30fps，按秒价会少收 20%。所以换算来的条目要记 `assumed_fps`，便于日后追溯该重算哪些。

---

## 备份与回滚

改配置前先备份：

```
E:\workspace\video-billing-config\backup_ModelPrice.json
```

回滚就是把备份写回 `ModelPrice`。规则表原始状态是空数组 `[]`。

---

## 相关代码位置

| 想看什么 | 去哪 |
|---|---|
| 规则类型、校验、匹配 | `setting/billing_setting/video_price.go` |
| 计费单位计算、分辨率归一化 | `relay/channel/task/taskcommon/second_billing.go` |
| 拒绝无法定价的请求 | `relay/relay_task.go` 步骤 5b |
| 各渠道维度解析 | `relay/channel/task/<渠道>/adaptor.go` 的 `resolveDimensions` |
| 扩展新维度的完整指南 | `relay/channel/task/AGENTS.md` |
| 设计决策与理由 | `docs/superpowers/specs/2026-08-12-video-per-second-billing-design.md` |
