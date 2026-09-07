package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUploadAndOpenWebsiteFeaturedMediaUsesContentAddressedPrivateStorage(t *testing.T) {
	store := &fakeAssetObjectStore{
		objects: map[string][]byte{},
		attrs:   map[string]AssetObjectAttrs{},
	}
	originalStore := assetObjectStore
	assetObjectStore = store
	t.Cleanup(func() { assetObjectStore = originalStore })
	t.Setenv("ASSET_STORAGE_BUCKET", "featured-test-bucket")
	t.Setenv("ASSET_KEY_PREFIX", "assets")
	payload := tinyPNG()

	result, err := UploadWebsiteFeaturedMedia(context.Background(), bytes.NewReader(payload))

	require.NoError(t, err)
	require.Equal(t, shaHex(payload), result.SHA256)
	require.Equal(t, "image/png", result.ContentType)
	require.Equal(t, int64(len(payload)), result.Size)
	require.Equal(t, "/media/website-featured/"+shaHex(payload)+".png", result.URL)
	require.Len(t, store.puts, 1)
	require.Equal(t, "assets/website-featured/"+shaHex(payload)+".png", store.puts[0].key)
	require.Equal(t, "public, max-age=31536000, immutable", store.puts[0].cacheControl)

	mediaID := strings.TrimPrefix(result.URL, websiteFeaturedMediaPathPrefix)
	media, err := OpenWebsiteFeaturedMedia(context.Background(), mediaID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = media.Body.Close() })
	readBack, err := io.ReadAll(media.Body)
	require.NoError(t, err)
	require.Equal(t, payload, readBack)
	require.Equal(t, "image/png", media.ContentType)
	require.Equal(t, int64(len(payload)), media.Size)
	require.Equal(t, `"`+shaHex(payload)+`"`, media.ETag)
}

func TestUploadWebsiteFeaturedMediaValidatesPayloadInsteadOfFilename(t *testing.T) {
	_, err := UploadWebsiteFeaturedMedia(context.Background(), strings.NewReader("not an image"))
	require.ErrorIs(t, err, ErrWebsiteFeaturedMediaUnsupported)

	_, err = UploadWebsiteFeaturedMedia(context.Background(), nil)
	require.ErrorIs(t, err, ErrWebsiteFeaturedMediaFileRequired)

	oversized := bytes.NewReader(make([]byte, WebsiteFeaturedMediaMaxBytes+1))
	_, err = UploadWebsiteFeaturedMedia(context.Background(), oversized)
	require.ErrorIs(t, err, ErrWebsiteFeaturedMediaTooLarge)
}

func TestOpenWebsiteFeaturedMediaRejectsInvalidAndMissingIDs(t *testing.T) {
	_, err := OpenWebsiteFeaturedMedia(context.Background(), "../../secret.png")
	require.ErrorIs(t, err, ErrWebsiteFeaturedMediaInvalidID)

	store := &fakeAssetObjectStore{attrsErr: errAssetObjectNotFound}
	originalStore := assetObjectStore
	assetObjectStore = store
	t.Cleanup(func() { assetObjectStore = originalStore })
	_, err = OpenWebsiteFeaturedMedia(context.Background(), strings.Repeat("a", 64)+".png")
	require.ErrorIs(t, err, ErrWebsiteFeaturedMediaNotFound)
}
