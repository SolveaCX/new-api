package service

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
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

var _ realPersonProvider = virtualCharacterRealPersonProvider{}

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

const virtualCharacterRealPersonCharacterPath = "/v1/virtual-characters/"

func (p virtualCharacterRealPersonProvider) GetAsset(ctx context.Context, upstreamAssetID string) (BytePlusAssetStatus, error) {
	managementID, err := virtualCharacterManagementID(upstreamAssetID)
	if err != nil {
		return BytePlusAssetStatus{}, virtualCharacterDefinitiveFailure(err)
	}
	raw, err := virtualCharacterRealPersonDo(ctx, p.channel, p.apiKey, p.gatewayOrigin, http.MethodGet, virtualCharacterRealPersonCharacterPath+managementID, nil, http.StatusOK)
	if err != nil {
		return BytePlusAssetStatus{}, err
	}
	var envelope virtualCharacterRealPersonCharacterResponse
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return BytePlusAssetStatus{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	if !envelope.Success || strconv.FormatInt(envelope.Data.ID, 10) != managementID || envelope.Data.SourceType != "volc_real_person" {
		return BytePlusAssetStatus{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	status, ok := virtualCharacterRealPersonAssetStatus(envelope.Data.Status)
	if !ok {
		return BytePlusAssetStatus{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	return BytePlusAssetStatus{
		UpstreamAssetID: managementID,
		Status:          status,
		ErrorMessage:    strings.TrimSpace(envelope.Data.LastError),
	}, nil
}

func (p virtualCharacterRealPersonProvider) DeleteAsset(ctx context.Context, upstreamAssetID string) (string, error) {
	managementID, err := virtualCharacterManagementID(upstreamAssetID)
	if err != nil {
		return "", virtualCharacterDefinitiveFailure(err)
	}
	if _, err := virtualCharacterRealPersonDo(ctx, p.channel, p.apiKey, p.gatewayOrigin, http.MethodDelete, virtualCharacterRealPersonCharacterPath+managementID, nil, http.StatusOK); err != nil {
		return "", err
	}
	return "", nil
}

func (p virtualCharacterRealPersonProvider) ListAssets(ctx context.Context, request BytePlusListAssetsRequest) (BytePlusListAssetsResult, error) {
	// No production caller: ListBytePlusRealPersonAssets reads the local table.
	// Return an empty result rather than a fabricated one.
	return BytePlusListAssetsResult{}, nil
}

func (p virtualCharacterRealPersonProvider) CreateAsset(ctx context.Context, request BytePlusCreateAssetRequest) (string, string, error) {
	managementID, err := virtualCharacterManagementID(strings.TrimSpace(request.GroupID))
	if err != nil {
		return "", "", virtualCharacterDefinitiveFailure(err)
	}
	assetType, maxSize, err := virtualCharacterAssetType(request.AssetType)
	if err != nil {
		return "", "", virtualCharacterDefinitiveFailure(err)
	}
	sourceURL := strings.TrimSpace(request.URL)
	if sourceURL == "" {
		return "", "", virtualCharacterDefinitiveFailure(errVirtualCharacterProtocol)
	}
	source, err := virtualCharacterAssetFetchSource(ctx, sourceURL)
	if err != nil || source == nil || source.Body == nil {
		if source != nil && source.Body != nil {
			_ = source.Body.Close()
		}
		if errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) {
			return "", "", newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", nil)
		}
		return "", "", virtualCharacterProcessingFailure(0, nil)
	}
	if source.StatusCode < 200 || source.StatusCode >= 300 {
		_ = source.Body.Close()
		return "", "", virtualCharacterProcessingFailure(0, nil)
	}
	if source.ContentLength > maxSize {
		_ = source.Body.Close()
		return "", "", virtualCharacterDefinitiveFailure(errVirtualCharacterTooLarge)
	}
	contentType, filename, err := virtualCharacterSourceMetadata(source, model.Asset{}, assetType)
	if err != nil {
		_ = source.Body.Close()
		return "", "", virtualCharacterDefinitiveFailure(err)
	}

	baseClient, err := virtualCharacterAssetHTTPClientFactory(p.channel)
	if err != nil || baseClient == nil {
		_ = source.Body.Close()
		return "", "", ErrAssetBindingUnavailable
	}
	client := *baseClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errVirtualCharacterRedirect }
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.gatewayOrigin, "/")+virtualCharacterRealPersonCharacterPath+managementID+"/asset", pipeReader)
	if err != nil {
		_ = source.Body.Close()
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return "", "", ErrAssetBindingUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	writeDone := make(chan error, 1)
	go func() {
		defer source.Body.Close()
		writeDone <- writeVirtualCharacterMultipart(pipeWriter, writer, opaqueBytePlusAssetName(), assetType, filename, contentType, source.Body, maxSize)
	}()
	response, requestErr := client.Do(req)
	_ = pipeReader.Close()
	writeErr := <-writeDone
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if errors.Is(writeErr, errVirtualCharacterTooLarge) {
		return "", "", virtualCharacterDefinitiveFailure(writeErr)
	}
	if requestErr != nil {
		if errors.Is(requestErr, context.DeadlineExceeded) || isNetTimeout(requestErr) {
			return "", "", newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", nil)
		}
		if errors.Is(requestErr, errVirtualCharacterRedirect) {
			return "", "", virtualCharacterDefinitiveFailure(errVirtualCharacterRedirect)
		}
		return "", "", virtualCharacterProcessingFailure(0, nil)
	}
	if response == nil {
		return "", "", virtualCharacterProcessingFailure(0, nil)
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, virtualCharacterRealPersonResponseMaxSize+1))
		var envelope virtualCharacterRealPersonCharacterResponse
		_ = common.Unmarshal(raw, &envelope)
		return "", "", newAssetMaterializeFailure(
			assetMaterializeClassForHTTPStatus(response.StatusCode, strings.TrimSpace(envelope.Error.Code)),
			response.StatusCode, strings.TrimSpace(envelope.Error.Code), 0, "", nil)
	}
	if writeErr != nil {
		return "", "", virtualCharacterProcessingFailure(response.StatusCode, nil)
	}
	return managementID, "", nil
}
