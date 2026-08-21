# Production Model Directory Metadata Backfill Plan

Generated at: 2026-08-21T14:42:44.939Z
Scopes:

- Full public pricing catalogue: `https://console.flatkey.ai/api/website/pricing`
- PLG models-directory catalogue: `https://console.flatkey.ai/api/website/pricing?group=plg`

This is a read-only review plan. No production database write was performed.

## Coverage snapshot

| Set | Count |
| --- | ---: |
| Current production full-catalogue models | 91 |
| Current production PLG models | 89 |
| Reviewed candidate metadata records | 113 |
| Full-catalogue exact-name overlap available for reuse | 89 |
| PLG exact-name overlap available for reuse | 87 |
| Production models without an exact reviewed record | 2 |
| Reviewed records not currently live in the full catalogue | 24 |
| Reviewed records not currently live in PLG | 26 |

The runtime table is currently empty in production (`metadata entries: 0`). The exact-name rule is intentional: a candidate record is not reused when its public model name differs by prefix, provider path, or alias.

## Proposed phases

1. Review and import the 89 exact-name records covering the full public catalogue. Of these, 87 cover the PLG models-directory catalogue.
2. Research and add exact-name records for `eleven_sound_v1` and `eleven_multilingual_v2`; do not infer their modality, context, release date, or distillability.
3. Keep the reviewed-but-not-live records out of production until their exact public names appear in the relevant catalogue scope.
4. Run the importer in `--dry-run` mode against production and review every insert/update/disable before any `--apply` execution.
5. After explicit approval, apply the reviewed set in one transaction, then rerun the read-only audit and require zero live metadata gaps.

## Production-only issue to resolve separately

`gemini-embedding-001` currently produces an output filter price of `0`. This is a pricing-pipeline/data-quality issue, not a metadata backfill. It must be explicitly accepted or corrected before the final production audit can be considered clean.

## Reviewed records not currently live in the full catalogue

- `bytedance/seedance-2.0`
- `bytedance/seedance-2.0-fast`
- `claude-3-5-haiku-20241022`
- `doubao/doubao-seedance-2-0-260128`
- `gemini-2.0-flash`
- `gemini-2.5-flash-preview-tts`
- `gemini-2.5-pro-preview-tts`
- `gemini-3.1-flash-lite-preview`
- `gemini-3.7-flash`
- `gemini-flash-latest`
- `gemini-flash-lite-latest`
- `gemini-pro-latest`
- `gemma-4-26b-a4b-it`
- `gemma-4-31b-it`
- `jimeng-image-4.5`
- `jimeng-image-5.0-lite`
- `jimeng-video-3.0-fast`
- `jimeng-video-3.0-pro`
- `jimeng-video-seedance-2.0`
- `jimeng-video-seedance-2.0-fast`
- `jimeng-video-seedance-2.0-mini`
- `nano-banana-pro-preview`
- `veo-3.1-fast-generate-preview`
- `veo-3.1-generate-preview`

The PLG-only non-live list additionally contains `mirothinker-1-7-deepresearch` and `mirothinker-1-7-deepresearch-mini`.
