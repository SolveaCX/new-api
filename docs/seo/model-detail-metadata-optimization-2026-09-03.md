# Model Detail Metadata Optimization

Status: validated

Collection date: 2026-09-03 (Asia/Shanghai)

Scope: all public model-detail routes under `website/`, including curated and
live-catalog models, across the ten locales declared in
`website/src/lib/locales.ts`: en, zh, es, fr, pt, ru, ja, vi, de, id.

## Executive summary

Model-detail metadata now uses one page-specific brand suffix (`| Flatkey`),
without the root layout appending `| flatkey.ai`. Generated descriptions identify
the model type, provider, supported task, current price, context window when the
catalog provides it, and older-model price/performance comparison intent. Every
generated description is capped at 160 characters after whitespace normalization.

The detail shell now uses one breadcrumb shape: `Flatkey > All models > Model`
(with the first two labels translated by locale). All model-detail videos retain
autoplay, loop, and inline playback behavior while being forced to play muted.

## Evidence and unavailable fields

| Evidence | Result |
| --- | --- |
| Source of model facts | Live pricing/catalog payload consumed by the existing Next.js server routes |
| Supported locales | `LOCALES` source: 10 locales listed above |
| Ahrefs API | Unavailable in this environment; no keyword volume, KD, CPC, TP, or SERP metric was inferred |
| Rendered HTML | Verified locally on English `claude-fable-5.1` and Chinese `seedance-2.5` pages |
| Build | `bun run build` passed; 885 static pages generated |

## Keyword and content coverage matrix

The exact matrix is available in the companion files:

- [JSON matrix](./model-detail-metadata-optimization-2026-09-03.json)
- [CSV coverage table](./model-detail-metadata-optimization-2026-09-03.csv)

Because Ahrefs data was not available, SV/GSV/KD/TP/CPC/Parent Topic and SERP
competition are recorded as `N/A`. The page assignment is based on verified
product intent and the existing model-detail route structure, not inferred
search metrics.

| Page family | Primary intent | Commercial/action intent | Secondary clusters | FAQ |
| --- | --- | --- | --- | --- |
| `/models/[slug]` | `{model} API` / localized equivalent | `{model} pricing` / localized equivalent | provider, model type, task, context window, API endpoint | Existing visible model-specific FAQ retained |

## Metadata layout

| Surface | Implemented behavior |
| --- | --- |
| Title | Model name + task/API wording + localized pricing/FAQ wording + exactly one `| Flatkey` |
| Description | Type + provider + task + price + context window or route-dependent context + older-model price/performance comparison; max 160 chars |
| H1/opening answer | Existing model-specific detail H1 and opening copy remain visible and server-rendered |
| H2/body | Existing pricing, performance, activity, API, capability, and comparison sections remain in the shared detail shell |
| FAQ | Existing target-specific FAQ section remains visible on indexable detail pages |
| CTA/internal links | Existing start/API actions and related-model links remain unchanged except for the unified breadcrumb shell |

## Exact implemented copy examples

English dynamic metadata:

`claude-fable-5.1 API (chat/completions), pricing & FAQs | Flatkey`

`claude-fable-5.1: Anthropic chat model for chat/code. Priced at $8/$40 per 1M tokens. context varies by route. Compare price/performance vs older models.`

Chinese video metadata:

`Seedance 2.5视频生成 API、价格与常见问题 | Flatkey`

`Seedance 2.5是ByteDance的视频模型，可用于文生视频和图生视频。价格为每秒 $0.084。上下文窗口随路由变化。比较旧模型价格/性能。`

## Shared-versus-unique review

The layout, structured data, pricing blocks, and breadcrumb hierarchy are
intentionally shared. Page-specific value remains unique through the live model
name, provider, detected model type, task capability, price unit/value, context
value, existing model copy, related-model set, and target-specific FAQ content.
No hidden keyword text or hard-coded provider claims were added.

## Verified claims and exclusions

- Prices are read from the existing live catalog payload and use the route's
  billing unit (tokens, image, audio, request, or second).
- Context is rendered only when a positive catalog context value exists; otherwise
  the description says that context varies by route in the relevant locale.
- The video change is playback-only: it sets the HTML video `muted` property and
  does not alter generation request defaults or audio capability claims.
- No Ahrefs or SERP metrics are claimed because the API key and evidence were
  unavailable.

## Files changed

- `website/src/lib/model-landing.ts`
- `website/src/lib/seo.ts`
- `website/src/app/(en)/models/[slug]/page.tsx`
- `website/src/app/[locale]/models/[slug]/page.tsx`
- `website/src/components/model-landing-page.tsx`
- `website/src/components/model-public-page.tsx`
- `website/src/components/cdn-media.tsx`
- Corresponding model landing and SEO tests

## Validation

- `bun test src/lib/model-landing.test.ts src/lib/seo.test.ts src/components/model-landing-page.test.tsx`: 66 passed, 0 failed.
- `bun run typecheck`: passed.
- `bun run lint`: passed with 26 pre-existing `<img>` warnings and 0 errors.
- `bun run build`: passed; 885 static pages generated.
- Local rendered checks on `http://localhost:4005/models/claude-fable-5.1` and `http://localhost:4005/zh/models/seedance-2.5`: title, description, breadcrumb, and muted video properties verified.
