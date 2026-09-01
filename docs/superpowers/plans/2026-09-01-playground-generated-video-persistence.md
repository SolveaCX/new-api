# Implementation Plan: Playground generated-video persistence

> **For implementation:** use the subagent-driven-development workflow and
> complete each task in order with a red-green test cycle.

**Goal:** Persist newly generated Playground videos through the existing Asset
upload lifecycle so refresh recovery can hydrate them by stable `assetId`.

**Architecture:** Keep task polling/provider archival unchanged. On completion,
fetch the task's binary content through the existing video content API, create a
temporary browser Blob URL, upload it with `uploadPlaygroundAttachments`, and
store the returned durable ID and signed preview in `GeneratedMedia`.

## Task 1: Lock the video completion contract with tests

**Files:**
- Modify `web/default/src/features/playground/hooks/use-media-generation.test.ts`.

**Work:**
- Mock `fetchVideoContent` alongside the existing task API mocks.
- Add a failing test that expects a completed video to fetch binary content,
  upload a `video` attachment, and commit `assetId`, preview URL, and MIME
  type.
- Add a failing test that makes the upload reject and verifies the temporary
  Blob URL is revoked and the assistant message becomes an error.
- Preserve existing in-flight task and audio behavior tests.

**Verification:**
`bun test web/default/src/features/playground/hooks/use-media-generation.test.ts`
must fail for the new assertions before production code is changed.

## Task 2: Upload completed video output through the durable Asset path

**Files:**
- Modify `web/default/src/features/playground/hooks/use-media-generation.ts`.

**Work:**
- Import `fetchVideoContent`.
- Add a video completion helper in `pollVideoTask` that validates Blob URL
  support, creates/tracks a temporary URL, calls
  `uploadPlaygroundAttachments` with a generated MP4 attachment, and maps the
  returned durable fields into `GeneratedMedia`.
- Revoke the temporary URL on success, abort, and failure; never complete with
  a transient URL after a failed durable upload.
- Keep the existing task URL only as a transport fallback for environments
  where durable browser uploads are unavailable (static render/tests).

**Verification:**
Re-run the Task 1 focused test until it passes, then run the attachment and
persistence test files.

## Task 3: Cover durable video restore/sanitization

**Files:**
- Modify `web/default/src/features/playground/lib/playground-attachments.test.ts`.
- Modify `web/default/src/features/playground/lib/playground-persistence.test.ts`.
- Modify `web/default/src/features/playground/lib/storage.test.ts` only if a
  video-specific regression is needed beyond the existing generated-audio
  cases.

**Work:**
- Add a video generated-media hydration case using `assetId` and a refreshed
  preview response.
- Add a persistence assertion that the video `assetId` remains while its
  signed URL is removed.
- Avoid changing sanitizer semantics for transient URLs or user attachments.

**Verification:**
Run the three focused test files and inspect the serialized payloads.

## Task 4: Review and final verification

**Work:**
- Run frontend typecheck and the focused test suite from the worktree.
- Inspect the diff for scope, URL-secret leakage, and object-URL cleanup.
- Dispatch a code reviewer with the base and head SHAs; address all important
  findings before pushing.
- Commit with the repository's Lore trailers, push
  `fix/playground-generated-video-persistence`, and create a PR targeting
  `main`.

**Commands:**

```powershell
bun test web/default/src/features/playground/hooks/use-media-generation.test.ts web/default/src/features/playground/lib/playground-attachments.test.ts web/default/src/features/playground/lib/playground-persistence.test.ts web/default/src/features/playground/lib/storage.test.ts
bun run typecheck
git diff --check
git push -u origin fix/playground-generated-video-persistence
gh pr create --base main
```
