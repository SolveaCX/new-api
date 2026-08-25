package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSeedanceProxyRealPersonVerificationUsesRESTGatewayContract(t *testing.T) {
	var createBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer seedance-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1"+seedanceProxyFaceVerificationPath:
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, common.DecodeJson(r.Body, &createBody))
			_, _ = io.WriteString(w, `{"verification_id":"fv_test_123","status":"waiting_user","h5_url":"https://gateway.example.invalid/verify/fv_test_123","expires_at":1783740000}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1"+seedanceProxyFaceVerificationPath+"/fv_test_123":
			_, _ = io.WriteString(w, `{"verification_id":"fv_test_123","status":"verified","group_id":"group-real-person","expires_at":1783740000}`)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
	binding := seedanceProxyRealPersonTestBinding(t, server.URL+"/v1", "seedance-key")

	session, err := binding.Provider.CreateVisualValidateSession(context.Background(), "https://customer.example/return")
	require.NoError(t, err)
	require.Equal(t, "fv_test_123", session.BytedToken)
	require.Equal(t, "https://gateway.example.invalid/verify/fv_test_123", session.H5Link)
	require.Empty(t, session.CallbackURL)
	require.Empty(t, createBody)

	result, err := binding.Provider.GetVisualValidateResult(context.Background(), session.BytedToken)
	require.NoError(t, err)
	require.Equal(t, "group-real-person", result.GroupID)
}

func TestSeedanceProxyRealPersonAssetsUseAuthenticatedGroupAndRESTPaths(t *testing.T) {
	var seenCreate seedanceProxyAssetCreateRequest
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer seedance-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == seedanceProxyAssetUploadPath:
			require.NoError(t, common.DecodeJson(r.Body, &seenCreate))
			_, _ = io.WriteString(w, `{"Result":{"Id":"asset-real-person","GroupId":"group-real-person","Status":"Processing"}}`)
		case r.Method == http.MethodGet && r.URL.Path == seedanceProxyAssetUploadPath:
			require.Equal(t, "LivenessFace", r.URL.Query().Get("GroupType"))
			require.Equal(t, "1", r.URL.Query().Get("PageNumber"))
			require.Equal(t, "25", r.URL.Query().Get("PageSize"))
			_, _ = io.WriteString(w, `{"Result":{"Items":[{"Id":"asset-real-person","GroupId":"group-real-person","AssetType":"Image","Status":"Active"}],"TotalCount":1}}`)
		case r.Method == http.MethodGet && r.URL.Path == seedanceProxyAssetUploadPath+"/asset-real-person":
			_, _ = io.WriteString(w, `{"Result":{"Id":"asset-real-person","GroupId":"group-real-person","Status":"Active"}}`)
		case r.Method == http.MethodDelete && r.URL.Path == seedanceProxyAssetUploadPath+"/asset-real-person":
			_, _ = io.WriteString(w, `{"Result":{}}`)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer server.Close()
	installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
	binding := seedanceProxyRealPersonTestBinding(t, server.URL, "seedance-key")

	assetID, _, err := binding.Provider.CreateAsset(context.Background(), BytePlusCreateAssetRequest{
		GroupID: "group-real-person", URL: "https://source.example/face.png", AssetType: "Image", Name: "Face reference",
	})
	require.NoError(t, err)
	require.Equal(t, "asset-real-person", assetID)
	require.Equal(t, "group-real-person", seenCreate.GroupID)

	status, err := binding.Provider.GetAsset(context.Background(), assetID)
	require.NoError(t, err)
	require.Equal(t, model.BytePlusAssetStatusActive, status.Status)

	assets, err := binding.Provider.ListAssets(context.Background(), BytePlusListAssetsRequest{GroupIDs: []string{"group-real-person"}, PageNumber: 1, PageSize: 25})
	require.NoError(t, err)
	require.Equal(t, 1, assets.TotalCount)
	require.Len(t, assets.Items, 1)

	_, err = binding.Provider.DeleteAsset(context.Background(), assetID)
	require.NoError(t, err)
}

func TestSeedanceProxyRealPersonCreateAssetRejectsMismatchedGroup(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		_, _ = io.WriteString(w, `{"Result":{"Id":"asset-cross-group","GroupId":"group-other","Status":"Processing"}}`)
	}))
	defer server.Close()
	installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
	binding := seedanceProxyRealPersonTestBinding(t, server.URL, "seedance-key")

	_, _, err := binding.Provider.CreateAsset(context.Background(), BytePlusCreateAssetRequest{
		GroupID: "group-real-person", URL: "https://source.example/face.png", AssetType: "Image",
	})

	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
}

func TestSeedanceProxyRealPersonListAssetsRejectsEmptyOrMismatchedGroups(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"Result":{"Items":[{"Id":"asset-cross-group","GroupId":"group-other","Status":"Active"}],"TotalCount":1}}`)
	}))
	defer server.Close()
	installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
	binding := seedanceProxyRealPersonTestBinding(t, server.URL, "seedance-key")

	_, err := binding.Provider.ListAssets(context.Background(), BytePlusListAssetsRequest{GroupIDs: []string{""}})
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))

	_, err = binding.Provider.ListAssets(context.Background(), BytePlusListAssetsRequest{GroupIDs: []string{"group-real-person"}})
	require.Error(t, err)
	require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
}

func TestSeedanceProxyRealPersonVerificationPendingIsRetryable(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = io.WriteString(w, `{"verification_id":"fv_pending","status":"resolving","expires_at":1783740000}`)
	}))
	defer server.Close()
	installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
	binding := seedanceProxyRealPersonTestBinding(t, server.URL, "seedance-key")

	_, err := binding.Provider.GetVisualValidateResult(context.Background(), "fv_pending")

	require.Error(t, err)
	require.True(t, IsRetryableAssetMaterializeError(err))
	require.Equal(t, AssetMaterializeErrorProcessing, AssetMaterializeErrorClass(err))
}

func TestSeedanceProxyRealPersonVerificationTerminalStatusesAreDefinitive(t *testing.T) {
	for _, status := range []string{"failed", "expired"} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"verification_id":"fv_terminal","status":"`+status+`","expires_at":1783740000}`)
			}))
			defer server.Close()
			installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())
			binding := seedanceProxyRealPersonTestBinding(t, server.URL, "seedance-key")

			_, err := binding.Provider.GetVisualValidateResult(context.Background(), "fv_terminal")

			require.Error(t, err)
			require.False(t, IsRetryableAssetMaterializeError(err))
			require.Equal(t, AssetMaterializeErrorDefinitive, AssetMaterializeErrorClass(err))
			require.Equal(t, status, seedanceProxyVerificationTerminalStatus(err))
		})
	}
}

func TestSeedanceProxyVerificationJobFinalizesGatewayTerminalStatuses(t *testing.T) {
	for _, status := range []string{"failed", "expired"} {
		t.Run(status, func(t *testing.T) {
			newBytePlusRealPersonJobsFixtureWithoutRows(t)
			insertBytePlusAssetChannel(t, 156, "default", common.ChannelStatusEnabled, "seedance-key")
			settings := dto.AssetMaterializationSettings{
				Provider:       assetMaterializationProviderSeedanceProxy,
				GatewayBaseURL: "https://gateway.example.invalid",
				GroupID:        "group-ordinary-material",
			}
			settingsJSON, err := common.Marshal(dto.ChannelOtherSettings{AssetMaterialization: &settings})
			require.NoError(t, err)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				_, _ = io.WriteString(w, `{"verification_id":"fv_terminal","status":"`+status+`"}`)
			}))
			defer server.Close()
			settings.GatewayBaseURL = server.URL
			settingsJSON, err = common.Marshal(dto.ChannelOtherSettings{AssetMaterialization: &settings})
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 156).Update("settings", string(settingsJSON)).Error)
			installSeedanceProxyRealPersonHTTPClientFactory(t, server.Client())

			profile, session := seedJobVerificationSession(t, "seedance_"+status, model.BytePlusVisualValidationSessionStatusPending, "fv_terminal", 2300)
			require.NoError(t, model.DB.Model(&profile).Update("channel_id", 156).Error)

			processed, err := runBytePlusRealPersonVerificationStatusJobs(context.Background(), 2000, 1950, 10)

			require.NoError(t, err)
			require.Equal(t, 1, processed)
			require.NoError(t, model.DB.First(&profile, profile.Id).Error)
			require.NoError(t, model.DB.First(&session, session.Id).Error)
			if status == "expired" {
				require.Equal(t, model.BytePlusRealPersonProfileStatusExpired, profile.Status)
				require.Equal(t, model.BytePlusVisualValidationSessionStatusExpired, session.Status)
			} else {
				require.Equal(t, model.BytePlusRealPersonProfileStatusFailed, profile.Status)
				require.Equal(t, model.BytePlusVisualValidationSessionStatusFailed, session.Status)
			}
		})
	}
}

func seedanceProxyRealPersonTestBinding(t *testing.T, gatewayURL, apiKey string) *realPersonProviderBinding {
	t.Helper()
	channel := channelWithAssetMaterializationSettings(t, constant.ChannelTypeBytePlus, dto.AssetMaterializationSettings{
		Provider:       assetMaterializationProviderSeedanceProxy,
		GatewayBaseURL: gatewayURL,
		GroupID:        "group-ordinary-material",
	})
	channel.Id = 156
	channel.Status = common.ChannelStatusEnabled
	channel.Key = apiKey
	binding, err := realPersonProviderForChannel(channel)
	require.NoError(t, err)
	return binding
}

func installSeedanceProxyRealPersonHTTPClientFactory(t *testing.T, client *http.Client) {
	t.Helper()
	originalFactory := seedanceProxyAssetHTTPClientFactory
	seedanceProxyAssetHTTPClientFactory = func(*model.Channel) (*http.Client, error) { return client, nil }
	t.Cleanup(func() { seedanceProxyAssetHTTPClientFactory = originalFactory })
}
