# 模型详情页 SEO 优化最终报告

生成日期：2026-08-27  
范围：官网公开模型详情页（`website/`）共 101 个模型

## 结论

101 个模型详情页已经在本地代码中接入模型专属内容包和 SEO metadata：

- 5 个主推模型（GPT-5.6 Sol、GPT Image 2、Kimi K3、DeepSeek V4 Pro、MiniMax H3）使用完整研究结果和人工策划内容包。
- Seedance 2.5 保留此前独立复审和已验证的专属内容，不重复调用 Ahrefs。
- 其余 95 个模型使用由 Global + Matching terms + 当前定价目录字段生成的快速内容包。
- 本地测试、typecheck、lint 和 production build 已通过；staging 浏览器验收、最终事实复核和 Google 索引观察仍待完成。
- 报告不把“内容已接入”说成“已取得 Google 排名”。

## 关键词需求快照

本轮以 Ahrefs Global Search Volume（GSV）作为全球需求主指标。Matching terms、Questions 和 SERP 接口需要国家参数，因此已保存数据中的 `SV/CPC/意图/SERP` 使用 `country=us` 作为 US-English proxy；它们不能直接代表巴西葡萄牙语量级。

| 模型 | 主关键词 | GSV | KD | 优先承接的支持词 |
|---|---|---:|---:|---|
| Kimi K3 | `kimi k3` | 121K | 70 | `kimi k3 pricing`、`kimi k3 api`、context/file |
| GPT Image 2 | `gpt image 2` | 56K | 58 | `gpt image 2 api`、`gpt image generator`、size/quality/format |
| DeepSeek V4 Pro | `deepseek v4 pro` | 26K | 52 | pricing、API/model name、context/file、local boundary |
| GPT-5.6 Sol | `gpt 5.6 sol` | 13K | 22 | pricing、API、context/modalities、vs 5.5/Terra/Luna |
| MiniMax H3 | `minimax h3` | 5.4K | 41 | API、ComfyUI、prompt guide、768P/2K、4–15s |
| Seedance 2.5 | `seedance 2.5` | 17K | 25 | release date、API、pricing、video、prompt、vs Kling |

KD 是 Ahrefs 估计值；`N/A` 仍表示没有返回难度，不会被改写成 0。

## 页面关键词布局

关键词不是机械地“一条 H2 放一个词”，而是按照同一搜索意图聚类：

| 页面位置 | 承接内容 | 实施口径 |
|---|---|---|
| Title | 主词 + API/生成类型 + pricing/FAQs | `buildModelLandingMetadata` 统一生成；描述由 `limitSeoDescription` 控制在 160 字符以内。 |
| Meta description | 模型类型、provider、任务、当前目录价格/计费单位、context 或设置 | 只使用目录或已核实事实；不承诺免费、无限、benchmark 或 provider 原生能力。 |
| H1 / opening | 主词的直接定义和使用答案 | 使用真实模型名、provider、路由、模态和价格维度。 |
| H2 | API、价格、能力、用途、对比、集成、FAQ 等意图簇 | 每个 H2 都围绕关键词簇和页面任务；`Input`/`Output` 仍是交互面板标签，不作为 SEO H2。 |
| API | API/action 词、model ID、endpoint、请求方式 | 文本模型显示 `/v1/chat/completions` 或 `/v1/messages`；图片、视频、音频按实际路由显示。 |
| Pricing | pricing/price/cost 与真实结算维度 | 读取当前 Flatkey 目录价格；按 token、秒、请求或图像输入维度说明，不拼造统一单价。 |
| Capabilities | 该模型的模态、上下文、尺寸、时长、格式或控制项 | 顶部不再显示通用 “Capabilities” eyebrow；功能点按模型类型和目录字段生成。 |
| Compare | 旧版本/相邻模型/托管与本地边界 | 只比较已核实字段；未知项明确写 Unknown/Not verified，不写质量排名。 |
| Prompt library | 图片/视频的 prompt 与用途示例 | 图片和视频保留；文本、embedding、音频页标记 `not_applicable`，避免无关内容。 |
| FAQ | Questions（主推）或 Matching terms（快速）对应的问题 | 每页都有 FAQ；主推页 6 个专属问题，Seedance 2.5 为 9 个，快速页为 4 个。 |
| Schema / canonical / hreflang | 页面实体、FAQ、稳定 slug 与多语言 URL | SSR 生成 JSON-LD、canonical 和 locale alternates；MiniMax 大小写 alias 重定向至 `/models/minimax-h3`。 |

## 反同质化设计

共享页面版型本身不会造成重复内容；重复风险来自正文、FAQ 和事实也完全相同。本轮用以下边界控制：

1. 共用 workbench、health、related、CTA、schema 组件，但每个页面的 `landingContent` 都绑定当前模型的名称、provider、endpoint、模态、价格维度、context 和目录分类。
2. 主推页使用人工策划的模型事实、对比表、API 说明、用例和 FAQ；不会用模型名替换通用句子。
3. 快速页也生成模型名和关键词簇对应的 opening、H2、价格/能力/API 段落和 FAQ；没有价格或事实时明确写“目录未提供”，不猜测。
4. 关键词自然变体只放在最匹配的区域，不在每个 H2、图片 alt 或按钮里重复堆砌。
5. `free`、`unlimited`、`local`、`download`、`NSFW`、竞品和 release/news 词只作为需求信号；没有事实支撑时以边界 FAQ 或另页承接。

## Ahrefs 研究与用量

研究数据已冻结在现有缓存和报告中，本次页面代码实施不新增 Ahrefs 请求：

- Global Overview：93 exact rows，另有 `minimax-h3` normalized alias。
- 补充商业/用途 Overview：14 rows，756 units。
- Matching terms：100 个页面、1,757 rows、95,128 units。
- 主推 Questions：5 个页面、110 rows、5,940 units。
- 主推 SERP：20 个查询、201 rows、9,050 units。
- 研究数据集估算：116,760 units；账户快照为 workspace 210,719 / 400,000 units。

完整原始路径、字段和国家边界见 [`seo-model-page-research-manifest.json`](./seo-model-page-research-manifest.json) 与 [`seo-model-page-keyword-research-brief.md`](./seo-model-page-keyword-research-brief.md)。

## 代码实施

- `website/src/lib/model-landing.ts`：为没有人工策划包的模型生成模型专属 landingContent、价格行、API 路由、SEO metadata、FAQ 和 locale SEO；主推页使用 curated overrides。
- `website/src/app/(en)/models/[slug]/page.tsx` 与 `website/src/app/[locale]/models/[slug]/page.tsx`：metadata 按解析出的页面类型覆盖模糊 provider endpoint，避免图片页被误标成 text/chat。
- `website/src/components/model-landing-page.tsx`：顶部 Quick Start 旁增加 View API；修复页面在顶部且旧 hash 为 `#api` 时错误选中 API tab；移除顶部通用 Capabilities eyebrow；FAQ 标题支持模型专属标题。
- `website/src/app/globals.css`：View API outline button 和移动端 hero action 样式。

## 验证结果

- 目标回归：**41 tests / 773 assertions，0 failures**。
- `bun run typecheck`：通过。
- `bun run lint -- --no-cache`：0 errors；仅 2 个既有 `online-home-page.tsx` `next/image` warnings。
- `bun run build`：Next.js 16.2.9 Turbopack production build 通过。
- 101 模型配置审计：101/101 有 landingContent，内容签名 101 个均不重复；每页至少 6 个意图型 H2 区域；没有单独使用 `Input`/`Output` 的 SEO H2；所有 locale metadata description 均不超过 160 字符。
- 本地 SSR 抽查通过：GPT-5.6 Sol、GPT Image 2、Kimi K3、DeepSeek V4 Pro、MiniMax H3、Seedance 2.5、Sonilo、gpt-4.1-mini、图片/视频快速页和 `/pt/` 路径均返回 HTTP 200，包含 canonical、FAQ、模型专属 H1/H2/API 文本；图片页 metadata 已按 image generation 任务词修正，Sonilo 的多语言页仍按音频任务呈现。
- View API：顶部链接为 `#api`，点击后滚动到 API 区域；顶部旧 hash 不会永久选中 API，Quick Start 可以回到工作台/性能区。

## 全球与巴西

英文根路径承接全球 GSV；葡语路径提供自然的 `API`、`preços`、`gerador de imagens/vídeo`、`como usar` 等表达。截图中的 Brazil `SV` 是英文 seed 在巴西的查询结果，不是葡萄牙语关键词量，也不能直接当作巴西 KD。若要进一步做巴西市场，应另采集葡萄牙语 seed + `country=br`，再单独评估 pt-BR title、FAQ 和内链。

## 上线建议与剩余事项

**Router deploy：not required。** 本次改动只涉及 `website/` 公开站内容、metadata、SSR 页面和官网组件，不改变 Go router、relay、billing 或 `/v1` 运行路径。其他目标：官网 staging/website deployment；不涉及 `newapi-router`、`newapi-console` 或 Terraform。

上线前仍需：

1. 将官网变更合并/推送到 `staging`，在测试环境逐页抽查 6 个主推页和代表性的图片、视频、音频、文本页。
2. 对当前目录价格、provider、endpoint、模态和 locale 文案做发布前事实复核。
3. 发布后用 Search Console 观察收录、展示、平均排名和查询词；只有产生真实数据后，才能判断是否抢占到目标关键词。

## 关联文件

- [`seo-model-page-executive-brief.md`](./seo-model-page-executive-brief.md)：管理层简报。
- [`seo-model-page-keyword-content-coverage.csv`](./seo-model-page-keyword-content-coverage.csv)：中英文表头的 101 行关键词/内容覆盖表。
- [`seo-model-page-content-coverage.json`](./seo-model-page-content-coverage.json)：机器可读的 101 页内容覆盖状态。
- [`seo-model-page-keyword-matrix.json`](./seo-model-page-keyword-matrix.json)：机器可读的主词、副词、GSV/KD 和布局策略。
- [`seo-model-page-research-manifest.json`](./seo-model-page-research-manifest.json)：研究、Ahrefs、缓存和验证参数。
- [`seo-model-page-optimizer.zip`](../seo-model-page-optimizer.zip)：可复用的 SEO 页面优化 skill。
