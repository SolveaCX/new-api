# 模型详情页 SEO 优化执行简报

日期：2026-08-27

## 当前结果

- 模型目录共 101 个页面，已经全部在本地代码中接入模型专属 SEO 内容包。
- 5 个主推模型走完整 Ahrefs 流程并使用策划内容；Seedance 2.5 保留独立复审内容；其余 95 个使用 Global + Matching terms 快速流程。
- 每页都有模型相关 Title、Meta、H1/首段、关键词聚类 H2、API/价格/能力/用途内容、FAQ、schema、canonical/hreflang 和内链。
- 共享版型保留，但正文、价格维度、API 路由、能力点、FAQ 和事实边界按模型目录动态生成，避免仅替换模型名造成同质内容。
- 当前状态是“本地已接入，待 staging/浏览器/事实验收与 Google 索引观察”，不是“已取得排名”。

## 主推关键词机会

| 模型 | 主关键词 | 全球 GSV | KD | 页面重点 |
|---|---|---:|---:|---|
| Kimi K3 | `kimi k3` | 121K | 70 | API、pricing、1,048,576 context、file、hosted/local 边界 |
| GPT Image 2 | `gpt image 2` | 56K | 58 | image generator、image API、尺寸/质量/格式、token/image pricing |
| DeepSeek V4 Pro | `deepseek v4 pro` | 26K | 52 | 双兼容 API、UTC pricing、context/file、V4 Flash 对比 |
| GPT-5.6 Sol | `gpt 5.6 sol` | 13K | 22 | API、pricing、长上下文、modalities、系列对比 |
| MiniMax H3 | `minimax h3` | 5.4K | 41 | video API、ComfyUI 边界、prompt、768P/2K、4–15 秒 |
| Seedance 2.5 | `seedance 2.5` | 17K | 25 | video API、480p/720p、4–30 秒、reference/audio、行业工作流 |

## 执行口径

1. 全球主指标使用 Ahrefs GSV；US `SV/CPC/SERP` 只作为接口所需的 US-English proxy。
2. 所有 H2 按搜索意图簇写入 API、价格、能力、用途、比较、集成或 FAQ；不把 `Input`/`Output` 这种交互标签当 SEO H2。
3. Meta description 由统一长度限制处理，目标不超过 160 字符；产品名、provider、任务、价格/单位和 context 只在有事实时出现。
4. 图片/视频页保留 prompt library；文本、embedding、音频页标记为不适用，避免为了凑关键词制造无关段落。
5. `free`、`unlimited`、`local`、`download`、`NSFW` 和竞品词没有事实支撑时不会写成产品承诺。

## 已验证与待办

已验证：41 个目标测试全部通过、typecheck 通过、lint 无 error、Turbopack production build 通过、本地 SSR 和 View API 行为抽查通过。  
待办：推送官网 staging、浏览器视觉检查、发布前事实复核、Search Console 收录/排名观测。

本次只改官网 `website/`，因此 **Router deploy：not required**；不涉及 Go router、relay、billing 或 `/v1`。

详细 H2、FAQ、事实基线、Ahrefs 单位、原始缓存和每个模型的主副关键词请查看 [`seo-model-page-final-report.md`](./seo-model-page-final-report.md)、[`seo-model-page-keyword-content-coverage.csv`](./seo-model-page-keyword-content-coverage.csv) 和 [`seo-model-page-content-coverage.json`](./seo-model-page-content-coverage.json)。
