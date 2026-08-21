# Production Model Directory Metadata Backfill Plan

Generated at: 2026-08-21T11:01:11.069Z
Scope: `https://console.flatkey.ai/api/website/pricing?group=plg`

This is a read-only review plan. No production database write was performed.

## Coverage snapshot

| Set | Count |
| --- | ---: |
| Current production PLG models | 101 |
| Reviewed candidate metadata records | 113 |
| Exact-name overlap available for reuse | 99 |
| Production models without an exact reviewed record | 2 |
| Reviewed records not currently live in production PLG | 14 |

The runtime table is currently empty in production (`metadata entries: 0`). The exact-name rule is intentional: a candidate record is not reused when its public model name differs by prefix, provider path, or alias.

## Proposed phases

1. Review and import the 99 exact-name records already present in the reviewed candidate document.
2. Research and add exact-name records for `eleven_sound_v1` and `eleven_multilingual_v2`; do not infer their modality, context, release date, or distillability.
3. Keep the 14 reviewed-but-not-live records out of production until the matching exact public names appear in the PLG catalogue.
4. Run the importer in `--dry-run` mode against production and review every insert/update/disable before any `--apply` execution.
5. After explicit approval, apply the reviewed set in one transaction, then rerun the read-only audit and require zero live metadata gaps.

## Production-only issue to resolve separately

`gemini-embedding-001` currently produces an output filter price of `0`. This is a pricing-pipeline/data-quality issue, not a metadata backfill. It must be explicitly accepted or corrected before the final production audit can be considered clean.

## Reviewed records not currently live

- `bytedance/seedance-2.0`
- `bytedance/seedance-2.0-fast`
- `claude-3-5-haiku-20241022`
- `doubao/doubao-seedance-2-0-260128`
- `gemini-2.0-flash`
- `jimeng-image-4.5`
- `jimeng-image-5.0-lite`
- `jimeng-video-3.0-fast`
- `jimeng-video-3.0-pro`
- `jimeng-video-seedance-2.0`
- `jimeng-video-seedance-2.0-fast`
- `jimeng-video-seedance-2.0-mini`
- `mirothinker-1-7-deepresearch`
- `mirothinker-1-7-deepresearch-mini`

