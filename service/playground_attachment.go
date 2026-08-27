package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// PlaygroundAttachmentPreviewResult is the browser-facing representation of a
// stored Playground attachment. PreviewURL is intentionally short-lived; the
// durable identifier is AssetID.
type PlaygroundAttachmentPreviewResult struct {
	AssetID     string
	AssetType   string
	ContentType string
	SizeBytes   int64
	PreviewURL  string
	ExpiresAt   int64
}

// PlaygroundAttachmentCleanupResult reports the best-effort cleanup work
// started after a conversation clear. A failed object-store delete is left in
// CleanupPending so the existing database-backed cleanup worker can retry it.
type PlaygroundAttachmentCleanupResult struct {
	Claimed int
	Deleted int
}

// GetPlaygroundAttachmentPreview verifies session ownership and signs a fresh
// read URL for an active, recoverable asset. It deliberately does not expose
// bucket names or object keys to callers.
func GetPlaygroundAttachmentPreview(ctx context.Context, userID int, publicID string) (*PlaygroundAttachmentPreviewResult, error) {
	publicID = strings.TrimSpace(publicID)
	if userID <= 0 || publicID == "" {
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
	if strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(asset.StorageBucket) == "" || strings.TrimSpace(asset.ObjectKey) == "" {
		return nil, ErrAssetStorageDisabled
	}
	previewURL, err := SignAssetSourceURL(ctx, *asset, cfg)
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(cfg.SignedURLTTL).Unix()
	if asset.SourceExpiresAt > 0 && expiresAt > asset.SourceExpiresAt {
		expiresAt = asset.SourceExpiresAt
	}
	return &PlaygroundAttachmentPreviewResult{
		AssetID:     asset.PublicId,
		AssetType:   asset.AssetType,
		ContentType: asset.ContentType,
		SizeBytes:   asset.SizeBytes,
		PreviewURL:  previewURL,
		ExpiresAt:   expiresAt,
	}, nil
}

// CompletePlaygroundAttachmentUpload composes the existing validated upload
// completion flow with a fresh preview URL for the authenticated Playground.
func CompletePlaygroundAttachmentUpload(ctx context.Context, request AssetCompleteUploadRequest) (*PlaygroundAttachmentPreviewResult, error) {
	if request.UserID <= 0 {
		return nil, ErrAssetUploadNotFound
	}
	assetResult, err := CompleteAssetUpload(ctx, request)
	if err != nil {
		// Completion is intentionally idempotent for the Playground adapter. A
		// client can lose the response after the CAS has activated the asset and
		// retry the same request; the generic token-facing service correctly
		// rejects a non-pending upload, but the session adapter can safely recover
		// the already-completed, owner-scoped asset and issue a fresh preview.
		if !errors.Is(err, ErrAssetUploadValidation) {
			return nil, err
		}
		upload, lookupErr := model.GetAssetUploadForOwner(request.UploadID, request.Owner)
		if lookupErr != nil || upload.UserId != request.UserID || upload.Status != model.AssetUploadStatusComplete {
			return nil, err
		}
		return GetPlaygroundAttachmentPreview(ctx, request.UserID, upload.PublicId)
	}
	return GetPlaygroundAttachmentPreview(ctx, request.UserID, assetResult.PublicID)
}

// CleanupPlaygroundAssets removes explicitly-cleared assets that no longer
// have a Playground reference or provider binding. It is intentionally
// idempotent and safe to call from a background goroutine after the clear
// transaction has committed.
func CleanupPlaygroundAssets(ctx context.Context, userID int, publicIDs []string) (PlaygroundAttachmentCleanupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if userID <= 0 || len(publicIDs) == 0 {
		return PlaygroundAttachmentCleanupResult{}, nil
	}
	suffix, _ := assetRandomSuffix()
	owner := fmt.Sprintf("playground-%d-%s", userID, suffix)
	if len(owner) > 64 {
		owner = owner[:64]
	}
	now := assetNow()
	claimed, err := model.ClaimPlaygroundAssetsForCleanup(
		userID,
		publicIDs,
		owner,
		now.Unix(),
		now.Add(assetCleanupLeaseTTL).Unix(),
	)
	if err != nil {
		return PlaygroundAttachmentCleanupResult{}, err
	}
	result := PlaygroundAttachmentCleanupResult{Claimed: len(claimed)}
	for _, asset := range claimed {
		// An already-empty source is equivalent to an idempotent object-store
		// not-found response and can be finalized directly.
		deleteErr := error(nil)
		if strings.TrimSpace(asset.StorageBucket) != "" && strings.TrimSpace(asset.ObjectKey) != "" {
			deleteErr = assetObjectStore.Delete(ctx, asset.StorageBucket, asset.ObjectKey, asset.ObjectGeneration)
		}
		if deleteErr != nil && !isAssetObjectNotFound(deleteErr) {
			// Leave the lease and CleanupPending state in place. The regular
			// cleanup worker will retry after the lease expires.
			continue
		}
		deleted, markErr := model.MarkAssetSourceExpiredIfCleanupLease(
			asset.Id,
			owner,
			asset.CleanupGeneration,
			now.Unix(),
		)
		if markErr != nil {
			return result, markErr
		}
		if deleted {
			result.Deleted++
		}
	}
	return result, nil
}
