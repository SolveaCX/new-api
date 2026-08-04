package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCreateAssetFromURLStoresOpaqueAvailableSourceWithSHAAndRevalidatesRedirects(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	png := tinyPNG()
	redirects := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start.png" {
			redirects++
			http.Redirect(w, r, "/final.png", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	t.Cleanup(server.Close)
	assetHTTPClient = server.Client()

	result, err := CreateAssetFromURL(context.Background(), AssetFromURLRequest{
		UserID:    42,
		AssetType: "Image",
		URL:       server.URL + "/start.png",
	})

	require.NoError(t, err)
	require.Equal(t, "ast_fixed", result.PublicID)
	require.Equal(t, model.AssetStatusActive, result.Status)
	require.Equal(t, 1, redirects)
	require.Len(t, store.puts, 1)
	require.Regexp(t, `^assets/42/20260725/ast_fixed/rand_fixed\.png$`, store.puts[0].key)
	require.NotContains(t, store.puts[0].key, "start")
	require.NotContains(t, store.puts[0].key, "final")
	require.Equal(t, "image/png", store.puts[0].contentType)

	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", "ast_fixed").Error)
	require.Equal(t, model.AssetSourceStatusAvailable, asset.SourceStatus)
	require.Equal(t, defaultAssetStorageBackend, asset.StorageBackend)
	require.Equal(t, "asset-test-bucket", asset.StorageBucket)
	require.Equal(t, int64(len(png)), asset.SizeBytes)
	require.Equal(t, shaHex(png), asset.SHA256)
	require.EqualValues(t, 1, asset.ObjectGeneration)
	require.Equal(t, assetNow().Add(30*24*time.Hour).Unix(), asset.SourceExpiresAt)
	require.Empty(t, result.SignedURL, "source ingestion must not return or persist signed GET URLs")
}

func TestCreateAssetFromURLRejectsPrivateAddressAndCredentialRedirect(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	_, err := CreateAssetFromURL(context.Background(), AssetFromURLRequest{UserID: 1, AssetType: "Image", URL: "https://user:pass@example.com/a.png"})
	require.ErrorIs(t, err, ErrAssetInvalidSourceURL)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://user:pass@example.com/a.png", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	assetHTTPClient = server.Client()
	_, err = CreateAssetFromURL(context.Background(), AssetFromURLRequest{UserID: 1, AssetType: "Image", URL: server.URL})
	require.ErrorIs(t, err, ErrAssetInvalidSourceURL)
}

func TestAssetFetchRejectsDialTimeDNSRebindAndDoesNotDialPrivateIP(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	dialed := false
	client := newAssetFetchHTTPClient(assetFetchHTTPClientConfig{
		Timeout: 2 * time.Second,
		Resolver: assetFetchResolverFunc(func(ctx context.Context, host string) ([]net.IP, error) {
			require.Equal(t, "asset.example", host)
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}),
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("dial should not be reached")
		},
	})
	oldHTTP := assetHTTPClient
	assetHTTPClient = client
	t.Cleanup(func() { assetHTTPClient = oldHTTP })

	_, err := fetchAssetSource(context.Background(), "https://asset.example/image.png")

	require.ErrorIs(t, err, ErrAssetInvalidSourceURL)
	require.False(t, dialed)
}

func TestAssetFetchIgnoresProxyEnvironment(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
	client := newAssetFetchHTTPClient(assetFetchHTTPClientConfig{
		Timeout: 2 * time.Second,
		Resolver: assetFetchResolverFunc(func(ctx context.Context, host string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}),
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("stop after hardened direct dial")
		},
	})
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	req, err := http.NewRequest(http.MethodGet, "https://asset.example/image.png", nil)
	require.NoError(t, err)
	proxyURL, err := transport.Proxy(req)

	require.NoError(t, err)
	require.Nil(t, proxyURL)
}

func TestUploadAssetEnforcesMultipartCapTypeLimitsAndMediaMIME(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	t.Setenv("ASSET_MULTIPART_MAX_BYTES", "8")
	t.Setenv("ASSET_IMAGE_MAX_BYTES", "7")
	_, err := UploadAsset(context.Background(), AssetUploadRequest{UserID: 1, AssetType: "Image", Filename: "ignored.png", Body: bytes.NewReader(append(tinyPNG(), 'x'))})
	require.ErrorIs(t, err, ErrAssetTooLarge)

	t.Setenv("ASSET_MULTIPART_MAX_BYTES", "99")
	t.Setenv("ASSET_IMAGE_MAX_BYTES", "99")
	_, err = UploadAsset(context.Background(), AssetUploadRequest{UserID: 1, AssetType: "Image", Filename: "ignored.txt", Body: strings.NewReader("not an image")})
	require.ErrorIs(t, err, ErrAssetUnsupportedMediaType)

	result, err := UploadAsset(context.Background(), AssetUploadRequest{UserID: 1, AssetType: "Audio", Filename: "voice.any", Body: bytes.NewReader(tinyMP3())})
	require.NoError(t, err)
	require.Equal(t, "ast_fixed", result.PublicID)
	require.Equal(t, "audio/mpeg", store.puts[len(store.puts)-1].contentType)
	require.True(t, strings.HasSuffix(store.puts[len(store.puts)-1].key, ".mp3"))
}

func TestCreateAssetUploadSessionSignsBoundedPutAndCompleteValidatesOwnershipAttrsAndHash(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	t.Setenv("ASSET_SIGNED_URL_TTL_SECONDS", "3600")
	t.Setenv("ASSET_SERVICE_ACCOUNT_EMAIL", "asset-signer@example.iam.gserviceaccount.com")
	png := tinyPNG()

	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	require.Equal(t, "upload_fixed", session.UploadID)
	require.Equal(t, "ast_fixed", session.PublicID)
	require.Equal(t, "https://signed.example/"+session.ObjectKey, session.SignedURL)
	require.Equal(t, assetNow().Add(time.Hour).Unix(), session.ExpiresAt)
	require.Len(t, store.signed, 1)
	require.Equal(t, http.MethodPut, store.signed[0].Method)
	require.Equal(t, time.Hour, store.signed[0].TTL)
	require.Equal(t, "image/png", store.signed[0].ContentType)
	require.Equal(t, "asset-signer@example.iam.gserviceaccount.com", store.signed[0].ServiceAccountEmail)
	require.Equal(t, []string{"x-goog-if-generation-match:0"}, store.signed[0].Headers)
	require.Equal(t, map[string]string{"x-goog-if-generation-match": "0"}, session.UploadHeaders)

	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png)), Generation: 9}
	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-8"})
	require.ErrorIs(t, err, ErrAssetUploadNotFound)

	result, err := CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})
	require.NoError(t, err)
	require.Equal(t, model.AssetStatusActive, result.Status)
	require.Equal(t, []fakeAssetOpen{{key: "asset-test-bucket/" + session.ObjectKey, generation: 9}}, store.opens)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, shaHex(png), asset.SHA256)
	require.EqualValues(t, 9, asset.ObjectGeneration)
	require.Equal(t, model.AssetSourceStatusAvailable, asset.SourceStatus)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusComplete, upload.Status)
	require.EqualValues(t, 9, upload.ObjectGeneration)
}

func TestCompleteAssetUploadFailsWhenExactGenerationDisappearsBeforeRead(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png)), Generation: 9}
	store.openErr = errAssetObjectNotFound

	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})

	require.ErrorIs(t, err, ErrAssetUploadValidation)
	require.Equal(t, []fakeAssetOpen{{key: "asset-test-bucket/" + session.ObjectKey, generation: 9}}, store.opens)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 9}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusFailed, upload.Status)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusFailed, asset.Status)
}

func TestSignAssetSourceURLBindsStoredGeneration(t *testing.T) {
	store := installAssetServiceTestDeps(t)
	asset := model.Asset{
		StorageBucket:    "asset-test-bucket",
		ObjectKey:        "assets/7/20260725/ast_fixed/rand_fixed.png",
		ObjectGeneration: 27,
		ContentType:      "image/png",
		StorageBackend:   defaultAssetStorageBackend,
		SourceStatus:     model.AssetSourceStatusAvailable,
		Status:           model.AssetStatusActive,
		SourceExpiresAt:  assetNow().Add(time.Hour).Unix(),
	}
	cfg := CurrentAssetStorageConfig()

	signedURL, err := SignAssetSourceURL(context.Background(), asset, cfg)

	require.NoError(t, err)
	require.Equal(t, "https://signed.example/"+asset.ObjectKey, signedURL)
	require.Len(t, store.signed, 1)
	require.Equal(t, http.MethodGet, store.signed[0].Method)
	require.Equal(t, "27", store.signed[0].QueryParameters.Get("generation"))
}

func TestCreateAssetUploadSessionSignFailureLeavesNoRows(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	store.signErr = errFakeAssetStore
	png := tinyPNG()

	_, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})

	require.ErrorIs(t, err, errFakeAssetStore)
	var assetCount int64
	require.NoError(t, model.DB.Model(&model.Asset{}).Count(&assetCount).Error)
	require.Zero(t, assetCount)
	var uploadCount int64
	require.NoError(t, model.DB.Model(&model.AssetUpload{}).Count(&uploadCount).Error)
	require.Zero(t, uploadCount)
}

func TestCompleteAssetUploadValidationFailureDeletesAndMarksTerminal(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	store.objects["asset-test-bucket/"+session.ObjectKey] = append(png, 'x')
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png) + 1), Generation: 17}

	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})

	require.ErrorIs(t, err, ErrAssetUploadValidation)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 17}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusFailed, upload.Status)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusFailed, asset.Status)
	require.Equal(t, model.AssetSourceStatusUnavailable, asset.SourceStatus)
	require.Empty(t, asset.ObjectKey)
	require.Zero(t, asset.ObjectGeneration)
}

func TestCompleteAssetUploadDeleteFailureMarksCleanupPendingForRetry(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	store.deleteErr = errFakeAssetStore
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	store.objects["asset-test-bucket/"+session.ObjectKey] = append(png, 'x')
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png) + 1), Generation: 18}

	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})

	require.ErrorIs(t, err, ErrAssetUploadValidation)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 18}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusFailed, upload.Status)
	require.EqualValues(t, 18, upload.ObjectGeneration)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusFailed, asset.Status)
	require.Equal(t, model.AssetSourceStatusCleanupPending, asset.SourceStatus)
	require.EqualValues(t, 18, asset.ObjectGeneration)

	store.deleteErr = nil
	result, err := CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Deleted)
	require.Len(t, store.deletes, 2)
	require.Equal(t, fakeAssetDelete{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 18}, store.deletes[1])
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetSourceStatusExpired, asset.SourceStatus)
}

func TestCleanupExpiredAssetSourcesExpiresNeverCompletedUploadSession(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.AssetUpload{}).Where("upload_id = ?", session.UploadID).Update("expires_at", int64(1000)).Error)
	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png)), Generation: 19}

	result, err := CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})

	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Deleted)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 19}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusExpired, upload.Status)
	require.EqualValues(t, 19, upload.ObjectGeneration)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusExpired, asset.Status)
	require.Equal(t, model.AssetSourceStatusExpired, asset.SourceStatus)
}

func TestCleanupExpiredAssetSourcesIsIdempotentAndUsesLeaseFencing(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	asset := model.Asset{
		PublicId:          "ast_cleanup",
		UserId:            7,
		AssetType:         "Image",
		Status:            model.AssetStatusActive,
		SourceStatus:      model.AssetSourceStatusAvailable,
		StorageBackend:    defaultAssetStorageBackend,
		StorageBucket:     "asset-test-bucket",
		ObjectKey:         "assets/7/20260725/ast_cleanup/r.png",
		ObjectGeneration:  44,
		SourceExpiresAt:   1000,
		CleanupLeaseOwner: "other",
		CleanupLeaseUntil: 1001,
		CleanupGeneration: 3,
		CreatedAt:         900,
		UpdatedAt:         900,
	}
	require.NoError(t, model.DB.Create(&asset).Error)
	require.NoError(t, model.DB.Create(&model.AssetBinding{AssetId: asset.Id, ChannelId: 1, Status: model.AssetStatusActive}).Error)
	store.objects["asset-test-bucket/"+asset.ObjectKey] = tinyPNG()

	result, err := CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Deleted)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + asset.ObjectKey, expectedGeneration: 44}}, store.deletes)
	var stored model.Asset
	require.NoError(t, model.DB.First(&stored, asset.Id).Error)
	require.Equal(t, model.AssetSourceStatusExpired, stored.SourceStatus)
	require.Equal(t, model.AssetStatusActive, stored.Status, "active BytePlus binding keeps public asset active")

	result, err = CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})
	require.NoError(t, err)
	require.Zero(t, result.Claimed)
}

func TestCleanupExpiredAssetSourcesDoesNotExpireOnGenerationMismatch(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	store.deleteErr = errors.New("generation mismatch")
	asset := model.Asset{
		PublicId:         "ast_cleanup_mismatch",
		UserId:           7,
		AssetType:        "Image",
		Status:           model.AssetStatusActive,
		SourceStatus:     model.AssetSourceStatusAvailable,
		StorageBackend:   defaultAssetStorageBackend,
		StorageBucket:    "asset-test-bucket",
		ObjectKey:        "assets/7/20260725/ast_cleanup_mismatch/r.png",
		ObjectGeneration: 45,
		SourceExpiresAt:  1000,
		CreatedAt:        900,
		UpdatedAt:        900,
	}
	require.NoError(t, model.DB.Create(&asset).Error)
	store.objects["asset-test-bucket/"+asset.ObjectKey] = tinyPNG()

	result, err := CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})

	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Zero(t, result.Deleted)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + asset.ObjectKey, expectedGeneration: 45}}, store.deletes)
	var stored model.Asset
	require.NoError(t, model.DB.First(&stored, asset.Id).Error)
	require.Equal(t, model.AssetSourceStatusAvailable, stored.SourceStatus)
	require.Equal(t, model.AssetStatusActive, stored.Status)
}

func TestUploadAssetStreamsWithoutOversizedReadAndCleansUpOverLimit(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	t.Setenv("ASSET_MULTIPART_MAX_BYTES", "16")
	t.Setenv("ASSET_IMAGE_MAX_BYTES", "16")

	_, err := UploadAsset(context.Background(), AssetUploadRequest{
		UserID:    1,
		AssetType: "Image",
		Filename:  "oversize.png",
		Body:      &chunkGuardReader{chunks: [][]byte{tinyPNG(), []byte("x")}, maxRead: 8},
	})

	require.ErrorIs(t, err, ErrAssetTooLarge)
	require.Len(t, store.puts, 1, "bounded stream should reach the object store instead of buffering first")
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + store.puts[0].key}}, store.deletes)
}

func TestCompleteAssetUploadRejectsExpiredUploadAndDoesNotReadObject(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.AssetUpload{}).Where("upload_id = ?", session.UploadID).Update("expires_at", assetNow().Unix()).Error)
	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png)), Generation: 9}

	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})

	require.ErrorIs(t, err, ErrAssetUploadValidation)
	require.Empty(t, store.opens)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 9}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusExpired, upload.Status)
	require.Empty(t, upload.StorageBackend)
	require.Empty(t, upload.StorageBucket)
	require.Empty(t, upload.ObjectKey)
	require.Zero(t, upload.ObjectGeneration)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusExpired, asset.Status)
	require.Equal(t, model.AssetSourceStatusExpired, asset.SourceStatus)
	require.Empty(t, asset.StorageBackend)
	require.Empty(t, asset.StorageBucket)
	require.Empty(t, asset.ObjectKey)
	require.Zero(t, asset.ObjectGeneration)
}

func TestCompleteAssetUploadExpiredDeleteFailureMarksCleanupPendingForRetry(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	store.deleteErr = errFakeAssetStore
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID:      7,
		Owner:       "user-7",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   int64(len(png)),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.AssetUpload{}).Where("upload_id = ?", session.UploadID).Update("expires_at", assetNow().Unix()).Error)
	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{ContentType: "image/png", Size: int64(len(png)), Generation: 11}

	_, err = CompleteAssetUpload(context.Background(), AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-7"})

	require.ErrorIs(t, err, ErrAssetUploadValidation)
	require.Empty(t, store.opens)
	require.Equal(t, []fakeAssetDelete{{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 11}}, store.deletes)
	var upload model.AssetUpload
	require.NoError(t, model.DB.First(&upload, "upload_id = ?", session.UploadID).Error)
	require.Equal(t, model.AssetUploadStatusExpired, upload.Status)
	require.EqualValues(t, 11, upload.ObjectGeneration)
	var asset model.Asset
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusExpired, asset.Status)
	require.Equal(t, model.AssetSourceStatusCleanupPending, asset.SourceStatus)
	require.EqualValues(t, 11, asset.ObjectGeneration)
	require.Equal(t, session.ObjectKey, asset.ObjectKey)

	store.deleteErr = nil
	result, err := CleanupExpiredAssetSources(context.Background(), CleanupExpiredAssetSourcesRequest{Owner: "node-a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Deleted)
	require.Len(t, store.deletes, 2)
	require.Equal(t, fakeAssetDelete{key: "asset-test-bucket/" + session.ObjectKey, expectedGeneration: 11}, store.deletes[1])
	require.NoError(t, model.DB.First(&asset, "public_id = ?", session.PublicID).Error)
	require.Equal(t, model.AssetStatusExpired, asset.Status)
	require.Equal(t, model.AssetSourceStatusExpired, asset.SourceStatus)
	require.Empty(t, asset.ObjectKey)
	require.Zero(t, asset.ObjectGeneration)
}

func newAssetServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Asset{}, &model.AssetBinding{}, &model.AssetUpload{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		_ = sqlDB.Close()
	})
	return db
}

func installAssetServiceTestDeps(t *testing.T) *fakeAssetObjectStore {
	t.Helper()
	store := &fakeAssetObjectStore{objects: map[string][]byte{}, attrs: map[string]AssetObjectAttrs{}}
	oldStore := assetObjectStore
	oldNow := assetNow
	oldPublicID := assetPublicID
	oldUploadID := assetUploadID
	oldRandom := assetRandomSuffix
	oldHTTP := assetHTTPClient
	oldValidate := assetValidateURL
	assetObjectStore = store
	assetNow = func() time.Time { return time.Date(2026, 7, 25, 10, 11, 12, 0, time.UTC) }
	assetPublicID = func() (string, error) { return "ast_fixed", nil }
	assetUploadID = func() (string, error) { return "upload_fixed", nil }
	assetRandomSuffix = func() (string, error) { return "rand_fixed", nil }
	assetHTTPClient = http.DefaultClient
	assetValidateURL = func(rawURL string) error { return nil }
	t.Setenv("ASSET_STORAGE_BUCKET", "asset-test-bucket")
	t.Setenv("ASSET_KEY_PREFIX", "assets")
	t.Cleanup(func() {
		assetObjectStore = oldStore
		assetNow = oldNow
		assetPublicID = oldPublicID
		assetUploadID = oldUploadID
		assetRandomSuffix = oldRandom
		assetHTTPClient = oldHTTP
		assetValidateURL = oldValidate
	})
	return store
}

func tinyPNG() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0, 'I', 'H', 'D', 'R'}
}

func tinyMP3() []byte {
	return []byte("ID3\x04\x00\x00\x00\x00\x00\x00payload")
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = errReader{err: errors.New("x")}

type chunkGuardReader struct {
	chunks  [][]byte
	maxRead int
}

func (r *chunkGuardReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		return 0, errors.New("oversized read")
	}
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[0])
	r.chunks[0] = r.chunks[0][n:]
	if len(r.chunks[0]) == 0 {
		r.chunks = r.chunks[1:]
	}
	return n, nil
}
