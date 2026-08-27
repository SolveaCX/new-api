# Playground Server-Persisted Attachments Design

## Goal

Keep Playground image and video attachments available after a user clears browser storage, refreshes on another device, or signs in from a new browser. Binary media must live in the existing server-side asset storage; Playground records store only durable asset references and metadata.

## Current context

- `web/default/src/features/playground/lib/attachments.ts` converts selected files to data URLs for the current chat request.
- `web/default/src/features/playground/lib/playground-persistence.ts` deliberately removes embedded base64 media before writing a Playground record.
- `controller/playground_record.go` rejects base64 data URLs at the API boundary.
- `service/asset.go` and `model/asset.go` already provide validated image/video storage in the configured object store, upload sessions, ownership checks, and cleanup state.
- Existing `/v1/assets/*` routes use API-token middleware. The browser Playground uses the authenticated web session, so it needs a web-session wrapper rather than exposing a token-only route to the browser.

## Chosen architecture

Use the existing Asset object store and add a Playground-scoped session-authenticated adapter:

1. The browser stages a file locally for its immediate thumbnail.
2. On send, binary image/video files are uploaded through a Playground endpoint that creates a signed upload session, accepts the direct object-store PUT, and completes the upload.
3. The response supplies a stable `asset_id` and a short-lived signed preview URL. The live message keeps the preview URL only for rendering; its durable attachment shape keeps `asset_id`, filename, media type, and kind.
4. The chat payload builder sends `asset://<asset_id>` for binary references. The existing relay asset-reference machinery can then materialize the user-owned asset for channels that support it. Text attachments remain inline text because they are already bounded and safe to persist.
5. The Playground record stores the attachment metadata plus `asset_id`; it never stores base64 or a signed URL.
6. On restore, the server returns the asset references. The browser requests fresh short-lived preview URLs and hydrates the message before rendering.
7. Clearing a Playground conversation removes its asset references and deletes an asset only when no other user-owned reference remains.

### Durable attachment shape

```ts
interface PlaygroundAttachment {
  kind: 'image' | 'video' | 'text'
  filename: string
  mediaType: string
  assetId?: string       // durable server reference for image/video
  url?: string            // transient local/blob/signed preview URL only
  text?: string          // bounded text attachment content
}
```

`assetId` is the only binary identifier allowed in a persisted Playground record. `url` is stripped when it is a data URL or a signed URL is no longer valid.

## API and data changes

### Playground attachment endpoints

Add session-authenticated routes beside the existing Playground record routes:

- `POST /api/playground/attachments/uploads` — validate `kind`, content type, and size; delegate to `service.CreateAssetUploadSession`; return `upload_id`, `asset_id`, signed PUT URL, required headers, and expiry.
- `POST /api/playground/attachments/uploads/:upload_id/complete` — delegate to `service.CompleteAssetUpload` with the authenticated user owner; return the asset metadata and a short-lived signed GET/preview URL.
- `GET /api/playground/attachments/:asset_id/preview` — verify ownership and return a short-lived signed GET URL (or stream the object when a signed URL cannot be exposed to the browser).

These wrappers must never accept a client-supplied owner or bucket/object key. Existing asset validation and upload rate limits remain authoritative.

### Record references

The JSON record payload remains backward compatible: `asset_id` is an optional attachment field, so old records without it still decode. Before saving a record, the controller/service validates every referenced asset belongs to the authenticated user, is active, and has the expected image/video type.

Add a small ownership/reference table (for example `playground_record_assets`) with:

- `user_id`, `conversation_id`, `record_id`, `asset_id`
- unique `(record_id, asset_id)` and lookup indexes by `(user_id, conversation_id)` and `asset_id`

Save-record and clear-conversation operations update this table transactionally with the Playground record. Asset deletion occurs only when the asset has no remaining references, so assets used by another feature or another record are not removed accidentally.

## Frontend flow

- Extend the attachment normalizer and `PlaygroundInput` submit contract to support an async upload phase.
- Upload image/video files before creating the user message. If any upload fails, do not send the chat request; keep the staged files and show the upstream-safe error.
- Keep a local object URL or returned preview URL for the current render, but make request construction prefer `asset://assetId`.
- Persist only the durable message shape through `buildPlaygroundRecordPayload` and the browser outbox.
- During `usePlaygroundPersistence` restore, resolve preview URLs for returned `assetId` values, then apply the existing same-conversation local merge only for legacy records that still contain local data URLs.
- If preview resolution fails, retain the attachment metadata and show a retryable unavailable-preview state rather than silently deleting the attachment.

## Lifecycle and failure handling

- Upload failure: block submission, preserve staged files, and show a localized retryable error.
- Record save failure after a successful upload: the outbox retries the record; the uploaded asset remains durable and is linked when the record eventually succeeds. A background orphan cleanup job removes unreferenced uploads after a grace period.
- Browser refresh or local-storage deletion: restore record metadata and resolve fresh preview URLs from the server.
- Conversation clear: server-side reference cleanup is authoritative; the client clears its local state only after the clear request succeeds.
- Asset storage disabled or unavailable: binary attachment submission fails explicitly with a storage-unavailable message instead of claiming the conversation is durable.
- Signed preview URLs are short-lived and never written to records, logs, or localStorage.

## Security and limits

- Enforce authenticated user ownership on upload, preview, record validation, and cleanup.
- Reuse the existing MIME sniffing, per-type size limits, signed URL, and upload-rate-limit checks in `service/asset.go`.
- Do not expose bucket names, object keys, provider credentials, or permanent public URLs.
- Keep text attachment limits unchanged; do not upload text unless a later requirement needs cross-device raw-file download.

## Testing strategy

### Backend

- Controller tests for upload-session, complete, preview ownership, invalid type/size, and missing-storage responses.
- Model/service tests for record-reference insertion, duplicate idempotency, cross-user rejection, clear cleanup, and shared-asset retention.
- Regression test that a record containing `asset_id` passes validation while a base64 URL remains rejected.

### Frontend

- Attachment upload client tests for request order, completion response mapping, and failure preventing send.
- Payload-builder tests asserting `asset://` references are sent and base64 is never sent to record persistence.
- Restore tests asserting asset IDs survive a server snapshot and preview URLs are rehydrated; legacy local-only data continues to use the current merge fallback.
- Browser smoke test covering upload → send → reload → preview → clear, once the QA WIF environment is available.

## Rollout and migration

Ship the server API and reference table before enabling the frontend upload path. Existing local-only messages remain readable through the current compatibility merge; they cannot be recovered after browser data is already deleted because their bytes were never uploaded. New binary attachments become server-durable at send time. Orphan cleanup should run after a grace period so transient upload/record retries are safe.

