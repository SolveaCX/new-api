package service

import (
	"context"
	"net/http"
	"net/http/httptest"
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
