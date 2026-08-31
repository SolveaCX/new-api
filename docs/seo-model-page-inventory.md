# 模型 SEO 优化清单（研究与实施审计）

> 批量数据快照：2026-08-25；Seedance 2.5 独立复审：2026-08-26。来源：官网公开定价/模型目录接口（101 个模型）与已保存 Ahrefs 数据。本清单标注研究批次和实施审计状态，不把关键词机会写成排名保证。

研究结果： [完整关键词/内容简报](./seo-model-page-keyword-research-brief.md) · [关键词+内容覆盖总表](./seo-model-page-keyword-coverage.md) · [CSV 表格](./seo-model-page-keyword-content-coverage.csv) · [可机器读取的 101 页矩阵](./seo-model-page-keyword-matrix.json) · [结构化内容覆盖 JSON](./seo-model-page-content-coverage.json) · [研究参数清单](./seo-model-page-research-manifest.json)

## 执行规则

- **完整流程（6 个主推模型）**：Global Overview、Matching terms、Questions、3 组 SERP、完整关键词/内容报告；其中 5 个已纳入批量研究，Seedance 2.5 走独立复审以避免重复扣费。
- **独立复审（1 个）**：Seedance 2.5 从批量请求中排除以避免重复扣费，随后复用已保存数据完成关键词、事实、计费和页面内容审计；不重复调用 Ahrefs。与上项合计 6 个主推模型。
- **快速流程（其余 95 个）**：Global Overview + Matching terms，已输出关键词、搜索意图、页面主题/H2 布局，以及每个模型专属的 FAQ 问题与答案要点；不额外调用 Ahrefs Questions/SERP。当前 95 页也已接入由目录字段驱动的模型专属内容包。
- Global Volume 是主要需求指标；Matching terms/SERP 需要国家参数，默认使用美国英文库作为明确标注的代理，不把美国 SV 当作全球 SV。
- 当前状态：**批量 Ahrefs 研究与报告已完成；101 个页面均已接入模型专属内容包；Seedance 2.5 保留独立复审内容，5 个主推页使用人工策划包，其余 95 页使用 Global + Matching 驱动的快速包。当前本地测试已通过，仍待 staging/浏览器/最终事实验收与 Google 索引观察**。任何页面发布前都要复核 canonical、hreflang、schema 和事实。
- FAQ 规则：101 个可索引详情页都必须有 FAQ；快速流程的 FAQ 由 Matching terms + 已核实产品事实生成，并标注为推断问题，不把它冒充为 Ahrefs Questions 数据。

## 主推完整流程（实施审计）

| # | 模型 ID | 页面路径 | 类型 | 状态 |
|---:|---|---|---|---|
| 1 | `gpt-5.6-sol` | `/models/gpt-5.6-sol` | text/chat | 研究完成；策划包已接入；配置/组件回归与本地检查通过；待 staging/浏览器/事实验收 |
| 2 | `gpt-image-2` | `/models/gpt-image-2` | image | 研究完成；策划包已接入；配置/组件回归与本地检查通过；待 staging/浏览器/事实验收 |
| 3 | `kimi-k3` | `/models/kimi-k3` | text/chat | 研究完成；策划包已接入；配置/组件回归与本地检查通过；待 staging/浏览器/事实验收 |
| 4 | `deepseek-v4-pro` | `/models/deepseek-v4-pro` | text/chat | 研究完成；策划包已接入；配置/组件回归与本地检查通过；待 staging/浏览器/事实验收 |
| 5 | `MiniMax-H3` | `/models/minimax-h3` | video | 研究完成；策划包与大小写 alias redirect 已接入；配置/组件回归与本地检查通过；待 staging/浏览器/事实验收 |

## 已完成/排除

| 模型 ID | 页面路径 | 类型 | 处理 |
|---|---|---|---|
| `seedance-2.5` | `/models/seedance-2.5` | video | 独立复审完成；关键词/事实/计费/API/FAQ 已更新 |

### Seedance 2.5 独立复审摘要

- **主词**：`seedance 2.5`，全球 GSV **17,000**、KD **25**；`SV=3,800` 是 Ahrefs `country=us` 的英文代理值。
- **高价值支持词**：`release date` GSV 1,700、`api` 250、`ai` 300、`video` 200、`free` 250、`pricing` 60、`price` 50、`cost` 40；KD 未返回时保留 `N/A`。
- **Questions**：`when is ... coming out` GSV 70、`how to use ...` 30、`what is ...` 20、`where can/where to use ...` 10；FAQ 使用自然改写并绑定已核实事实。
- **事实基线**：官方资料记录音视频联合生成、单次最多 30 秒、可多轮延长和参考媒体；Flatkey 目录记录 `text/video/image` modalities、`/v1/videos`、4–30 秒、480p/720p 与参考上限。官方文章日期（2026-07-31）和目录 `released_at`（2026-08-04）分开呈现。
- **计费**：无视频参考时 480p `$0.140 × 输出秒数`、720p `$0.314 × 输出秒数`；有视频参考时 480p `$0.084 × 输入视频总秒数`、720p `$0.188 × 输入视频总秒数`。目录 `$0.14` 仅为基准字段，不能当作所有请求的固定价格。
- **反同质化**：共享 workbench/health/providers/CTA 版型，但 opening、六组行业工作流、pricing 公式、`content[]` API 示例和 9 个 FAQ 均为 Seedance 专属；不机械按“一 H2 一关键词”堆词，也不填 Ahrefs 未索引的 SV/KD。
- **巴西边界**：截图中的 Brazil `SV=600` 是英文精确查询 `seedance 2.5`，不是 pt-BR 搜索量；如要推巴西，须另采集葡萄牙语关键词和 `country=br` 数据。

## 全部模型目录

| # | 模型 ID（以接口原值为准） | 类型 | 执行模式 | 页面路径 |
|---:|---|---|---|---|
| 1 | `gemini-3.1-flash-image` | image | 快速流程 | `/models/gemini-3.1-flash-image` |
| 2 | `seedance-2.0-pro` | video | 快速流程 | `/models/seedance-2.0-pro` |
| 3 | `eleven_sound_v1` | audio | 快速流程 | `/models/eleven_sound_v1` |
| 4 | `gemini-embedding-001` | embedding | 快速流程 | `/models/gemini-embedding-001` |
| 5 | `gemini-3.5-flash-lite` | text/chat | 快速流程 | `/models/gemini-3.5-flash-lite` |
| 6 | `glm-4.7` | text/chat | 快速流程 | `/models/glm-4.7` |
| 7 | `gpt-5.6-sol` | text/chat | 完整流程/已实施待验收 | `/models/gpt-5.6-sol` |
| 8 | `claude-opus-4-6` | text/chat | 快速流程 | `/models/claude-opus-4-6` |
| 9 | `grok-imagine-video` | video | 快速流程 | `/models/grok-imagine-video` |
| 10 | `glm-5.1` | text/chat | 快速流程 | `/models/glm-5.1` |
| 11 | `gemini-3.1-flash-lite` | text/chat | 快速流程 | `/models/gemini-3.1-flash-lite` |
| 12 | `claude-sonnet-5` | text/chat | 快速流程 | `/models/claude-sonnet-5` |
| 13 | `kimi-k2.6` | text/chat | 快速流程 | `/models/kimi-k2.6` |
| 14 | `gemini-3.1-flash-lite-image` | image | 快速流程 | `/models/gemini-3.1-flash-lite-image` |
| 15 | `gemini-3.5-flash` | text/chat | 快速流程 | `/models/gemini-3.5-flash` |
| 16 | `grok-imagine-image` | image | 快速流程 | `/models/grok-imagine-image` |
| 17 | `gemini-2.5-flash-tts` | audio | 快速流程 | `/models/gemini-2.5-flash-tts` |
| 18 | `veo-3.1-fast-generate-preview` | video | 快速流程 | `/models/veo-3.1-fast-generate-preview` |
| 19 | `gpt-5.4-nano` | text/chat | 快速流程 | `/models/gpt-5.4-nano` |
| 20 | `gemini-2.5-pro-tts` | audio | 快速流程 | `/models/gemini-2.5-pro-tts` |
| 21 | `gemma-4-31b-it` | text/chat | 快速流程 | `/models/gemma-4-31b-it` |
| 22 | `qwen3.5-plus` | text/chat | 快速流程 | `/models/qwen3.5-plus` |
| 23 | `seedance-2.0-fast` | video | 快速流程 | `/models/seedance-2.0-fast` |
| 24 | `grok-imagine-video-1.5` | video | 快速流程 | `/models/grok-imagine-video-1.5` |
| 25 | `gemini-2.5-flash-image` | image | 快速流程 | `/models/gemini-2.5-flash-image` |
| 26 | `gemini-3.1-flash-tts-preview` | audio | 快速流程 | `/models/gemini-3.1-flash-tts-preview` |
| 27 | `gpt-5.4-mini` | text/chat | 快速流程 | `/models/gpt-5.4-mini` |
| 28 | `gpt-5.6-luna` | text/chat | 快速流程 | `/models/gpt-5.6-luna` |
| 29 | `deepseek-v3.2` | text/chat | 快速流程 | `/models/deepseek-v3.2` |
| 30 | `claude-opus-5` | text/chat | 快速流程 | `/models/claude-opus-5` |
| 31 | `gemini-3-pro-image` | image | 快速流程 | `/models/gemini-3-pro-image` |
| 32 | `gemini-3.7-flash` | text/chat | 快速流程 | `/models/gemini-3.7-flash` |
| 33 | `gpt-5-mini` | text/chat | 快速流程 | `/models/gpt-5-mini` |
| 34 | `glm-5.3` | text/chat | 快速流程 | `/models/glm-5.3` |
| 35 | `gemini-pro-latest` | text/chat | 快速流程 | `/models/gemini-pro-latest` |
| 36 | `gpt-5.6-terra` | text/chat | 快速流程 | `/models/gpt-5.6-terra` |
| 37 | `gpt-image-2` | image | 完整流程/已实施待验收 | `/models/gpt-image-2` |
| 38 | `macaron-v1-coding-venti` | text/chat | 快速流程 | `/models/macaron-v1-coding-venti` |
| 39 | `kimi-k2.5` | text/chat | 快速流程 | `/models/kimi-k2.5` |
| 40 | `qwen3.5-flash` | text/chat | 快速流程 | `/models/qwen3.5-flash` |
| 41 | `claude-opus-4-7` | text/chat | 快速流程 | `/models/claude-opus-4-7` |
| 42 | `deepseek-v3.2-thinking` | text/chat | 快速流程 | `/models/deepseek-v3.2-thinking` |
| 43 | `minimax-m2` | text/chat | 快速流程 | `/models/minimax-m2` |
| 44 | `qwen3.6-plus` | text/chat | 快速流程 | `/models/qwen3.6-plus` |
| 45 | `deepseek-v4-flash` | text/chat | 快速流程 | `/models/deepseek-v4-flash` |
| 46 | `gemini-robotics-er-1.6-preview` | text/chat | 快速流程 | `/models/gemini-robotics-er-1.6-preview` |
| 47 | `glm-5.2` | text/chat | 快速流程 | `/models/glm-5.2` |
| 48 | `kimi-k3` | text/chat | 完整流程/已实施待验收 | `/models/kimi-k3` |
| 49 | `gpt-4o` | text/chat | 快速流程 | `/models/gpt-4o` |
| 50 | `grok-imagine-image-quality` | image | 快速流程 | `/models/grok-imagine-image-quality` |
| 51 | `qwen3.7-plus` | text/chat | 快速流程 | `/models/qwen3.7-plus` |
| 52 | `gemini-2.5-pro-preview-tts` | audio | 快速流程 | `/models/gemini-2.5-pro-preview-tts` |
| 53 | `sonilo-video-to-music` | video | 快速流程 | `/models/sonilo-video-to-music` |
| 54 | `gemini-3.1-pro-preview-customtools` | text/chat | 快速流程 | `/models/gemini-3.1-pro-preview-customtools` |
| 55 | `seedance-2.0-mini` | video | 快速流程 | `/models/seedance-2.0-mini` |
| 56 | `claude-haiku-4-5-20251001` | text/chat | 快速流程 | `/models/claude-haiku-4-5-20251001` |
| 57 | `eleven_multilingual_v2` | audio | 快速流程 | `/models/eleven_multilingual_v2` |
| 58 | `gemini-3.1-flash-lite-preview` | text/chat | 快速流程 | `/models/gemini-3.1-flash-lite-preview` |
| 59 | `gemini-flash-latest` | text/chat | 快速流程 | `/models/gemini-flash-latest` |
| 60 | `qwen3.8-max` | text/chat | 快速流程 | `/models/qwen3.8-max` |
| 61 | `seedance-2.0` | video | 快速流程 | `/models/seedance-2.0` |
| 62 | `gemini-2.5-flash` | text/chat | 快速流程 | `/models/gemini-2.5-flash` |
| 63 | `qwen3.7-max` | text/chat | 快速流程 | `/models/qwen3.7-max` |
| 64 | `claude-fable-5` | text/chat | 快速流程 | `/models/claude-fable-5` |
| 65 | `claude-opus-4-8` | text/chat | 快速流程 | `/models/claude-opus-4-8` |
| 66 | `gpt-5.4` | text/chat | 快速流程 | `/models/gpt-5.4` |
| 67 | `macaron-v1-venti` | text/chat | 快速流程 | `/models/macaron-v1-venti` |
| 68 | `qwen3.5-397b-a17b` | text/chat | 快速流程 | `/models/qwen3.5-397b-a17b` |
| 69 | `gemini-2.5-pro` | text/chat | 快速流程 | `/models/gemini-2.5-pro` |
| 70 | `qwen3.6-max-preview` | text/chat | 快速流程 | `/models/qwen3.6-max-preview` |
| 71 | `deepseek-v3.1` | text/chat | 快速流程 | `/models/deepseek-v3.1` |
| 72 | `deepseek-v4-pro` | text/chat | 完整流程/已实施待验收 | `/models/deepseek-v4-pro` |
| 73 | `minimax-m2.5` | text/chat | 快速流程 | `/models/minimax-m2.5` |
| 74 | `gemini-3-flash-preview` | text/chat | 快速流程 | `/models/gemini-3-flash-preview` |
| 75 | `gemini-2.5-flash-lite` | text/chat | 快速流程 | `/models/gemini-2.5-flash-lite` |
| 76 | `gemini-flash-lite-latest` | text/chat | 快速流程 | `/models/gemini-flash-lite-latest` |
| 77 | `gpt-4o-mini` | text/chat | 快速流程 | `/models/gpt-4o-mini` |
| 78 | `gemini-2.5-flash-preview-tts` | audio | 快速流程 | `/models/gemini-2.5-flash-preview-tts` |
| 79 | `MiniMax-H3` | video | 完整流程/已实施待验收（canonical `/models/minimax-h3`） | `/models/MiniMax-H3` → `/models/minimax-h3` |
| 80 | `glm-5` | text/chat | 快速流程 | `/models/glm-5` |
| 81 | `gpt-4.1-mini` | text/chat | 快速流程 | `/models/gpt-4.1-mini` |
| 82 | `qwen3.5-35b-a3b` | text/chat | 快速流程 | `/models/qwen3.5-35b-a3b` |
| 83 | `claude-sonnet-4-5-20250929` | text/chat | 快速流程 | `/models/claude-sonnet-4-5-20250929` |
| 84 | `deepseek-v4-pro-0813` | text/chat | 快速流程 | `/models/deepseek-v4-pro-0813` |
| 85 | `claude-opus-4-5` | text/chat | 快速流程 | `/models/claude-opus-4-5` |
| 86 | `gemma-4-26b-a4b-it` | text/chat | 快速流程 | `/models/gemma-4-26b-a4b-it` |
| 87 | `seedance-2.5` | video | 独立复审/已实施 | `/models/seedance-2.5` |
| 88 | `grok-imagine-image-pro` | image | 快速流程 | `/models/grok-imagine-image-pro` |
| 89 | `deepseek-v3` | text/chat | 快速流程 | `/models/deepseek-v3` |
| 90 | `minimax-m2.7` | text/chat | 快速流程 | `/models/minimax-m2.7` |
| 91 | `gemini-3.1-pro-preview` | text/chat | 快速流程 | `/models/gemini-3.1-pro-preview` |
| 92 | `gemini-3.6-flash` | text/chat | 快速流程 | `/models/gemini-3.6-flash` |
| 93 | `gpt-5.5` | text/chat | 快速流程 | `/models/gpt-5.5` |
| 94 | `nano-banana-pro-preview` | image | 快速流程 | `/models/nano-banana-pro-preview` |
| 95 | `veo-3.1-generate-preview` | video | 快速流程 | `/models/veo-3.1-generate-preview` |
| 96 | `glm-5-turbo` | text/chat | 快速流程 | `/models/glm-5-turbo` |
| 97 | `qwen3.5-27b` | text/chat | 快速流程 | `/models/qwen3.5-27b` |
| 98 | `qwen3.5-plus-2026-02-15` | text/chat | 快速流程 | `/models/qwen3.5-plus-2026-02-15` |
| 99 | `claude-sonnet-4-5` | text/chat | 快速流程 | `/models/claude-sonnet-4-5` |
| 100 | `claude-sonnet-4-6` | text/chat | 快速流程 | `/models/claude-sonnet-4-6` |
| 101 | `macaron-v1-tall` | text/chat | 快速流程 | `/models/macaron-v1-tall` |

## 备注

- 页面路径使用模型目录返回的 `model_name` 做 URL 编码；不要凭展示名称猜测 slug。
- 如果样式重构过程中模型目录发生变化，启动前重新拉取公开目录并做差异核对；新增模型默认进入快速流程，除非用户将其列为主推。
- 本轮“开始”后已按本清单完成 101 页代码接入：Seedance 2.5 不重复扣费，5 个主推页按完整研究结果落地，其余 95 页按 Global + Matching 快速模式落地。任何页面发布前都要重新核验产品事实，并用 Search Console/排名跟踪验证实际效果。
