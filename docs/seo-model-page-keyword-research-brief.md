# 模型详情页 SEO 关键词研究简报（研究与实施审计）

> 批量研究日期：2026-08-25；Seedance 2.5 独立复审日期：2026-08-26。本文档记录关键词证据、事实边界与页面实施审计；不把估算值写成真实产品数据。

## 一页结论

- 目录快照共 **101 个模型**：6 个主推模型做完整流程，其中 5 个纳入批量请求，Seedance 2.5 为避免重复扣费先排除后按本报告单独复审并实施；其余 **95 个**做快速流程并已接入模型专属内容包。
- 全局优先级按 `global_volume`；Ahrefs Matching terms、Questions、SERP 必须带国家参数，本次统一用 `country=us` 作为 **US-English proxy**。报告中的 `GSV` 才是全球需求指标；`SV/CPC/意图` 等国家字段不能当成巴西或全球数据。
- 最终可用于决策的主推词需求排序：**Kimi K3（GSV 121K，KD 70）→ GPT Image 2（56K，KD 58）→ DeepSeek V4 Pro（26K，KD 52）→ GPT-5.6 Sol（13K，KD 22）→ MiniMax H3（5.4K，KD 41）**。KD 是 Ahrefs 估计，`N/A` 不等于 0。
- 最值得先做的切入点不是硬抢所有头词：GPT-5.6 Sol 的 `pricing`/API 长尾、MiniMax H3 的 `ComfyUI`/prompt、DeepSeek 的 API/model-name、Kimi 的 API/pricing、GPT Image 2 的 API/use-case 更适合先做独立答案。
- Seedance 2.5 的头词在全球有 **17K GSV、KD 25**，但其 `SV=3.8K` 只是 US-English proxy；巴西截图中的 `SV=600` 是在巴西查询英文词 `seedance 2.5` 的结果，不是葡萄牙语关键词量，不能据此宣称 pt-BR 需求或难度。
- Seedance 页面已按独立复审补齐 API、分辨率/时长/参考视频计费、行业场景与专属 FAQ；排名仍取决于索引、内容质量、链接与 SERP 变化，本报告不作排名保证。

## 主推页实施审计（2026-08-27）

研究与代码实施是两个状态：Ahrefs 研究数据已经冻结在本报告和矩阵中；5 个主推页的专属 landing 配置已接入当前工作区，配置/组件回归、本地 production build 及 canonical/metadata SSR 检查已通过，但在真实测试环境浏览器验收和事实复核完成前，不标记为“已发布”或“已取得排名”。

| 页面 | 研究 | 代码实施 | 当前验收状态 |
|---|---|---|---|
| `/models/gpt-5.6-sol` | 完整（Global/Matching/Questions/SERP） | 专属 hero、token pricing、API、context/modalities、comparison、FAQ 已接入 | 配置/组件回归、本地 build/SSR 通过；待真实环境浏览器/事实验收 |
| `/models/gpt-image-2` | 完整（Global/Matching/Questions/SERP） | 专属 image API、尺寸/格式/质量控制、token/image pricing、FAQ 已接入 | 配置/组件回归、本地 build/SSR 通过；待真实环境浏览器/事实验收 |
| `/models/kimi-k3` | 完整（Global/Matching/Questions/SERP） | 专属双兼容端点、1,048,576 context、file 边界、pricing、FAQ 已接入 | 配置/组件回归、本地 build/SSR 通过；待真实环境浏览器/事实验收 |
| `/models/deepseek-v4-pro` | 完整（Global/Matching/Questions/SERP） | 专属双端点、UTC peak/off-peak pricing、context/file 边界、FAQ 已接入 | 配置/组件回归、本地 build/SSR 通过；待真实环境浏览器/事实验收 |
| `/models/minimax-h3` | 完整（Global/Matching/Questions/SERP） | 专属视频控制、异步 `/v1/videos`、pricing、FAQ；大小写 alias 已 redirect 到 canonical | 配置/组件回归、本地 build/SSR 与 alias 响应头通过；待真实环境浏览器/事实验收 |

“已接入”表示代码中存在目标模型内容，不代表 Google 已抓取或排名。若回归测试发现 FAQ 数量、locale metadata 或静态/动态路由回退，状态应退回“实施修正中”，不能用通用模板内容冒充完成。

### 验证快照（2026-08-27）

- `bun run typecheck`：通过。
- 相关网站回归：**41 tests / 773 assertions，0 failures**（模型配置、详情组件、sitemap、copy）；五个主推模型均有专属 hero、pricing、endpoint 和至少 6 个 FAQ 的断言。
- 全 locale 内容包审计：**45 packs（5 个主推模型 × 9 个非英文 locale），0 failures**；逐包核对 localized hero、FAQ 数量、endpoint 与价格 literal，避免局部英文回退或技术事实丢失。
- 关键词落位审计：5 个主推页的 primary、commercial/API 与主要 secondary clusters 均在 metadata、H1/首段、相关 H2/正文或 FAQ 中有自然承接；连字符/空格变体按搜索规范化处理，不重复堆入每个标题。`/pt/` 另核对 `preços`、`gerador de imagens/vídeo`、`como usar` 等本地意图表达。
- 关键词语言质量复核（最新构建）：英文 API H2 使用 `Call the … API`，葡语使用 `Chame a API do …`、`Preços do …` 等完整语序；图像/视频 API 标题按 locale 重排，GPT Image 2 的 pt-BR title 使用 `gerador de imagens do GPT Image 2`，能力 H2 使用 `… do [modelo]` 结构，未通过机械拼接模型名与词根制造标题。英文首段统一为 `an OpenAI catalog model` 与 `1,048,576-token context`。
- `bun run lint`：0 errors；仅有 `online-home-page.tsx` 中既有的 2 个 `next/image` 建议 warning。
- `bun run build`：Turbopack production build 通过。第一次在受限沙箱中因子进程端口绑定被拒绝，提升权限后同一构建成功；这不是代码编译错误。
- 本地 standalone SSR curl（4010 临时端口）：五个 canonical 英文页、五个 `/pt/` 页及另外 40 个非英文优先页均返回 HTTP 200，包含 canonical、hreflang、FAQ 结构化标记和对应 API endpoint；`/pt/` 标题/描述为葡萄牙语。对 45 个非英文页面扫描未发现 `Unknown here`、`Live estimate`、`Check catalog`、`Verified Kimi K3`、`tokens context and` 或未经证实的 `$0.13` 残留。`/models/MiniMax-H3` 返回 **307** 并指向 `/models/minimax-h3`。
- 尚未完成：staging/真实测试环境的浏览器视觉验收、发布前最终事实复核，以及 Google Search Console 的索引/排名观察。因此以下页面仍不能标为“已发布”或“已取得排名”。

### 关键词落位复核（当前实施）

关键词不是按“每个 H2 塞一个词”处理，而是按同一搜索意图聚类到最能回答问题的区域。下表是当前代码的落位摘要；`title/meta` 使用搜索友好的主词与动作词，H1/首段给出直接答案，H2/正文承接 API、价格、设置或用途，FAQ 覆盖问题型长尾。

| 页面 | 主词（GSV/KD） | 商业/API 词 | 当前落位 | 反堆词与事实边界 |
|---|---|---|---|---|
| `/models/gpt-5.6-sol` | `gpt 5.6 sol`（13K/22） | `gpt 5.6 sol pricing`（350）；`gpt 5.6 sol api`（50） | Title、description、H1、首段；H2 覆盖 pricing、context/modalities、vs 5.5/Terra/Luna、API；FAQ 覆盖 model ID、cost、release | 用 OpenAI catalog、`/v1/chat/completions`、1,048,576 context 和 token 维度解释，不写 ChatGPT/Codex 或 benchmark 结论 |
| `/models/gpt-image-2` | `gpt image 2`（56K/58） | `gpt image 2 api`（1.6K）；`gpt image generator`（3.1K） | Title/meta 明确 image generator；H1/首段回答 image API；H2 覆盖 pricing、size/quality/format/background/moderation、prompt/use case；FAQ 覆盖 endpoint、价格、尺寸格式 | 用 token/image-dimension 价格和已验证字段，不承诺固定单图价、透明背景或免费生成 |
| `/models/kimi-k3` | `kimi k3`（121K/70） | `kimi k3 pricing`（4.6K）；`kimi k3 api`（1K） | Title、H1、首段；H2 覆盖双兼容端点、pricing、context/file、hosted vs local；FAQ 覆盖 API、free、open-source/local 边界 | `1,048,576` context、`/v1/chat/completions` + `/v1/messages` 和 token 价格均绑定目录事实，不把 distillable 写成开源 |
| `/models/deepseek-v4-pro` | `deepseek v4 pro`（26K/52） | `deepseek v4 pro pricing`（1.1K）；`deepseek v4 pro api`（700） | Title、H1、首段；H2 覆盖 UTC peak/off-peak pricing、model name/endpoints、context/file、V4 Flash 对比；FAQ 覆盖价格时段、API、local/vision | 只比较已记录字段；保留两套 UTC 费率，不写 vision、下载权重或 benchmark 优势 |
| `/models/minimax-h3` | `minimax h3`（5.4K/41） | `minimax h3 api`（80）；`minimax h3 comfyui`（450/22） | metadata 使用空格变体 `MiniMax H3`，正文保留官方 `MiniMax-H3`；H2 覆盖 video API、prompt guide、768P/2K、4–15s、ComfyUI/local；FAQ 覆盖设置、价格、异步任务和本地边界 | 连字符/空格按搜索词规范化，不重复堆词；只写目录 `$0.08/sec` 基准，不写未核实 `$0.13/sec`、原生 ComfyUI 或本地权重 |

**全球与巴西说明**：英文根路径承接全球 `global_volume` 主需求；`/pt/` 页面使用葡萄牙语的 `API`、`preços`、`gerador de imagens/vídeo`、`como usar` 等自然表达。Ahrefs 的巴西截图查询的是英文 seed（例如 `seedance 2.5`），其 `SV` 只能作为巴西英文查询信号，不能冒充葡萄牙语搜索量或难度。后续若要做 pt-BR 量级决策，应另跑葡萄牙语 seed 的 `country=br` 数据。

## 研究范围、来源与 Ahrefs 账单

| 项目 | 结果 | 说明 |
|---|---:|---|
| 官网模型目录 | 101 | `/tmp/flatkey-pricing.json`，快照 2026-08-25 |
| 最终 Global Overview | 93 exact rows + 1 normalized `minimax-h3` alias | 8 个 ID 没有 exact row；MiniMax alias 已恢复，实际 7 个仍无可用 Overview row；不填 KD=0 |
| 主推商业/用途词校准 Overview | 14 rows、756 units | 用于补齐 pricing/API/use-case 的 GSV/KD，不改变批量筛选 |
| 最终 Matching terms | 100 页、1,757 rows、95,128 units | `where global_volume >= 10`，limit：full 50 / quick 20 |
| 主推 Questions | 5 页、110 rows、5,940 units | `where global_volume >= 10` |
| 主推 SERP | 20 queries、201 rows、9,050 units | head/commercial/use-case；空结果保留 N/A |
| Seedance 2.5 独立复审 | 1 页、关键词/事实/页面审计 | 复用已保存 Ahrefs 数据，不重复调用 Matching/Questions/SERP |
| 最终研究数据集估算 | 116,760 units | Global Overview + support Overview + global Matching + global Questions + SERP；不含废弃重跑 |
| 账户当前用量 | 210,719 / 400,000 workspace units | 免费 limits endpoint 读取；API key usage 210,419；重跑/废弃 US-filter 批次也计入 |

Ahrefs API 的请求成本按 `max(base_cost, per_row_cost × rows)` 计算；每个请求的 `x-api-*` headers 已用于审计。最终报告只采用全球筛选批次；前一轮 `US volume >= 10` 结果保留为临时审计数据，不作为关键词布局依据。官方参数和计费规则见 [Matching terms 文档](https://docs.ahrefs.com/en/api/reference/keywords-explorer/get-matching-terms)、[SERP Overview 文档](https://docs.ahrefs.com/en/api/reference/serp-overview/get-serp-overview) 与 [limits 文档](https://docs.ahrefs.com/en/api/docs/limits-consumption)。

### 数据缺口

- 没有 exact Overview row 的目录 ID：`seedance-2.0-pro`、`eleven_sound_v1`、`gemini-3.7-flash`、`macaron-v1-coding-venti`、`macaron-v1-tall`、`sonilo-video-to-music`、`deepseek-v4-pro-0813`、`MiniMax-H3`；后者有规范化 `minimax-h3` alias，因此实际仍缺 7 个可用 Overview 主题行。
- 最终 Matching terms 没有返回行的 5 个页面：`eleven_sound_v1`、`macaron-v1-coding-venti`、`sonilo-video-to-music`、`deepseek-v4-pro-0813`、`macaron-v1-tall`。这表示 Ahrefs 当前没有符合筛选条件的匹配词，不表示没有搜索需求。
- SERP commercial/use-case 的空结果必须标为 `N/A`，不能用头词竞争度代替。

## 5 个主推模型：完整流程结果

### 主推词总览

| 模型 | 主关键词（SV/GSV/KD） | 商业词 | 用途/API词 | 机会判断 |
|---|---|---|---|---|
| kimi-k3 | `kimi k3` · SV 3,900 / GSV 121,000 / KD 70 | `kimi k3 pricing` · GSV 4,600 | `kimi k3 api` · GSV 1,000 | D: hard / authority needed; API/长尾优先于头词 |
| gpt-image-2 | `gpt image 2` · SV 5,900 / GSV 56,000 / KD 58 | `gpt image 2 api` · GSV 1,600 | `gpt image generator` · GSV 3,100 | C: competitive; API/长尾优先于头词 |
| deepseek-v4-pro | `deepseek v4 pro` · SV 2,600 / GSV 26,000 / KD 52 | `deepseek v4 pro pricing` · GSV 1,100 | `deepseek v4 pro api` · GSV 700 | C: competitive; API/长尾优先于头词 |
| gpt-5.6-sol | `gpt 5.6 sol` · SV 1,700 / GSV 13,000 / KD 22 | `gpt 5.6 sol pricing` · GSV 350 | `gpt 5.6 sol api` · GSV 50 | B: favorable; API/长尾优先于头词 |
| MiniMax-H3 | `minimax h3` · SV 700 / GSV 5,400 / KD 41 | `minimax h3 api` · GSV 80 | `minimax h3 comfyui` · GSV 450 | C: competitive; API/长尾优先于头词 |

> 这些是关键词机会判断，不是排名保证。头词的实际 SERP 主要由官方站、高 DR 站点或新闻占据；页面应靠更完整的 API、价格、设置、事实边界和 FAQ 覆盖来获得长尾切入。

### gpt-5.6-sol

**实施状态（2026-08-27）**：专属 landing 文案已接入；配置/组件回归、本地 build/SSR 检查已通过，真实环境浏览器与事实复核待完成。

**建议 metadata**
- Title: `GPT-5.6 Sol API and pricing | Flatkey`
- Meta description: Use GPT-5.6 Sol through Flatkey with OpenAI-compatible API access, a 1,048,576-token context, current token pricing, and one API key.
- H1: `GPT-5.6 Sol API, pricing, and model details`

**已验证事实基线**：OpenAI catalog entry; /v1/chat/completions; text, image, and file modalities; 1,048,576-token context; released 2026-06-20; PLG token rates: $4 input, $24 output, $0.40 cache read, $5 cache creation per 1M tokens.

**H2/主题布局（按搜索意图聚类，不是一 H2 一关键词）**

| H2 | 关键词证据 | 页面写法 |
|---|---|---|
| What is GPT-5.6 Sol? | gpt 5.6 sol (GSV 13K, KD 22); Questions: what is GPT-5.6 Sol (GSV 250) | Opening definition and verified catalog facts. |
| GPT-5.6 Sol API and model ID | gpt 5.6 sol api (GSV 50); model/interface terms (GSV 60) | Show endpoint /v1/chat/completions and exact model ID; explain access flow. |
| GPT-5.6 Sol pricing | gpt 5.6 sol pricing (GSV 350) | Use current Flatkey token rates; do not invent benchmark or consumer ChatGPT pricing. |
| Context window, modalities, and endpoint | context/modalities terms; catalog fact (1,048,576) | Describe text/image/file inputs only as recorded. |
| GPT-5.6 Sol vs GPT-5.5, Terra, and Luna | vs 5.5 (GSV 300); series comparisons (GSV 60–80) | Fact comparison table only; avoid unsupported quality claims. |
| FAQ | Questions endpoint (12 rows; global_volume filter) | Answer model, API, pricing, access, context, and release questions. |

**FAQ 覆盖**

| 问题 | 来源/需求信号 | 答案必须包含 |
|---|---|---|
| What is GPT-5.6 Sol? | Ahrefs Questions, GSV 250 | OpenAI catalog model exposed on Flatkey; do not call it a ChatGPT consumer product. |
| How do I use GPT-5.6 Sol? | Ahrefs Questions, GSV 10 | Use the Flatkey API endpoint /v1/chat/completions with the model ID shown on the page. |
| What is the GPT-5.6 Sol API model ID? | Matching term, GSV 50 (API); exact ID verified in catalog | Show the exact ID and endpoint; do not imply Codex support. |
| How much does GPT-5.6 Sol cost? | Overview, GSV 350 (pricing) | Use the four current token dimensions and date the price block. |
| What context and inputs does it support? | Catalog fact; question demand not separately quantified | 1,048,576-token context; text/image/file modalities in metadata. |
| When was GPT-5.6 Sol released? | Ahrefs Questions, GSV 10 | Catalog release date 2026-06-20; label as catalog data. |

**排除/另页**：Do not target Codex, ChatGPT consumer, Ultra, benchmark-superiority, or Terra/Luna performance claims on this page without separate verified evidence.

**SERP 竞争快照（US-English proxy；只统计有 URL 的 organic positions）**

| 查询 | 返回行 | Top 10 organic 中位 DR | 中位 UR | 中位 referring domains | 观察 |
|---|---:|---:|---:|---:|---|
| gpt 5.6 sol pricing (commercial) | 0 | N/A | N/A | N/A | SERP not found / N/A |
| gpt 5.6 sol (head) | 18 | 92.0 | 5.0 | 196.5 | 1. Previewing GPT-5.6 Sol: a next-generation model；4. Previewing GPT-5.6 Sol: a next-generation model |
| gpt 5.6 sol api (use_case) | 0 | N/A | N/A | N/A | SERP not found / N/A |

### MiniMax-H3

**实施状态（2026-08-27）**：专属 landing 文案与 canonical alias redirect 已接入；配置/组件回归、本地 SSR 与 alias 响应头检查已通过，FAQ/locale 真实环境验收与事实复核待完成。

**建议 metadata**
- Title: `MiniMax H3 video API and pricing | Flatkey`
- Meta description: Use MiniMax H3 through Flatkey's /v1/videos endpoint with 768P or 2K settings, 4–15 second clips, ratio controls, and current per-second pricing.
- H1: `MiniMax H3 video API, pricing, and settings`

**已验证事实基线**：MiniMax catalog entry; /v1/videos; text/image/video/audio modalities in metadata; current PLG display rate `$0.08/sec` (catalog snapshot 2026-08-25); page generator fields: 768P/2K, 4–15s, ratios 21:9 through 9:16/adaptive, AIGC watermark. The public catalog snapshot does not expose a separate 2K price row; do not publish a `$0.13/sec` tier unless a dated upstream source is added to the evidence pack.

**H2/主题布局（按搜索意图聚类，不是一 H2 一关键词）**

| H2 | 关键词证据 | 页面写法 |
|---|---|---|
| What is MiniMax H3? | minimax h3 (GSV 5.4K, KD 41); what is (GSV 20) | Define it as the catalog's MiniMax video endpoint; avoid open-weight claims. |
| MiniMax H3 video API | video model (GSV 100); API terms (GSV 80) | Explain /v1/videos and the hosted request flow. |
| MiniMax H3 prompting guide | prompting guide (GSV 150) | Give concise prompt structure and shot/action guidance. |
| Resolution, duration, ratio, and watermark | generator settings (verified page config) | List only the configured controls and defaults. |
| MiniMax H3 pricing | API/pricing cluster (GSV 80) | State $0.08/sec current PLG base; distinguish 768P/2K page tiers if shown. |
| ComfyUI and local setup: what is verified | ComfyUI (GSV 450, KD 22); local (GSV 70) | Use an evidence box; do not imply native Flatkey ComfyUI or local deployment. |
| FAQ | Questions endpoint (4 rows; global_volume filter) | Answer video API, settings, price, and verification boundaries. |

**FAQ 覆盖**

| 问题 | 来源/需求信号 | 答案必须包含 |
|---|---|---|
| What is MiniMax H3? | Ahrefs Questions, GSV 20 | MiniMax video endpoint available through /v1/videos. |
| How do I use MiniMax H3? | Ahrefs Questions, GSV 10 | Configure a video request on the page, then continue to the API flow. |
| What settings does MiniMax H3 support? | Verified page config | 768P/2K, 4–15 seconds, ratio options, and watermark control as configured. |
| How much does MiniMax H3 cost? | Overview/API, GSV 80 | Current PLG catalog rate is $0.08 per second; label timestamp and tier details. |
| Does Flatkey provide a native ComfyUI or local MiniMax H3 install? | Matching ComfyUI GSV 450; answer is fact boundary | The catalog does not verify native ComfyUI/local delivery; link only if a separate integration is documented. |
| Is MiniMax H3 censored? | Ahrefs Questions, GSV 10 | Do not speculate; current catalog does not publish a moderation policy. |

**排除/另页**：Do not claim open weights, local/VRAM support, native ComfyUI, uncensored generation, or a text-chat endpoint; the verified endpoint is video.

**SERP 竞争快照（US-English proxy；只统计有 URL 的 organic positions）**

| 查询 | 返回行 | Top 10 organic 中位 DR | 中位 UR | 中位 referring domains | 观察 |
|---|---:|---:|---:|---:|---|
| minimax h3 api (commercial) | 0 | N/A | N/A | N/A | SERP not found / N/A |
| minimax h3 (head) | 16 | 79.0 | 4.0 | 22.0 | 1. MiniMaxAI/MiniMax-H3；2. MiniMax H3: An Open Model Breaking the Boundaries ... |
| minimax h3 comfyui (use_case) | 13 | 83.5 | 2.0 | 68.0 | 1. Comfy-Org/MiniMax-H3；2. MiniMax H3 in ComfyUI: T2V, I2V, and R2V Video Workflows |

### deepseek-v4-pro

**实施状态（2026-08-27）**：专属 landing 文案已接入；配置/组件回归、本地 SSR 检查已通过，UTC 分层价格和双 API 路径仍需真实环境/事实复核，发布前不视为排名结果。

**建议 metadata**
- Title: `DeepSeek V4 Pro API and dynamic pricing | Flatkey`
- Meta description: Call DeepSeek V4 Pro through OpenAI-compatible or Anthropic-compatible endpoints with a 1,048,576-token context, file input, and time-tiered pricing.
- H1: `DeepSeek V4 Pro API, pricing, and model details`

**已验证事实基线**：DeepSeek catalog entry; /v1/chat/completions and /v1/messages; text/file modalities; 1,048,576-token context; tiered UTC pricing: peak p $1.32 + cache read $0.044 + output $3.96, off-peak p $0.66 + cache read $0.022 + output $1.98 per 1M tokens; distillable metadata is not an open-source claim.

**H2/主题布局（按搜索意图聚类，不是一 H2 一关键词）**

| H2 | 关键词证据 | 页面写法 |
|---|---|---|
| What is DeepSeek V4 Pro? | deepseek v4 pro (GSV 26K, KD 52, TP 1.6K); what is (GSV 10) | Opening answer with verified model and context facts. |
| DeepSeek V4 Pro API and model name | API (GSV 700); model name API (GSV 50) | Show both /v1/chat/completions and /v1/messages and exact model ID. |
| Dynamic DeepSeek V4 Pro pricing | pricing (GSV 1.1K); price-reduction terms (GSV 10) | Explain UTC peak/off-peak expression; never collapse to one static rate. |
| Context window and file input | context window (GSV 20); catalog fact | Describe 1,048,576-token context and file modality. |
| V4 Pro vs V4 Flash | comparison (GSV 100) | Compare documented endpoint/price fields only; no unsupported benchmark ranking. |
| Local, download, and open-source status | download/official/local/open-source (GSV 150/100/40) | Clearly mark what the current catalog does not verify. |
| FAQ | Questions endpoint (12 rows; global_volume filter) | Answer use, API, price timing, context, and status questions. |

**FAQ 覆盖**

| 问题 | 来源/需求信号 | 答案必须包含 |
|---|---|---|
| What is DeepSeek V4 Pro? | Ahrefs Questions, GSV 10 | DeepSeek catalog model with text/file modalities and 1,048,576-token context. |
| How do I use DeepSeek V4 Pro? | Ahrefs Questions, GSV 80 | Use either documented compatible endpoint and the exact model ID. |
| What is the DeepSeek V4 Pro API model name? | Matching term, GSV 50 | Display the model ID and both endpoint paths. |
| How is DeepSeek V4 Pro priced? | Matching pricing, GSV 1.1K | Show peak/off-peak UTC tiers and all token dimensions. |
| Does DeepSeek V4 Pro support local download or open-source use? | Questions/matching local/open-source cluster | Current catalog does not verify downloadable weights or local deployment; do not infer from distillable=true. |
| Does it support vision or multimodal input? | Matching vision/multimodal terms; no verified metadata | Only claim text/file modalities until an authoritative model document confirms more. |

**排除/另页**：Exclude single-price claims, benchmark superiority, parameter counts, GGUF/VRAM/local availability, vision claims, and open-source/download claims unless separately verified.

**SERP 竞争快照（US-English proxy；只统计有 URL 的 organic positions）**

| 查询 | 返回行 | Top 10 organic 中位 DR | 中位 UR | 中位 referring domains | 观察 |
|---|---:|---:|---:|---:|---|
| deepseek v4 pro api (commercial) | 0 | N/A | N/A | N/A | SERP not found / N/A |
| deepseek v4 pro (head) | 18 | 87.0 | 6.0 | 13.0 | 2. deepseek-ai/DeepSeek-V4-Pro；3. DeepSeek V4 Pro 0423 - API Pricing & Benchmarks |
| deepseek v4 pro coding (use_case) | 0 | N/A | N/A | N/A | SERP not found / N/A |

### kimi-k3

**实施状态（2026-08-27）**：专属 landing 文案已接入；配置/组件回归、本地 SSR/locale metadata 检查已通过，真实环境 FAQ 去重与事实验收待完成。

**建议 metadata**
- Title: `Kimi K3 API and pricing | Flatkey`
- Meta description: Use Moonshot AI's Kimi K3 through OpenAI- and Anthropic-compatible endpoints with a 1,048,576-token context, file input, and current token pricing.
- H1: `Kimi K3 API, pricing, and model details`

**已验证事实基线**：Moonshot AI catalog entry; /v1/chat/completions and /v1/messages; text/file modalities; 1,048,576-token context; PLG rates $2.40 input, $12 output, $0.24 cache per 1M tokens; distillable metadata is not an open-source claim.

**H2/主题布局（按搜索意图聚类，不是一 H2 一关键词）**

| H2 | 关键词证据 | 页面写法 |
|---|---|---|
| What is Kimi K3? | kimi k3 (GSV 121K, KD 70, TP 3.4K); what is (GSV 350, KD 4) | Head demand is large but competitive; use a clear factual opening answer. |
| Kimi K3 API and access | API (GSV 1K); how to access/use (Questions GSV 400/30) | Show both compatible endpoint paths and access steps. |
| Kimi K3 pricing | pricing (GSV 4.6K); cost questions (GSV 40) | Use current token dimensions and date the data. |
| Coding and knowledge-work use cases | Kimi K3 AI (GSV 6.3K); paper (GSV 450); coding/use terms | Frame use cases without claiming benchmark superiority. |
| Context window and file input | catalog fact (1,048,576) | Describe the verified context/file fields. |
| Local, open-source, and hardware status | free (GSV 800); open source (GSV 350); local (GSV 200); hardware (GSV 150) | Answer demand honestly: current catalog does not verify free/local/weights/VRAM. |
| FAQ | Questions endpoint (50 rows; global_volume filter) | Prioritize free/use/open-source/access/cost questions with factual boundaries. |

**FAQ 覆盖**

| 问题 | 来源/需求信号 | 答案必须包含 |
|---|---|---|
| What is Kimi K3? | Ahrefs Questions, GSV 350, KD 4 | Moonshot AI model with file input and 1,048,576-token context. |
| How do I use Kimi K3? | Ahrefs Questions, GSV 400 | Use the displayed compatible endpoint and API key flow. |
| What is the Kimi K3 API model ID? | Matching API cluster, GSV 1K | Show exact model ID for /v1/chat/completions and /v1/messages. |
| How much does Kimi K3 cost? | Questions/matching pricing, GSV 4.6K | Current PLG rates: $2.40 input/$12 output/$0.24 cache per 1M tokens. |
| Is Kimi K3 free? | Ahrefs Questions, GSV 800 | The current catalog shows paid token rates; do not promise free access. |
| Is Kimi K3 open source or available locally? | Ahrefs Questions, GSV 350/200 | Current catalog does not verify open weights, local deployment, or hardware requirements. |
| Who makes Kimi K3? | Matching/verified catalog | Moonshot AI; distinguish vendor metadata from Flatkey routing availability. |

**排除/另页**：Do not claim free access, open weights, local deployment, VRAM/hardware requirements, benchmark superiority, or paper-derived product facts.

**SERP 竞争快照（US-English proxy；只统计有 URL 的 organic positions）**

| 查询 | 返回行 | Top 10 organic 中位 DR | 中位 UR | 中位 referring domains | 观察 |
|---|---:|---:|---:|---:|---|
| kimi k3 api (commercial) | 0 | N/A | N/A | N/A | SERP not found / N/A |
| kimi k3 (head) | 25 | 91.0 | 8.0 | 207.0 | 1. Kimi AI 官网- K3 上线，专为智能体编程与知识工作打造；2. Kimi K3 Tech Blog: Open Frontier Intelligence |
| how to run kimi k3 locally (use_case) | 0 | N/A | N/A | N/A | SERP not found / N/A |

### gpt-image-2

**实施状态（2026-08-27）**：专属 image API、控制项与多维价格文案已接入；配置/组件回归、本地 build/SSR/schema 检查已通过，真实环境浏览器与事实验收待完成。

**建议 metadata**
- Title: `GPT Image 2 API and image generator | Flatkey`
- Meta description: Prepare GPT Image 2 requests through Flatkey with image API access, current token-dimension pricing, size and quality controls, formats, background, and moderation settings.
- H1: `GPT Image 2 API, pricing, and image generation`

**已验证事实基线**：OpenAI image model; /v1/images/generations (and openai-compatible chat route); generator fields n 1–10, sizes 1024x1024/1536x1024/1024x1536/auto, quality auto/high/medium/low, PNG/JPEG/WebP, background opaque/auto, moderation auto/low; live pricing is token-dimension based (input $4, output $24, cache $1, image $6.40 per 1M dimensions in catalog).

**H2/主题布局（按搜索意图聚类，不是一 H2 一关键词）**

| H2 | 关键词证据 | 页面写法 |
|---|---|---|
| What is GPT Image 2? | gpt image 2 (GSV 56K, KD 58, TP 25K); Questions what is (GSV 80) | Define it as the catalog's OpenAI image model; include both GPT Image 2/GPT-image-2 spellings naturally. |
| GPT Image 2 API and access | API (GSV 1.6K, KD 20); access (Questions GSV 60) | Show /v1/images/generations and the pre-signup configuration flow. |
| GPT Image 2 pricing | pricing (GSV 500); API price (GSV 100) | Explain live token-dimension pricing; separate any static per-image marketing row. |
| How to use GPT Image 2 and write prompts | Questions how to use (GSV 350); prompt Overview (GSV 700) | Provide prompt structure and examples without unsupported safety claims. |
| Sizes, quality, formats, background, and moderation | transparent/background terms (GSV 50); verified generator fields | List controls exactly; do not promise transparent output when the field says opaque/auto. |
| Product, ecommerce, and ad image use cases | image-generator parent topic (GSV 3.1K, KD 66, TP 178K); catalog Marketing category | Use product/ecommerce examples as positioning, not model capability guarantees. |
| Migration from GPT Image 1 | Question GSV 10 | Include only if official migration details are verified; otherwise link to a separate guide. |
| FAQ | Questions endpoint (32 rows; global_volume filter) | Prioritize how-to, access, pricing, controls, and free/no-signup boundaries. |

**FAQ 覆盖**

| 问题 | 来源/需求信号 | 答案必须包含 |
|---|---|---|
| What is GPT Image 2? | Ahrefs Questions, GSV 80 | OpenAI image model available through the documented image generation endpoint. |
| How do I access GPT Image 2? | Ahrefs Questions, GSV 60 | Configure request fields on Flatkey, then continue with an account/API key. |
| What is the GPT Image 2 API endpoint? | Matching/API SERP, GSV 1.6K | Use /v1/images/generations; show the exact request fields supported by this page. |
| How much does GPT Image 2 cost? | Matching pricing, GSV 500 | Use live token-dimension pricing; do not reduce it to one universal per-image price. |
| What sizes and formats are supported? | Verified generator config | 1024x1024, 1536x1024, 1024x1536, auto; PNG/JPEG/WebP; quality and background options as configured. |
| Is GPT Image 2 free or available without signup? | Matching free/no-signup cluster; answer is product policy | Do not promise free/no-signup generation; the page saves a draft before signup and requires the normal access flow. |
| Can it create transparent backgrounds or handle NSFW prompts? | Matching transparent/NSFW terms; no verified claim | Only state the configured background/moderation options and link to an authoritative policy if added. |

**排除/另页**：Do not promise free/no-signup generation, transparent backgrounds, NSFW policy outcomes, ChatGPT availability, or GPT Image 1 migration behavior without authoritative evidence; distinguish token pricing from static per-image display rows.

**SERP 竞争快照（US-English proxy；只统计有 URL 的 organic positions）**

| 查询 | 返回行 | Top 10 organic 中位 DR | 中位 UR | 中位 referring domains | 观察 |
|---|---:|---:|---:|---:|---|
| gpt image 2 api (commercial) | 10 | 87.0 | 6.0 | 51.0 | 1. GPT Image 2 Model \| OpenAI API；2. GPT-5.4 Image 2 - API Pricing & Benchmarks |
| gpt image 2 (head) | 16 | 93.0 | 11.0 | 45.5 | 1. Introducing ChatGPT Images 2.0；2. GPT Image 2 Model \| OpenAI API |
| gpt image 2 prompt (use_case) | 0 | N/A | N/A | N/A | SERP not found / N/A |

## Seedance 2.5：独立复审、关键词布局与事实边界

### Ahrefs 关键词证据（全球优先，US 仅作代理）

下表中的 `SV*` 均为 Ahrefs `country=us` 的英文代理值，`GSV` 为全球值；`KD` 为 Ahrefs 估计，`N/A` 表示 Ahrefs 没有返回数据，不是 0。关键词按搜索意图聚类，不能把每个词机械地分配给一个 H2。

| 关键词 | SV* | GSV | KD | 意图 | 页面布局/处理 |
|---|---:|---:|---:|---|---|
| `seedance 2.5` | 3,800 | 17,000 | 25 | 品牌/信息 | Title、H1、首段定义；全页唯一主词 |
| `seedance 2.5 release date` | 500 | 1,700 | N/A | 信息 | FAQ；区分官方文章日期与目录 `released_at`，不猜发布日期 |
| `seedance 2.5 api` | 80 | 250 | N/A | 商业/交易 | API/access 区块；展示 Flatkey `/v1/videos` 和 `content[]` 请求形态 |
| `seedance 2.5 release` | 60 | 150 | N/A | 信息 | FAQ/事实时间线；避免把新闻词写成实时状态 |
| `bytedance seedance 2.5` | 50 | 200 | N/A | 品牌/信息 | 首段和事实说明；来源归属只按官方资料表述 |
| `seedance 2.5 ai` | 30 | 300 | N/A | 信息 | 定义段和能力区块；不写未经证实的性能排名 |
| `seedance 2.5 video` | 10 | 200 | N/A | 信息/商业 | H1 与视频工作流说明；不承诺免费或无限量 |
| `seedance 2.5 free` | 30 | 250 | N/A | 交易 | FAQ 事实边界；只说明当前页面没有“免费/免注册”承诺 |
| `seedance 2.5 pricing` | 20 | 60 | N/A | 商业 | 独立 Pricing 区块；列出分辨率、时长、视频参考三种计费情形 |
| `seedance 2.5 price` | 10 | 50 | N/A | 商业 | Pricing FAQ；不要把目录基准价当成所有请求价格 |
| `seedance 2.5 cost` | 20 | 40 | N/A | 商业 | Pricing FAQ；说明最终结算受任务估算与账户限制影响 |
| `seedance 2.5 ai video` | 50 | 80 | N/A | 信息/商业 | 首段、能力与场景；使用“AI video generation”自然变体 |
| `seedance 2.5 ai video generator` | 30 | 40 | N/A | 商业 | H1/首段/CTA 的自然变体；不堆叠同义词 |
| `seedance 2.5 image to video` | 0–10 | 0–10 | N/A | 用途 | 参考媒体与工作流区块；只描述已支持的 image reference 能力 |
| `seedance 2.5 text to video` | N/A | N/A | N/A | 用途 | API 示例与能力说明；Ahrefs 未建立稳定数据，不虚构搜索量 |
| `seedance 2.5 prompts` | 0–10 | 0–10 | N/A | 信息/操作 | 6 个行业工作流示例；每个示例使用不同目标、镜头和约束 |
| `seedance 2.5 api key` | 未索引 | 未索引 | N/A | 交易 | API 区块用“Flatkey API key”事实回答，不把未索引词写成有量词 |
| `seedance 2.5 vs kling` | 0–10 | 0–10 | N/A | 比较 | 仅保留谨慎的能力维度说明；不作未经测试的质量胜负结论 |

### Questions 需求与 FAQ 映射

Ahrefs Questions 的全球量信号为：`when is seedance 2.5 coming out`（GSV 70）、`how to use seedance 2.5`（30）、`when does seedance 2.5 come out`（30）、`what is seedance 2.5`（20）、`where can/where to use seedance 2.5`（各 10）、`when will seedance 2.5 be released`（10）。页面 FAQ 使用这些问题的自然改写，并以已核实的产品事实回答；不把同一个“是什么/多少钱”模板复制到其他模型页。

### 已核实事实与计费边界

- 官方 Seedance 2.5 页面将其描述为音视频联合生成模型；官方文章发表于 **2026-07-31**，记录单次最多 30 秒、可多轮延长、参考媒体、时间戳编辑、绿幕与镜头视角/参考编辑，并说明当时已在 Jimeng/Doubao 上线、BytePlus ModelArk API 即将推出（以文章发布日期为准，不把它写成当前 Flatkey 供应承诺）。来源：[官方产品页](https://seed.bytedance.com/en/seedance2_5)、[官方发布文章](https://seed.bytedance.com/en/blog/one-take-creation-flexible-referencing-introducing-seedance-2-5)。
- Flatkey 目录快照记录 `modalities=text/video/image`、`supported_endpoint_types=[video]`、`released_at=2026-08-04`；这是目录字段，和官方文章日期分别标注，不能合并成一个“release date”事实。
- 当前任务契约支持 4–30 秒、480p/720p、`16:9/4:3/1:1/3:4/9:16/adaptive`，参考输入上限为图片 30、视频 10、音频 10（总计 50）；这些是接口/页面配置事实，不是模型质量保证。
- Flatkey 计费设计（内部文档，非官方供应商价）为：无视频参考时 480p `$0.140 × 输出秒数`、720p `$0.314 × 输出秒数`；有视频参考时 480p `$0.084 × 输入视频总秒数`、720p `$0.188 × 输入视频总秒数`。目录 `$0.14` 是基准字段，不能代替上述全部公式；远程视频时长未知时预估与最终结算可能不同。来源：[计费设计文档](./superpowers/specs/2026-08-11-modelapi-seedance-25-billing-design.md)。

### 页面结构与反同质化控制

| 页面区域 | 目标关键词簇 | Seedance 专属内容 |
|---|---|---|
| Title/H1/首段 | 主词、AI video、video generator、ByteDance | 定义音视频联合生成、Flatkey `/v1/videos` 访问和参考媒体；不套用文本模型开场 |
| Capabilities/Scenes | text-to-video、image-to-video、prompts、video | 微短剧/漫画、广告电商 UGC、电影预演、游戏动画、知识创作者、市场调研六组不同工作流 |
| Pricing | pricing、price、cost | 480p/720p 与有/无视频参考的公式表，明确计费基准和结算边界 |
| API/How to use | api、api key、how to use | 官方 `content[]` 结构、duration/resolution/ratio/reference 限制；不声称 OpenAI 兼容或流式输出 |
| FAQ | release、free、where/how to use、what is | 9 个 Seedance 专属问答；每个答案都绑定时间、接口、限制或计费事实 |
| Schema/metadata | 主词与用途词 | JSON-LD 不输出误导性的固定 Product Offer；canonical/hreflang 仍由统一页面架构管理 |

共用 workbench、health、providers、related、CTA 和导航属于版型复用，不等于正文复用。Seedance 页面必须保持自己的事实、价格公式、工作流示例和 FAQ；后续兄弟页检查应比较完整句子、问题与答案，不只是替换模型名。

## 全站 101 页关键词矩阵

> `Primary` 是建议页面主题词，不等于把该词机械塞入每个 H2。`Commercial` 是 Matching terms 候选；`Secondary` 是相关主题候选。快速模式未调用 Questions endpoint，FAQ 标记为推断问题。

> **数据修正（2026-08-27）**：原始生成脚本曾按目录位置读取 Matching terms；Seedance 2.5 被排除后导致第 88–101 行的 commercial/secondary 错位。本表已按每个 Matching JSON 的 `seed` 重新 join；后续生成必须按 model key/seed join，不能按数组位置 zip。

矩阵中的主推行额外记录 `research_status`、`implementation_status`、`validation_status`（以及 MiniMax 的 `canonical_status`）；`code_implemented_pending_validation` 只表示专属数据已进入工作区，不表示已发布、已索引或已获得排名。

| # | 模型 | 页面 | 模式 | Primary | Commercial/action | Secondary clusters | Intent | SV* | GSV | KD | TP | CPC* | FAQ | 机会 |
|---:|---|---|---|---|---|---|---|---:|---:|---:|---:|---:|---|---|
| 1 | `gemini-3.1-flash-image` | `/models/gemini-3.1-flash-image` | quick | `gemini-3.1-flash-image` | `gemini 3.1 flash image api pricing` | gemini-3.1-flash-image-preview pricing · gemini 3.1 flash image preview pricing · gemini-3.1-flash-image-preview price | branded/informational (inferred) | 40 | 250 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 2 | `seedance-2.0-pro` | `/models/seedance-2.0-pro` | quick | `seedance 2.0 pro` | `seedance 2.0 pro ai video generator` | seedance pro 2.0 · seedance 2.0 pro free · seedance 2.0 pro subscription cost | N/A | N/A | N/A | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 3 | `eleven_sound_v1` | `/models/eleven_sound_v1` | quick | `eleven sound v1` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | fact-led inferred; no Matching terms | E: KD N/A / long-tail test |
| 4 | `gemini-embedding-001` | `/models/gemini-embedding-001` | quick | `gemini-embedding-001` | `models/embedding-001 gemini api` | gemini-embedding-001 dimensions · gemini embedding 001 pricing · gemini-embedding-001 model | informational+branded | 200 | 1,400 | N/A | N/A | 100 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 5 | `gemini-3.5-flash-lite` | `/models/gemini-3.5-flash-lite` | quick | `gemini-3.5-flash-lite` | `gemini 3.5 flash lite api pricing` | gemini 3.5 flash lite pricing · gemini 3.5 flash lite price · gemini 3.5 flash lite benchmark | branded/informational (inferred) | 10 | 70 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 6 | `glm-4.7` | `/models/glm-4.7` | quick | `glm-4.7` | `glm-4.7 api pricing` | glm-4.7 context window · glm-4.7 api pricing · livecodebench v6 glm-4.7 score | informational+commercial+branded | 500 | 3,400 | 53 | 1,400 | 80 | inferred from Matching terms + verified facts | C: competitive |
| 7 | `gpt-5.6-sol` | `/models/gpt-5.6-sol` | full | `gpt 5.6 sol` | `gpt 5.6 sol pricing` | openai gpt 5.6 sol · gpt-5.6-sol pricing · gpt 5.6 sol vs 5.5 | informational+commercial+branded | 1,700 | 13,000 | 22 | N/A | 160 | Ahrefs Questions (global filter) | B: favorable |
| 8 | `claude-opus-4-6` | `/models/claude-opus-4-6` | quick | `claude-opus-4-6` | `claude-opus-4-6 model name anthropic api 2026` | claude-opus-4-6-thinking · anthropic/claude-opus-4-6 · claude-opus-4-6 pricing | informational+commercial+transactional+branded | 200 | 800 | N/A | N/A | 130 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 9 | `grok-imagine-video` | `/models/grok-imagine-video` | quick | `grok-imagine-video` | `grok imagine image to video how to use` | grok imagine image to video how to use · grok imagine xai image to video feature · grok imagine video generation failed | branded/informational (inferred) | 60 | 350 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 10 | `glm-5.1` | `/models/glm-5.1` | quick | `glm-5.1` | `glm-5.1 api pricing 2026` | glm-5.1-fp8 · glm-5.1 api pricing 2026 · z.ai glm-5.1 pricing official | informational+commercial+branded | 600 | 4,600 | N/A | N/A | 80 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 11 | `gemini-3.1-flash-lite` | `/models/gemini-3.1-flash-lite` | quick | `gemini-3.1-flash-lite` | `gemini 3.1 flash lite api price` | gemini 3.1 flash lite price · gemini 3.1 flash lite openrouter · gemini 3.1 flash-lite google | informational+commercial+transactional+branded | 150 | 800 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 12 | `claude-sonnet-5` | `/models/claude-sonnet-5` | quick | `claude-sonnet-5` | `anthropic api model claude-3-5-sonnet-20241022` | claude-3-5-sonnet-20241022 model · anthropic model claude-3-5-sonnet-20241022 · anthropic api model claude-3-5-sonnet-20241022 | informational | 50 | 250 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 13 | `kimi-k2.6` | `/models/kimi-k2.6` | quick | `kimi-k2.6` | `moonshot kimi k2.6 api pricing official` | kimi k2.6 parameter count · moonshot kimi k2.6 api pricing official · moonshot kimi k2.6 api pricing official 2026 | informational+branded | 150 | 900 | N/A | N/A | 110 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 14 | `gemini-3.1-flash-lite-image` | `/models/gemini-3.1-flash-lite-image` | quick | `gemini-3.1-flash-lite-image` | `N/A` | N/A | branded/informational (inferred) | 10 | 50 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 15 | `gemini-3.5-flash` | `/models/gemini-3.5-flash` | quick | `gemini-3.5-flash` | `gemini 3.5 flash pricing api cost` | gemini 3.5 flash coding · gemini 3.5 flash ai · gemini 3.5 flash pricing api cost | branded/informational (inferred) | 100 | 500 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 16 | `grok-imagine-image` | `/models/grok-imagine-image` | quick | `grok-imagine-image` | `grok imagine image to video how to use` | grok imagine image generation limit reset time · grok imagine image to video how to use · supergrok image generation limit grok imagine | branded/informational (inferred) | 30 | 200 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 17 | `gemini-2.5-flash-tts` | `/models/gemini-2.5-flash-tts` | quick | `gemini-2.5-flash-tts` | `gemini 2.5 flash tts pricing` | gemini-2.5-flash-preview-tts · gemini 2.5 flash tts pricing · gemini 2.5 flash preview tts pricing | branded/informational (inferred) | 10 | 40 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 18 | `veo-3.1-fast-generate-preview` | `/models/veo-3.1-fast-generate-preview` | quick | `veo-3.1-fast-generate-preview` | `N/A` | N/A | branded/informational (inferred) | 10 | 90 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 19 | `gpt-5.4-nano` | `/models/gpt-5.4-nano` | quick | `gpt-5.4-nano` | `openai api pricing gpt-5.4 mini nano` | openai gpt-5.4 mini nano · openai gpt-5.4 nano · gpt-5.4 mini nano openai | informational+branded | 100 | 500 | N/A | N/A | 140 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 20 | `gemini-2.5-pro-tts` | `/models/gemini-2.5-pro-tts` | quick | `gemini-2.5-pro-tts` | `gemini 2.5 pro tts api` | gemini 2.5 pro preview tts · gemini-2.5-pro-preview-tts · gemini 2.5 pro tts pricing | branded/informational (inferred) | 10 | 20 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 21 | `gemma-4-31b-it` | `/models/gemma-4-31b-it` | quick | `gemma-4-31b-it` | `gemma-4-31b-it api` | gemma-4-31b-it-nvfp4 · nvidia/gemma-4-31b-it-nvfp4 · cyankiwi/gemma-4-31b-it-awq-4bit | informational+commercial+transactional+branded | 150 | 700 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 22 | `qwen3.5-plus` | `/models/qwen3.5-plus` | quick | `qwen3.5-plus` | `qwen3.5-plus api` | qwen3.5-plus features and capabilities · what are the main features of qwen3.5-plus? · qwen3.5 plus model details | informational+transactional+branded | 250 | 1,600 | N/A | N/A | 120 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 23 | `seedance-2.0-fast` | `/models/seedance-2.0-fast` | quick | `seedance-2.0-fast` | `seedance 2.0 fast api price per second 2026` | dreamina seedance 2.0 fast · seedance 2.0 fast free · seedance 2.0 fast unlimited | branded/informational (inferred) | 0 | 0 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 24 | `grok-imagine-video-1.5` | `/models/grok-imagine-video-1.5` | quick | `grok-imagine-video-1.5` | `how to access grok imagine video 1.5` | grok imagine video 1.5 preview · grok imagine video 1.5 features · grok imagine video 1.5 preview xai | branded/informational (inferred) | 10 | 50 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 25 | `gemini-2.5-flash-image` | `/models/gemini-2.5-flash-image` | quick | `gemini-2.5-flash-image` | `gemini 2.5 flash image generation api` | models/gemini-2.5-flash-image · google gemini 2.5 flash image nano banana · models/gemini-2.5-flash-image-preview | branded/informational (inferred) | 50 | 800 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 26 | `gemini-3.1-flash-tts-preview` | `/models/gemini-3.1-flash-tts-preview` | quick | `gemini-3.1-flash-tts-preview` | `gemini-3.1-flash-tts-preview pricing` | gemini-3.1-flash-tts-preview pricing · gemini 3.1 flash tts preview pricing | branded/informational (inferred) | 20 | 100 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 27 | `gpt-5.4-mini` | `/models/gpt-5.4-mini` | quick | `gpt-5.4-mini` | `openai api pricing gpt-5.4 mini $0.75 $4.50 2026` | openai gpt-5.4 mini model · gpt-5.4 mini benchmarks · gpt-5.4-mini api model name | informational+commercial+branded | 200 | 1,200 | N/A | N/A | 3,000 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 28 | `gpt-5.6-luna` | `/models/gpt-5.6-luna` | quick | `gpt-5.6-luna` | `gpt 5.6 luna pricing` | gpt-5.6 luna terra sol · gpt 5.6 luna pricing · gpt 5.6 luna terra | informational+branded | 60 | 350 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 29 | `deepseek-v3.2` | `/models/deepseek-v3.2` | quick | `deepseek-v3.2` | `deepseek-v3.2 api pricing` | deepseek-v3.2 api pricing · deepseek-v3.2 paper arxiv 2512.02814 · deepseek v3.2 685b parameters ttft baseten | informational+transactional+branded | 350 | 1,900 | N/A | N/A | 130 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 30 | `claude-opus-5` | `/models/claude-opus-5` | quick | `claude-opus-5` | `how to use opus 5 in claude code` | claude-opus-4-5-20251101 · how to use opus 5 in claude code · claude opus 4-5 | branded/informational (inferred) | 30 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 31 | `gemini-3-pro-image` | `/models/gemini-3-pro-image` | quick | `gemini-3-pro-image` | `gemini-3-pro-image-preview google api` | gemini 3 pro image preview · gemini 3 pro image preview pricing · gemini-3-pro-image-preview-2k (nano-banana-pro) | branded/informational (inferred) | 30 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 32 | `gemini-3.7-flash` | `/models/gemini-3.7-flash` | quick | `gemini 3.7 flash` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 33 | `gpt-5-mini` | `/models/gpt-5-mini` | quick | `gpt-5-mini` | `openai gpt-5 mini model name api 2026` | openai gpt-5 mini model chatgpt documentation · gpt 5 mini parameters · gpt-5 mini specs | informational+commercial+branded | 700 | 3,100 | 23 | 1,000 | 200 | inferred from Matching terms + verified facts | B: favorable |
| 34 | `glm-5.3` | `/models/glm-5.3` | quick | `glm-5.3` | `N/A` | glm 5.3 when | branded/informational (inferred) | 30 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 35 | `gemini-pro-latest` | `/models/gemini-pro-latest` | quick | `gemini-pro-latest` | `gemini nano banana pro prompts latest` | gemini 3 pro latest · gemini-1.5-pro-latest · gemini pro latest updates 2026 | branded/informational (inferred) | 10 | 30 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 36 | `gpt-5.6-terra` | `/models/gpt-5.6-terra` | quick | `gpt-5.6-terra` | `gpt 5.6 terra cost` | gpt-5.6 luna terra sol · gpt 5.6 terra cost · gpt 5.6 luna terra | branded/informational (inferred) | 40 | 200 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 37 | `gpt-image-2` | `/models/gpt-image-2` | full | `gpt image 2` | `gpt image 2 api` | gpt image 2 api · gpt image generator · gpt image 2 prompt | informational+commercial+branded | 5,900 | 56,000 | 58 | 25,000 | 90 | Ahrefs Questions (global filter) | C: competitive |
| 38 | `macaron-v1-coding-venti` | `/models/macaron-v1-coding-venti` | quick | `macaron v1 coding venti` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | fact-led inferred; no Matching terms | E: KD N/A / long-tail test |
| 39 | `kimi-k2.5` | `/models/kimi-k2.5` | quick | `kimi-k2.5` | `moonshot ai kimi k2.5 api pricing 2026` | moonshot ai kimi k2.5 api pricing 2026 · moonshot ai kimi k2.5 pricing · kimi k2.5 performance | branded/informational (inferred) | 250 | 1,000 | N/A | N/A | 80 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 40 | `qwen3.5-flash` | `/models/qwen3.5-flash` | quick | `qwen3.5-flash` | `qwen3.5-flash api` | qwen3.5-flash features and capabilities · qwen/qwen3.5-flash-02-23 · qwen3.5-flash model details | branded/informational (inferred) | 80 | 350 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 41 | `claude-opus-4-7` | `/models/claude-opus-4-7` | quick | `claude-opus-4-7` | `anthropic claude-opus-4-7 model name api 2026` | claude-opus-4-7-thinking · claude-opus-4-7[1m] · claude-opus-4-7 pricing | branded/informational (inferred) | 70 | 300 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 42 | `deepseek-v3.2-thinking` | `/models/deepseek-v3.2-thinking` | quick | `deepseek-v3.2-thinking` | `N/A` | deepseek-v3.2-exp-thinking · deepseek v3.2-exp thinking non-thinking variant · deepseek-v3.2 thinking livecodebench score | branded/informational (inferred) | 10 | 20 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 43 | `minimax-m2` | `/models/minimax-m2` | quick | `minimax-m2` | `minimax m2 pricing api` | minimax m2 pricing · minimax m2 price · minimax m2 swe-bench score | informational+commercial+branded | 350 | 1,700 | N/A | N/A | 110 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 44 | `qwen3.6-plus` | `/models/qwen3.6-plus` | quick | `qwen3.6-plus` | `qwen3.6-plus api documentation` | qwen3.6-plus pricing · qwen3.6 plus features · qwen3.6 plus coding benchmark | informational+commercial+transactional+branded | 350 | 2,300 | N/A | N/A | 170 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 45 | `deepseek-v4-flash` | `/models/deepseek-v4-flash` | quick | `deepseek-v4-flash` | `deepseek api pricing v4 pro v4 flash 2026` | deepseek-v4-flash model · deepseek v4 flash jailbreak · deepseek v4 flash model api 2026 | informational+transactional+branded | 400 | 3,200 | N/A | N/A | 150 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 46 | `gemini-robotics-er-1.6-preview` | `/models/gemini-robotics-er-1.6-preview` | quick | `gemini-robotics-er-1.6-preview` | `N/A` | N/A | branded/informational (inferred) | 0 | 20 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 47 | `glm-5.2` | `/models/glm-5.2` | quick | `glm-5.2` | `zhipu glm-5.2 api pricing` | glm 5.2 openrouter · glm 5.2 provider · glm 5.2 logo | informational+commercial+branded | 2,100 | 11,000 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 48 | `kimi-k3` | `/models/kimi-k3` | full | `kimi k3` | `kimi k3 pricing` | kimi k3 ai · kimi k3 pricing · how to run kimi k3 locally | informational+branded | 3,900 | 121,000 | 70 | 3,400 | 60 | Ahrefs Questions (global filter) | D: hard / authority needed |
| 49 | `gpt-4o` | `/models/gpt-4o` | quick | `gpt-4o` | `openai api pricing gpt-4o mini august 2025` | gpt-4o 32k context window · gpt-4o-min · openai api pricing gpt-4o mini august 2025 | informational+branded | 3,900 | 33,000 | 64 | 16,000 | 80 | inferred from Matching terms + verified facts | D: hard / authority needed |
| 50 | `grok-imagine-image-quality` | `/models/grok-imagine-image-quality` | quick | `grok-imagine-image-quality` | `N/A` | grok imagine image quality review 2026 · grok imagine image generation quality review 2026 · grok imagine image generation quality 2026 | branded/informational (inferred) | 20 | 80 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 51 | `qwen3.7-plus` | `/models/qwen3.7-plus` | quick | `qwen3.7-plus` | `dashscope qwen3.7-plus model name api 2026` | what is the current version of qwen3.7-plus? · what are the key features of qwen3.7-plus? · dashscope qwen3.7-plus model name api 2026 | branded/informational (inferred) | 70 | 600 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 52 | `gemini-2.5-pro-preview-tts` | `/models/gemini-2.5-pro-preview-tts` | quick | `gemini-2.5-pro-preview-tts` | `gemini 2.5 pro preview tts pricing` | gemini 2.5 pro preview tts pricing · google ai studio gemini 2.5 pro preview tts · gemini-2.5-pro-preview-tts pricing | branded/informational (inferred) | 20 | 90 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 53 | `sonilo-video-to-music` | `/models/sonilo-video-to-music` | quick | `sonilo video to music` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | fact-led inferred; no Matching terms | E: KD N/A / long-tail test |
| 54 | `gemini-3.1-pro-preview-customtools` | `/models/gemini-3.1-pro-preview-customtools` | quick | `gemini-3.1-pro-preview-customtools` | `N/A` | gemini-3.1-pro-preview-customtools what is it · google/gemini-3.1-pro-preview-customtools | branded/informational (inferred) | 70 | 250 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 55 | `seedance-2.0-mini` | `/models/seedance-2.0-mini` | quick | `seedance-2.0-mini` | `seedance 2.0 mini api` | dreamina seedance 2.0 mini · seedance 2.0 mini api · seedance 2.0 mini unlimited | branded/informational (inferred) | 0 | 0 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 56 | `claude-haiku-4-5-20251001` | `/models/claude-haiku-4-5-20251001` | quick | `claude-haiku-4-5-20251001` | `claude-haiku-4-5-20251001 pricing` | claude-haiku-4-5-20251001 pricing · claude-haiku-4-5-20251001 model · claude-haiku-4-5-20251001 anthropic model | informational+branded | 150 | 600 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 57 | `eleven_multilingual_v2` | `/models/eleven_multilingual_v2` | quick | `eleven_multilingual_v2` | `elevenlabs api model_id eleven_multilingual_v2` | elevenlabs model eleven_multilingual_v2 · elevenlabs model_id eleven_multilingual_v2 · elevenlabs model_id eleven_multilingual_v2 docs | branded/informational (inferred) | 20 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 58 | `gemini-3.1-flash-lite-preview` | `/models/gemini-3.1-flash-lite-preview` | quick | `gemini-3.1-flash-lite-preview` | `gemini 3.1 flash-lite preview pricing` | gemini-3.1-flash-lite-preview price · google/gemini-3.1-flash-lite-preview · gemini 3.1 flash-lite preview pricing | informational+transactional+branded | 200 | 1,200 | N/A | N/A | 150 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 59 | `gemini-flash-latest` | `/models/gemini-flash-latest` | quick | `gemini-flash-latest` | `gemini-flash-latest pricing` | gemini-flash-lite-latest · gemini flash lite latest · gemini 2.5 flash latest | branded/informational (inferred) | 30 | 200 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 60 | `qwen3.8-max` | `/models/qwen3.8-max` | quick | `qwen3.8-max` | `qwen3.8-max pricing` | qwen3.8-max-preview · qwen3.8 max preview · qwen3.8-max alibaba | branded/informational (inferred) | 50 | 500 | 1 | N/A | N/A | inferred from Matching terms + verified facts | A: low KD |
| 61 | `seedance-2.0` | `/models/seedance-2.0` | quick | `seedance-2.0` | `byteplus seedance 2.0 api pricing official` | how to get seedance 2.0 · seedance 2.0 video ai · seedance 2.0 video generation pricing | branded/informational (inferred) | 30 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 62 | `gemini-2.5-flash` | `/models/gemini-2.5-flash` | quick | `gemini-2.5-flash` | `gemini 2.5 flash api pricing per token` | openrouter gemini 2.5 flash · vertex ai gemini 2.5 flash pricing · models/gemini-2.5-flash-image-preview | informational+branded | 200 | 1,800 | N/A | N/A | 70 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 63 | `qwen3.7-max` | `/models/qwen3.7-max` | quick | `qwen3.7-max` | `qwen3.7-max api pricing` | qwen3.7-max api pricing · qwen3.7 max price · qwen3.7-max alibaba | informational+branded | 200 | 1,300 | N/A | N/A | 170 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 64 | `claude-fable-5` | `/models/claude-fable-5` | quick | `claude-fable-5` | `claude fable 5 api model id` | anthropic claude fable 5 ai model · claude fable 5 for free · claude fable 5 performance | informational+branded | 100 | 500 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 65 | `claude-opus-4-8` | `/models/claude-opus-4-8` | quick | `claude-opus-4-8` | `anthropic claude-opus-4-8 model api 2026` | anthropic claude-opus-4-8 model api 2026 · anthropic api claude-opus-4-8 model id 2026 · anthropic api model id claude-opus-4-8 2026 | branded/informational (inferred) | 40 | 250 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 66 | `gpt-5.4` | `/models/gpt-5.4` | quick | `gpt-5.4` | `openai api pricing official gpt-5.4 mini nano` | openai gpt-5.4 · gpt-5.4-2026-03-05 · gpt-5.4 openai developer | informational+commercial+branded | 700 | 4,200 | 37 | 3,300 | 130 | inferred from Matching terms + verified facts | B: favorable |
| 67 | `macaron-v1-venti` | `/models/macaron-v1-venti` | quick | `macaron-v1-venti` | `N/A` | N/A | branded/informational (inferred) | 0 | 10 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 68 | `qwen3.5-397b-a17b` | `/models/qwen3.5-397b-a17b` | quick | `qwen3.5-397b-a17b` | `qwen3.5-397b-a17b api` | qwen3.5-397b-a17b-fp8 · qwen3.5 397b a17b model · qwen3.5-397b-a17b hardware requirements | informational+branded | 350 | 1,700 | N/A | N/A | 100 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 69 | `gemini-2.5-pro` | `/models/gemini-2.5-pro` | quick | `gemini-2.5-pro` | `gemini 2.5 pro api price per 1m tokens` | gemini 2.5 pro benchmarks 2025 · gemini-2.5-pro pricing · google gemini 2.5 pro announcement date | informational+commercial+branded | 150 | 700 | N/A | N/A | 3 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 70 | `qwen3.6-max-preview` | `/models/qwen3.6-max-preview` | quick | `qwen3.6-max-preview` | `N/A` | qwen3.6-max-preview features and capabilities · qwen3.6-max-preview model details · qwen3.6-max-preview alibaba | branded/informational (inferred) | 60 | 200 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 71 | `deepseek-v3.1` | `/models/deepseek-v3.1` | quick | `deepseek-v3.1` | `deepseek v3.1 api pricing per million tokens` | ollama run deepseek-v3.1 · openrouter deepseek/deepseek-chat-v3.1 · v3.1 deepseek | informational+transactional+branded | 250 | 1,300 | N/A | N/A | 300 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 72 | `deepseek-v4-pro` | `/models/deepseek-v4-pro` | full | `deepseek v4 pro` | `deepseek v4 pro pricing` | deepseek v4 pro model name api · deepseek v4 pro coding · deepseek v4 pro local | informational+commercial+transactional+branded | 2,600 | 26,000 | 52 | 1,600 | 250 | Ahrefs Questions (global filter) | C: competitive |
| 73 | `minimax-m2.5` | `/models/minimax-m2.5` | quick | `minimax-m2.5` | `minimax m2.5 monthly cost heavy use` | minimax/minimax-m2.5 · minimax m2.5 model benchmarks · minimax m2.5 ai model 2026 | informational+commercial+branded | 250 | 1,300 | N/A | N/A | 130 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 74 | `gemini-3-flash-preview` | `/models/gemini-3-flash-preview` | quick | `gemini-3-flash-preview` | `openrouter gemini 3 flash preview pricing` | gemini flash 3 preview · openrouter gemini 3 flash preview pricing · gemini 3 flash preview cost | informational+branded | 350 | 2,600 | N/A | N/A | 130 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 75 | `gemini-2.5-flash-lite` | `/models/gemini-2.5-flash-lite` | quick | `gemini-2.5-flash-lite` | `google gemini 2.5 flash-lite api pricing 2026` | google gemini 2.5 flash-lite · gemini 2.5 flash-lite model · gemini 2.5 flash-lite benchmarks | informational+transactional+branded | 150 | 1,100 | N/A | N/A | 140 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 76 | `gemini-flash-lite-latest` | `/models/gemini-flash-lite-latest` | quick | `gemini-flash-lite-latest` | `gemini-flash-lite-latest pricing` | gemini-flash-lite-latest pricing | branded/informational (inferred) | 20 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 77 | `gpt-4o-mini` | `/models/gpt-4o-mini` | quick | `gpt-4o-mini` | `openai api pricing gpt-4o mini august 2025` | gpt-4o-mini openai model documentation · openai/gpt-4o-mini · gpt-4o-mini tts | informational+commercial+branded | 900 | 3,500 | 52 | 3,900 | 80 | inferred from Matching terms + verified facts | C: competitive |
| 78 | `gemini-2.5-flash-preview-tts` | `/models/gemini-2.5-flash-preview-tts` | quick | `gemini-2.5-flash-preview-tts` | `gemini 2.5 flash preview tts pricing` | gemini 2.5 flash preview tts pricing · gemini-2.5-flash-preview-tts pricing · gemini-2.5-flash-preview-tts model name | branded/informational (inferred) | 20 | 100 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 79 | `MiniMax-H3` | `/models/minimax-h3` | full | `minimax h3` | `minimax h3 api` | minimax h3 comfyui · minimax h3 prompting guide · minimax h3 video model | informational+commercial+branded | 700 | 5,400 | 41 | N/A | N/A | Ahrefs Questions (global filter) | C: competitive |
| 80 | `glm-5` | `/models/glm-5` | quick | `glm-5` | `glm-5 pricing api cost` | free glm 5 · glm 5 parameters · glm-5 llm | informational+branded | 1,600 | 9,000 | 65 | 5,100 | 50 | inferred from Matching terms + verified facts | D: hard / authority needed |
| 81 | `gpt-4.1-mini` | `/models/gpt-4.1-mini` | quick | `gpt-4.1-mini` | `openai api pricing 2025 gpt-4.1 mini price` | openai api pricing gpt-4.1 mini nano official · gpt-4.1-mini model name · openai api pricing 2026 gpt-4.1 mini | informational+branded | 400 | 1,500 | N/A | N/A | 90 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 82 | `qwen3.5-35b-a3b` | `/models/qwen3.5-35b-a3b` | quick | `qwen3.5-35b-a3b` | `N/A` | qwen/qwen3.5-35b-a3b-fp8 · qwen3.5-35b-a3b gguf · qwen3.5-35b-a3b number of layers | informational+branded | 600 | 2,500 | 23 | 700 | 70 | inferred from Matching terms + verified facts | B: favorable |
| 83 | `claude-sonnet-4-5-20250929` | `/models/claude-sonnet-4-5-20250929` | quick | `claude-sonnet-4-5-20250929` | `anthropic api model claude-sonnet-4-5-20250929` | claude-sonnet-4-5-20250929 model · claude-sonnet-4-5-20250929-thinking-32k · claude-sonnet-4-5-20250929 model id | informational+branded | 200 | 700 | N/A | N/A | 180 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 84 | `deepseek-v4-pro-0813` | `/models/deepseek-v4-pro-0813` | quick | `deepseek v4 pro 0813` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | fact-led inferred; no Matching terms | E: KD N/A / long-tail test |
| 85 | `claude-opus-4-5` | `/models/claude-opus-4-5` | quick | `claude-opus-4-5` | `anthropic api claude-opus-4-5 model id 2025` | claude-opus-4-5-20251101-thinking-32k · claude-opus-4-5-20251101 · claude-opus-4-5-2025 | branded/informational (inferred) | 60 | 350 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 86 | `gemma-4-26b-a4b-it` | `/models/gemma-4-26b-a4b-it` | quick | `gemma-4-26b-a4b-it` | `openrouter google/gemma-4-26b-a4b-it pricing` | google/gemma-4-26b-a4b-it · cyankiwi/gemma-4-26b-a4b-it-awq-4bit · unsloth/gemma-4-26b-a4b-it-gguf | informational+commercial+transactional+branded | 100 | 450 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 87 | `seedance-2.5` | `/models/seedance-2.5` | re-audited/implemented | `seedance 2.5` | `seedance 2.5 api` · `seedance 2.5 pricing` | release date · ai video · video generator · text-to-video · image-to-video · prompts · cost/price/free (需求信号) | informational+commercial+transactional+branded | 3,800 (US proxy) | 17,000 | 25 | N/A | N/A | Ahrefs Questions (global filter; GSV 10–70) + verified facts | B: favorable head term; API/pricing/use-case long tails first |
| 88 | `grok-imagine-image-pro` | `/models/grok-imagine-image-pro` | quick | `grok-imagine-image-pro` | `grok-imagine-image-pro api` | grok-imagine-image-pro api · grok imagine image pro xai · grok imagine image pro model | branded/informational (inferred) | 30 | 90 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 89 | `deepseek-v3` | `/models/deepseek-v3` | quick | `deepseek-v3` | `N/A` | deepseek v3 benchmark · deepseek v3/r1 · deepseek-v3 pricing per 1m tokens | informational+transactional+branded | 400 | 3,300 | N/A | N/A | 200 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 90 | `minimax-m2.7` | `/models/minimax-m2.7` | quick | `minimax-m2.7` | `N/A` | minimax m2.7 coding benchmark swe-bench 2026 · minimax m2.7 llm model · minimax m2.7 model specs | informational+commercial+transactional+branded | 250 | 1,300 | N/A | N/A | 170 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 91 | `gemini-3.1-pro-preview` | `/models/gemini-3.1-pro-preview` | quick | `gemini-3.1-pro-preview` | `gemini 3.1 pro preview pricing api` | google gemini 3.1 pro preview · gemini 3.1 pro preview developer · gemini-3.1-pro-preview api | informational+commercial+transactional+branded | 250 | 1,300 | N/A | N/A | 130 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 92 | `gemini-3.6-flash` | `/models/gemini-3.6-flash` | quick | `gemini-3.6-flash` | `gemini 3.6 flash lite pricing` | gemini 3.6 flash benchmark · gemini 3.6 flash cost · gemini 3.6 flash lite pricing | branded/informational (inferred) | 20 | 150 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 93 | `gpt-5.5` | `/models/gpt-5.5` | quick | `gpt-5.5` | `openai gpt-5.5 pricing tokens` | openai gpt 5.5 model · gpt 5.5 pro benchmarks · openai gpt-5.5 features | informational+branded | 800 | 5,600 | 57 | 13,000 | 60 | inferred from Matching terms + verified facts | C: competitive |
| 94 | `nano-banana-pro-preview` | `/models/nano-banana-pro-preview` | quick | `nano-banana-pro-preview` | `N/A` | gemini-3-pro-image-preview-2k (nano-banana-pro) · gemini-3-pro-image-preview (nano-banana-pro) · gemini-3-pro-image-preview nano banana pro | branded/informational (inferred) | 0 | 10 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 95 | `veo-3.1-generate-preview` | `/models/veo-3.1-generate-preview` | quick | `veo-3.1-generate-preview` | `veo-3.1-generate-preview pricing` | veo-3.1-generate-preview · veo-3.1-fast-generate-preview · veo-3.1-lite-generate-preview | branded/informational (inferred) | 20 | 100 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 96 | `glm-5-turbo` | `/models/glm-5-turbo` | quick | `glm-5-turbo` | `N/A` | glm turbo 5 · glm-5 turbo zhipu ai · glm 5 turbo 和 glm 5 的 区别 | informational+commercial+transactional+branded | 200 | 900 | N/A | N/A | 70 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 97 | `qwen3.5-27b` | `/models/qwen3.5-27b` | quick | `qwen3.5-27b` | `qwen3.5-27b api` | qwen3.5-27b-claude-4.6-opus-reasoning-distilled · qwen3.5 27b model · qwen3.5-27b benchmarks | informational+branded | 300 | 1,100 | N/A | N/A | 90 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 98 | `qwen3.5-plus-2026-02-15` | `/models/qwen3.5-plus-2026-02-15` | quick | `qwen3.5-plus-2026-02-15` | `N/A` | qwen: qwen3.5 plus 2026-02-15 | branded/informational (inferred) | 10 | 10 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 99 | `claude-sonnet-4-5` | `/models/claude-sonnet-4-5` | quick | `claude-sonnet-4-5` | `claude-sonnet-4-5 api` | claude-sonnet-4-5-20250929 model id · claude-sonnet-4-5 api · claude-sonnet-4-5-20250514 | branded/informational (inferred) | 80 | 250 | N/A | N/A | N/A | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 100 | `claude-sonnet-4-6` | `/models/claude-sonnet-4-6` | quick | `claude-sonnet-4-6` | `anthropic claude-sonnet-4-6 model name api 2026` | anthropic claude-sonnet-4-6 model name api 2026 · anthropic api model id claude-sonnet-4-6 2026 · anthropic claude-sonnet-4-6 model id api 2026 | informational | 150 | 600 | N/A | N/A | 140 | inferred from Matching terms + verified facts | E: KD N/A / long-tail test |
| 101 | `macaron-v1-tall` | `/models/macaron-v1-tall` | quick | `macaron v1 tall` | `N/A` | N/A | N/A | N/A | N/A | N/A | N/A | N/A | fact-led inferred; no Matching terms | E: KD N/A / long-tail test |

`*SV/CPC` 是 `country=us` proxy 字段；`GSV` 是全球字段。对于没有 exact Overview row 的模型，N/A 是真实数据缺口。

## 快速流程页面的主题与 FAQ 规则

快速页面不是复制一套正文：

1. Title/H1 使用该模型的 canonical 名称 + 一个真实用途（API、image、video、audio、embedding 或 coding），不要只换模型名。
2. Opening answer 先回答“它是什么、如何访问、已验证的一个差异”；没有 Ahrefs Questions 数据时，FAQ 问题标成 inferred，不把 Matching terms 冒充 Questions volume。
3. 共享版型可以保留：workbench、health、providers、related、FAQ、CTA、schema 和内部链接结构；必须重写的是 opening、price explanation、use-case examples、API/access、FAQ answers。
4. 每页至少 5–7 个 target-specific FAQ；兄弟页去重检查要比较 question wording、answer wording 和事实，不仅比较模型名。
5. 含 `free`、`unlimited`、`NSFW`、`local`、`download`、`release/news`、竞品比较的词只作为需求信号；没有产品事实或独立内容页时排除或另页。

### 当前发现的技术/内容风险（实施后复核）

- `MiniMax-H3` 的大小写 alias 已在路由层 redirect 到 `/models/minimax-h3`，并从 sitemap 使用 canonical slug；仍需在测试环境验证 308/307、canonical 与 hreflang 的最终 HTML，避免自竞争。
- 5 个主推页已提供模型专属 FAQ 草案，但回归测试必须保证每页至少 6 个问题、问题/答案不回退到通用两问，并检查兄弟页的完整句子去重。
- 动态目录页与显式静态配置是两条解析路径；必须分别验证 `getModelLandingConfigForPricingModel` 和静态 route 分支，确保 GPT-5.6 Sol、DeepSeek V4 Pro 不落回 family generic 文案。
- DeepSeek V4 Pro 的 tiered expression 与 GPT Image 2 的 token/image 维度价格不能被通用静态价格组件折成一个数字；页面上要保留计费维度和时间条件，并将报价标成带日期的 Flatkey 目录快照。
- 未核实的 local/open-source/ComfyUI、免费、无限、质量/benchmark 优势不能写入正文；Ahrefs 关键词只代表需求信号，不是产品事实或排名承诺。

## 全球与巴西的处理边界

本轮按你的最新决定只做全球研究，因此 GSV 是主指标，不把巴西 SV 当全球量。截图中的巴西 `SV=600` 对应英文精确查询 `seedance 2.5`，只能记为 Brazil-English 观察值，不能当作 `pt-BR` 的搜索量、KD 或排名难度。若后续要专门推巴西，建议只对主词及 API/pricing/use-case 词再跑 `country=br`，同时用葡萄牙语词（如 `gerador de vídeo Seedance 2.5`）单独采集并增加 pt-BR 标题、描述、FAQ 和内部链接；不要用本报告的 US proxy SERP 直接判断巴西排名难度。

## 实施与后续验证清单

- [x] Seedance 2.5 已按独立复审更新 opening、关键词布局、pricing/API/FAQ 和 JSON-LD 价格边界；已补充 10 个 locale 的事实文案，仍需在测试环境验证 SSR、hreflang、canonical 与交互。
- [ ] 样式重构完成后，确认不改变既有路由、workbench、pricing/health 数据和 schema 结构。
- [x] 5 个完整页已接入专属 hero/API/pricing/FAQ 配置，且配置/组件回归、本地 build/SSR 检查已通过；待完成真实环境浏览器、locale 和最终事实验收后再发布。
- [ ] 按机会与事实风险复核顺序：Kimi K3 → GPT Image 2 → DeepSeek V4 Pro → GPT-5.6 Sol → MiniMax H3；该顺序是执行队列，不是 Google 排名保证。
- [ ] 对 95 个 quick 页按矩阵批量生成 target-specific opening/H2/FAQ 草案，再逐页核验产品事实；不直接批量替换模型名。
- [x] 代码层统一 MiniMax H3 canonical（`/models/minimax-h3`；大小写 alias redirect、sitemap canonical）；[x] 已验证本地 metadata、SSR HTML 与 alias 响应头；[ ] 真实环境 FAQ schema、hreflang、内部链接、CTA 与浏览器视觉检查仍待完成。
- [ ] 发布前重新采集 Search Console/排名数据；Ahrefs 的 GSV/KD 只用于机会判断，不作为排名承诺。

## 研究文件与审计说明

- 模型清单：[docs/seo-model-page-inventory.md](./seo-model-page-inventory.md)
- 本报告的最终矩阵数据来自临时缓存：Global Overview、最终 `global_volume >= 10` Matching terms、主推 Questions，以及先前保存的 SERP JSON。Seedance 2.5 复审复用了已保存数据，未重复调用 Ahrefs；API key 未写入任何文件或报告。
- Skill 已包含本次的全球口径、US proxy 标注、full/quick 深度、FAQ 去重、共享版型/独特内容、批量矩阵和简报流程；Seedance 2.5 的实施审计记录在本节，后续页面应沿用同一事实核验与反同质化检查。
