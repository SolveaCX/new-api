# Model showcase regeneration — 2026-09-14

This is a partial regeneration of the 123 curated gallery/starter examples (63 images and 60 videos) after the English prompt revision. Each accepted output was requested from its named Flatkey model with the exact displayed English prompt. No model substitution was used for this batch.

## Included

- Nano Banana Pro Preview: all six gallery images and the interior Playground starter (7).
- Veo 3.1 Generate Preview: all six profession videos (6).
- Veo 3.1 Fast Generate Preview: paper theater, running shoe, stone golem and waveform performer (4).

The 17 accepted assets are packaged under `public/assets/model-regeneration/20260914/`. Images are WebP conversions of generated originals; MP4 streams are remuxed with fast-start metadata; each video poster is extracted from that same clip. The generation manifest records model, scene, exact prompt SHA-256, packaged asset SHA-256 and actual dimensions/duration. Its regression test checks the current served prompt/media binding and the matching fallback. Existing examples remain in place for the 106 items that have not passed regeneration and review.

## Outstanding

- Seedance 2.0 / Pro and Grok image/video calls: current generation credential group has no available channel.
- Seedance 2.5 / Fast / Mini and MiniMax-H3: absent from the accessible model list.
- GPT Image 2: upstream image generation returned HTTP 500, including one retry.
- Four Gemini image variants: requests did not return after more than two hours. The client was stopped and the requests are marked uncertain; reconcile account logs before resubmitting to avoid duplicate charges.
- Veo Fast film concept: task completed, but repeated content downloads returned HTTP 502. Recover the existing task output rather than generating it again.
- Veo Fast physics explainer: rejected after a retake because the ball reverses up the ramp and unrequested text remains. The rejected files are not included.

Detailed private job ledgers and original outputs remain outside the repository in the local regeneration workspace. This report and the checked-in manifest contain no credentials, task IDs or expiring source URLs. Continue using working channels for the original named models; do not relabel another model's output to fill missing slots.

## Validation and deployment

22 targeted tests pass, including prompt-to-asset provenance. Website lint has no errors (26 pre-existing warnings); typecheck and production build pass. Local browser checks verify the Nano Banana images and Veo video/poster bindings. No output is claimed to satisfy every fine-grained prompt instruction perfectly; reviewed entries preserve the requested scene and visual treatment.

Router deploy: **not required**. Only the `newapi-web` website artifact changes. No console/backend/infra deployment or multi-node coordination change is involved. Production publication still depends on the website deployment workflow; merging this content does not prove deployment.
