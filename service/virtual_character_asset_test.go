package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestVirtualCharacterAssetMaterializerStreamsThreeMediaTypes(t *testing.T) {
	tests := []struct {
		assetType   string
		contentType string
		objectKey   string
		wantExt     string
	}{
		{assetType: "Image", contentType: "image/png", objectKey: "sources/reference.png", wantExt: ".png"},
		{assetType: "Video", contentType: "video/mp4", objectKey: "sources/reference.mp4", wantExt: ".mp4"},
		{assetType: "Audio", contentType: "audio/mpeg", objectKey: "sources/reference.mp3", wantExt: ".mp3"},
	}
	for index, test := range tests {
		t.Run(test.assetType, func(t *testing.T) {
			payload := "media-" + strings.ToLower(test.assetType)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, virtualCharacterAssetPath, r.URL.Path)
				require.Equal(t, "Bearer channel-key", r.Header.Get("Authorization"))
				require.Equal(t, "idem-upload", r.Header.Get("Idempotency-Key"))
				require.NoError(t, r.ParseMultipartForm(1<<20))
				require.Equal(t, test.assetType, r.FormValue("asset_type"))
				require.NotEmpty(t, r.FormValue("name"))
				file, header, err := r.FormFile("file")
				require.NoError(t, err)
				defer file.Close()
				require.True(t, strings.HasSuffix(header.Filename, test.wantExt), header.Filename)
				require.Equal(t, test.contentType, header.Header.Get("Content-Type"))
				body, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, payload, string(body))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, `{"success":true,"data":{"id":`+string(rune('1'+index))+`,"status":"creating","provider_asset_id":"provider_early","source_type":"volc_aigc","asset_type":"`+test.assetType+`"}}`)
			}))
			defer server.Close()
			withVirtualCharacterTestClients(t, server.Client(), func(_ context.Context, rawURL string) (*http.Response, error) {
				require.Equal(t, "https://source.example.invalid/media", rawURL)
				require.NotContains(t, rawURL, "channel-key")
				return virtualCharacterSourceResponse(payload, test.contentType), nil
			})

			result, err := (virtualCharacterAssetBindingMaterializer{}).CreateAsset(context.Background(), AssetMaterializeInput{
				Asset:          model.Asset{AssetType: test.assetType, ObjectKey: test.objectKey, ContentType: test.contentType},
				Channel:        virtualCharacterTestChannel(server.URL),
				APIKey:         "channel-key",
				IdempotencyKey: "idem-upload",
				SourceURL:      "https://source.example.invalid/media",
			})
			require.NoError(t, err)
			require.Equal(t, string(rune('1'+index)), result.UpstreamAssetID)
			require.Empty(t, result.UpstreamGroupID, "creating references must remain private until active")
			require.Equal(t, model.AssetStatusProcessing, result.Status)
		})
	}
}

func TestVirtualCharacterAssetMaterializerQueriesUntilActive(t *testing.T) {
	responses := map[string]string{
		"41": `{"success":true,"data":{"id":41,"status":"creating","provider_asset_id":"provider_early","source_type":"volc_aigc","asset_type":"Image"}}`,
		"42": `{"success":true,"data":{"id":42,"status":"active","provider_asset_id":"provider_ready_42","source_type":"volc_aigc","asset_type":"Image"}}`,
		"43": `{"success":true,"data":{"id":43,"status":"blocked","provider_asset_id":"provider_blocked","source_type":"volc_aigc","asset_type":"Image"}}`,
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer channel-key", r.Header.Get("Authorization"))
		id := strings.TrimPrefix(r.URL.Path, virtualCharacterAssetPath+"/")
		body, ok := responses[id]
		require.True(t, ok)
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	materializer := virtualCharacterAssetBindingMaterializer{}
	input := AssetMaterializeInput{Asset: model.Asset{AssetType: "Image"}, Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key"}

	processing, err := materializer.GetAsset(context.Background(), input, "41")
	require.NoError(t, err)
	require.Equal(t, model.AssetStatusProcessing, processing.Status)
	require.Empty(t, processing.UpstreamGroupID)

	active, err := materializer.GetAsset(context.Background(), input, "42")
	require.NoError(t, err)
	require.Equal(t, "42", active.UpstreamAssetID)
	require.Equal(t, "provider_ready_42", active.UpstreamGroupID)
	require.Equal(t, model.AssetStatusActive, active.Status)

	failed, err := materializer.GetAsset(context.Background(), input, "43")
	require.NoError(t, err)
	require.Equal(t, model.AssetStatusFailed, failed.Status)
}

func TestVirtualCharacterAssetMaterializerMapsCreateStatesWithoutDuplicateUpload(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		providerID  string
		wantStatus  string
		wantPrivate string
	}{
		{name: "creating", status: "creating", providerID: "provider_early", wantStatus: model.AssetStatusProcessing},
		{name: "active", status: "active", providerID: "provider_ready", wantStatus: model.AssetStatusActive, wantPrivate: "provider_ready"},
		{name: "active incomplete", status: "active", wantStatus: model.AssetStatusProcessing},
		{name: "failed", status: "failed", providerID: "provider_failed", wantStatus: model.AssetStatusFailed},
		{name: "unknown", status: "new_upstream_state", providerID: "provider_unknown", wantStatus: model.AssetStatusProcessing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := virtualCharacterCreateResult(virtualCharacterAssetResponse{
				Success: true,
				Data: virtualCharacterAssetData{
					ID: 91, Status: test.status, ProviderAssetID: test.providerID, SourceType: "volc_aigc", AssetType: "Image",
				},
			}, "Image")
			require.NoError(t, err)
			require.Equal(t, "91", result.UpstreamAssetID)
			require.Equal(t, test.wantStatus, result.Status)
			require.Equal(t, test.wantPrivate, result.UpstreamGroupID)
		})
	}
}

func TestVirtualCharacterAssetMaterializerRejectsInvalidQueryResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "wrong management id", body: `{"success":true,"data":{"id":99,"status":"active","provider_asset_id":"provider_ok","source_type":"volc_aigc","asset_type":"Image"}}`},
		{name: "missing provider id", body: `{"success":true,"data":{"id":51,"status":"active","provider_asset_id":"","source_type":"volc_aigc","asset_type":"Image"}}`},
		{name: "malicious provider id", body: `{"success":true,"data":{"id":51,"status":"active","provider_asset_id":"safe/../escape?token=x","source_type":"volc_aigc","asset_type":"Image"}}`},
		{name: "wrong source type", body: `{"success":true,"data":{"id":51,"status":"active","provider_asset_id":"provider_ok","source_type":"real_person","asset_type":"Image"}}`},
		{name: "unknown status", body: `{"success":true,"data":{"id":51,"status":"mystery","provider_asset_id":"provider_ok","source_type":"volc_aigc","asset_type":"Image"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, test.body) }))
			defer server.Close()
			withVirtualCharacterTestClients(t, server.Client(), nil)
			_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
				Asset: model.Asset{AssetType: "Image"}, Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key",
			}, "51")
			require.Error(t, err)
			require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
			require.True(t, IsRetryableAssetMaterializeError(err))
			require.NotContains(t, err.Error(), "channel-key")
		})
	}
}

func TestVirtualCharacterAssetMaterializerStopsAuthorizedRedirect(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		require.Empty(t, r.Header.Get("Authorization"))
	}))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+virtualCharacterAssetPath+"/61", http.StatusFound)
	}))
	defer redirect.Close()
	withVirtualCharacterTestClients(t, redirect.Client(), nil)

	_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
		Asset: model.Asset{AssetType: "Image"}, Channel: virtualCharacterTestChannel(redirect.URL), APIKey: "channel-key",
	}, "61")
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorDefinitive, AssetMaterializeErrorClass(err))
	require.Zero(t, targetCalls.Load())
}

func TestVirtualCharacterAssetMaterializerClassifiesTimeoutAnd5xx(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		client := &http.Client{Transport: virtualCharacterRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, virtualCharacterTimeoutError{}
		})}
		withVirtualCharacterTestClients(t, client, nil)
		_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
			Channel: virtualCharacterTestChannel("https://upstream.example.invalid"), APIKey: "channel-key",
		}, "62")
		require.Error(t, err)
		require.Equal(t, AssetMaterializeErrorTimeout, AssetMaterializeErrorClass(err))
	})

	t.Run("5xx", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"success":false,"error":{"code":"temporary"}}`)
		}))
		defer server.Close()
		withVirtualCharacterTestClients(t, server.Client(), nil)
		_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
			Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key",
		}, "63")
		require.Error(t, err)
		require.Equal(t, AssetMaterializeErrorUpstream5xx, AssetMaterializeErrorClass(err))
	})
}

func TestVirtualCharacterAssetMaterializerClassifies429AndRetryAfter(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"success":false,"error":{"code":"rate_limited"}}`)
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)

	_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
		Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key",
	}, "71")
	require.Error(t, err)
	var failure *AssetMaterializeFailure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, AssetMaterializeErrorThrottled, failure.Class)
	require.Equal(t, 17*time.Second, failure.RetryAfter)
	require.NotContains(t, err.Error(), "channel-key")
}

func TestVirtualCharacterAssetMaterializerClosesEarly4xxResponseWithoutWaitingForSource(t *testing.T) {
	responseBody := &virtualCharacterTrackingReadCloser{Reader: strings.NewReader(`{"success":false,"error":{"code":"invalid_file"}}`)}
	client := &http.Client{Transport: virtualCharacterRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer channel-key", request.Header.Get("Authorization"))
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       responseBody,
			Request:    request,
		}, nil
	})}
	withVirtualCharacterTestClients(t, client, func(context.Context, string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(&virtualCharacterSizedReader{remaining: virtualCharacterVideoMaxSize}),
		}, nil
	})

	_, err := (virtualCharacterAssetBindingMaterializer{}).CreateAsset(context.Background(), AssetMaterializeInput{
		Asset: model.Asset{AssetType: "Video", ObjectKey: "source.mp4"}, Channel: virtualCharacterTestChannel("https://upstream.example.invalid"), APIKey: "channel-key", SourceURL: "https://source.example.invalid/video",
	})
	require.Error(t, err)
	var failure *AssetMaterializeFailure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, AssetMaterializeErrorDefinitive, failure.Class)
	require.Equal(t, http.StatusBadRequest, failure.HTTPStatus)
	require.True(t, responseBody.closed.Load())
}

func TestVirtualCharacterAssetMaterializerBoundsSourceAndResponse(t *testing.T) {
	t.Run("source content length", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("oversize source must not be uploaded") }))
		defer server.Close()
		withVirtualCharacterTestClients(t, server.Client(), func(context.Context, string) (*http.Response, error) {
			response := virtualCharacterSourceResponse("x", "audio/mpeg")
			response.ContentLength = virtualCharacterAudioMaxSize + 1
			return response, nil
		})
		_, err := (virtualCharacterAssetBindingMaterializer{}).CreateAsset(context.Background(), AssetMaterializeInput{
			Asset: model.Asset{AssetType: "Audio"}, Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key", SourceURL: "https://source.example.invalid/audio",
		})
		require.Error(t, err)
		require.Equal(t, AssetMaterializeErrorDefinitive, AssetMaterializeErrorClass(err))
	})

	t.Run("source stream", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"success":true,"data":{"id":82,"status":"creating","provider_asset_id":"provider_early","source_type":"volc_aigc","asset_type":"Audio"}}`)
		}))
		defer server.Close()
		withVirtualCharacterTestClients(t, server.Client(), func(context.Context, string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
				Body:       io.NopCloser(&virtualCharacterSizedReader{remaining: virtualCharacterAudioMaxSize + 1}),
			}, nil
		})
		_, err := (virtualCharacterAssetBindingMaterializer{}).CreateAsset(context.Background(), AssetMaterializeInput{
			Asset: model.Asset{AssetType: "Audio"}, Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key", SourceURL: "https://source.example.invalid/audio",
		})
		require.Error(t, err)
		require.Equal(t, AssetMaterializeErrorDefinitive, AssetMaterializeErrorClass(err))
	})

	t.Run("response body", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", virtualCharacterAssetResponseMaxSize+1))
		}))
		defer server.Close()
		withVirtualCharacterTestClients(t, server.Client(), nil)
		_, err := (virtualCharacterAssetBindingMaterializer{}).GetAsset(context.Background(), AssetMaterializeInput{
			Channel: virtualCharacterTestChannel(server.URL), APIKey: "channel-key",
		}, "81")
		require.Error(t, err)
		require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
	})
}

func TestVirtualCharacterAssetMaterializationConfigAndScope(t *testing.T) {
	channel := virtualCharacterTestChannel("https://EXAMPLE.invalid/base/path/")
	config, ok := virtualCharacterMaterializationConfig(channel)
	require.True(t, ok)
	require.Equal(t, "https://example.invalid", config.GatewayOrigin)
	require.Empty(t, config.GroupID)
	require.NotEmpty(t, virtualCharacterBindingScope(config.GatewayOrigin, "key-a"))
	require.Equal(t, virtualCharacterBindingScope(config.GatewayOrigin, "key-a"), virtualCharacterBindingScope(config.GatewayOrigin, "key-a"))
	require.NotEqual(t, virtualCharacterBindingScope(config.GatewayOrigin, "key-a"), virtualCharacterBindingScope(config.GatewayOrigin, "key-b"))
	require.True(t, virtualCharacterValidProviderAssetID("provider_asset-123.abc"))
	require.True(t, virtualCharacterValidProviderAssetID(strings.Repeat("a", virtualCharacterProviderAssetIDMax)))
	require.False(t, virtualCharacterValidProviderAssetID(strings.Repeat("a", virtualCharacterProviderAssetIDMax+1)))
	require.False(t, virtualCharacterValidProviderAssetID("asset://provider"))

	for _, raw := range []string{"http://example.invalid", "https://user@example.invalid", "https://example.invalid?q=1", "https://example.invalid/#fragment"} {
		_, err := validateVirtualCharacterAssetMaterializationConfig(assetMaterializationChannelConfig{GatewayBaseURL: raw})
		require.Error(t, err, raw)
	}
}

func TestVirtualCharacterAssetSourceFormatsMatchOfficialContract(t *testing.T) {
	tests := []struct {
		name        string
		assetType   string
		contentType string
		objectKey   string
		wantType    string
		wantSuffix  string
		wantError   bool
	}{
		{name: "jpeg", assetType: "Image", contentType: "image/jpeg", objectKey: "source.jpeg", wantType: "image/jpeg", wantSuffix: ".jpeg"},
		{name: "heic", assetType: "Image", contentType: "image/heic", objectKey: "source", wantType: "image/heic", wantSuffix: ".heic"},
		{name: "mov", assetType: "Video", contentType: "video/quicktime", objectKey: "source.mov", wantType: "video/quicktime", wantSuffix: ".mov"},
		{name: "wav", assetType: "Audio", contentType: "audio/x-wav", objectKey: "source.wav", wantType: "audio/x-wav", wantSuffix: ".wav"},
		{name: "octet stream inferred", assetType: "Image", contentType: "application/octet-stream", objectKey: "source.webp", wantType: "image/webp", wantSuffix: ".webp"},
		{name: "mismatched supported extension corrected", assetType: "Image", contentType: "image/png", objectKey: "source.jpg", wantType: "image/png", wantSuffix: ".png"},
		{name: "webm rejected", assetType: "Video", contentType: "video/webm", objectKey: "source.webm", wantError: true},
		{name: "m4a rejected", assetType: "Audio", contentType: "audio/mp4", objectKey: "source.m4a", wantError: true},
		{name: "ogg rejected", assetType: "Audio", contentType: "audio/ogg", objectKey: "source.ogg", wantError: true},
		{name: "unknown octet stream rejected", assetType: "Video", contentType: "application/octet-stream", objectKey: "source.bin", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := virtualCharacterSourceResponse("media", test.contentType)
			contentType, filename, err := virtualCharacterSourceMetadata(response, model.Asset{AssetType: test.assetType, ObjectKey: test.objectKey}, test.assetType)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantType, contentType)
			require.True(t, strings.HasSuffix(filename, test.wantSuffix), filename)
		})
	}
}

func virtualCharacterTestChannel(origin string) *model.Channel {
	return &model.Channel{OtherSettings: `{"asset_materialization":{"provider":"virtual_character","gateway_base_url":"` + origin + `","group_id":"ignored"}}`}
}

func virtualCharacterSourceResponse(body, contentType string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func withVirtualCharacterTestClients(t *testing.T, client *http.Client, source func(context.Context, string) (*http.Response, error)) {
	t.Helper()
	originalClientFactory := virtualCharacterAssetHTTPClientFactory
	originalFetchSource := virtualCharacterAssetFetchSource
	virtualCharacterAssetHTTPClientFactory = func(*model.Channel) (*http.Client, error) { return client, nil }
	if source != nil {
		virtualCharacterAssetFetchSource = source
	}
	t.Cleanup(func() {
		virtualCharacterAssetHTTPClientFactory = originalClientFactory
		virtualCharacterAssetFetchSource = originalFetchSource
	})
}

type virtualCharacterRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn virtualCharacterRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type virtualCharacterTimeoutError struct{}

func (virtualCharacterTimeoutError) Error() string   { return "timeout" }
func (virtualCharacterTimeoutError) Timeout() bool   { return true }
func (virtualCharacterTimeoutError) Temporary() bool { return true }

var _ error = virtualCharacterTimeoutError{}

type virtualCharacterSizedReader struct {
	remaining int64
}

type virtualCharacterTrackingReadCloser struct {
	io.Reader
	closed atomic.Bool
}

func (body *virtualCharacterTrackingReadCloser) Close() error {
	body.closed.Store(true)
	return nil
}

func (reader *virtualCharacterSizedReader) Read(buffer []byte) (int, error) {
	if reader.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	for index := range buffer {
		buffer[index] = 'x'
	}
	reader.remaining -= int64(len(buffer))
	return len(buffer), nil
}
