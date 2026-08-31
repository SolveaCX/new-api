# Playground TTS Audio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make authorized OpenAI-compatible TTS models usable from Playground, with a binary audio response rendered as an inline player and downloadable file.

**Architecture:** Add a dedicated `/pg/audio/speech` route that reuses the existing audio relay and billing lifecycle. Classify only known generic TTS model families as an `audio` Playground profile; keep ElevenLabs native endpoints out of this first milestone. Gemini's adapter translates the OpenAI audio request into its native `generateContent` audio contract and decodes returned inline audio.

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

- [ ] **Step 1: Add the failing route assertion.** Extend `TestSetRelayRouterRegistersPlaygroundMediaRoutes` with `"POST /pg/audio/speech": false`. Run `go test ./router -run TestSetRelayRouterRegistersPlaygroundMediaRoutes -count=1`. Expected result: failure saying `missing route POST /pg/audio/speech`.

- [ ] **Step 2: Add failing Gemini conversion/response tests.** In `tts_test.go`, add tests that call package helpers without a network request. `buildGeminiTTSRequest(dto.AudioRequest{Model: "gemini-2.5-flash-preview-tts", Input: "你好，世界", Voice: "alloy"})` must produce one user text part, `ResponseModalities == []string{"AUDIO"}`, and speechConfig JSON `{"voiceConfig":{"prebuiltVoiceConfig":{"voiceName":"Kore"}}}`. `extractGeminiTTSAudio` given an inlineData payload with `mimeType:"audio/pcm", data:"SGk="` must return bytes `Hi` and MIME `audio/pcm`. Run `go test ./relay/channel/gemini -run 'TestBuildGeminiTTSRequest|TestExtractGeminiTTSAudio' -count=1`. Expected result: failure because the helpers do not exist.

- [ ] **Step 3: Register the Playground route.** Add `playgroundRouter.POST("/audio/speech", controller.PlaygroundAudio)` and a thin controller wrapper that calls `runPlaygroundRelay(c, types.RelayFormatOpenAIAudio, func() { Relay(c, types.RelayFormatOpenAIAudio) })`.

- [ ] **Step 4: Implement Gemini request construction.** In `tts.go`, define `buildGeminiTTSRequest(dto.AudioRequest) (*dto.GeminiChatRequest, error)`. Reject blank input, create one user text part, set `ResponseModalities` to `[]string{"AUDIO"}`, and serialize `SpeechConfig` through `common.Marshal`. Map OpenAI aliases `alloy/echo/fable/onyx/nova/shimmer` to `Kore/Puck/Charon/Fenrir/Aoede/Leda`; pass through a non-empty Gemini voice name and default blank voice to `Kore`.

- [ ] **Step 5: Wire the Gemini adaptor and response decoder.** Replace the `not implemented` body in `Adaptor.ConvertAudioRequest` with a RelayMode check, `buildGeminiTTSRequest`, and `common.Marshal` into a `bytes.Reader`. Implement `extractGeminiTTSAudio` with `common.Unmarshal` and strict base64 decoding of the first audio `inlineData` part. Implement `GeminiTTSHandler(c, info, resp)` to copy binary audio, set a safe `Content-Type` (fallback `audio/wav`), and return usage from `buildUsageFromGeminiMetadata` or the input estimate when metadata is empty. Route `Adaptor.DoResponse` to this handler for TTS model names before ordinary Gemini chat/image handling.

- [ ] **Step 6: Run backend focused tests.** Run `go test ./router -run TestSetRelayRouterRegistersPlaygroundMediaRoutes -count=1` and `go test ./relay/channel/gemini -run 'TestBuildGeminiTTSRequest|TestExtractGeminiTTSAudio' -count=1`. Expected result: both pass.

- [ ] **Step 7: Commit the backend slice.** Before editing each existing symbol, run `gitnexus impact --direction upstream <symbol>` (or record `gitnexus status` as the unavailable-index fallback); after tests, run `git diff --check` and `gitnexus detect_changes --scope all`, then create a Lore-formatted commit describing the route and Gemini TTS boundary.

### Task 2: Add the audio Playground model profile and Blob transport (RED → GREEN)

**Files:**
- Test: `web/default/src/features/playground/lib/playground-model-filter.test.ts`, `media-generation.test.ts`, `api.test.ts`
- Modify: `web/default/src/features/playground/lib/media-generation.ts`, `api.ts`, `types.ts`

- [ ] **Step 1: Add failing model/profile tests.** Assert that `gemini-2.5-flash-preview-tts`, `gemini-2.5-pro-tts`, `gemini-3.1-flash-tts-preview`, and known MiniMax/Volcengine TTS aliases resolve to `audio` and are supported. Assert that `eleven_multilingual_v2`, `eleven_sound_v1`, `gpt-4o-audio-preview`, and `sonilo-video-to-music` retain their existing classifications. Assert that `buildMediaGenerationRequest("你好", "gemini-2.5-flash-preview-tts", "plg", {})` returns kind `audio`, endpoint `/pg/audio/speech`, and payload defaults `voice: "Kore"`, `response_format: "mp3"`. Run from `web/default`: `bun test src/features/playground/lib/playground-model-filter.test.ts src/features/playground/lib/media-generation.test.ts --max-concurrency=1`. Expected result: failure because `audio` is not a valid kind/profile.

- [ ] **Step 2: Extend media types and profile.** Add `audio` to `PlaygroundModelKind`; add a `tts` family and an audio profile with voice/response-format select fields. Recognize only explicit Gemini TTS patterns and known generic MiniMax/Volcengine TTS identifiers; leave ElevenLabs and native audio-understanding patterns unsupported/chat respectively. Build `/pg/audio/speech` payloads with normalized defaults and reject attachments.

- [ ] **Step 3: Make media transport binary-aware.** Extend `MediaGenerationRequest.kind` to include `audio`. In `sendMediaGeneration`, set Axios `responseType: "blob"` only for audio; preserve JSON behavior for image/video. Add `GeneratedMedia.type = "audio"` for metadata compatibility, but never persist object URLs.

- [ ] **Step 4: Run profile/API tests.** Run `bun test src/features/playground/lib/playground-model-filter.test.ts src/features/playground/lib/media-generation.test.ts src/features/playground/api.test.ts --max-concurrency=1`. Expected result: pass.

- [ ] **Step 5: Commit the frontend profile slice.** Run `git diff --check` and `gitnexus detect_changes --scope all` (or record the unavailable-index fallback), then create a Lore-formatted commit.

### Task 3: Render and clean up generated audio in Playground (RED → GREEN)

**Files:**
- Test: `web/default/src/features/playground/hooks/use-media-generation.test.ts`, `components/playground-chat.test.tsx`
- Modify: `web/default/src/features/playground/hooks/use-media-generation.ts`, `components/playground-chat.tsx`, `index.tsx`, `types.ts`

- [ ] **Step 1: Add failing hook/render tests.** Cover an audio Blob creating an object URL and completing the assistant message with an audio media entry, a non-audio Blob becoming a user-visible error, chat rendering `<audio controls>`, and cleanup revoking the URL on message deletion/unmount.

- [ ] **Step 2: Implement audio completion.** In `useMediaGeneration`, validate `Blob.type` against `audio/*`, create an object URL, and call `completeMedia` with `{ type: "audio", url }`. Track generated object URLs in a ref so replacement, abort, and unmount call `URL.revokeObjectURL` exactly once. Keep video polling unchanged.

- [ ] **Step 3: Render audio safely.** In `playground-chat.tsx`, render audio media as `<audio controls preload="metadata">` with a download action; do not pass untrusted data URLs through the image sanitizer. Keep image/video branches unchanged and avoid serializing blob URLs through persistence.

- [ ] **Step 4: Update deletion/unmount cleanup.** In `index.tsx` and hook cleanup, revoke any `blob:` URL associated with removed audio/video media, and never revoke relative or HTTPS URLs.

- [ ] **Step 5: Run focused frontend tests.** Run `bun test src/features/playground/hooks/use-media-generation.test.ts src/features/playground/components/playground-chat.test.tsx --max-concurrency=1`. Expected result: pass.

- [ ] **Step 6: Commit the rendering slice.** Run `git diff --check` and `gitnexus detect_changes --scope all` (or record the unavailable-index fallback), then create a Lore-formatted commit.

### Task 4: Full verification and production-ready handoff

**Files:** no new production files; inspect all changed files and generated artifacts.

- [ ] **Step 1: Run backend verification.** Run `go test ./router ./controller ./relay/channel/gemini ./relay -count=1` and `go build ./...`; both must exit 0.

- [ ] **Step 2: Run frontend verification.** From `web/default`, run `bun test src/features/playground --max-concurrency=1` and `bun run build`; both must exit 0.

- [ ] **Step 3: Start the affected local preview.** Start the normal Go/console development server for this checkout, open the local Playground URL, select a TTS model, submit a short sentence, and confirm the network request is `POST /pg/audio/speech`, response MIME is audio, and playback/download work. If local preview cannot start, record the exact blocker and do not claim local verification.

- [ ] **Step 4: Run the production smoke test.** Against the authenticated production Playground, test one authorized Gemini TTS model and one authorized existing TTS model with a short non-sensitive sentence. Capture status, response content type, and UI playback result without exposing credentials or audio contents.

- [ ] **Step 5: Run `gitnexus detect_changes --scope compare --base-ref main`, inspect `git diff --stat`/`git status`, and confirm only route, Gemini TTS, Playground, tests, and design artifacts changed. If `gitnexus status` still says the repository is not indexed, record that exact output and use the diff/stat review as the fallback.

- [ ] **Step 6: Push the feature branch and open a PR against `main` only after all checks pass.** Include evidence, root cause, scope, router deploy requirement (`required`), and the remaining ElevenLabs non-goal in the PR note.
