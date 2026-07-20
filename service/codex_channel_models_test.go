package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestFetchCodexChannelModelsRejectsMultiKey(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCodex}
	channel.ChannelInfo.IsMultiKey = true

	models, err := FetchCodexChannelModels(channel)

	require.ErrorContains(t, err, "does not support multi-key")
	require.Nil(t, models)
}

func TestFetchCodexChannelModelsAddsCompactVariantsExceptAutoReview(t *testing.T) {
	allowPrivateCodexModelFetch(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-5.6-sol"},{"slug":"codex-auto-review"}]}`))
	}))
	defer server.Close()

	channel := &model.Channel{
		Type: constant.ChannelTypeCodex,
		Key:  `{"access_token":"token","account_id":"account"}`,
	}
	models, err := fetchCodexChannelModels(
		context.Background(),
		channel,
		server.URL,
		server.Client(),
		"0.144.6",
	)

	require.NoError(t, err)
	require.Equal(t, []string{
		"gpt-5.6-sol",
		"codex-auto-review",
		ratio_setting.WithCompactModelSuffix("gpt-5.6-sol"),
	}, models)
}

func TestFetchCodexChannelModelsReturnsExpiredCredentialError(t *testing.T) {
	allowPrivateCodexModelFetch(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type: constant.ChannelTypeCodex,
		Key:  `{"access_token":"expired","account_id":"account"}`,
	}
	models, err := fetchCodexChannelModels(
		context.Background(),
		channel,
		server.URL,
		server.Client(),
		"0.144.6",
	)

	require.ErrorContains(t, err, "credential expired")
	require.Nil(t, models)
}

func TestFetchCodexChannelModelsRejectsSavedChannelDiscovery(t *testing.T) {
	channel := &model.Channel{Id: 1, Type: constant.ChannelTypeCodex}
	models, err := FetchCodexChannelModels(channel)

	require.ErrorContains(t, err, "saved Codex channel model discovery is deferred")
	require.Nil(t, models)
}
