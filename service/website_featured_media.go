package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
)

const (
	WebsiteFeaturedMediaMaxBytes     = int64(8 << 20)
	websiteFeaturedMediaCacheControl = "public, max-age=31536000, immutable"
	websiteFeaturedMediaPathPrefix   = "/media/website-featured/"
)

var (
	ErrWebsiteFeaturedMediaFileRequired = errors.New("website featured media file is required")
	ErrWebsiteFeaturedMediaTooLarge     = errors.New("website featured media is too large")
	ErrWebsiteFeaturedMediaUnsupported  = errors.New("unsupported website featured media type")
	ErrWebsiteFeaturedMediaInvalidID    = errors.New("invalid website featured media id")
	ErrWebsiteFeaturedMediaNotFound     = errors.New("website featured media not found")
	websiteFeaturedMediaIDPattern       = regexp.MustCompile(`^[a-f0-9]{64}\.(?:gif|jpg|png|webp)$`)
)

type WebsiteFeaturedMediaUploadResult struct {
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type WebsiteFeaturedMedia struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
	ETag        string
}

func UploadWebsiteFeaturedMedia(ctx context.Context, body io.Reader) (*WebsiteFeaturedMediaUploadResult, error) {
	if body == nil {
		return nil, ErrWebsiteFeaturedMediaFileRequired
	}
	payload, err := io.ReadAll(io.LimitReader(body, WebsiteFeaturedMediaMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, ErrWebsiteFeaturedMediaFileRequired
	}
	if int64(len(payload)) > WebsiteFeaturedMediaMaxBytes {
		return nil, ErrWebsiteFeaturedMediaTooLarge
	}

	contentType, extension := normalizeWebsiteFeaturedMediaType(http.DetectContentType(payload))
	if contentType == "" {
		return nil, ErrWebsiteFeaturedMediaUnsupported
	}
	digest := sha256.Sum256(payload)
	sha := hex.EncodeToString(digest[:])
	mediaID := sha + extension
	cfg := CurrentAssetStorageConfig()
	objectKey := websiteFeaturedMediaObjectKey(cfg.KeyPrefix, mediaID)
	if err := assetObjectStore.Put(ctx, cfg.Bucket, objectKey, bytes.NewReader(payload), AssetObjectPutOptions{
		ContentType:  contentType,
		CacheControl: websiteFeaturedMediaCacheControl,
	}); err != nil {
		return nil, err
	}

	return &WebsiteFeaturedMediaUploadResult{
		URL:         websiteFeaturedMediaPathPrefix + mediaID,
		SHA256:      sha,
		ContentType: contentType,
		Size:        int64(len(payload)),
	}, nil
}

func OpenWebsiteFeaturedMedia(ctx context.Context, mediaID string) (*WebsiteFeaturedMedia, error) {
	mediaID = strings.TrimSpace(mediaID)
	if !websiteFeaturedMediaIDPattern.MatchString(mediaID) {
		return nil, ErrWebsiteFeaturedMediaInvalidID
	}
	cfg := CurrentAssetStorageConfig()
	objectKey := websiteFeaturedMediaObjectKey(cfg.KeyPrefix, mediaID)
	attrs, err := assetObjectStore.Attrs(ctx, cfg.Bucket, objectKey)
	if err != nil {
		if isAssetObjectNotFound(err) {
			return nil, ErrWebsiteFeaturedMediaNotFound
		}
		return nil, err
	}
	body, err := assetObjectStore.Open(ctx, cfg.Bucket, objectKey, attrs.Generation)
	if err != nil {
		if isAssetObjectNotFound(err) {
			return nil, ErrWebsiteFeaturedMediaNotFound
		}
		return nil, err
	}
	contentType, _ := normalizeWebsiteFeaturedMediaType(attrs.ContentType)
	if contentType == "" {
		contentType = websiteFeaturedMediaTypeFromID(mediaID)
	}
	return &WebsiteFeaturedMedia{
		Body:        body,
		ContentType: contentType,
		Size:        attrs.Size,
		ETag:        `"` + strings.TrimSuffix(mediaID, path.Ext(mediaID)) + `"`,
	}, nil
}

func normalizeWebsiteFeaturedMediaType(contentType string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/gif":
		return "image/gif", ".gif"
	case "image/jpeg", "image/jpg":
		return "image/jpeg", ".jpg"
	case "image/png":
		return "image/png", ".png"
	case "image/webp":
		return "image/webp", ".webp"
	default:
		return "", ""
	}
}

func websiteFeaturedMediaTypeFromID(mediaID string) string {
	switch path.Ext(mediaID) {
	case ".gif":
		return "image/gif"
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func websiteFeaturedMediaObjectKey(prefix string, mediaID string) string {
	cleanPrefix := strings.Trim(path.Clean("/"+strings.TrimSpace(prefix)), "/")
	if cleanPrefix == "." || cleanPrefix == "" {
		cleanPrefix = defaultAssetKeyPrefix
	}
	return path.Join(cleanPrefix, "website-featured", mediaID)
}
