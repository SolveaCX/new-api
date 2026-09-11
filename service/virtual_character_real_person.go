package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
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

const virtualCharacterRealPersonResponseMaxSize = 1 << 20

// virtualCharacterRealPersonDo performs one susciyuan real-person REST call.
// It blocks redirects, caps the body, and maps transport/HTTP failures onto
// AssetMaterializeFailure classes so the state machine's retry/terminal logic
// (via isRealPersonDefinitiveResponse) behaves identically to TokenSpace.
// On a 2xx whose status equals expectStatus it returns the raw body for the
// caller to decode into its own envelope struct.
func virtualCharacterRealPersonDo(ctx context.Context, channel *model.Channel, apiKey, gatewayOrigin, method, path string, body []byte, expectStatus int) ([]byte, error) {
	baseClient, err := virtualCharacterAssetHTTPClientFactory(channel)
	if err != nil || baseClient == nil {
		return nil, ErrAssetBindingUnavailable
	}
	client := *baseClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errVirtualCharacterRedirect }

	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(gatewayOrigin, "/")+path, reader)
	if err != nil {
		return nil, ErrAssetBindingUnavailable
	}
	req.Header.Set("Accept", "application/json")
	if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) {
			return nil, newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", err)
		}
		if errors.Is(err, errVirtualCharacterRedirect) {
			return nil, virtualCharacterDefinitiveFailure(errVirtualCharacterRedirect)
		}
		return nil, virtualCharacterProcessingFailure(0, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, virtualCharacterRealPersonResponseMaxSize+1))
	if err != nil || len(raw) > virtualCharacterRealPersonResponseMaxSize {
		return nil, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var envelope virtualCharacterRealPersonSessionResponse
		_ = common.Unmarshal(raw, &envelope)
		return nil, newAssetMaterializeFailure(
			assetMaterializeClassForHTTPStatus(response.StatusCode, strings.TrimSpace(envelope.Error.Code)),
			response.StatusCode, strings.TrimSpace(envelope.Error.Code),
			parseAssetMaterializeRetryAfter(response.Header.Get("Retry-After"), time.Now()), "", nil)
	}
	if expectStatus != 0 && response.StatusCode != expectStatus {
		return nil, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	return raw, nil
}

type virtualCharacterRealPersonCreateSessionRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Language    string   `json:"language"`
}

const virtualCharacterRealPersonValidationSessionPath = "/v1/virtual-characters/validation-sessions"

func (p virtualCharacterRealPersonProvider) CreateVisualValidateSession(ctx context.Context, _ string) (BytePlusVisualValidationSession, error) {
	// Upstream stores non-ASCII name/description as U+FFFD, so send ASCII only.
	payload, err := common.Marshal(virtualCharacterRealPersonCreateSessionRequest{
		Name:     "flatkey-real-person",
		Language: "en",
	})
	if err != nil {
		return BytePlusVisualValidationSession{}, virtualCharacterProcessingFailure(0, err)
	}
	raw, err := virtualCharacterRealPersonDo(ctx, p.channel, p.apiKey, p.gatewayOrigin, http.MethodPost, virtualCharacterRealPersonValidationSessionPath, payload, http.StatusOK)
	if err != nil {
		return BytePlusVisualValidationSession{}, err
	}
	var envelope virtualCharacterRealPersonSessionResponse
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return BytePlusVisualValidationSession{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	sessionID := strings.TrimSpace(envelope.Data.ID)
	h5 := strings.TrimSpace(envelope.Data.LaunchURL)
	if !envelope.Success || sessionID == "" || h5 == "" {
		return BytePlusVisualValidationSession{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	return BytePlusVisualValidationSession{BytedToken: sessionID, H5Link: h5}, nil
}

func (p virtualCharacterRealPersonProvider) GetVisualValidateResult(ctx context.Context, bytedToken string) (BytePlusVisualValidationResult, error) {
	bytedToken = strings.TrimSpace(bytedToken)
	if bytedToken == "" {
		return BytePlusVisualValidationResult{}, virtualCharacterProcessingFailure(0, errVirtualCharacterProtocol)
	}
	raw, err := virtualCharacterRealPersonDo(ctx, p.channel, p.apiKey, p.gatewayOrigin, http.MethodGet, virtualCharacterRealPersonValidationSessionPath+"/"+bytedToken, nil, http.StatusOK)
	if err != nil {
		return BytePlusVisualValidationResult{}, err
	}
	var envelope virtualCharacterRealPersonSessionResponse
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return BytePlusVisualValidationResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	switch strings.ToLower(strings.TrimSpace(envelope.Data.Status)) {
	case "succeeded":
		if envelope.Data.CharacterID <= 0 {
			return BytePlusVisualValidationResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
		}
		return BytePlusVisualValidationResult{GroupID: strconv.FormatInt(envelope.Data.CharacterID, 10)}, nil
	case "pending", "processing", "":
		// Human has not finished H5 yet -- retryable, drives the poll backoff.
		return BytePlusVisualValidationResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	default: // failed, expired, cancelled, blocked
		return BytePlusVisualValidationResult{}, virtualCharacterDefinitiveFailure(errVirtualCharacterProtocol)
	}
}
