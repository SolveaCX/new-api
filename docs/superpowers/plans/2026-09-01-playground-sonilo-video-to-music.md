# Playground Sonilo Video-to-Music Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sonilo-video-to-music` selectable in the `plg` Playground group and complete a local-video-to-playable-audio task flow without changing the public Sonilo API.

**Architecture:** Add a dedicated `/pg/video-to-music` alias that reuses the existing task relay, Sonilo adaptor, billing, persistence, and anonymous content proxy. Extend the Playground media registry with a video-input/audio-output profile, multipart request construction, a Sonilo response parser, and task-aware polling/resume state. Existing `/pg/videos`, TTS, image generation, and chat paths remain separate.

**Tech Stack:** Go/Gin task relay and route tests, React/TypeScript Playground, Axios/FormData, Bun tests, existing asset upload and media rendering utilities.

---

## Files and responsibilities

- Backend route and canonical path handling: `router/relay-router.go`, `controller/playground.go`, `relay/constant/relay_mode.go`, `middleware/distributor.go`, `relay/relay_task.go`, plus their focused tests.
- Frontend request/profile and input metadata: `web/default/src/features/playground/lib/media-generation.ts`, `web/default/src/features/playground/lib/attachments.ts`, `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/components/playground-input.tsx`.
- Frontend transport and task response parsing: `web/default/src/features/playground/api.ts`, `web/default/src/features/playground/lib/media-response.ts`.
- Frontend task lifecycle and persistence-compatible resume: `web/default/src/features/playground/hooks/use-media-generation.ts` and the focused hook/media tests. Existing `playground-chat.tsx` audio rendering is reused; only add a regression test if a relative Sonilo proxy URL needs coverage.

## Task 1: Make the Playground alias a real video-to-music task endpoint

**Files:**
- Modify: `router/relay-router.go`, `controller/playground.go`, `relay/constant/relay_mode.go`, `middleware/distributor.go`, `relay/relay_task.go`
- Test: `router/relay_router_playground_test.go`, `relay/constant/relay_mode_test.go`, `service/channel_select_endpoint_test.go`, `relay/relay_task_usage_test.go`

- [ ] **Step 1: Add failing route and canonical-mode tests.**

  Add the following route expectations to `TestSetRelayRouterRegistersPlaygroundMediaRoutes`:

  ```go
  "POST /pg/video-to-music":       false,
  "GET /pg/video-to-music/:task_id": false,
  ```

  Add `/pg/video-to-music` to the `Path2RelayMode` table with expected `RelayModeVideoSubmit`, and assert that `requestedEndpointType` sees `EndpointTypeVideoToMusic` after Playground normalization. The test must use a POST context with a multipart body containing `model=sonilo-video-to-music` and `group=plg`, so it exercises the same parser used by the distributor.

  Run:

  ```text
  go test ./router ./relay/constant ./service -run 'PlaygroundMediaRoutes|Path2RelayMode|VideoToMusic' -count=1
  ```

  Expected result: the new assertions fail because the alias and canonical mode are not registered yet.

- [ ] **Step 2: Register the route and controller wrappers.**

  Add wrappers next to the existing Playground video handlers:

  ```go
  func PlaygroundVideoToMusicSubmit(c *gin.Context) {
      runPlaygroundRelay(c, types.RelayFormatTask, func() { RelayTask(c) })
  }

  func PlaygroundVideoToMusicFetch(c *gin.Context) {
      runPlaygroundRelay(c, types.RelayFormatTask, func() { RelayTaskFetch(c) })
  }
  ```

  Register both handlers in the `/pg` group. Extend `Path2RelayMode` with the canonical `/v1/video-to-music` branch before generic video handling. Change the distributor’s video-to-music check to use its already-normalized `requestPath`, so `/pg/video-to-music` gets `RelayModeVideoSubmit` and extracts multipart `model`/`group` before channel selection.

- [ ] **Step 3: Normalize the fetch converter path.**

  Update `isVideoToMusicFetchPath` in `relay/relay_task.go` to map `/pg/...` to `/v1/...` before checking the prefix, mirroring `isOpenAIVideoFetchPath`. Add table-driven assertions in `relay/relay_task_usage_test.go` for both `/v1/video-to-music/task_x` and `/pg/video-to-music/task_x`; the latter must return true so `videoFetchByIDRespBodyBuilder` reaches the Sonilo `VideoToMusicConverter` branch.

- [ ] **Step 4: Run route and endpoint tests.**

  ```text
  go test ./router ./relay/constant ./service ./relay -run 'Playground|VideoToMusic|Path2RelayMode' -count=1
  ```

  Expected result: all new and existing route/endpoint tests pass, including the assertion that a Sonilo channel is selected and a non-Sonilo channel is rejected for the dedicated endpoint.

## Task 2: Add the Sonilo Playground profile, video metadata, and multipart request builder

**Files:**
- Modify: `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/lib/attachments.ts`, `web/default/src/features/playground/lib/media-generation.ts`, `web/default/src/features/playground/components/playground-input.tsx`
- Test: `web/default/src/features/playground/lib/attachments.test.ts`, `web/default/src/features/playground/lib/media-generation.test.ts`, `web/default/src/features/playground/lib/playground-model-filter.test.ts`, `web/default/src/features/playground/components/playground-input.test.tsx`

- [ ] **Step 1: Add failing profile and attachment tests.**

  Cover these concrete expectations:

  ```ts
  expect(resolvePlaygroundModelKind('sonilo-video-to-music')).toBe('audio')
  expect(resolveMediaGenerationProfile('sonilo-video-to-music')).toMatchObject({
    kind: 'audio',
    family: 'sonilo-video-to-music',
    inputKind: 'video',
    requiresAttachment: true,
  })
  expect(validateMediaGenerationAttachments('sonilo-video-to-music', [])).toBe(
    'Upload one video to use this model'
  )
  ```

  Add a normalized MP4 fixture whose `durationSeconds` is `12`, and assert that a malformed/multiple/non-video attachment is rejected. In the input component test, assert that the Sonilo profile uses `accept="video/mp4,.mp4"` and does not disable the attach button.

  Run:

  ```text
  bun test src/features/playground/lib/attachments.test.ts src/features/playground/lib/media-generation.test.ts src/features/playground/lib/playground-model-filter.test.ts src/features/playground/components/playground-input.test.tsx --max-concurrency=1
  ```

  Expected result: the new cases fail because the profile, duration metadata, and input-kind handling do not exist.

- [ ] **Step 2: Extend the shared types and read video duration safely.**

  Add `durationSeconds?: number` to `PlaygroundAttachment`, `inputKind?: 'image' | 'video'`, and `requiresAttachment?: boolean` to `MediaGenerationProfile`. Add `video-to-music` to `MediaGenerationFamily`; Task 3 defines the request discriminator that carries its `FormData` payload.

  Implement an exported `readPlaygroundVideoDuration(url: string): Promise<number | undefined>` in `attachments.ts`. It must return `undefined` outside a browser, create a metadata-only `<video>`, resolve only finite values in `(0, 3600]`, remove event listeners, clear the source, and revoke any temporary object URL. In the MP4 branch of `normalizePlaygroundAttachments`, await this helper and copy a rounded value into `durationSeconds` when available. Existing image/audio/text normalization stays unchanged. The unit test must replace `document.createElement('video')` with a fake element that invokes `onloadedmetadata` with `duration=12` and verify cleanup; Node test environments must continue to return `undefined` without a DOM.

- [ ] **Step 3: Define the Sonilo profile and validation rules.**

  Add a profile with these exact defaults and fields:

  ```ts
  const soniloVideoToMusicProfile: MediaGenerationProfile = {
    kind: 'audio',
    family: 'sonilo-video-to-music',
    inputKind: 'video',
    requiresAttachment: true,
    defaults: {
      duration: 10,
      outputFormat: 'mp3',
      preserveSpeech: false,
      ducking: false,
    },
    fields: [
      { key: 'duration', labelKey: 'Video duration', control: 'number', min: 1, max: 3600, step: 0.1, unitKey: 'seconds' },
      selectField('outputFormat', 'Output format', ['mp3', 'm4a', 'wav']),
      { key: 'preserveSpeech', labelKey: 'Preserve speech', control: 'switch' },
      { key: 'ducking', labelKey: 'Ducking', control: 'switch' },
    ],
  }
  ```

  Add `preserveSpeech` and `ducking` to `MediaParameterKey`. Resolve the exact normalized model name before TTS detection, and remove only Sonilo’s exact-name rejection; keep ElevenLabs/native audio/task patterns unsupported. For this profile, validation must require exactly one `video` attachment and reject every other attachment kind.

- [ ] **Step 4: Build a multipart request without overriding the boundary.**

  Add `buildSoniloVideoToMusicFormData` that appends `model`, `group`, `video_url`, `prompt` (when non-empty), `duration_seconds`, `output_format`, `mode=async`, `preserve_speech`, `ducking`, and `variants_num=1`. Use `attachment.durationSeconds` before the profile’s `duration` setting; return `undefined` when the selected video has no safe URL or no valid duration. Return:

  ```ts
  {
    kind: 'video-to-music',
    endpoint: '/pg/video-to-music',
    payload: formData,
  }
  ```

  Keep existing JSON request payloads and endpoints byte-for-byte unchanged. `normalizeMediaGenerationSettings` must clamp the new number field and coerce switches exactly like existing media controls.

- [ ] **Step 5: Make the input control honor `inputKind`.**

  In `playground-input.tsx`, choose the video-only accept string when `mediaProfile.inputKind === 'video'`, and disable the attach button only for audio profiles without an input kind. Keep TTS audio profiles attachment-free. The existing submit path may still be invoked without a file; `validateMediaGenerationAttachments` must reject that submission before upload or billing.

- [ ] **Step 6: Run profile and attachment tests.**

  ```text
  bun test src/features/playground/lib/attachments.test.ts src/features/playground/lib/media-generation.test.ts src/features/playground/lib/playground-model-filter.test.ts src/features/playground/components/playground-input.test.tsx --max-concurrency=1
  ```

  Expected result: all new profile, duration, validation, and picker tests pass while existing media profile tests remain green.

## Task 3: Add typed multipart transport and Sonilo task response parsing

**Files:**
- Modify: `web/default/src/features/playground/lib/media-generation.ts`, `web/default/src/features/playground/api.ts`, `web/default/src/features/playground/lib/media-response.ts`
- Test: `web/default/src/features/playground/lib/media-generation.test.ts`, `web/default/src/features/playground/api.test.ts`, `web/default/src/features/playground/lib/media-response.test.ts`

- [ ] **Step 1: Add failing transport/parser tests.**

  Assert that `buildMediaGenerationRequest` returns a `FormData` payload whose entries include `video_url`, `duration_seconds`, `mode`, and `variants_num`, and that `sendMediaGeneration` passes FormData without a forced JSON `Content-Type`. Add parser fixtures for:

  ```json
  {"task_id":"task_1","status":"processing"}
  {"task_id":"task_1","status":"succeeded","audio":[{"url":"/v1/video-to-music/task_1/content?variant=0","content_type":"audio/mpeg"}]}
  {"task_id":"task_1","status":"failed","error":{"message":"task failed"}}
  ```

  The success state must expose an audio list with URL and MIME type; unsafe URLs and completed responses with no audio must not be treated as successful media.

- [ ] **Step 2: Introduce the request discriminator and API endpoint.**

  Extend `MediaGenerationRequest` with a `video-to-music` variant. Use the existing image endpoint literals explicitly so the type is self-contained:

  ```ts
  type MediaGenerationRequest =
    | { kind: 'image'; endpoint: '/pg/images/generations' | '/pg/images/edits'; payload: Record<string, unknown> }
    | { kind: 'audio'; endpoint: '/pg/audio/speech'; payload: Record<string, unknown> }
    | { kind: 'video'; endpoint: '/pg/videos'; payload: Record<string, unknown> }
    | { kind: 'video-to-music'; endpoint: '/pg/video-to-music'; payload: FormData }
  ```

  Update `sendMediaGeneration` to return JSON for `video-to-music`, set `responseType: 'blob'` only for synchronous TTS audio, and leave the Axios-generated multipart boundary intact. Add `fetchPlaygroundVideoToMusicTask(taskId, signal)` for `GET /pg/video-to-music/:task_id`.

- [ ] **Step 3: Implement the Sonilo parser.**

  Export `parseVideoToMusicTaskResponse` with a state shape `{ taskId, status, progress?, audio?: GeneratedMedia[], error? }`. Accept direct and `{ data: ... }` roots, map `queued/pending`, `processing/running/in_progress`, `succeeded/completed/success`, and `failed/failure/cancelled/canceled`, and sanitize every returned URL with `sanitizeGeneratedMediaUrl`. Convert `content_type` to `mimeType`; reject a successful state when `audio` is empty or every URL is unsafe.

- [ ] **Step 4: Run transport and parser tests.**

  ```text
  bun test src/features/playground/lib/media-generation.test.ts src/features/playground/api.test.ts src/features/playground/lib/media-response.test.ts --max-concurrency=1
  ```

  Expected result: all FormData and response-shape tests pass, with existing image/TTS/video request tests unchanged.

## Task 4: Poll Sonilo tasks and render/resume audio messages

**Files:**
- Modify: `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/hooks/use-media-generation.ts`
- Test: `web/default/src/features/playground/hooks/use-media-generation.test.ts`, `web/default/src/features/playground/components/playground-chat.test.tsx`

- [ ] **Step 1: Add failing lifecycle tests.**

  Add a Sonilo generation fixture where POST returns `processing`, the dedicated GET returns `succeeded` with one proxy audio URL, and the assistant version ends with:

  ```ts
  generatedMedia: [{
    type: 'audio',
    url: '/v1/video-to-music/task_1/content?variant=0',
    mimeType: 'audio/mpeg',
  }]
  ```

  Assert that the assistant stores `videoTaskId` plus `mediaTaskType: 'video-to-music'` while waiting, clears both on completion/failure/stop, and that reload recovery calls the Sonilo GET helper rather than the ordinary video helper. Add a static chat assertion for `<audio controls>` using the relative proxy URL.

- [ ] **Step 2: Persist task type alongside the existing task ID.**

  Add `mediaTaskType?: 'video' | 'video-to-music'` to `Message`. Update the task cleanup helper to delete both `videoTaskId` and `mediaTaskType`; update progress writes to set the type. Existing messages without the new field default to ordinary video polling, preserving backward compatibility.

- [ ] **Step 3: Implement the dedicated Sonilo polling branch.**

  Keep the existing video poller unchanged. Add a `pollVideoToMusicTask` branch that fetches `fetchPlaygroundVideoToMusicTask`, parses with `parseVideoToMusicTaskResponse`, updates progress, and on success calls:

  ```ts
  completeMedia(messageKey, i18next.t('Audio'), task.audio)
  ```

  In `generateMedia`, dispatch `request.kind === 'video-to-music'` to this branch before the existing image/audio/video branches. On submission, save `mediaTaskType` with the public task ID. On mount, select the newest resumable message and route by `mediaTaskType`.

- [ ] **Step 4: Keep URL and error boundaries safe.**

  Only pass parser-approved relative/HTTPS URLs to `GeneratedMedia`. A completed Sonilo task with no valid audio throws a user-facing “No audio was generated” error. Abort and timeout paths must call the same cleanup helper used by ordinary video tasks; no Sonilo URL or raw provider error is copied into persisted message text.

- [ ] **Step 5: Run lifecycle and rendering tests.**

  ```text
  bun test src/features/playground/hooks/use-media-generation.test.ts src/features/playground/components/playground-chat.test.tsx --max-concurrency=1
  ```

  Expected result: ordinary video and TTS lifecycle tests stay green, and the new Sonilo task renders an actual audio player and resumes correctly after reload.

## Task 5: Integrated verification and one implementation commit

**Files:** `router/relay-router.go`, `controller/playground.go`, `relay/constant/relay_mode.go`, `middleware/distributor.go`, `relay/relay_task.go`, `router/relay_router_playground_test.go`, `relay/constant/relay_mode_test.go`, `service/channel_select_endpoint_test.go`, `relay/relay_task_usage_test.go`, `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/lib/attachments.ts`, `web/default/src/features/playground/lib/media-generation.ts`, `web/default/src/features/playground/components/playground-input.tsx`, `web/default/src/features/playground/lib/attachments.test.ts`, `web/default/src/features/playground/lib/media-generation.test.ts`, `web/default/src/features/playground/lib/playground-model-filter.test.ts`, `web/default/src/features/playground/components/playground-input.test.tsx`, `web/default/src/features/playground/api.ts`, `web/default/src/features/playground/lib/media-response.ts`, `web/default/src/features/playground/api.test.ts`, `web/default/src/features/playground/lib/media-response.test.ts`, `web/default/src/features/playground/hooks/use-media-generation.ts`, `web/default/src/features/playground/hooks/use-media-generation.test.ts`, and `web/default/src/features/playground/components/playground-chat.test.tsx`.

- [ ] **Step 1: Run the complete focused frontend suite.**

  ```text
  bun test src/features/playground --max-concurrency=1
  bun run typecheck
  bun run build:check
  ```

  Expected result: zero failures, type errors, or build-check errors.

- [ ] **Step 2: Run backend focused tests and compile checks.**

  ```text
  go test -p 1 ./router ./relay/constant ./middleware ./service ./relay ./controller -run 'Playground|VideoToMusic|Path2RelayMode' -count=1
  go build ./...
  ```

  Expected result: route, distributor, endpoint selection, task conversion, and controller packages pass; the repository builds successfully.

- [ ] **Step 3: Inspect the final diff and commit once.**

  Run `git diff --check`, `git status --short`, and `git diff --stat`. Confirm only the Sonilo Playground route/profile/parser/task lifecycle and their tests/spec/plan changed. Create one implementation commit using the Lore trailers and do not amend the already-published audio PR commit.

- [ ] **Step 4: Perform the production Playground smoke test after deployment.**

  In the authenticated console, select group `plg`, choose `sonilo-video-to-music`, upload one short MP4, and verify task creation, completion, playable/downloadable audio, reload recovery, and absence of `api.sonilo.com` in the browser-visible response. Record any deployment-only gap instead of claiming live success without this check.
