package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestVirtualCharacterRealPersonProviderStaticContract(t *testing.T) {
	p := virtualCharacterRealPersonProvider{}
	require.False(t, p.RequiresCallback())
	require.Equal(t, int64(30*60), p.VerificationTTLSeconds())
}

func TestVirtualCharacterRealPersonAssetStatus(t *testing.T) {
	cases := map[string]struct {
		status string
		want   string
		ok     bool
	}{
		"creating": {"creating", model.BytePlusAssetStatusProcessing, true},
		"pending":  {"pending", model.BytePlusAssetStatusProcessing, true},
		"active":   {"active", model.BytePlusAssetStatusActive, true},
		"failed":   {"failed", model.BytePlusAssetStatusFailed, true},
		"blocked":  {"blocked", model.BytePlusAssetStatusFailed, true},
		"offline":  {"offline", model.BytePlusAssetStatusFailed, true},
		"unknown":  {"weird", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := virtualCharacterRealPersonAssetStatus(tc.status)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVirtualCharacterRealPersonDoBlocksRedirect(t *testing.T) {
	redirectTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)

	_, err := virtualCharacterRealPersonDo(context.Background(), &model.Channel{}, "k", server.URL, http.MethodGet, "/v1/virtual-characters/1", nil, http.StatusOK)
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorDefinitive, AssetMaterializeErrorClass(err))
}

func TestVirtualCharacterRealPersonDoMapsHTTPError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":"upstream"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)

	_, err := virtualCharacterRealPersonDo(context.Background(), &model.Channel{}, "k", server.URL, http.MethodGet, "/v1/virtual-characters/1", nil, http.StatusOK)
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorUpstream5xx, AssetMaterializeErrorClass(err))
	require.False(t, isRealPersonDefinitiveResponse(err))
}

func TestVirtualCharacterRealPersonDoReturnsBodyOn2xx(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer secret-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"abc"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)

	body, err := virtualCharacterRealPersonDo(context.Background(), &model.Channel{}, "secret-key", server.URL, http.MethodGet, "/v1/virtual-characters/validation-sessions/abc", nil, http.StatusOK)
	require.NoError(t, err)
	require.Contains(t, string(body), `"id":"abc"`)
}

func TestVirtualCharacterCreateVisualValidateSession(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/virtual-characters/validation-sessions", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		require.NotContains(t, string(body), "�")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"sess-hash","character_id":2558,"launch_url":"https://susciyuan.com/api/virtual-characters/validation/launch/sess-hash","status":"pending"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	session, err := p.CreateVisualValidateSession(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "sess-hash", session.BytedToken)
	require.Contains(t, session.H5Link, "launch/sess-hash")
}

func TestVirtualCharacterGetVisualValidateResultSucceeded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/virtual-characters/validation-sessions/sess-hash", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"sess-hash","character_id":2558,"status":"succeeded"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	result, err := p.GetVisualValidateResult(context.Background(), "sess-hash")
	require.NoError(t, err)
	require.Equal(t, "2558", result.GroupID)
}

func TestVirtualCharacterGetVisualValidateResultPendingIsProcessing(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"sess-hash","character_id":2558,"status":"pending"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	_, err := p.GetVisualValidateResult(context.Background(), "sess-hash")
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
	require.False(t, isRealPersonDefinitiveResponse(err))
}

func TestVirtualCharacterGetVisualValidateResultFailedIsDefinitive(t *testing.T) {
	for _, status := range []string{"failed", "expired", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true,"data":{"id":"sess-hash","status":"` + status + `","last_error":"liveness rejected"}}`))
			}))
			defer server.Close()
			withVirtualCharacterTestClients(t, server.Client(), nil)
			p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

			_, err := p.GetVisualValidateResult(context.Background(), "sess-hash")
			require.Error(t, err)
			require.True(t, isRealPersonDefinitiveResponse(err))
		})
	}
}

func TestVirtualCharacterProviderErrorsNeverLeakKey(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":"upstream"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "super-secret-key", gatewayOrigin: server.URL}

	_, err := p.GetVisualValidateResult(context.Background(), "sess-hash")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "super-secret-key")
}

func TestVirtualCharacterGetAssetMapsCharacterStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/virtual-characters/2558", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":2558,"source_type":"volc_real_person","status":"active","validation_status":"accepted","provider_asset_id":"pa_xyz","authorization":{"status":"active"}}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	status, err := p.GetAsset(context.Background(), "2558")
	require.NoError(t, err)
	require.Equal(t, "2558", status.UpstreamAssetID)
	require.Equal(t, model.BytePlusAssetStatusActive, status.Status)
}

func TestVirtualCharacterGetAssetRejectsIDMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":9999,"source_type":"volc_real_person","status":"creating"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	_, err := p.GetAsset(context.Background(), "2558")
	require.Error(t, err)
}

func TestVirtualCharacterDeleteAsset(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/virtual-characters/2558", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":2558}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), nil)
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	_, err := p.DeleteAsset(context.Background(), "2558")
	require.NoError(t, err)
}

func TestVirtualCharacterCreateAssetUploadsPortrait(t *testing.T) {
	portrait := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-portrait-bytes"))
	}))
	defer portrait.Close()
	var gotMultipart bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/virtual-characters/2558/asset", r.URL.Path)
		gotMultipart = strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":2558,"source_type":"volc_real_person","status":"creating"}}`))
	}))
	defer server.Close()
	withVirtualCharacterTestClients(t, server.Client(), func(ctx context.Context, url string) (*http.Response, error) {
		return portrait.Client().Get(url)
	})
	p := virtualCharacterRealPersonProvider{channel: &model.Channel{}, apiKey: "k", gatewayOrigin: server.URL}

	upstreamID, providerAssetID, err := p.CreateAsset(context.Background(), BytePlusCreateAssetRequest{
		GroupID:   "2558",
		URL:       portrait.URL + "/portrait.png",
		AssetType: "image",
		Name:      "portrait",
	})
	require.NoError(t, err)
	require.Equal(t, "2558", upstreamID)
	require.Empty(t, providerAssetID)
	require.True(t, gotMultipart)
}
