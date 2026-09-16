# Unified showcase generators — 2026-09-14

Resumed and revalidated on 2026-09-16.

The requested generation plan was changed to use Image 2 for every image and Seedance 2.0 for every video, while keeping each destination page's exact English prompt. This supersedes the per-model generator plan in `media-regeneration-20260914.md`.

## Completed image assets

49 of 63 requested image assets are regenerated, visually reviewed, and bound to the displayed prompts. The manifest records the destination model separately from `generationModel`, plus prompt and asset SHA-256 hashes, actual dimensions, and completion time.

| Destination page | Image 2 assets / requested |
| --- | --- |
| `gpt-image-2` | 7 / 7 |
| `gemini-2.5-flash-image` | 7 / 7 |
| `gemini-3-pro-image` | 7 / 7 |
| `gemini-3.1-flash-image` | 6 / 7 |
| `gemini-3.1-flash-lite-image` | 5 / 7 |
| `grok-imagine-image` | 4 / 7 |
| `grok-imagine-image-pro` | 5 / 7 |
| `grok-imagine-image-quality` | 4 / 7 |
| `nano-banana-pro-preview` | 4 / 7 |

All new artwork is packaged as local WebP assets in `public/assets/model-regeneration/20260914-image2/`. The playground, generated examples, and prompt library credit the actual generator as **Image 2**, including on other model pages. Existing localized credit text is reused in all supported locales.

## Outstanding video generation

The old Flatkey connection accepted Image 2 calls, but the Seedance 2.0 canary returned HTTP 400 `fail_to_fetch_task` with an upstream insufficient-quota message. No Seedance 2.0 video was produced in this batch. All 60 requested video replacements remain pending channel quota recovery; existing videos are retained and are not relabeled as Seedance output.

The output preview also stops falling back from an existing regenerated local video to an unrelated older clip, matching the prompt gallery's existing behavior.

## Failure handling and provenance

Image failures were investigated through read-only router logs: the upstream reported overloaded servers. Initial automatic retries covered explicit failures and retained the exact same prompt and actual model. In-flight requests were not duplicated. On September 16, the user requested continuation; missing outputs were recreated after preserving DNS-failure and expired-timeout records. The two expired image timeouts had no retrievable output, so their prior server outcomes remain unknown. Credentials and base64 API responses are excluded from the repository.

The generated assets were inspected against scene content, clothing/props, composition, palette, and requested text before integration. Automated provenance checks verify displayed prompt hashes and asset bindings. Native outputs are converted to WebP for delivery without altering their depicted content.

## Deployment scope

Website only (`newapi-web`). Router deploy: **not required**. No Go runtime, console, billing, database, or infrastructure changes; no distributed-state implications. Merging does not demonstrate production deployment, which remains subject to the website workflow approval gate.

## Pending image replacements

Existing assets are retained for these slots; they are not credited as new Image 2 output.

| Destination | Scene | Remaining work |
| --- | --- | --- |
| `gemini-3.1-flash-image` | `social-ad` | Regenerate after upstream overload clears. |
| `gemini-3.1-flash-lite-image` | `food-editorial` | Regenerate after upstream overload clears. |
| `gemini-3.1-flash-lite-image` | `playground` | Correct the contact-sheet panel 98 label (rendered as 88); Image 2 edit also failed with upstream overload. |
| `grok-imagine-image` | `social-ad` | Regenerate after upstream overload clears. |
| `grok-imagine-image` | `editorial-portrait` | Regenerate after upstream overload clears. |
| `grok-imagine-image` | `playground` | Regenerate after upstream overload clears. |
| `grok-imagine-image-pro` | `product-hero` | Regenerate after upstream overload clears. |
| `grok-imagine-image-pro` | `playground` | Regenerate after upstream overload clears. |
| `grok-imagine-image-quality` | `product-hero` | Regenerate after upstream overload clears. |
| `grok-imagine-image-quality` | `editorial-portrait` | Regenerate after upstream overload clears. |
| `grok-imagine-image-quality` | `food-editorial` | Regenerate after upstream overload clears. |
| `nano-banana-pro-preview` | `social-ad` | Regenerate after upstream overload clears. |
| `nano-banana-pro-preview` | `food-editorial` | Retry after the network recovers; the resumed request returned no output. |
| `nano-banana-pro-preview` | `playground` | Regenerate after upstream overload clears. |
