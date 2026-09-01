# Playground generated-video persistence design

## Goal

Keep a video generated in Playground available after a page refresh, with the
same durable Asset identity used by uploaded reference videos. A successful
generation must not be recorded with only a provider URL or a process-local
Blob URL.

## Existing boundaries

- Video task polling and provider-side archival remain responsible for
  determining whether a task succeeded and for serving its content.
- `uploadPlaygroundAttachments` remains the single browser-to-Asset upload
  path. It already validates the media, creates a signed upload session,
  completes the upload, and returns an owner-scoped preview URL plus `assetId`.
- Playground record persistence already stores generated media and creates the
  ownership edge when an `assetId` is present. Restore hydration already asks
  for a fresh preview by that ID.

## Chosen flow

1. When a video task reaches `completed`, request its binary content through
   `fetchVideoContent(taskId)`. This uses the authenticated/proxied content
   endpoint and does not trust a provider URL returned in task metadata.
2. Create one temporary Blob URL for immediate playback and pass that URL to
   `uploadPlaygroundAttachments` as a video attachment. The upload helper
   enforces the same media validation and ownership checks as user-provided
   reference videos.
3. If the upload succeeds, complete the assistant message with the returned
   `assetId`, fresh preview URL, and canonical MIME type. Revoke the temporary
   Blob URL as soon as the durable preview is available.
4. If the upload is aborted or fails, revoke the temporary URL and do not commit
   a generated-media item containing a transient URL. The existing error path
   clears the in-flight task ID and reports the failure.

## Error and lifecycle rules

- A missing/invalid binary response is a generation failure, not a successful
  message with an unrecoverable URL.
- Aborting while downloading or uploading must leave no tracked Blob URL.
- Existing task-level archive/proxy behavior is unchanged; this change only
  adds the Playground Asset edge after successful content retrieval.
- Existing local-storage and server-record sanitizers remain the source of
  truth: they retain the durable ID and strip signed preview URLs.

## Verification

- Hook tests prove that completed videos are downloaded, uploaded, and stored
  with `assetId`/preview metadata.
- Hook tests prove failed uploads release the temporary URL and surface an
  error.
- Attachment/persistence tests prove a video generated-media item hydrates by
  ID and serializes without its expiring URL.
- Run the focused Playground Bun tests and the frontend typecheck.
