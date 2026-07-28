package model_setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const (
	AutoModelConfigOptionKey           = "auto_model.config"
	AutoModelClassifierAPIKeyOptionKey = "auto_model.classifier_api_key"
	AutoModelAPIKeyMask                = "********"
	AutoModelConfigVersion             = 1

	DefaultAutoModelClassifierTimeoutMS = 800
	DefaultAutoModelInputMaxChars       = 8000
)

var autoModelRoutes = [...]string{"general", "coding", "reasoning", "translation"}

type AutoModelConfig struct {
	Version                 int                 `json:"version"`
	Enabled                 bool                `json:"enabled"`
	ClassifierBaseURL       string              `json:"classifier_base_url"`
	ClassifierModel         string              `json:"classifier_model"`
	ClassifierTimeoutMS     int                 `json:"classifier_timeout_ms"`
	ClassifierInputMaxChars int                 `json:"classifier_input_max_chars"`
	DefaultModel            string              `json:"default_model"`
	Routes                  map[string][]string `json:"routes"`
	CredentialVersion       string              `json:"credential_version"`
}

type AutoModelCredential struct {
	Version string `json:"version"`
	APIKey  string `json:"api_key"`
}

// AutoModelSnapshot is immutable after publication. Callers must not mutate
// Config.Routes or its slices.
type AutoModelSnapshot struct {
	Config           AutoModelConfig
	ClassifierAPIKey string
	Initialized      bool
	Configured       bool
	Invalid          bool
}

var autoModelSnapshot atomic.Pointer[AutoModelSnapshot]

func init() {
	autoModelSnapshot.Store(&AutoModelSnapshot{Config: DefaultAutoModelConfig()})
}

func DefaultAutoModelConfig() AutoModelConfig {
	return AutoModelConfig{
		Version:                 AutoModelConfigVersion,
		ClassifierTimeoutMS:     DefaultAutoModelClassifierTimeoutMS,
		ClassifierInputMaxChars: DefaultAutoModelInputMaxChars,
		Routes:                  map[string][]string{},
	}
}

func GetAutoModelSnapshot() *AutoModelSnapshot {
	return autoModelSnapshot.Load()
}

func NormalizeAutoModelConfig(raw string) (AutoModelConfig, string, error) {
	var cfg AutoModelConfig
	if err := unmarshalStrictObject(raw, autoModelConfigJSONKeys(), &cfg); err != nil {
		return AutoModelConfig{}, "", fmt.Errorf("invalid auto model config: %w", err)
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		return AutoModelConfig{}, "", err
	}
	data, err := common.Marshal(cfg)
	if err != nil {
		return AutoModelConfig{}, "", fmt.Errorf("serialize auto model config: %w", err)
	}
	return cfg, string(data), nil
}

func ParseAutoModelCredential(raw string) (AutoModelCredential, error) {
	var credential AutoModelCredential
	if strings.TrimSpace(raw) == "" {
		return credential, nil
	}
	if err := unmarshalStrictObject(raw, map[string]struct{}{
		"version": {},
		"api_key": {},
	}, &credential); err != nil {
		return AutoModelCredential{}, fmt.Errorf("invalid auto model credential: %w", err)
	}
	credential.Version = strings.TrimSpace(credential.Version)
	credential.APIKey = strings.TrimSpace(credential.APIKey)
	if strings.ContainsAny(credential.APIKey, "\r\n") {
		return AutoModelCredential{}, errors.New("auto model classifier API key contains a line break")
	}
	return credential, nil
}

func MarshalAutoModelCredential(credential AutoModelCredential) (string, error) {
	credential.Version = strings.TrimSpace(credential.Version)
	credential.APIKey = strings.TrimSpace(credential.APIKey)
	if credential.Version == "" {
		return "", errors.New("auto model credential version is required")
	}
	if strings.ContainsAny(credential.APIKey, "\r\n") {
		return "", errors.New("auto model classifier API key contains a line break")
	}
	data, err := common.Marshal(credential)
	if err != nil {
		return "", fmt.Errorf("serialize auto model credential: %w", err)
	}
	return string(data), nil
}

func IsAutoModelAPIKeyPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == AutoModelAPIKeyMask
}

// ReloadAutoModelSnapshot publishes a new immutable snapshot only when the
// complete config/credential pair is valid. On failure the previous snapshot
// remains active.
func ReloadAutoModelSnapshot(configRaw, credentialRaw string) error {
	if strings.TrimSpace(configRaw) == "" && strings.TrimSpace(credentialRaw) == "" {
		autoModelSnapshot.Store(&AutoModelSnapshot{
			Config:      DefaultAutoModelConfig(),
			Initialized: true,
		})
		return nil
	}
	cfg, _, err := NormalizeAutoModelConfig(configRaw)
	if err != nil {
		return autoModelSnapshotReloadError(err)
	}
	if !cfg.Enabled {
		credential, err := ParseAutoModelCredential(credentialRaw)
		if err != nil {
			return autoModelSnapshotReloadError(err)
		}
		cfg.CredentialVersion = credential.Version
		autoModelSnapshot.Store(&AutoModelSnapshot{
			Config:      cfg,
			Initialized: true,
			Configured:  true,
		})
		return nil
	}
	credential, err := ParseAutoModelCredential(credentialRaw)
	if err != nil {
		return autoModelSnapshotReloadError(err)
	}
	if cfg.CredentialVersion == "" || credential.Version == "" || cfg.CredentialVersion != credential.Version {
		return autoModelSnapshotReloadError(errors.New("auto model config and credential versions do not match"))
	}
	if credential.APIKey == "" {
		return autoModelSnapshotReloadError(errors.New("auto model classifier API key is required"))
	}
	autoModelSnapshot.Store(&AutoModelSnapshot{
		Config:           cfg,
		ClassifierAPIKey: credential.APIKey,
		Initialized:      true,
		Configured:       true,
	})
	return nil
}

func autoModelSnapshotReloadError(err error) error {
	current := autoModelSnapshot.Load()
	if current == nil || !current.Initialized || !current.Configured || current.Invalid {
		autoModelSnapshot.Store(&AutoModelSnapshot{
			Config:      DefaultAutoModelConfig(),
			Initialized: true,
			Configured:  true,
			Invalid:     true,
		})
	}
	return err
}

func (c *AutoModelConfig) NormalizeAndValidate() error {
	if c.Version != AutoModelConfigVersion {
		return fmt.Errorf("unsupported auto model config version %d", c.Version)
	}
	c.ClassifierBaseURL = strings.TrimSpace(c.ClassifierBaseURL)
	c.ClassifierModel = strings.TrimSpace(c.ClassifierModel)
	c.DefaultModel = strings.TrimSpace(c.DefaultModel)
	c.CredentialVersion = strings.TrimSpace(c.CredentialVersion)
	if c.ClassifierTimeoutMS == 0 {
		c.ClassifierTimeoutMS = DefaultAutoModelClassifierTimeoutMS
	}
	if c.ClassifierInputMaxChars == 0 {
		c.ClassifierInputMaxChars = DefaultAutoModelInputMaxChars
	}
	if c.ClassifierTimeoutMS < 200 || c.ClassifierTimeoutMS > 2000 {
		return errors.New("auto model classifier timeout must be between 200 and 2000 ms")
	}
	if c.ClassifierInputMaxChars < 1000 || c.ClassifierInputMaxChars > 32000 {
		return errors.New("auto model classifier input limit must be between 1000 and 32000 characters")
	}
	if c.ClassifierModel == "auto" {
		return errors.New("auto model classifier model must not be the virtual model auto")
	}
	if c.ClassifierBaseURL != "" {
		u, err := url.Parse(c.ClassifierBaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("auto model classifier base URL must be an HTTPS URL without credentials, query, or fragment")
		}
		if port := u.Port(); port != "" && port != "443" {
			return errors.New("auto model classifier base URL must use port 443")
		}
		c.ClassifierBaseURL = strings.TrimRight(c.ClassifierBaseURL, "/")
	}

	allowedRoutes := make(map[string]struct{}, len(autoModelRoutes))
	for _, route := range autoModelRoutes {
		allowedRoutes[route] = struct{}{}
	}
	if c.Routes == nil {
		c.Routes = map[string][]string{}
	}
	uniqueCandidates := make(map[string]struct{})
	for route, models := range c.Routes {
		if _, ok := allowedRoutes[route]; !ok {
			return fmt.Errorf("unsupported auto model route %q", route)
		}
		seen := make(map[string]struct{}, len(models))
		normalized := make([]string, 0, len(models))
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				return fmt.Errorf("auto model route %q contains an empty model", route)
			}
			if model == "auto" {
				return fmt.Errorf("auto model route %q cannot contain the virtual model auto", route)
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			uniqueCandidates[model] = struct{}{}
			normalized = append(normalized, model)
		}
		c.Routes[route] = normalized
	}

	if !c.Enabled {
		return nil
	}
	if c.ClassifierBaseURL == "" || c.ClassifierModel == "" {
		return errors.New("auto model classifier URL and model are required when enabled")
	}
	for _, route := range autoModelRoutes {
		if len(c.Routes[route]) == 0 {
			return fmt.Errorf("auto model route %q requires at least one candidate", route)
		}
	}
	if len(uniqueCandidates) < 5 || len(uniqueCandidates) > 10 {
		return errors.New("auto model requires between 5 and 10 unique candidate models")
	}
	if c.DefaultModel == "" || c.DefaultModel == "auto" {
		return errors.New("auto model default model must be a real candidate model")
	}
	if _, ok := uniqueCandidates[c.DefaultModel]; !ok {
		return errors.New("auto model default model must belong to the candidate set")
	}
	return nil
}

func autoModelConfigJSONKeys() map[string]struct{} {
	return map[string]struct{}{
		"version": {}, "enabled": {}, "classifier_base_url": {},
		"classifier_model": {}, "classifier_timeout_ms": {},
		"classifier_input_max_chars": {}, "default_model": {},
		"routes": {}, "credential_version": {},
	}
}

func unmarshalStrictObject(raw string, allowed map[string]struct{}, dst any) error {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal([]byte(raw), &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("value must be a JSON object")
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return common.Unmarshal([]byte(raw), dst)
}
