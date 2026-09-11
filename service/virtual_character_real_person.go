package service

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// virtualCharacterRealPersonSessionTTLSeconds mirrors the susciyuan validation
// session lifetime (~30 min) observed at creation (expires_at - created_at).
const virtualCharacterRealPersonSessionTTLSeconds = int64(30 * 60)

// virtualCharacterRealPersonProvider adapts the susciyuan virtual-characters
// real-person REST endpoints to the realPersonProvider interface. Verification
// is session + character_id + human H5 liveness; there is no HTTPS callback.
type virtualCharacterRealPersonProvider struct {
	channel       *model.Channel
	apiKey        string
	gatewayOrigin string
}

func (virtualCharacterRealPersonProvider) RequiresCallback() bool {
	return false
}

func (virtualCharacterRealPersonProvider) VerificationTTLSeconds() int64 {
	return virtualCharacterRealPersonSessionTTLSeconds
}

// upstream response envelopes (real field names captured 2026-09-10).

type virtualCharacterRealPersonSessionData struct {
	ID          string `json:"id"`
	CharacterID int64  `json:"character_id"`
	LaunchURL   string `json:"launch_url"`
	Status      string `json:"status"`
	LastError   string `json:"last_error"`
	ExpiresAt   int64  `json:"expires_at"`
}

type virtualCharacterRealPersonSessionResponse struct {
	Success bool                                  `json:"success"`
	Message string                                `json:"message"`
	Data    virtualCharacterRealPersonSessionData `json:"data"`
	Error   struct {
		Code string `json:"code"`
	} `json:"error"`
}

type virtualCharacterRealPersonCharacterData struct {
	ID               int64  `json:"id"`
	SourceType       string `json:"source_type"`
	Status           string `json:"status"`
	ValidationStatus string `json:"validation_status"`
	ProviderAssetID  string `json:"provider_asset_id"`
	AssetType        string `json:"asset_type"`
	LastError        string `json:"last_error"`
	Authorization    struct {
		Status string `json:"status"`
	} `json:"authorization"`
}

type virtualCharacterRealPersonCharacterResponse struct {
	Success bool                                    `json:"success"`
	Message string                                  `json:"message"`
	Data    virtualCharacterRealPersonCharacterData `json:"data"`
	Error   struct {
		Code string `json:"code"`
	} `json:"error"`
}

// virtualCharacterRealPersonAssetStatus maps susciyuan character lifecycle
// states onto the BytePlus asset status vocabulary the state machine uses.
func virtualCharacterRealPersonAssetStatus(status string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return model.BytePlusAssetStatusActive, true
	case "creating", "pending", "processing":
		return model.BytePlusAssetStatusProcessing, true
	case "failed", "blocked", "offline", "deleting":
		return model.BytePlusAssetStatusFailed, true
	default:
		return "", false
	}
}
