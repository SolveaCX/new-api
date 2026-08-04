package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

const (
	assetPublicIDPrefix    = "ast_"
	assetUploadIDPrefix    = "upl_"
	assetPublicIDRandomLen = 32
	assetRedirectLimit     = 5
	assetCleanupLeaseTTL   = 5 * time.Minute
)

var (
	ErrAssetInvalidSourceURL     = errors.New("invalid asset source url")
	ErrAssetUnsupportedMediaType = errors.New("unsupported asset media type")
	ErrAssetTooLarge             = errors.New("asset too large")
	ErrAssetStorageDisabled      = errors.New("asset storage is disabled")
	ErrAssetUploadNotFound       = errors.New("asset upload not found")
	ErrAssetUploadValidation     = errors.New("asset upload validation failed")
	ErrAssetFileRequired         = errors.New("asset file is required")
	assetNow                     = time.Now
	assetHTTPClient              = http.DefaultClient
	assetValidateURL             = validateAssetURLWithFetchSetting
	assetPublicID                = func() (string, error) { return randomAssetID(assetPublicIDPrefix) }
	assetUploadID                = func() (string, error) { return randomAssetID(assetUploadIDPrefix) }
	assetRandomSuffix            = func() (string, error) { return common.GenerateRandomCharsKey(24) }
)

type AssetFromURLRequest struct {
	UserID    int
	AssetType string
	URL       string
}

type AssetUploadRequest struct {
	UserID    int
	AssetType string
	Filename  string
	Body      io.Reader
}

type AssetUploadSessionRequest struct {
	UserID      int
	Owner       string
	AssetType   string
	ContentType string
	SizeBytes   int64
}

type AssetCompleteUploadRequest struct {
	UploadID string
	Owner    string
}

type CleanupExpiredAssetSourcesRequest struct {
	Owner string
	Limit int
}

type CleanupExpiredAssetSourcesResult struct {
	Claimed int
	Deleted int
}

type AssetResult struct {
	PublicID    string
	Status      string
	SignedURL   string
	ContentType string
	SizeBytes   int64
	SHA256      string
}

type AssetUploadSessionResult struct {
	UploadID  string
	PublicID  string
	ObjectKey string
	SignedURL string
	ExpiresAt int64
}

func CreateAssetFromURL(ctx context.Context, request AssetFromURLRequest) (*AssetResult, error) {
	cfg := CurrentAssetStorageConfig()
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, ErrAssetStorageDisabled
	}
	request.AssetType = normalizeAssetType(request.AssetType)
	limit, ok := cfg.TypeLimits[request.AssetType]
	if !ok {
		return nil, ErrAssetUnsupportedMediaType
	}
	finalResp, err := fetchAssetSource(ctx, request.URL)
	if err != nil {
		return nil, err
	}
	defer finalResp.Body.Close()
	body, detected, sha, size, err := readAndValidateAssetMedia(finalResp.Body, request.AssetType, limit)
	if err != nil {
		return nil, err
	}
	publicID, objectKey, err := newAssetObjectKey(cfg, request.UserID, request.AssetType, detected)
	if err != nil {
		return nil, err
	}
	if err := assetObjectStore.Put(ctx, cfg.Bucket, objectKey, bytes.NewReader(body), detected); err != nil {
		return nil, err
	}
	now := assetNow()
	asset, err := model.CreateAsset(model.Asset{
		PublicId:        publicID,
		UserId:          request.UserID,
		AssetType:       request.AssetType,
		Status:          model.AssetStatusActive,
		SourceStatus:    model.AssetSourceStatusAvailable,
		StorageBackend:  defaultAssetStorageBackend,
		StorageBucket:   cfg.Bucket,
		ObjectKey:       objectKey,
		ContentType:     detected,
		SizeBytes:       size,
		SHA256:          sha,
		SourceExpiresAt: now.Add(cfg.SourceRetention).Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	})
	if err != nil {
		_ = assetObjectStore.Delete(ctx, cfg.Bucket, objectKey)
		return nil, err
	}
	return resultFromAsset(asset), nil
}

func UploadAsset(ctx context.Context, request AssetUploadRequest) (*AssetResult, error) {
	cfg := CurrentAssetStorageConfig()
	if request.Body == nil {
		return nil, ErrAssetFileRequired
	}
	request.AssetType = normalizeAssetType(request.AssetType)
	typeLimit, ok := cfg.TypeLimits[request.AssetType]
	if !ok {
		return nil, ErrAssetUnsupportedMediaType
	}
	limit := typeLimit
	if cfg.MultipartMaxBytes < limit {
		limit = cfg.MultipartMaxBytes
	}
	body, detected, sha, size, err := readAndValidateAssetMedia(request.Body, request.AssetType, limit)
	if err != nil {
		return nil, err
	}
	publicID, objectKey, err := newAssetObjectKey(cfg, request.UserID, request.AssetType, detected)
	if err != nil {
		return nil, err
	}
	if err := assetObjectStore.Put(ctx, cfg.Bucket, objectKey, bytes.NewReader(body), detected); err != nil {
		return nil, err
	}
	now := assetNow()
	asset, err := model.CreateAsset(model.Asset{
		PublicId:        publicID,
		UserId:          request.UserID,
		AssetType:       request.AssetType,
		Status:          model.AssetStatusActive,
		SourceStatus:    model.AssetSourceStatusAvailable,
		StorageBackend:  defaultAssetStorageBackend,
		StorageBucket:   cfg.Bucket,
		ObjectKey:       objectKey,
		ContentType:     detected,
		SizeBytes:       size,
		SHA256:          sha,
		SourceExpiresAt: now.Add(cfg.SourceRetention).Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	})
	if err != nil {
		_ = assetObjectStore.Delete(ctx, cfg.Bucket, objectKey)
		return nil, err
	}
	return resultFromAsset(asset), nil
}

func CreateAssetUploadSession(ctx context.Context, request AssetUploadSessionRequest) (*AssetUploadSessionResult, error) {
	cfg := CurrentAssetStorageConfig()
	request.AssetType = normalizeAssetType(request.AssetType)
	limit, ok := cfg.TypeLimits[request.AssetType]
	if !ok {
		return nil, ErrAssetUnsupportedMediaType
	}
	if request.SizeBytes <= 0 || request.SizeBytes > limit {
		return nil, ErrAssetTooLarge
	}
	contentType, ext := normalizeAssetContentType(request.AssetType, request.ContentType)
	if contentType == "" {
		return nil, ErrAssetUnsupportedMediaType
	}
	publicID, err := assetPublicID()
	if err != nil {
		return nil, err
	}
	random, err := assetRandomSuffix()
	if err != nil {
		return nil, err
	}
	objectKey := buildAssetObjectKey(cfg.KeyPrefix, request.UserID, publicID, random, ext)
	uploadID, err := assetUploadID()
	if err != nil {
		return nil, err
	}
	now := assetNow()
	asset, err := model.CreateAssetWithUploadSession(model.Asset{
		PublicId:       publicID,
		UserId:         request.UserID,
		AssetType:      request.AssetType,
		Status:         model.AssetStatusCreating,
		SourceStatus:   model.AssetSourceStatusUnavailable,
		StorageBackend: defaultAssetStorageBackend,
		StorageBucket:  cfg.Bucket,
		ObjectKey:      objectKey,
		ContentType:    contentType,
		SizeBytes:      request.SizeBytes,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}, model.AssetUpload{
		UploadId:    uploadID,
		Owner:       request.Owner,
		ContentType: contentType,
		SizeBytes:   request.SizeBytes,
		ExpiresAt:   now.Add(cfg.SignedURLTTL).Unix(),
		Status:      model.AssetUploadStatusPending,
		CreatedAt:   now.Unix(),
		UpdatedAt:   now.Unix(),
	})
	if err != nil {
		return nil, err
	}
	signedURL, err := assetObjectStore.SignURL(ctx, cfg.Bucket, objectKey, AssetSignedURLRequest{Method: http.MethodPut, TTL: cfg.SignedURLTTL, ContentType: contentType})
	if err != nil {
		return nil, err
	}
	return &AssetUploadSessionResult{UploadID: uploadID, PublicID: asset.PublicId, ObjectKey: objectKey, SignedURL: signedURL, ExpiresAt: now.Add(cfg.SignedURLTTL).Unix()}, nil
}

func CompleteAssetUpload(ctx context.Context, request AssetCompleteUploadRequest) (*AssetResult, error) {
	upload, err := model.GetAssetUploadForOwner(request.UploadID, request.Owner)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssetUploadNotFound
		}
		return nil, err
	}
	if upload.Status != model.AssetUploadStatusPending {
		return nil, ErrAssetUploadValidation
	}
	attrs, err := assetObjectStore.Attrs(ctx, upload.StorageBucket, upload.ObjectKey)
	if err != nil {
		return nil, err
	}
	if attrs.Size != upload.SizeBytes {
		return nil, ErrAssetUploadValidation
	}
	reader, err := assetObjectStore.Open(ctx, upload.StorageBucket, upload.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	cfg := CurrentAssetStorageConfig()
	body, detected, sha, size, err := readAndValidateAssetMedia(reader, upload.AssetType, cfg.TypeLimits[upload.AssetType])
	_ = body
	if err != nil {
		return nil, err
	}
	if size != attrs.Size || detected != upload.ContentType || detected != attrs.ContentType {
		return nil, ErrAssetUploadValidation
	}
	now := assetNow()
	completed, err := model.CompleteAssetUploadCAS(upload.UploadId, request.Owner, model.AssetUploadCompletion{
		ContentType:     detected,
		SizeBytes:       size,
		SHA256:          sha,
		SourceExpiresAt: now.Add(cfg.SourceRetention).Unix(),
		Now:             now.Unix(),
	})
	if err != nil {
		return nil, err
	}
	if !completed {
		return nil, ErrAssetUploadNotFound
	}
	asset, err := model.GetAssetByIDForUser(upload.AssetId, upload.UserId)
	if err != nil {
		return nil, err
	}
	return resultFromAsset(asset), nil
}

func CleanupExpiredAssetSources(ctx context.Context, request CleanupExpiredAssetSourcesRequest) (CleanupExpiredAssetSourcesResult, error) {
	now := assetNow()
	claimed, err := model.ClaimExpiredAssetSources(request.Owner, now.Unix(), now.Add(assetCleanupLeaseTTL).Unix(), request.Limit)
	if err != nil {
		return CleanupExpiredAssetSourcesResult{}, err
	}
	result := CleanupExpiredAssetSourcesResult{Claimed: len(claimed)}
	for _, asset := range claimed {
		err := assetObjectStore.Delete(ctx, asset.StorageBucket, asset.ObjectKey)
		if err != nil && !isAssetObjectNotFound(err) {
			continue
		}
		ok, err := model.MarkAssetSourceExpiredIfCleanupLease(asset.Id, request.Owner, asset.CleanupGeneration, now.Unix())
		if err != nil {
			return result, err
		}
		if ok {
			result.Deleted++
		}
	}
	return result, nil
}

func fetchAssetSource(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := validateAssetSourceURL(rawURL); err != nil {
		return nil, err
	}
	client := *assetHTTPClient
	redirects := 0
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		redirects++
		if redirects > assetRedirectLimit {
			return errors.New("asset source redirect limit exceeded")
		}
		return validateAssetSourceURL(req.URL.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, ErrAssetInvalidSourceURL
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("asset source returned status %d", resp.StatusCode)
	}
	return resp, nil
}

func validateAssetSourceURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || strings.ToLower(parsed.Scheme) != "https" {
		return ErrAssetInvalidSourceURL
	}
	if strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".") == "localhost" {
		return ErrAssetInvalidSourceURL
	}
	if err := assetValidateURL(rawURL); err != nil {
		return ErrAssetInvalidSourceURL
	}
	return nil
}

func validateAssetURLWithFetchSetting(rawURL string) error {
	fetchSetting := system_setting.GetFetchSetting()
	return common.ValidateURLWithFetchSetting(
		rawURL,
		true,
		false,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		true,
	)
}

func readAndValidateAssetMedia(reader io.Reader, assetType string, maxBytes int64) ([]byte, string, string, int64, error) {
	if maxBytes <= 0 {
		return nil, "", "", 0, ErrAssetTooLarge
	}
	limited := io.LimitReader(reader, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", "", 0, err
	}
	if int64(len(body)) > maxBytes {
		return nil, "", "", 0, ErrAssetTooLarge
	}
	contentType := http.DetectContentType(body)
	normalized, _ := normalizeAssetContentType(assetType, contentType)
	if normalized == "" {
		return nil, "", "", 0, ErrAssetUnsupportedMediaType
	}
	sum := sha256.Sum256(body)
	return body, normalized, hex.EncodeToString(sum[:]), int64(len(body)), nil
}

func newAssetObjectKey(cfg AssetStorageConfig, userID int, assetType string, contentType string) (string, string, error) {
	publicID, err := assetPublicID()
	if err != nil {
		return "", "", err
	}
	random, err := assetRandomSuffix()
	if err != nil {
		return "", "", err
	}
	_, ext := normalizeAssetContentType(assetType, contentType)
	return publicID, buildAssetObjectKey(cfg.KeyPrefix, userID, publicID, random, ext), nil
}

func buildAssetObjectKey(prefix string, userID int, publicID string, random string, ext string) string {
	cleanPrefix := strings.Trim(path.Clean("/"+strings.TrimSpace(prefix)), "/")
	if cleanPrefix == "." || cleanPrefix == "" {
		cleanPrefix = defaultAssetKeyPrefix
	}
	return path.Join(cleanPrefix, fmt.Sprintf("%d", userID), assetNow().UTC().Format("20060102"), publicID, random+ext)
}

func normalizeAssetType(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "image":
		return "Image"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	default:
		return strings.TrimSpace(assetType)
	}
}

func normalizeAssetContentType(assetType string, contentType string) (string, string) {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = parsed
	}
	switch assetType {
	case "Image":
		switch contentType {
		case "image/jpeg", "image/jpg":
			return "image/jpeg", ".jpg"
		case "image/png":
			return "image/png", ".png"
		case "image/webp":
			return "image/webp", ".webp"
		}
	case "Video":
		switch contentType {
		case "video/mp4":
			return "video/mp4", ".mp4"
		case "video/webm":
			return "video/webm", ".webm"
		case "video/quicktime":
			return "video/quicktime", ".mov"
		}
	case "Audio":
		switch contentType {
		case "audio/mpeg", "audio/mp3":
			return "audio/mpeg", ".mp3"
		case "audio/wav", "audio/x-wav":
			return "audio/wav", ".wav"
		case "audio/ogg":
			return "audio/ogg", ".ogg"
		case "audio/mp4":
			return "audio/mp4", ".m4a"
		}
	}
	return "", ""
}

func randomAssetID(prefix string) (string, error) {
	random, err := common.GenerateRandomCharsKey(assetPublicIDRandomLen)
	if err != nil {
		return "", err
	}
	return prefix + random, nil
}

func resultFromAsset(asset *model.Asset) *AssetResult {
	return &AssetResult{
		PublicID:    asset.PublicId,
		Status:      asset.Status,
		ContentType: asset.ContentType,
		SizeBytes:   asset.SizeBytes,
		SHA256:      asset.SHA256,
	}
}
