# Playground Server-Persisted Attachments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Playground image and MP4 attachments survive refreshes and browser-local storage loss by uploading binary bytes to the existing Asset object store, persisting only user-owned asset references, and rehydrating short-lived preview URLs on restore.

**Architecture:** Add a session-authenticated Playground adapter around the existing Asset upload-session/completion service, plus a `playground_record_assets` relation table maintained in the same transaction as each record save/clear. The browser uploads image/video data before dispatching a turn, keeps the returned preview URL only in live state, serializes `asset://<asset_id>` references in durable request snapshots, and requests a fresh signed HTTPS preview URL when restoring. Text attachments remain bounded inline text.

**Tech Stack:** Go 1.22, Gin, GORM (SQLite/MySQL/PostgreSQL), existing `service/asset.go` object-store abstraction, React 19, TypeScript, Axios, Bun/Vitest.

---

## File map and ownership

- Backend DTO/HTTP adapter: `dto/playground_attachment.go`, `controller/playground_attachment.go`, `router/api-router.go`.
- Backend asset preview business logic: `service/playground_attachment.go`, reusing `service.CreateAssetUploadSession`, `service.CompleteAssetUpload`, and `service.SignAssetSourceURL`.
- Durable ownership relation and transaction helpers: `model/playground_record_asset.go`, `model/playground_record.go`, `model/main.go`.
- Browser API/upload/hydration client: `web/default/src/features/playground/api.ts`, `web/default/src/features/playground/constants.ts`, new `web/default/src/features/playground/lib/playground-attachments.ts`, `web/default/src/features/playground/lib/index.ts`.
- Browser serialization and restore: `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/lib/message-utils.ts`, `web/default/src/features/playground/lib/playground-persistence.ts`, `web/default/src/features/playground/hooks/use-playground-persistence.ts`.
- Browser send orchestration: `web/default/src/features/playground/components/playground-input.tsx`, `web/default/src/features/playground/index.tsx`.
- Tests: matching Go controller/model/service tests and Playground Bun tests in `web/default/src/features/playground/`.

## Task 1: Add the authenticated Playground attachment API

**Files:**
- Create: `dto/playground_attachment.go`
- Create: `service/playground_attachment.go`
- Create: `controller/playground_attachment.go`
- Modify: `router/api-router.go`
- Test: `controller/playground_attachment_test.go`, `service/playground_attachment_test.go`

- [ ] **Step 1: Write failing service tests for preview URL ownership and expiry.**

Add a test fixture that migrates `User`, `Asset`, and `AssetUpload`, injects the existing fake object store from `service/asset_storage_test.go`, and creates one active image asset for user 11. Assert that `GetPlaygroundAttachmentPreview(context.Background(), 11, publicID)` returns the same public ID, `image/png`, a non-empty signed URL, and an expiry no later than `SourceExpiresAt`. Assert that user 12 and an expired/unavailable asset return `ErrAssetUploadNotFound` and `ErrAssetExpired` respectively.

```go
func TestGetPlaygroundAttachmentPreviewEnforcesOwnerAndSourceState(t *testing.T) {
	setupAssetTestDB(t)
	asset := createActiveAssetForTest(t, 11, "ast_preview", "Image", "image/png")
	preview, err := GetPlaygroundAttachmentPreview(context.Background(), 11, asset.PublicId)
	require.NoError(t, err)
	require.Equal(t, asset.PublicId, preview.AssetID)
	require.Equal(t, "image/png", preview.ContentType)
	require.NotEmpty(t, preview.PreviewURL)
	require.LessOrEqual(t, preview.ExpiresAt, asset.SourceExpiresAt)

	_, err = GetPlaygroundAttachmentPreview(context.Background(), 12, asset.PublicId)
	require.ErrorIs(t, err, ErrAssetUploadNotFound)
}
```

- [ ] **Step 2: Run the focused service test and confirm the missing symbol failure.**

Run: `go test ./service -run TestGetPlaygroundAttachmentPreviewEnforcesOwnerAndSourceState -count=1`

Expected: FAIL because `GetPlaygroundAttachmentPreview` and its result type do not exist.

- [ ] **Step 3: Define the DTO and service result contract.**

Create the following exact JSON fields; the browser-facing API uses `asset_id` while the TypeScript attachment keeps `assetId`:

```go
package dto

type PlaygroundAttachmentPreviewResponse struct {
	AssetID     string `json:"asset_id"`
	AssetType   string `json:"asset_type"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	PreviewURL  string `json:"preview_url"`
	ExpiresAt   int64  `json:"expires_at"`
}
```

In `service/playground_attachment.go`, add:

```go
type PlaygroundAttachmentPreviewResult struct {
	AssetID     string
	AssetType   string
	ContentType string
	SizeBytes   int64
	PreviewURL  string
	ExpiresAt   int64
}

func GetPlaygroundAttachmentPreview(ctx context.Context, userID int, publicID string) (*PlaygroundAttachmentPreviewResult, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, ErrAssetUploadNotFound
	}
	asset, err := model.GetAssetByPublicIDForUser(userID, publicID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAssetUploadNotFound
	}
	if err != nil {
		return nil, err
	}
	if asset.Status != model.AssetStatusActive || asset.SourceStatus != model.AssetSourceStatusAvailable {
		return nil, ErrAssetExpired
	}
	now := assetNow()
	if asset.SourceExpiresAt > 0 && asset.SourceExpiresAt <= now.Unix() {
		return nil, ErrAssetExpired
	}
	cfg := CurrentAssetStorageConfig()
	previewURL, err := SignAssetSourceURL(ctx, *asset, cfg)
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(cfg.SignedURLTTL).Unix()
	if asset.SourceExpiresAt > 0 && expiresAt > asset.SourceExpiresAt {
		expiresAt = asset.SourceExpiresAt
	}
	return &PlaygroundAttachmentPreviewResult{
		AssetID: asset.PublicId, AssetType: asset.AssetType, ContentType: asset.ContentType,
		SizeBytes: asset.SizeBytes, PreviewURL: previewURL, ExpiresAt: expiresAt,
	}, nil
}

func CompletePlaygroundAttachmentUpload(ctx context.Context, request AssetCompleteUploadRequest) (*PlaygroundAttachmentPreviewResult, error) {
	assetResult, err := CompleteAssetUpload(ctx, request)
	if err != nil {
		return nil, err
	}
	return GetPlaygroundAttachmentPreview(ctx, request.UserID, assetResult.PublicID)
}
```

Extend `AssetCompleteUploadRequest` with `UserID int` (the existing token controller leaves it zero; the completion service uses it only for the Playground wrapper), and keep the existing `CompleteAssetUpload` behavior unchanged.

- [ ] **Step 4: Implement the session-authenticated controller adapter.**

Use `common.GetContextKeyInt(c, constant.ContextKeyUserId)`, `assetUploadOwner(userID)`, and the existing `writeAssetServiceError` mapping. Return `common.ApiSuccess` envelopes so the frontend can use one parser:

```go
func CreatePlaygroundAttachmentUpload(c *gin.Context) {
	var request dto.AssetUploadSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := createPlaygroundAttachmentUploadSession(c.Request.Context(), service.AssetUploadSessionRequest{
		UserID: userID, Owner: assetUploadOwner(userID), AssetType: strings.TrimSpace(request.AssetType),
		ContentType: strings.TrimSpace(request.ContentType), SizeBytes: request.SizeBytes,
	})
	if err != nil { writeAssetServiceError(c, err); return }
	common.ApiSuccess(c, dto.AssetUploadSessionResponse{
		UploadID: result.UploadID, AssetID: result.PublicID, Object: "asset.upload", Status: "pending",
		UploadURL: result.SignedURL, UploadHeaders: result.UploadHeaders, ExpiresAt: result.ExpiresAt,
	})
}

func CompletePlaygroundAttachmentUpload(c *gin.Context) {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := completePlaygroundAttachmentUpload(c.Request.Context(), service.AssetCompleteUploadRequest{
		UploadID: strings.TrimSpace(c.Param("upload_id")), Owner: assetUploadOwner(userID), UserID: userID,
	})
	if err != nil { writeAssetServiceError(c, err); return }
	common.ApiSuccess(c, dto.PlaygroundAttachmentPreviewResponse{
		AssetID: result.AssetID, AssetType: result.AssetType, ContentType: result.ContentType,
		SizeBytes: result.SizeBytes, PreviewURL: result.PreviewURL, ExpiresAt: result.ExpiresAt,
	})
}

func GetPlaygroundAttachmentPreview(c *gin.Context) {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := getPlaygroundAttachmentPreview(c.Request.Context(), userID, c.Param("asset_id"))
	if err != nil { writeAssetServiceError(c, err); return }
	common.ApiSuccess(c, dto.PlaygroundAttachmentPreviewResponse{
		AssetID: result.AssetID, AssetType: result.AssetType, ContentType: result.ContentType,
		SizeBytes: result.SizeBytes, PreviewURL: result.PreviewURL, ExpiresAt: result.ExpiresAt,
	})
}
```

Define injectable package variables for all three service calls so controller tests can assert user/owner forwarding without touching object storage.

- [ ] **Step 5: Register routes with session auth, route tags, and upload throttling.**

Add beside the records group in `router/api-router.go`:

```go
	playgroundAttachmentRoute := apiRouter.Group("/playground/attachments")
	playgroundAttachmentRoute.Use(middleware.RouteTag("playground-attachments"), middleware.UserAuth())
	{
		playgroundAttachmentRoute.POST("/uploads", middleware.UploadRateLimit(), controller.CreatePlaygroundAttachmentUpload)
		playgroundAttachmentRoute.POST("/uploads/:upload_id/complete", middleware.UploadRateLimit(), controller.CompletePlaygroundAttachmentUpload)
		playgroundAttachmentRoute.GET("/:asset_id/preview", controller.GetPlaygroundAttachmentPreview)
	}
```

- [ ] **Step 6: Add controller tests for malformed requests, owner scoping, and response envelopes.**

Test `POST /api/playground/attachments/uploads` with an authenticated user and a stubbed session creator; assert the owner is exactly `user-11`, no client owner is forwarded, and the response has `success: true` and `data.asset_id`. Test empty `upload_id` and a foreign preview ID return the existing asset-not-found error code.

- [ ] **Step 7: Run backend API tests.**

Run: `go test ./controller ./service -run 'PlaygroundAttachment|AssetUpload' -count=1`

Expected: PASS.

## Task 2: Persist Playground-to-Asset references transactionally

**Files:**
- Create: `model/playground_record_asset.go`
- Modify: `model/playground_record.go`, `controller/playground_record.go`, `model/main.go`
- Test: `model/playground_record_asset_test.go`, `controller/playground_record_test.go`, `model/playground_record_test.go`

- [ ] **Step 1: Write failing model tests for duplicate references, owner/type validation, and clear cleanup.**

Add `PlaygroundRecordAsset` to the SQLite test migration and create active image/video assets for two users. Save a record with repeated image IDs and assert one relation row per `(record_id, asset_id)`. Assert a foreign-user asset and an image referenced as `video` return explicit errors. Save a second record sharing the image, clear the first conversation, and assert the shared asset still has one relation while the first conversation has none.

```go
func TestPlaygroundRecordAssetReferencesAreIdempotentAndScoped(t *testing.T) {
	setupPlaygroundRecordAssetDB(t)
	image := createActiveAssetForTest(t, 11, "ast_image", "Image", "image/png")
	record := playgroundRecordWithAssets(11, "record-1", "conversation-1", []PlaygroundAssetReference{{AssetID: image.PublicId, AssetType: "Image"}, {AssetID: image.PublicId, AssetType: "Image"}})
	require.NoError(t, SavePlaygroundRecord(record))
	require.NoError(t, SavePlaygroundRecord(record))
	var count int64
	DB.Model(&PlaygroundRecordAsset{}).Where("record_id = ?", record.RecordID).Count(&count)
	require.EqualValues(t, 1, count)
}
```

- [ ] **Step 2: Run the focused model test and confirm the missing relation model failure.**

Run: `go test ./model -run TestPlaygroundRecordAssetReferencesAreIdempotentAndScoped -count=1`

Expected: FAIL because the relation model and reference field are not defined.

- [ ] **Step 3: Define the cross-database relation model and reference type.**

Create `model/playground_record_asset.go`:

```go
package model

import "time"

type PlaygroundAssetReference struct {
	AssetID   string
	AssetType string
}

type PlaygroundRecordAsset struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	UserID         int       `json:"user_id" gorm:"not null;index:idx_playground_asset_ref_user_conversation,priority:1;index:idx_playground_asset_ref_asset,priority:1"`
	ConversationID  string    `json:"conversation_id" gorm:"type:varchar(64);not null;index:idx_playground_asset_ref_user_conversation,priority:2"`
	RecordID       string    `json:"record_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_playground_asset_ref_record_asset,priority:1"`
	AssetID        string    `json:"asset_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_playground_asset_ref_record_asset,priority:2;index:idx_playground_asset_ref_asset,priority:2"`
	CreatedAt      time.Time `json:"created_at"`
}
```

Add `AssetReferences []PlaygroundAssetReference `gorm:"-" json:"-"`` to `PlaygroundRecord`.

- [ ] **Step 4: Implement reference validation and transactional replacement.**

Add these model functions, using GORM queries only:

```go
func ValidatePlaygroundAssetReferences(userID int, refs []PlaygroundAssetReference) error
func ReplacePlaygroundRecordAssets(tx *gorm.DB, record *PlaygroundRecord) error
func DeletePlaygroundConversationAssetReferences(tx *gorm.DB, userID int, conversationID string) ([]string, error)
```

`ValidatePlaygroundAssetReferences` deduplicates IDs, rejects IDs not beginning with `ast_`, loads `Asset` rows by `user_id` and `public_id`, requires `Status == AssetStatusActive` and `SourceStatus == AssetSourceStatusAvailable`, and compares `AssetType` (`Image`/`Video`) when the reference supplies one. It returns `ErrAssetUploadNotFound`, `ErrAssetExpired`, or `ErrAssetTypeMismatch` from the existing service package through a model-local error type is forbidden by the package dependency direction; therefore define model errors `ErrPlaygroundAssetNotFound`, `ErrPlaygroundAssetExpired`, and `ErrPlaygroundAssetTypeMismatch`, and map them in the controller.

`ReplacePlaygroundRecordAssets` first checks `DB.Migrator().HasTable(&PlaygroundRecordAsset{})` for legacy test databases, deletes rows for the record/user, then inserts sorted unique references with `clause.OnConflict{DoNothing: true}`. The function runs on the transaction supplied by `SavePlaygroundRecord`.

- [ ] **Step 5: Wire relation replacement into every record-save path.**

In `SavePlaygroundRecord`, call `ReplacePlaygroundRecordAssets(tx, record)` immediately after each successful `Create` and after `updateExistingPlaygroundRecord`. Preserve relation rows when an older record is stored with an empty snapshot; its attachment references still belong to that record.

Update `updateExistingPlaygroundRecord` to call the helper after its `Updates` succeeds, while retaining the existing “only latest records keep snapshots” rule.

- [ ] **Step 6: Wire relation deletion into clear and expose cleanup IDs.**

Add `ClearPlaygroundConversationWithAssets(...) ([]string, error)` containing the current clear transaction. Before returning from the transaction, call `DeletePlaygroundConversationAssetReferences(tx, userID, conversationID)` and return the deduplicated public IDs. Keep the existing `ClearPlaygroundConversation(...) error` as a compatibility wrapper that discards the slice so current callers/tests remain source-compatible.

The controller calls the new function and schedules best-effort orphan cleanup through a new service function that only deletes an asset when `playground_record_assets` count is zero and `AssetBinding` count is zero. Object-store deletion failures leave `SourceStatusCleanupPending` for the existing cleanup worker; a failed cleanup must not turn a successful conversation clear into an HTTP failure.

- [ ] **Step 7: Extract and validate IDs from all persisted JSON fields.**

In `controller/playground_record.go`, recursively inspect `user_message`, `request_messages`, `assistant_message`, and `messages_snapshot`. Accept both `assetId` and `asset_id`; require string values; infer expected type from sibling `kind`, `image_url`, or `video_url`. Reject malformed IDs before saving, call `model.ValidatePlaygroundAssetReferences`, and attach the sorted refs to `record.AssetReferences`.

Also sanitize the server-bound JSON copy: when an attachment object has an `assetId`/`asset_id`, remove its binary `url`; in request content, retain only `asset://<asset_id>` for an asset-backed image/video part. Keep legacy non-base64 HTTPS URLs accepted when no durable asset ID is present. Existing recursive rejection of `b64_json` and `data:*;base64` remains unchanged.

- [ ] **Step 8: Register the relation table in ordered migrations.**

Insert `{&PlaygroundRecordAsset{}, "PlaygroundRecordAsset"}` immediately after `{&PlaygroundRecord{}, "PlaygroundRecord"}` in `model/orderedMigrationModels()`. Do not add foreign-key constraints that require a migration order unavailable on all three supported databases; ownership and cleanup are enforced in transaction code.

- [ ] **Step 9: Add controller/model regression tests.**

Extend `controller/playground_record_test.go` with an authenticated save containing `asset_id` (PASS), a base64 URL (HTTP 400 with “embedded base64 media is not allowed”), a foreign asset (HTTP 200 business error with the mapped asset-not-found code), and clear cleanup. Extend `model/playground_record_test.go` migration setup to include the relation model and assert existing idempotency/current-record tests still pass.

- [ ] **Step 10: Run focused backend tests.**

Run: `go test ./model ./controller -run 'PlaygroundRecord|PlaygroundAsset' -count=1`

Expected: PASS on SQLite.

## Task 3: Build the browser upload and preview client

**Files:**
- Modify: `web/default/src/features/playground/types.ts`, `web/default/src/features/playground/constants.ts`, `web/default/src/features/playground/api.ts`, `web/default/src/features/playground/lib/index.ts`
- Create: `web/default/src/features/playground/lib/playground-attachments.ts`
- Test: `web/default/src/features/playground/api.test.ts`, `web/default/src/features/playground/lib/playground-attachments.test.ts`

- [ ] **Step 1: Write failing client tests for upload ordering and preview hydration.**

Mock Axios and `globalThis.fetch`. For one PNG attachment, assert the order `POST /uploads` → signed `PUT` → `POST /complete`, and assert the mapped attachment has `assetId` plus `preview_url`. For two messages sharing one asset ID, assert hydration calls the preview endpoint once and assigns the returned URL to both attachments. Assert a rejected preview request leaves metadata and clears the stale URL.

```ts
test('uploads binary attachments in session, PUT, complete order', async () => {
  mockApiPost.mockResolvedValueOnce({ data: { success: true, data: {
    upload_id: 'upl_1', asset_id: 'ast_1', upload_url: 'https://upload.test/1',
    upload_headers: { 'Content-Type': 'image/png' }, expires_at: 2000,
  } } })
  globalThis.fetch = mockFetchReturning(200)
  mockApiPost.mockResolvedValueOnce({ data: { success: true, data: {
    asset_id: 'ast_1', asset_type: 'Image', content_type: 'image/png',
    size_bytes: 2, preview_url: 'https://preview.test/1', expires_at: 3000,
  } } })

  await expect(uploadPlaygroundAttachments([pngAttachment()])).resolves.toEqual([
    expect.objectContaining({ assetId: 'ast_1', url: 'https://preview.test/1' }),
  ])
  expect(mockApiPost.mock.calls.map(([url]) => url)).toEqual([
    API_ENDPOINTS.PLAYGROUND_ATTACHMENT_UPLOADS,
    `${API_ENDPOINTS.PLAYGROUND_ATTACHMENT_UPLOADS}/upl_1/complete`,
  ])
  expect(globalThis.fetch).toHaveBeenCalledWith('https://upload.test/1', expect.objectContaining({ method: 'PUT' }))
})
```

- [ ] **Step 2: Run the focused Bun tests and confirm missing client symbols.**

Run: `bun test src/features/playground/lib/playground-attachments.test.ts`

Expected: FAIL because `uploadPlaygroundAttachments` and attachment API functions do not exist.

- [ ] **Step 3: Extend the attachment type and endpoint constants.**

Add `assetId?: string` to `PlaygroundAttachment`. Add:

```ts
PLAYGROUND_ATTACHMENT_UPLOADS: '/api/playground/attachments/uploads',
PLAYGROUND_ATTACHMENT_PREVIEW: '/api/playground/attachments',
```

- [ ] **Step 4: Add API envelope parsers and signed-upload calls.**

In `api.ts`, add typed functions:

```ts
export interface PlaygroundAttachmentUploadSession {
  upload_id: string
  asset_id: string
  upload_url: string
  upload_headers: Record<string, string>
  expires_at: number
}

export interface PlaygroundAttachmentPreview {
  asset_id: string
  asset_type: string
  content_type: string
  size_bytes: number
  preview_url: string
  expires_at: number
}

export async function createPlaygroundAttachmentUploadSession(input: {
  assetType: 'Image' | 'Video'
  contentType: string
  sizeBytes: number
}): Promise<PlaygroundAttachmentUploadSession>

export async function completePlaygroundAttachmentUpload(uploadID: string): Promise<PlaygroundAttachmentPreview>

export async function getPlaygroundAttachmentPreview(assetID: string): Promise<PlaygroundAttachmentPreview>
```

Each Axios call uses `skipErrorHandler: true`, asserts the `{success,data}` envelope, and validates required string/number fields before returning. The direct object-store PUT uses native `fetch` with `credentials: 'omit'`, the server-provided headers, `method: 'PUT'`, and the caller's `AbortSignal`.

- [ ] **Step 5: Implement data-URL decoding, sequential upload, and deduplicated hydration.**

Create `playground-attachments.ts` with:

```ts
export async function uploadPlaygroundAttachments(
  attachments: PlaygroundAttachment[],
  signal?: AbortSignal
): Promise<PlaygroundAttachment[]>

export async function hydratePlaygroundMessages(messages: Message[]): Promise<Message[]>
```

Text attachments and attachments already carrying `assetId` pass through. For each new image/video, decode a `data:` URL with `atob` into a `Blob`, create a session using `Image`/`Video`, PUT the blob, complete the session, and return `{ ...attachment, assetId: result.asset_id, url: result.preview_url }`. Process files with `for...of` so each session/PUT/complete sequence is deterministic. `hydratePlaygroundMessages` gathers unique asset IDs, calls preview APIs with `Promise.allSettled`, replaces URLs for fulfilled previews, and removes stale URLs for rejected previews without dropping filename, kind, media type, or asset ID.

- [ ] **Step 6: Export the client and run tests.**

Export the new functions from `lib/index.ts`, then run: `bun test src/features/playground/api.test.ts src/features/playground/lib/playground-attachments.test.ts`

Expected: PASS.

## Task 4: Make durable serialization safe and restore previews

**Files:**
- Modify: `web/default/src/features/playground/lib/message-utils.ts`, `web/default/src/features/playground/lib/playground-persistence.ts`, `web/default/src/features/playground/hooks/use-playground-persistence.ts`
- Test: `web/default/src/features/playground/lib/message-utils.test.ts`, `web/default/src/features/playground/lib/playground-persistence.test.ts`

- [ ] **Step 1: Add failing tests for signed live URLs and sanitized durable payloads.**

Assert `buildMessageContent` accepts `https://preview.test/image` for an image and video while still rejecting `http://`, malformed URLs, and embedded base64. Build a record whose attachment is `{ assetId: 'ast_1', url: 'https://preview.test/image' }`; assert `messages_snapshot[*].versions[*].attachments[*].url` is absent and the matching request part is `{ type: 'image_url', image_url: { url: 'asset://ast_1' } }`. Assert a data URL with no asset ID is removed from the persisted request and message snapshot.

- [ ] **Step 2: Run focused frontend tests and confirm current behavior fails.**

Run: `bun test src/features/playground/lib/message-utils.test.ts src/features/playground/lib/playground-persistence.test.ts`

Expected: FAIL because remote signed URLs are currently rejected and request sanitization drops rather than converts asset references.

- [ ] **Step 3: Expand safe media URL handling in `message-utils.ts`.**

Keep the existing strict data URL regexes and add an HTTPS check:

```ts
function isSafeRemoteMediaUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && url.hostname.length > 0
  } catch {
    return false
  }
}
```

Use `isSafeRemoteMediaUrl` OR the existing safe data URL regex for image/video parts. Do not allow `asset://` in live ordinary chat requests; the Playground chat endpoint receives signed HTTPS URLs.

- [ ] **Step 4: Rewrite persistence sanitization around asset IDs.**

Keep `sanitizePersistedText` and base64 rejection compatibility. Add a URL-to-asset map collected from all message versions. `sanitizeAttachment` removes `url` for any binary attachment with `assetId` and for any embedded data URL; it preserves `assetId`, filename, media type, and kind. `sanitizeRequestMessage` replaces a matching binary URL with `asset://<assetId>`, removes unmatched data URLs, and preserves legacy non-base64 HTTPS URLs. Make `sanitizePlaygroundRecordPayload` exported for direct tests.

Change attachment merge semantics so a server attachment with `assetId` counts as complete data and is never overwritten by an older local data URL. A server attachment without `assetId` may still receive a matching local URL only for the existing same-conversation legacy fallback.

- [ ] **Step 5: Hydrate restored messages before local merge.**

In `use-playground-persistence.ts`, import `hydratePlaygroundMessages` and call it immediately after `restorePlaygroundSession` returns `result.current`, before `mergeRestoredMessages`:

```ts
const hydratedMessages = await hydratePlaygroundMessages(result.current.messages)
const restoredMessages =
  result.current.conversation_id === conversationIdRef.current
    ? mergeRestoredMessages(hydratedMessages, messagesRef.current)
    : hydratedMessages
```

If a preview request fails, keep the record and metadata in state; only the `url` is absent, allowing the next restore or explicit retry to request it again.

- [ ] **Step 6: Run persistence tests and typecheck the feature.**

Run: `bun test src/features/playground/lib/message-utils.test.ts src/features/playground/lib/playground-persistence.test.ts`

Expected: PASS.

## Task 5: Upload before send without losing retryable staged files

**Files:**
- Modify: `web/default/src/features/playground/components/playground-input.tsx`, `web/default/src/features/playground/index.tsx`
- Test: `web/default/src/features/playground/components/playground-input.test.tsx`, `web/default/src/features/playground/index.test.tsx`

- [ ] **Step 1: Write failing UI tests for async upload success and failure.**

Mock `uploadPlaygroundAttachments`. Submit a PNG with a chat model and assert the user message is created only after the mock resolves, contains `assetId` and preview URL, and dispatches one chat turn. Reject the mock and assert no user/assistant messages are added, the staged attachment remains in `PromptInput`, and a localized retryable toast is shown.

- [ ] **Step 2: Run the focused UI tests and confirm the synchronous callback contract fails.**

Run: `bun test src/features/playground/components/playground-input.test.tsx src/features/playground/index.test.tsx`

Expected: FAIL because `PlaygroundInputProps.onSubmit` returns only `void` and `handleSendMessage` does not await an upload phase.

- [ ] **Step 3: Make the input callback promise-aware.**

Change `PlaygroundInputProps.onSubmit` to return `void | Promise<void>`. Keep `handleSubmit` async, `await onSubmit(...)`, and rethrow upload errors so the existing `PromptInput` implementation does not clear files on rejection. Keep the media-generation attachment guard before upload.

- [ ] **Step 4: Add the upload phase and gate state to `index.tsx`.**

Import `uploadPlaygroundAttachments`, add `isUploadingAttachments` state, and make `handleSendMessage` async. Call `prepareSend` first; then await binary upload, create the user/assistant messages with the uploaded attachment result, and dispatch generation. On failure, set `generationDispatchRef.current = false`, show `toast.error`, and rethrow so staged files remain. Pass `isGenerating || isUploadingAttachments` to `PlaygroundInput` and `PlaygroundChat`; disable model/attachment controls during upload.

```ts
const handleSendMessage = useCallback(async (text, model, attachments = []) => {
  const targetModel = model || config.model
  const attachmentError = validateMediaGenerationAttachments(targetModel, attachments)
  if (attachmentError) { toast.error(i18next.t(attachmentError)); return }
  if (!prepareSend(targetModel)) return

  setIsUploadingAttachments(attachments.some((item) => item.kind !== 'text' && !item.assetId))
  try {
    const durableAttachments = await uploadPlaygroundAttachments(attachments)
    clearModelGeneratorDraft()
    clearPlaygroundHandoffSearch()
    const userMessage = createUserMessage(text, durableAttachments)
    if (model) { setUserPickedModel(true); updateConfig('model', model) }
    const assistantMessage = createLoadingAssistantMessage()
    const newMessages = [...messages, userMessage, assistantMessage]
    updateMessages(newMessages)
    dispatchGeneration(text, newMessages, assistantMessage.key, model)
  } catch (error) {
    generationDispatchRef.current = false
    toast.error(error instanceof Error ? error.message : i18next.t('Unable to upload attachment'))
    throw error
  } finally {
    setIsUploadingAttachments(false)
  }
}, [
  clearModelGeneratorDraft,
  clearPlaygroundHandoffSearch,
  config.model,
  dispatchGeneration,
  messages,
  prepareSend,
  setUserPickedModel,
  updateConfig,
  updateMessages,
])
```

Use the concrete dependency array shown above; `uploadPlaygroundAttachments` is an imported module function and does not enter the array. Do not add a second generation dispatcher.

- [ ] **Step 5: Keep regenerate/edit flows compatible with durable attachments.**

Regenerate uses an existing `assetId` and therefore passes through without re-upload. If a legacy local-only data URL is regenerated, route it through the same async upload function before creating the replacement user message; failed upload leaves the original conversation untouched and releases the dispatch gate. Editing text remains synchronous when no new binary is attached.

- [ ] **Step 6: Run UI tests and TypeScript validation.**

Run: `bun test src/features/playground/components/playground-input.test.tsx src/features/playground/index.test.tsx`

Expected: PASS.

Run: `bun run typecheck`

Expected: exit code 0 with no TypeScript diagnostics.

## Task 6: End-to-end regression verification and documentation

**Files:**
- Modify: `docs/superpowers/specs/2026-08-27-playground-server-attachments-design.md` only if implementation details differ from the chosen contract; otherwise leave it unchanged.
- Test: existing Go and Bun suites plus the local browser smoke path.

- [ ] **Step 1: Run all targeted backend and frontend checks.**

Run from repository root:

```powershell
go test ./dto/... ./model/... ./service/... ./controller/...
```

Run from `web/default/`:

```powershell
bun test src/features/playground
bun run typecheck
bun run lint
bun run build:check
```

Expected: all commands exit 0. If an unrelated pre-existing failure appears, record the exact package, test name, and output in the final report and continue with the narrowest affected verification.

- [ ] **Step 2: Start the local preview and exercise the authenticated browser flow.**

Start the Go API with `go run .` from the repository root and the default-theme preview with `bun run dev --host 127.0.0.1` from `web/default/` (use the next free port if either default port is occupied). In the browser, authenticate, attach a small PNG and MP4, send each through a chat-capable model, reload after the assistant response, confirm both previews render from newly requested signed URLs, then clear the conversation and reload to confirm no current record remains. Inspect the record-save request in DevTools and verify it contains `assetId`/`asset://` references but no `data:*;base64` and no signed preview URL.

- [ ] **Step 3: Run GitNexus change-scope verification before committing.**

Run the repository's GitNexus `detect_changes()` check (or the installed CLI equivalent) against `origin/main`. Confirm only the declared controller/service/model/router/DTO/Playground frontend and test files changed; investigate any unrelated execution flow before committing.

- [ ] **Step 4: Commit with the repository Lore protocol.**

Use a commit message whose first line states why the change exists, followed by the required trailers:

```text
Keep Playground media durable across browser refreshes

Constraint: binary bytes stay in the existing Asset object store and persisted records contain no base64 or signed URL
Rejected: browser-only data URLs | they disappear when local storage is cleared
Confidence: high
Scope-risk: moderate
Directive: preserve server-side ownership checks for upload, preview, record references, and clear cleanup
Tested: Go controller/model/service tests, Bun Playground tests, typecheck, lint, and build:check
Not-tested: production object-store credentials and cross-browser authenticated smoke flow when unavailable
```

- [ ] **Step 5: Push and open the PR against `main` only after verification.**

Push the feature branch to `SolveaCX/new-api`, create a PR targeting `main`, include the evidence chain (base64/404/refresh symptoms → browser-local contract mismatch → Asset-backed upload/reference/preview fix → tests and smoke path), and note deployment advice: `Router deploy: required` because the shared Go runtime and database migration are part of the API request path; `new-api-console` is also required for the authenticated Playground UI, while `newapi-web` and Terraform are not affected.

## Plan self-review

- Spec coverage: upload session, completion, preview URL, record asset IDs, transactional save/clear references, shared-asset retention, base64 rejection, browser refresh hydration, upload failure retry behavior, storage error propagation, cross-database migration registration, tests, and rollout verification are covered in Tasks 1–6.
- Placeholder scan: no `TBD`, `TODO`, “implement later”, “appropriate”, or “similar to Task N” instructions appear in the task steps; all code snippets use the symbols defined in earlier tasks.
- Type consistency: backend uses `AssetUploadSessionRequest`, `AssetCompleteUploadRequest{UserID}`, and `PlaygroundAttachmentPreviewResult`; frontend maps snake-case API fields to `PlaygroundAttachment.assetId`; persistence uses `asset://` only for durable request snapshots and HTTPS only for live ordinary chat requests.
- Boundary risk: existing token-auth `/v1/assets/*` routes remain unchanged; the new `/api/playground/attachments/*` wrapper is session-authenticated and reuses the existing object-store validation and upload limits.
