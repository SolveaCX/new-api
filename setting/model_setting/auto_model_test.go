package model_setting

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func validAutoModelConfig() AutoModelConfig {
	return AutoModelConfig{
		Version:                 AutoModelConfigVersion,
		Enabled:                 true,
		ClassifierBaseURL:       "https://classifier.example.com/v1/",
		ClassifierModel:         "router-mini",
		ClassifierTimeoutMS:     800,
		ClassifierInputMaxChars: 8000,
		DefaultModel:            "model-a",
		Routes: map[string][]string{
			"general":     {"model-a", "model-b"},
			"coding":      {"model-c"},
			"reasoning":   {"model-d"},
			"translation": {"model-e"},
		},
	}
}

func marshalAutoModelConfigForTest(t *testing.T, cfg AutoModelConfig) string {
	t.Helper()
	data, err := common.Marshal(cfg)
	require.NoError(t, err)
	return string(data)
}

func marshalAutoModelCredentialForTest(t *testing.T, version, apiKey string) string {
	t.Helper()
	raw, err := MarshalAutoModelCredential(AutoModelCredential{Version: version, APIKey: apiKey})
	require.NoError(t, err)
	return raw
}

func TestNormalizeAutoModelConfig(t *testing.T) {
	cfg := validAutoModelConfig()
	cfg.ClassifierTimeoutMS = 0
	cfg.ClassifierInputMaxChars = 0
	cfg.Routes["general"] = []string{" model-a ", "model-a", "model-b"}

	normalized, raw, err := NormalizeAutoModelConfig(marshalAutoModelConfigForTest(t, cfg))
	require.NoError(t, err)
	require.Equal(t, "https://classifier.example.com/v1", normalized.ClassifierBaseURL)
	require.Equal(t, DefaultAutoModelClassifierTimeoutMS, normalized.ClassifierTimeoutMS)
	require.Equal(t, DefaultAutoModelInputMaxChars, normalized.ClassifierInputMaxChars)
	require.Equal(t, []string{"model-a", "model-b"}, normalized.Routes["general"])
	require.NotContains(t, raw, "model-a ")
}

func TestNormalizeAutoModelConfigRejectsUnknownFieldsAndInvalidCandidates(t *testing.T) {
	cfg := validAutoModelConfig()
	raw := marshalAutoModelConfigForTest(t, cfg)
	raw = raw[:len(raw)-1] + `,"unexpected":true}`
	_, _, err := NormalizeAutoModelConfig(raw)
	require.ErrorContains(t, err, "unknown field")

	cfg = validAutoModelConfig()
	cfg.Routes["translation"] = []string{"model-d"}
	_, _, err = NormalizeAutoModelConfig(marshalAutoModelConfigForTest(t, cfg))
	require.ErrorContains(t, err, "between 5 and 10")
}

func TestNormalizeAutoModelConfigRejectsRecursiveClassifierAndNonStandardPort(t *testing.T) {
	cfg := validAutoModelConfig()
	cfg.ClassifierModel = "auto"
	_, _, err := NormalizeAutoModelConfig(marshalAutoModelConfigForTest(t, cfg))
	require.ErrorContains(t, err, "must not be")

	cfg = validAutoModelConfig()
	cfg.ClassifierBaseURL = "https://classifier.example.com:8443/v1"
	_, _, err = NormalizeAutoModelConfig(marshalAutoModelConfigForTest(t, cfg))
	require.ErrorContains(t, err, "port 443")
}

func TestReloadAutoModelSnapshotPublishesDisabledConfigWithoutCredential(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, ReloadAutoModelSnapshot("", "")) })
	cfg := validAutoModelConfig()
	cfg.Enabled = false
	cfg.CredentialVersion = "client-supplied-version"

	require.NoError(t, ReloadAutoModelSnapshot(marshalAutoModelConfigForTest(t, cfg), ""))
	snapshot := GetAutoModelSnapshot()
	require.False(t, snapshot.Config.Enabled)
	require.Empty(t, snapshot.Config.CredentialVersion)
	require.Empty(t, snapshot.ClassifierAPIKey)
}

func TestReloadAutoModelSnapshotKeepsPreviousPairOnMismatch(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, ReloadAutoModelSnapshot("", "")) })
	cfg := validAutoModelConfig()
	cfg.CredentialVersion = "v1"
	require.NoError(t, ReloadAutoModelSnapshot(
		marshalAutoModelConfigForTest(t, cfg),
		marshalAutoModelCredentialForTest(t, "v1", "sk-one"),
	))

	before := GetAutoModelSnapshot()
	require.Equal(t, "sk-one", before.ClassifierAPIKey)
	err := ReloadAutoModelSnapshot(
		marshalAutoModelConfigForTest(t, cfg),
		marshalAutoModelCredentialForTest(t, "v2", "sk-two"),
	)
	require.Error(t, err)
	require.Same(t, before, GetAutoModelSnapshot())
}

func TestReloadAutoModelSnapshotMarksColdStartMismatchInvalid(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, ReloadAutoModelSnapshot("", "")) })
	require.NoError(t, ReloadAutoModelSnapshot("", ""))

	cfg := validAutoModelConfig()
	cfg.CredentialVersion = "config-version"
	err := ReloadAutoModelSnapshot(
		marshalAutoModelConfigForTest(t, cfg),
		marshalAutoModelCredentialForTest(t, "credential-version", "sk-mismatch"),
	)
	require.ErrorContains(t, err, "versions do not match")

	snapshot := GetAutoModelSnapshot()
	require.True(t, snapshot.Initialized)
	require.True(t, snapshot.Configured)
	require.True(t, snapshot.Invalid)
	require.False(t, snapshot.Config.Enabled)
	require.Empty(t, snapshot.ClassifierAPIKey)
}

func TestAutoModelSnapshotConcurrentReadsObserveCompletePairs(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, ReloadAutoModelSnapshot("", "")) })

	loadPair := func(version string) error {
		cfg := validAutoModelConfig()
		cfg.CredentialVersion = version
		configRaw, err := common.Marshal(cfg)
		if err != nil {
			return err
		}
		credentialRaw, err := MarshalAutoModelCredential(AutoModelCredential{
			Version: version,
			APIKey:  "key-" + version,
		})
		if err != nil {
			return err
		}
		return ReloadAutoModelSnapshot(string(configRaw), credentialRaw)
	}
	require.NoError(t, loadPair("v0"))

	var wg sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for reader := 0; reader < 8; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				snapshot := GetAutoModelSnapshot()
				if snapshot.ClassifierAPIKey != "key-"+snapshot.Config.CredentialVersion {
					errorsSeen <- fmt.Errorf("observed mixed snapshot version=%q key=%q", snapshot.Config.CredentialVersion, snapshot.ClassifierAPIKey)
					return
				}
			}
		}()
	}
	for i := 1; i <= 100; i++ {
		require.NoError(t, loadPair(fmt.Sprintf("v%d", i)))
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		require.NoError(t, err)
	}
}
