package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	seedanceProxyFaceVerificationPath  = "/api/seedance/face-verifications"
	seedanceProxyRealPersonResponseMax = techMobiAssetResponseMaxSize
)

type seedanceProxyRealPersonProvider struct {
	channel        *model.Channel
	apiKey         string
	gatewayBaseURL string
}

type seedanceProxyVerificationResponse struct {
	VerificationID string `json:"verification_id"`
	Status         string `json:"status"`
	H5URL          string `json:"h5_url"`
	GroupID        string `json:"group_id"`
	ExpiresAt      int64  `json:"expires_at"`
}

type seedanceProxyRealPersonAssetListResponse struct {
	Items      []BytePlusListedAsset `json:"Items"`
	TotalCount int                   `json:"TotalCount"`
}

type seedanceProxyVerificationTerminalError struct {
	status  string
	failure *AssetMaterializeFailure
}

func (e *seedanceProxyVerificationTerminalError) Error() string {
	if e == nil || e.failure == nil {
		return "seedance verification reached terminal status"
	}
	return e.failure.Error()
}

func (e *seedanceProxyVerificationTerminalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.failure
}

func seedanceProxyVerificationTerminalStatus(err error) string {
	var terminal *seedanceProxyVerificationTerminalError
	if errors.As(err, &terminal) && terminal != nil {
		return strings.TrimSpace(terminal.status)
	}
	return ""
}

func (seedanceProxyRealPersonProvider) RequiresCallback() bool {
	return false
}

func (seedanceProxyRealPersonProvider) VerificationTTLSeconds() int64 {
	return tokenSpaceRealPersonSessionTTLSeconds
}

func (p seedanceProxyRealPersonProvider) CreateVisualValidateSession(ctx context.Context, _ string) (BytePlusVisualValidationSession, error) {
	body, err := p.doJSON(ctx, http.MethodPost, seedanceProxyFaceVerificationPath, []byte("{}"), nil)
	if err != nil {
		return BytePlusVisualValidationSession{}, err
	}
	var response seedanceProxyVerificationResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return BytePlusVisualValidationSession{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, err)
	}
	status := strings.ToLower(strings.TrimSpace(response.Status))
	if status == "failed" || status == "expired" {
		return BytePlusVisualValidationSession{}, seedanceProxyVerificationTerminalFailure(status, http.StatusOK)
	}
	verificationID := strings.TrimSpace(response.VerificationID)
	h5URL := strings.TrimSpace(response.H5URL)
	if verificationID == "" || h5URL == "" {
		return BytePlusVisualValidationSession{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance verification result missing"))
	}
	return BytePlusVisualValidationSession{BytedToken: verificationID, H5Link: h5URL}, nil
}

func (p seedanceProxyRealPersonProvider) GetVisualValidateResult(ctx context.Context, verificationID string) (BytePlusVisualValidationResult, error) {
	verificationID = strings.TrimSpace(verificationID)
	if verificationID == "" {
		return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(0, errors.New("seedance verification id missing"))
	}
	body, err := p.doJSON(ctx, http.MethodGet, seedanceProxyFaceVerificationPath+"/"+url.PathEscape(verificationID), nil, nil)
	if err != nil {
		return BytePlusVisualValidationResult{}, err
	}
	var response seedanceProxyVerificationResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, err)
	}
	if observedID := strings.TrimSpace(response.VerificationID); observedID != "" && observedID != verificationID {
		return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance verification id mismatch"))
	}
	status := strings.ToLower(strings.TrimSpace(response.Status))
	switch status {
	case "verified":
		groupID := strings.TrimSpace(response.GroupID)
		if groupID == "" {
			return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance verification group missing"))
		}
		return BytePlusVisualValidationResult{GroupID: groupID}, nil
	case "failed", "expired":
		return BytePlusVisualValidationResult{}, seedanceProxyVerificationTerminalFailure(status, http.StatusOK)
	case "waiting_user", "callback_received", "resolving", "":
		return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance verification still pending"))
	default:
		return BytePlusVisualValidationResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance verification status invalid"))
	}
}

func (p seedanceProxyRealPersonProvider) CreateAsset(ctx context.Context, request BytePlusCreateAssetRequest) (string, string, error) {
	request.GroupID = strings.TrimSpace(request.GroupID)
	request.URL = strings.TrimSpace(request.URL)
	assetType, err := seedanceProxyAssetNormalizeType(request.AssetType)
	if err != nil || request.GroupID == "" || request.URL == "" {
		if err == nil {
			err = errors.New("seedance real person asset input missing")
		}
		return "", "", seedanceProxyRealPersonProtocolFailure(0, err)
	}
	payload, err := common.Marshal(seedanceProxyAssetCreateRequest{
		GroupID:   request.GroupID,
		URL:       request.URL,
		AssetType: assetType,
		Name:      strings.TrimSpace(request.Name),
	})
	if err != nil {
		return "", "", seedanceProxyRealPersonProtocolFailure(0, err)
	}
	body, err := p.doJSON(ctx, http.MethodPost, seedanceProxyAssetUploadPath, payload, nil)
	if err != nil {
		return "", "", err
	}
	var response seedanceProxyAssetResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return "", "", seedanceProxyRealPersonProtocolFailure(http.StatusOK, err)
	}
	if upstreamGroupID := strings.TrimSpace(response.Result.GroupID); upstreamGroupID != "" && upstreamGroupID != request.GroupID {
		return "", "", seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance real person asset group mismatch"))
	}
	assetID := strings.TrimSpace(response.Result.ID)
	if assetID == "" {
		return "", "", seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance real person asset id missing"))
	}
	return assetID, "", nil
}

func (p seedanceProxyRealPersonProvider) GetAsset(ctx context.Context, upstreamAssetID string) (BytePlusAssetStatus, error) {
	upstreamAssetID = strings.TrimSpace(upstreamAssetID)
	if upstreamAssetID == "" {
		return BytePlusAssetStatus{}, seedanceProxyRealPersonProtocolFailure(0, errors.New("seedance real person asset id missing"))
	}
	body, err := p.doJSON(ctx, http.MethodGet, seedanceProxyAssetUploadPath+"/"+url.PathEscape(upstreamAssetID), nil, nil)
	if err != nil {
		return BytePlusAssetStatus{}, err
	}
	var response seedanceProxyAssetResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return BytePlusAssetStatus{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, err)
	}
	observedID := strings.TrimSpace(response.Result.ID)
	if observedID != "" && observedID != upstreamAssetID {
		return BytePlusAssetStatus{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance real person asset id mismatch"))
	}
	status, ok := seedanceProxyAssetNormalizeStatus(response.Result.Status, false)
	if !ok {
		return BytePlusAssetStatus{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance real person asset status invalid"))
	}
	return BytePlusAssetStatus{
		UpstreamAssetID: upstreamAssetID,
		Status:          status,
		ErrorMessage:    strings.TrimSpace(response.Result.Error.Message),
	}, nil
}

func (p seedanceProxyRealPersonProvider) ListAssets(ctx context.Context, request BytePlusListAssetsRequest) (BytePlusListAssetsResult, error) {
	allowedGroups := make(map[string]bool, len(request.GroupIDs))
	for _, groupID := range request.GroupIDs {
		if groupID = strings.TrimSpace(groupID); groupID != "" {
			allowedGroups[groupID] = true
		}
	}
	if len(allowedGroups) == 0 {
		return BytePlusListAssetsResult{}, seedanceProxyRealPersonProtocolFailure(0, errors.New("seedance real person group missing"))
	}
	query := url.Values{}
	query.Set("GroupType", "LivenessFace")
	if request.PageNumber > 0 {
		query.Set("PageNumber", strconv.Itoa(request.PageNumber))
	}
	if request.PageSize > 0 {
		query.Set("PageSize", strconv.Itoa(request.PageSize))
	}
	if name := strings.TrimSpace(request.Name); name != "" {
		query.Set("Name", name)
	}
	for _, status := range request.Statuses {
		if status = strings.TrimSpace(status); status != "" {
			query.Add("Statuses", status)
		}
	}
	body, err := p.doJSON(ctx, http.MethodGet, seedanceProxyAssetUploadPath, nil, query)
	if err != nil {
		return BytePlusListAssetsResult{}, err
	}
	var response struct {
		Result seedanceProxyRealPersonAssetListResponse `json:"Result"`
	}
	if err := common.Unmarshal(body, &response); err != nil {
		return BytePlusListAssetsResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, err)
	}
	items := make([]BytePlusListedAsset, 0, len(response.Result.Items))
	for _, item := range response.Result.Items {
		if groupID := strings.TrimSpace(item.GroupID); groupID == "" || !allowedGroups[groupID] {
			return BytePlusListAssetsResult{}, seedanceProxyRealPersonProtocolFailure(http.StatusOK, errors.New("seedance real person asset group mismatch"))
		}
		items = append(items, item)
	}
	return BytePlusListAssetsResult{Items: items, TotalCount: response.Result.TotalCount}, nil
}

func (p seedanceProxyRealPersonProvider) DeleteAsset(ctx context.Context, upstreamAssetID string) (string, error) {
	upstreamAssetID = strings.TrimSpace(upstreamAssetID)
	if upstreamAssetID == "" {
		return "", seedanceProxyRealPersonProtocolFailure(0, errors.New("seedance real person asset id missing"))
	}
	_, err := p.doJSON(ctx, http.MethodDelete, seedanceProxyAssetUploadPath+"/"+url.PathEscape(upstreamAssetID), nil, nil)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (p seedanceProxyRealPersonProvider) doJSON(ctx context.Context, method, path string, payload []byte, query url.Values) ([]byte, error) {
	requestURL := strings.TrimRight(p.gatewayBaseURL, "/") + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, seedanceProxyRealPersonProtocolFailure(0, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(p.apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client, err := seedanceProxyAssetHTTPClientFactory(p.channel)
	if err != nil || client == nil {
		if err == nil {
			err = errors.New("seedance gateway http client unavailable")
		}
		return nil, seedanceProxyRealPersonProtocolFailure(0, err)
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) {
			return nil, newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", err)
		}
		return nil, newAssetMaterializeFailure(AssetMaterializeErrorProcessing, 0, "", 0, "", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, seedanceProxyRealPersonResponseMax+1))
	if readErr != nil || len(responseBody) > seedanceProxyRealPersonResponseMax {
		return nil, seedanceProxyRealPersonProtocolFailure(response.StatusCode, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, seedanceProxyRealPersonHTTPFailure(response, responseBody)
	}
	return responseBody, nil
}

func seedanceProxyVerificationTerminalFailure(status string, httpStatus int) error {
	status = strings.ToLower(strings.TrimSpace(status))
	failure := newAssetMaterializeFailure(AssetMaterializeErrorDefinitive, httpStatus, "", 0, "", errors.New("seedance verification terminal status"))
	return &seedanceProxyVerificationTerminalError{status: status, failure: failure}
}

func seedanceProxyRealPersonProtocolFailure(status int, cause error) error {
	if cause == nil {
		cause = errors.New("seedance gateway response invalid")
	}
	return newAssetMaterializeFailure(AssetMaterializeErrorProcessing, status, "", 0, "", cause)
}

func seedanceProxyRealPersonHTTPFailure(response *http.Response, body []byte) error {
	status := 0
	var headers http.Header
	if response != nil {
		status = response.StatusCode
		headers = response.Header
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		LegacyError struct {
			Code string `json:"Code"`
		} `json:"Error"`
	}
	_ = common.Unmarshal(body, &envelope)
	code := strings.TrimSpace(envelope.Error.Code)
	if code == "" {
		code = strings.TrimSpace(envelope.LegacyError.Code)
	}
	return newAssetMaterializeFailure(assetMaterializeClassForHTTPStatus(status, code), status, code, parseAssetMaterializeRetryAfter(headers.Get("Retry-After"), time.Now()), "", nil)
}
