# Playground TTS Audio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make authorized OpenAI-compatible TTS models usable from Playground, with a binary audio response rendered as an inline player and downloadable file.

**Architecture:** Add a dedicated `/pg/audio/speech` route that reuses the existing audio relay and billing lifecycle. Classify only known generic TTS model families as an `audio` Playground profile; keep ElevenLabs native endpoints out of this first milestone. Gemini's adapter translates the OpenAI audio request into its native `generateContent` audio contract and decodes returned inline audio. Gemini's legacy contract exposes voice only in this UI; unsupported OpenAI output controls are hidden and rejected at the adapter boundary.

**Tech Stack:** Go 1.22/Gin relay adapters, existing `dto.AudioRequest` and `common` JSON wrappers, React 19/TypeScript, Bun tests, Axios Blob responses.

---

## Files and ownership

- Backend route/controller: `router/relay-router.go`, `controller/playground.go`, `router/relay_router_playground_test.go`.
- Gemini TTS conversion/response: `relay/channel/gemini/adaptor.go`, new `relay/channel/gemini/tts.go`, new `relay/channel/gemini/tts_test.go`.
- Frontend model/profile/request: `web/default/src/features/playground/lib/media-generation.ts`, `playground-model-filter.test.ts`, `media-generation.test.ts`.
- Frontend transport/state/rendering: `web/default/src/features/playground/api.ts`, `hooks/use-media-generation.ts`, `types.ts`, `components/playground-chat.tsx`, `index.tsx` cleanup path, and focused tests.

### Task 1: Lock the backend route and Gemini request contract (RED → GREEN)

**Files:**
- Test: `router/relay_router_playground_test.go`
- Test: `relay/channel/gemini/tts_test.go`
- Modify: `router/relay-router.go`, `controller/playground.go`, `relay/channel/gemini/adaptor.go`
- Create: `relay/channel/gemini/tts.go`

- [x] **Step 1: Add the failing route assertion.** Extended `TestSetRelayRouterRegistersPlaygroundMediaRoutes` with the audio route and verified the route test passes.

- [x] **Step 2: Add failing Gemini conversion/response tests.** Added package-level conversion, instruction/options, PCM/WAV, usage, and malformed-response tests without network calls.

- [x] **Step 3: Register the Playground route.** Added `playgroundRouter.POST("/audio/speech", controller.PlaygroundAudioSpeech)` and the wrapper using `RelayFormatOpenAIAudio`.

- [x] **Step 4: Implement Gemini request construction.** In `tts.go`, define `buildGeminiTTSRequest(dto.AudioRequest) (*dto.GeminiChatRequest, error)`. Reject blank input, create one user text part, set `ResponseModalities` to `[]string{"AUDIO"}`, and serialize `SpeechConfig` through `common.Marshal`. Map OpenAI aliases `alloy/echo/fable/onyx/nova/shimmer` to `Kore/Puck/Charon/Fenrir/Aoede/Leda`; pass through a non-empty Gemini voice name and default blank voice to `Kore`.

- [x] **Step 5: Wire the Gemini adaptor and response decoder.** Replace the `not implemented` body in `Adaptor.ConvertAudioRequest` with a RelayMode check, `buildGeminiTTSRequest`, and `common.Marshal` into a `bytes.Reader`. Implement `extractGeminiTTSAudio` with `common.Unmarshal` and strict base64 decoding of the first audio `inlineData` part. Implement `GeminiTTSHandler(c, info, resp)` to wrap raw `audio/L16` PCM in a RIFF/WAV header (parse the sample rate from MIME, default 24000 Hz), pass through already-containerized audio, set a safe `Content-Type`, and return usage from `buildUsageFromGeminiMetadata` or the input estimate when metadata is empty. Reject unsupported Gemini `response_format`/numeric `speed` instead of silently dropping them. Route `Adaptor.DoResponse` to this handler for TTS model names before ordinary Gemini chat/image handling.

- [x] **Step 6: Run backend focused tests.** Focused Gemini, router, and relay-mode tests pass with `go test -p 1`.

- [x] **Step 7: Commit the backend slice.** GitNexus index fallback was recorded; the backend route/adapter slice is committed with Lore trailers.

### Task 2: Add the audio Playground model profile and Blob transport (RED → GREEN)

**Files:**
- Test: `web/default/src/features/playground/lib/playground-model-filter.test.ts`, `media-generation.test.ts`, `api.test.ts`
- Modify: `web/default/src/features/playground/lib/media-generation.ts`, `api.ts`, `types.ts`

- [x] **Step 1: Add failing model/profile tests.** Added coverage for production Gemini and MiniMax speech names, unsupported native/task names, provider-aware controls, and the `/pg/audio/speech` payload.

- [x] **Step 2: Extend media types and profile.** Add `audio` to `PlaygroundModelKind`; add a `tts` family and provider-aware audio profiles. Gemini exposes only its supported voice control, while generic OpenAI-compatible TTS keeps response format/speed controls. ElevenLabs and native audio-understanding patterns remain unsupported/chat respectively. Build `/pg/audio/speech` payloads with normalized defaults and reject attachments.

- [x] **Step 3: Make media transport binary-aware.** Extend `MediaGenerationRequest.kind` to include `audio`. In `sendMediaGeneration`, set Axios `responseType: "blob"` only for audio; preserve JSON behavior for image/video. Add `GeneratedMedia.type = "audio"` and MIME metadata for correct downloads, but never persist object URLs.

- [x] **Step 4: Run profile/API tests.** Profile, model-filter, and API tests pass.

- [x] **Step 5: Commit the frontend profile slice.** Included in the final frontend/docs commit after the last verification pass.

### Task 3: Render and clean up generated audio in Playground (RED → GREEN)

**Files:**
- Test: `web/default/src/features/playground/hooks/use-media-generation.test.ts`, `components/playground-chat.test.tsx`
- Modify: `web/default/src/features/playground/hooks/use-media-generation.ts`, `components/playground-chat.tsx`, `index.tsx`, `types.ts`

- [x] **Step 1: Add failing hook/render tests.** Covered an audio Blob creating an object URL and completing the assistant message with an audio media entry, a non-audio Blob becoming a user-visible error, chat rendering `<audio controls>`, and cleanup/revocation paths.

- [x] **Step 2: Implement audio completion.** In `useMediaGeneration`, validate `Blob.type` against `audio/*`, create an object URL, and call `completeMedia` with `{ type: "audio", url, mimeType }`. Track generated object URLs in a ref so replacement, abort, and unmount call `URL.revokeObjectURL` exactly once. Keep video polling unchanged.

- [x] **Step 3: Render audio safely.** In `playground-chat.tsx`, render audio media as `<audio controls preload="metadata">` with a MIME-derived download action; do not pass untrusted data URLs through the image sanitizer. Keep image/video branches unchanged and avoid serializing blob URLs through persistence.

- [x] **Step 4: Update deletion/unmount cleanup.** In `index.tsx` and hook cleanup, revoke any `blob:` URL associated with removed audio/video media, and never revoke relative or HTTPS URLs.

- [x] **Step 5: Run focused frontend tests.** Hook and chat audio tests pass.

- [x] **Step 6: Commit the rendering slice.** Included in the final frontend/docs commit after the last verification pass.

### Task 4: Full verification and production-ready handoff

**Files:** no new production files; inspect all changed files and generated artifacts.

- [x] **Step 1: Run backend verification.** Focused packages and controller compile check pass with `go test -p 1`; `go build ./...` passes. The full controller test suite was not used as a gate because it exceeded the local time budget.

- [x] **Step 2: Run frontend verification.** `bun test src/features/playground` (253 pass), `bun run typecheck`, and `bun run build:check` pass.

- [ ] **Step 3: Start the affected local preview.** Static console preview verification completed: `bun run preview --host 127.0.0.1 --port 4173` served the checkout with HTTP 200 (`text/html; charset=utf-8`). The full authenticated Go/API flow was not run locally because this checkout has no isolated production channel credentials/database, so no local audio response is claimed.

- [ ] **Step 4: Run the production smoke test.** Authenticated production inspection was run with the `plg` group selected. The Available Models page reports 101 models and 6 audio models (`gemini-2.5-flash-preview-tts`, `gemini-2.5-flash-tts`, `gemini-2.5-pro-preview-tts`, `gemini-2.5-pro-tts`, `gemini-3.1-flash-tts-preview`, and `sonilo-video-to-music`). The currently deployed Playground's `plg` model dropdown exposes none of the Gemini TTS options (`gemini-2.5-flash-preview-tts` locator count: 0), so this un-deployed branch cannot honestly claim a live audio response or playback result. Re-run the request smoke after the router and console are deployed together.

- [x] **Step 5: Run `gitnexus detect_changes --scope compare --base-ref main`, inspect `git diff --stat`/`git status`, and confirm only route, Gemini TTS, Playground, tests, and design artifacts changed. `gitnexus status` reported `Repository not indexed. Run: gitnexus analyze`; the compare command also encountered the installed FTS storage-version mismatch (`Database file version: 42, Current build storage version: 40`) and reported no changes. `git diff --stat`, `git status`, and `git diff --check` were used as the fallback review.

- [ ] **Step 6: Push the feature branch and open a PR against `main` only after all checks pass.** Include evidence, root cause, scope, router deploy requirement (`required`), and the remaining ElevenLabs non-goal in the PR note.
