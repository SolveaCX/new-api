# 模型详情页权威资料与内容审计

生成日期：2026-08-28  
范围：官网公开模型详情页（`website/`）共 101 页  
市场口径：全球需求以 Ahrefs Global Search Volume（GSV）为主；Matching terms、Questions 和 SERP 使用 `country=us` 的 US-English proxy。截图中的 Brazil `SV` 不等于 pt-BR 搜索量。

## 结论先看

本次对全部 101 个详情页做了内容和事实边界检查，并把能够从官方资料确认的字段补回页面内容。当前页面仍保持同一版型，但按模型类型和可核实字段呈现不同内容：

| 页面类型 | 目录数量 | 本地示例 | 说明 |
|---|---:|---|---|
| Video 视频 | 11 | [`/models/seedance-2.5`](http://localhost:4000/models/seedance-2.5) | 保留视频工作台、参考素材、时长/比例、价格、Prompt 和 API |
| Image 图像 | 9 | [`/models/gpt-image-2`](http://localhost:4000/models/gpt-image-2) | 使用图像生成/编辑、尺寸、质量、格式、背景和 moderation 主题 |
| Text/Chat 文本/对话 | 74 | [`/models/kimi-k3`](http://localhost:4000/models/kimi-k3) | 使用上下文、输入模态、兼容 endpoint、计费和集成主题 |
| Audio 音频 | 6 | [`/models/gemini-2.5-flash-tts`](http://localhost:4000/models/gemini-2.5-flash-tts) | 使用音频 API、计费单位和输入/输出边界，不展示无关的 Prompt 工作台 |
| Embedding 向量 | 1 | — | 不属于本次要求的四类示例，保留目录型 API 内容 |

6 个重点页（GPT-5.6 Sol、GPT Image 2、Kimi K3、DeepSeek V4 Pro、MiniMax-H3、Seedance 2.5）采用完整/独立复审内容；其余 95 页使用 Global + Matching terms 快速流程和目录字段生成的模型专属内容。这里的“已接入”只表示本地代码和 SSR 内容已完成，不代表 Google 已收录或已经取得排名。

## 权威资料核验矩阵

以下来源只用于支持页面事实；Flatkey 的结算价格始终以页面实时目录为准，不能把 provider 直价替换成 Flatkey 价格。

| 模型/页面 | 权威来源 | 已确认并用于页面的事实 | 明确不外推的内容 |
|---|---|---|---|
| Seedance 2.5 | [ByteDance 模型页](https://seed.bytedance.com/en/seedance2_5)、[官方发布文章](https://seed.bytedance.com/en/blog/one-take-creation-flexible-referencing-introducing-seedance-2-5) | 音视频联合生成；单次最长 30 秒并可多轮延长；参考图片/视频/音频；首帧/尾帧和时间点编辑；绿幕、镜头运动和 blocking 等参考控制；页面目录的 `/v1/videos`、4–30 秒和参考上限 | 不把官方即将开放的 provider API 文字写成 Flatkey 已开通；不承诺免费或无限生成 |
| GPT Image 2 | [OpenAI 模型文档](https://developers.openai.com/api/docs/models/gpt-image-2)、[OpenAI 官方费率说明](https://help.openai.com/en/articles/20001415) | 图像生成/编辑模型；文本和图片输入、图片输出；`/v1/images/generations` 与 `/v1/images/edits`；尺寸、质量、格式、背景和 moderation 是请求控制项；官方直价按文本/图片输入和图片输出 token、尺寸/质量计算 | 页面显示的是 Flatkey 目录费率，不把 OpenAI 直价当作 Flatkey 结算；不承诺音频/视频、透明背景或固定每张图价格，除非当前路由明确提供 |
| Kimi K3 | [Kimi API 概览](https://www.kimi.ai/help/kimi-api/api-overview)、[Kimi API 价格](https://www.kimi.ai/help/kimi-api/api-pricing) | API key、Chat Completions、多轮对话、文件解析和 web search；Kimi K3 的 1M 上下文；按 token 计费，web search 另计调用费，缓存命中有折扣；页面区分 Moonshot 上游能力与 Flatkey hosted route | 不把 Kimi 会员权益当成 API 免费额度；硬件/VRAM、下载权和本地部署须以具体上游许可和权重页面为准 |
| DeepSeek V4 Pro | [DeepSeek 官方价格与模型文档](https://api-docs.deepseek.com/quick_start/pricing/) | `DeepSeek-V4-Pro-0813`；1M context、最大 384K 输出；JSON、tool calls、Responses/Anthropic API 支持；UTC peak/off-peak 的 cache-miss、cache-hit、output 费率与时间段；Flatkey 页面展示文本/文件字段和两个兼容路线 | 不把目录 `file` 字段写成原生 vision；不发布 benchmark 排名；不把上游开源公告当成 Flatkey 已提供本地权重 |
| MiniMax-H3 | [MiniMax 视频生成文档](https://platform.minimax.io/docs/guides/video-generation)、[MiniMax H3 开源公告](https://www.minimax.io/news/minimax-h3-open-source) | 文本/图片/视频/音频输入；768P/2K；4–15 秒整数时长；常用/自适应比例；最多 9 图片、3 视频、3 音频或 12 个混合文件；上游 prompt 7,000 字符；异步 task/poll/download；H3 权重和 ComfyUI/local 选项在上游另行公布 | 页面以 Flatkey `/v1/videos` hosted route 为准；不承诺 Flatkey 已打包本地权重、硬件配置或完全等同的上游字段 |
| 普通目录页 | Flatkey 当前 pricing/model catalog 快照；对应 provider 官方文档需逐页复核 | provider、model ID、endpoint、模态、上下文、计费维度和发布日期只在目录有值时呈现；无值显示 `N/A`/未公布 | 不用模型名推断 benchmark、free/unlimited、下载、NSFW、原生模态、provider SLA 或本地运行 |
| 音频示例 | [Google Gemini 2.5 Flash TTS 文档](https://ai.google.dev/gemini-api/docs/models/gemini-2.5-flash-preview-tts)、[Gemini TTS 指南](https://ai.google.dev/gemini-api/docs/speech-generation) | Google 文档确认文本输入、音频输出、单/多说话人 TTS，以及风格和节奏控制；示例页仍以 Flatkey 目录 endpoint/价格为准 | `sonilo-video-to-music` 是 Flatkey 目录中的视频转音乐路线，当前未找到可公开核验的上游规格；不把它写成 Google 或其他 provider 的产品 |

## 四类页面的内容架构

同类型页面共享结构和交互组件，但每页必须替换事实、关键词、示例和 FAQ。以下是本次审计后可复用的四类结构：

### Video 视频页

1. H1：模型名称 + `AI video generator`/`video API` 的自然表达。
2. Opening：provider、文生视频/图生视频、参考素材、时长/分辨率和 Flatkey endpoint。
3. Pricing：按秒、分辨率、参考视频秒数或目录实际单位展示；不把某一档价格写成所有请求的固定价。
4. Capability H2：文本/图像工作流、参考控制、时长/比例/分辨率、音频或水印等真正适用的字段。
5. Compare：相邻版本、托管与本地边界或已核实的迁移字段；未知值写明未核实。
6. Prompt library：只保留视频模型适用的 6 个工作流示例，并让每个模型的题材和提示词不同。
7. API + FAQ：model ID、endpoint、异步 task 或 content[] 约束，以及模型专属问题。

### Image 图像页

1. H1/opening：图像生成/编辑任务、provider、图像输入输出和 Flatkey 路由。
2. Pricing：token/图像输入、输出尺寸和质量维度；不写“每张固定价格”。
3. Capability H2：数量、尺寸、质量、格式、背景/moderation 五个代表性控制项；不把这些卡片误命名为通用功能介绍。
4. Compare：上一代图像模型的 endpoint、尺寸、格式和迁移字段；质量优劣不做未经验证的排名。
5. API + FAQ：generations/edits、model ID、透明背景边界、免费/免注册边界。
6. Prompt library：保留图像页的 6 个提示词卡片，但示例针对产品图、广告、分镜等图像任务。

### Text/Chat 文本页

1. H1/opening：模型名称、文本/文件/视觉等已核实模态、上下文窗口和 API 入口。
2. Pricing：输入、输出、缓存或 UTC 时段费率；不把不同 provider 的费率拼成统一价格。
3. Capability H2：context、输入模态、兼容 endpoint、工具/agent 或文件工作流。
4. Compare：旧版/相邻模型的 ID、上下文、endpoint 和已核实字段；benchmark/质量不做无证据排序。
5. API + FAQ：model ID、兼容客户端、API key、上下文/文件限制、local/open-source 边界。
6. Prompt library：文本模型不展示无关的视频/图像 Prompt 工作台；用 API/集成说明承接搜索意图。

### Audio 音频页

1. H1/opening：音频生成、TTS、音乐或视频转音乐任务，provider 和真实 endpoint。
2. Pricing：按音频输入/输出 token、秒或请求显示当前目录单位。
3. Capability H2：说话人/语音、风格与节奏、音频格式、媒体输入或 speech-preserving 等已核实字段。
4. Compare：不同音频路线的输入输出、延迟、格式和计费维度；未知字段保持未核实。
5. API + FAQ：调用方式、model ID、是否异步、输入输出限制；不强行生成视频 Prompt 卡片。
6. Prompt library：音频页不展示与任务无关的图像/视频 Prompt 库，避免同质内容和意图错配。

## 关键词与内容覆盖摘要

关键词仍然按搜索意图聚类，而不是“一条 H2 放一个词”。主推页的全球需求快照如下：

| 页面 | 主关键词 | GSV | KD | 页面承接 |
|---|---|---:|---:|---|
| Kimi K3 | `kimi k3` | 121K | 70 | H1、opening、pricing、1M context、API、local/open-source 边界、FAQ |
| GPT Image 2 | `gpt image 2` | 56K | 58 | image generator、image API、尺寸/质量/格式、token/image pricing、edits FAQ |
| DeepSeek V4 Pro | `deepseek v4 pro` | 26K | 52 | 双兼容 API、UTC pricing、1M context、文件输入、V4 Flash 对比 |
| Seedance 2.5 | `seedance 2.5` | 17K | 25 | video API、4–30 秒、参考素材、音频、pricing、prompt、行业工作流 |
| GPT-5.6 Sol | `gpt 5.6 sol` | 13K | 22 | API、pricing、长上下文、modalities、系列对比 |
| MiniMax H3 | `minimax h3` | 5.4K | 41 | video API、768P/2K、4–15 秒、参考上限、ComfyUI/local 边界 |

完整 101 行关键词、页面位置、FAQ 来源、KD/GSV 和状态见：

- [关键词+内容覆盖 CSV（中英文表头）](./seo-model-page-keyword-content-coverage.csv)
- [101 页内容覆盖 JSON](./seo-model-page-content-coverage.json)
- [101 页关键词矩阵 JSON](./seo-model-page-keyword-matrix.json)
- [研究参数与 Ahrefs 用量清单](./seo-model-page-research-manifest.json)
- [此前完整实施报告](./seo-model-page-final-report.md)

## 反同质化检查

- 共享：workbench、health、pricing/compare/API/FAQ 的版型、schema 形状、related-model 组件和 CTA。
- 必须唯一：页面 title、meta description、H1/opening、主副关键词簇、provider/model ID、价格维度、能力卡正文、Prompt 示例、FAQ 问法与答案、内链锚文本。
- 101 页当前均有模型专属 `landingContent` 和可见 FAQ；快速页 FAQ 来源标为 `Matching terms + verified facts`，不会冒充 Ahrefs Questions 数据。
- `Input`/`Output` 只作为交互工作台标签，不作为 SEO H2；顶部不再显示通用 `Capabilities` eyebrow。
- `free`、`unlimited`、`download`、`NSFW`、竞品和 release/news 词只作为需求信号；缺乏事实时不转成产品承诺。
- MiniMax-H3 的分辨率、整数时长、参考文件上限和 prompt 字符上限已补入 10 个 locale 的重点页内容，并保留“上游字段可能与 Flatkey 路由不同”的边界说明。

## 验证与剩余工作

已完成的本地验证：

- 目标回归测试：`43 tests / 820 assertions`，0 failures。
- `bun run typecheck`：通过。
- `bun run lint -- --no-cache`：0 errors；仅保留既有 `next/image` warnings。
- `bun run build`：Next.js production build 通过。
- 101 页配置审计：每页有 landingContent、模型专属 FAQ、意图型 H2；description 不超过 160 字符；内容签名不重复。

发布前仍需：

1. 推送官网 `staging`，按上面四个 URL 和 6 个重点页做桌面/平板/手机浏览器抽查。
2. 发布前再次核对实时 provider、endpoint、价格和可用地区；价格变动以目录实时值为准。
3. 音频页中 `sonilo-video-to-music` 的上游公开规格仍未核实，不能把目录字段写成 provider 官方保证。
4. 上线后用 Search Console 观察收录、展示、平均排名和查询词；在真实数据出现前，不宣称“已抢占排名”。

本次只涉及官网 `website/` 与 `docs/`，不改变 Go router、relay、billing 或 `/v1` 运行路径；Router deploy：**not required**。官网 staging/website 部署仍需按项目发布流程执行。
