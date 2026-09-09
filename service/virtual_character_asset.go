package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	virtualCharacterAssetPath            = "/v1/virtual-characters"
	virtualCharacterAssetResponseMaxSize = 1 << 20
	virtualCharacterImageMaxSize         = 30 << 20
	virtualCharacterVideoMaxSize         = 50 << 20
	virtualCharacterAudioMaxSize         = 15 << 20
	virtualCharacterProviderAssetIDMax   = 191
)

var (
	errVirtualCharacterProtocol = errors.New("virtual character response invalid")
	errVirtualCharacterRedirect = errors.New("virtual character redirect rejected")
	errVirtualCharacterTooLarge = errors.New("virtual character source too large")

	virtualCharacterAssetFetchSource       = fetchAssetSource
	virtualCharacterAssetHTTPClientFactory = func(channel *model.Channel) (*http.Client, error) {
		return GetHttpClientWithProxy(strings.TrimSpace(channel.GetSetting().Proxy))
	}
)

type virtualCharacterAssetBindingMaterializer struct{}

type virtualCharacterAssetResponse struct {
	Success bool                      `json:"success"`
	Data    virtualCharacterAssetData `json:"data"`
	Error   struct {
		Code string `json:"code"`
	} `json:"error"`
}

type virtualCharacterAssetData struct {
	ID              int64  `json:"id"`
	Status          string `json:"status"`
	ProviderAssetID string `json:"provider_asset_id"`
	SourceType      string `json:"source_type"`
	AssetType       string `json:"asset_type"`
}

func (virtualCharacterAssetBindingMaterializer) CreateAsset(ctx context.Context, input AssetMaterializeInput) (AssetMaterializeResult, error) {
	config, ok := virtualCharacterMaterializationConfig(input.Channel)
	if !ok || strings.TrimSpace(input.APIKey) == "" {
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	assetType, maxSize, err := virtualCharacterAssetType(input.Asset.AssetType)
	if err != nil {
		return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(err)
	}
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" && input.SignSource != nil {
		sourceURL, err = input.SignSource(ctx, input.Asset)
		if err != nil {
			return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
		}
	}
	if sourceURL == "" {
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	source, err := virtualCharacterAssetFetchSource(ctx, sourceURL)
	if err != nil || source == nil || source.Body == nil {
		if source != nil && source.Body != nil {
			_ = source.Body.Close()
		}
		if errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) {
			return AssetMaterializeResult{}, newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", nil)
		}
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
	}
	if source.StatusCode < 200 || source.StatusCode >= 300 {
		_ = source.Body.Close()
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
	}
	if source.ContentLength > maxSize {
		_ = source.Body.Close()
		return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(errVirtualCharacterTooLarge)
	}
	contentType, filename, err := virtualCharacterSourceMetadata(source, input.Asset, assetType)
	if err != nil {
		_ = source.Body.Close()
		return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(err)
	}

	baseClient, err := virtualCharacterAssetHTTPClientFactory(input.Channel)
	if err != nil || baseClient == nil {
		_ = source.Body.Close()
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	client := *baseClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errVirtualCharacterRedirect }
	requestURL := config.GatewayOrigin + virtualCharacterAssetPath
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, pipeReader)
	if err != nil {
		_ = source.Body.Close()
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.APIKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if idempotencyKey := strings.TrimSpace(input.IdempotencyKey); idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	writeDone := make(chan error, 1)
	go func() {
		defer source.Body.Close()
		writeDone <- writeVirtualCharacterMultipart(pipeWriter, writer, opaqueBytePlusAssetName(), assetType, filename, contentType, source.Body, maxSize)
	}()
	response, requestErr := client.Do(req)
	_ = pipeReader.Close()
	_ = source.Body.Close()
	writeErr := <-writeDone
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if errors.Is(writeErr, errVirtualCharacterTooLarge) {
		return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(writeErr)
	}
	if requestErr != nil {
		if errors.Is(requestErr, context.DeadlineExceeded) || isNetTimeout(requestErr) {
			return AssetMaterializeResult{}, newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", nil)
		}
		if errors.Is(requestErr, errVirtualCharacterRedirect) {
			return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(errVirtualCharacterRedirect)
		}
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
	}
	if response == nil {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, err := readVirtualCharacterResponse(response)
		return AssetMaterializeResult{}, err
	}
	if writeErr != nil {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(response.StatusCode, nil)
	}
	upstream, err := readVirtualCharacterResponse(response)
	if err != nil {
		return AssetMaterializeResult{}, err
	}
	if response.StatusCode != http.StatusCreated {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	return virtualCharacterCreateResult(upstream, assetType)
}

func (virtualCharacterAssetBindingMaterializer) GetAsset(ctx context.Context, input AssetMaterializeInput, upstreamAssetID string) (AssetMaterializeResult, error) {
	config, ok := virtualCharacterMaterializationConfig(input.Channel)
	if !ok || strings.TrimSpace(input.APIKey) == "" {
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	managementID, err := virtualCharacterManagementID(upstreamAssetID)
	if err != nil {
		return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(err)
	}
	baseClient, err := virtualCharacterAssetHTTPClientFactory(input.Channel)
	if err != nil || baseClient == nil {
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	client := *baseClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errVirtualCharacterRedirect }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.GatewayOrigin+virtualCharacterAssetPath+"/"+managementID, nil)
	if err != nil {
		return AssetMaterializeResult{}, ErrAssetBindingUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.APIKey))
	response, requestErr := client.Do(req)
	if response != nil && response.Body != nil && requestErr != nil {
		_ = response.Body.Close()
	}
	if requestErr != nil {
		if errors.Is(requestErr, context.DeadlineExceeded) || isNetTimeout(requestErr) {
			return AssetMaterializeResult{}, newAssetMaterializeFailure(AssetMaterializeErrorTimeout, 0, "", 0, "", nil)
		}
		if errors.Is(requestErr, errVirtualCharacterRedirect) {
			return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(errVirtualCharacterRedirect)
		}
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(0, nil)
	}
	defer response.Body.Close()
	upstream, err := readVirtualCharacterResponse(response)
	if err != nil {
		return AssetMaterializeResult{}, err
	}
	if response.StatusCode != http.StatusOK || strconv.FormatInt(upstream.Data.ID, 10) != managementID {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	expectedType := ""
	if strings.TrimSpace(input.Asset.AssetType) != "" {
		expectedType, _, err = virtualCharacterAssetType(input.Asset.AssetType)
		if err != nil {
			return AssetMaterializeResult{}, virtualCharacterDefinitiveFailure(err)
		}
	}
	return virtualCharacterGetResult(upstream, managementID, expectedType)
}

func virtualCharacterMaterializationConfig(channel *model.Channel) (assetMaterializationChannelConfig, bool) {
	config, explicit, err := assetMaterializationConfigForChannel(channel)
	if err != nil || !explicit || config.Provider != assetMaterializationProviderVirtualCharacter {
		return assetMaterializationChannelConfig{}, false
	}
	return config, true
}

func validateVirtualCharacterAssetMaterializationConfig(config assetMaterializationChannelConfig) (assetMaterializationChannelConfig, error) {
	origin, err := normalizedGatewayOrigin(config.GatewayBaseURL)
	if err != nil {
		return assetMaterializationChannelConfig{}, err
	}
	config.GatewayOrigin = origin
	config.GatewayBaseURL = origin
	config.GroupID = ""
	return config, nil
}

func virtualCharacterBindingScope(origin, apiKey string) string {
	origin = strings.TrimSpace(origin)
	apiKey = strings.TrimSpace(apiKey)
	if origin == "" || apiKey == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(origin + "\x00" + apiKey))
	return virtualCharacterBindingScopePrefix + hex.EncodeToString(digest[:])
}

func virtualCharacterValidProviderAssetID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > virtualCharacterProviderAssetIDMax {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func virtualCharacterAssetType(value string) (string, int64, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image":
		return "Image", virtualCharacterImageMaxSize, nil
	case "video":
		return "Video", virtualCharacterVideoMaxSize, nil
	case "audio":
		return "Audio", virtualCharacterAudioMaxSize, nil
	default:
		return "", 0, errors.New("virtual character asset type unsupported")
	}
}

func virtualCharacterManagementID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != trimmed {
		return "", errors.New("virtual character management id invalid")
	}
	return trimmed, nil
}

func virtualCharacterCreateResult(upstream virtualCharacterAssetResponse, expectedType string) (AssetMaterializeResult, error) {
	managementID := strconv.FormatInt(upstream.Data.ID, 10)
	if !upstream.Success || upstream.Data.ID <= 0 || upstream.Data.SourceType != "volc_aigc" || upstream.Data.AssetType != expectedType {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(http.StatusCreated, errVirtualCharacterProtocol)
	}
	result := AssetMaterializeResult{UpstreamAssetID: managementID}
	switch upstream.Data.Status {
	case "creating":
		result.Status = model.AssetStatusProcessing
		return result, nil
	case "active":
		if virtualCharacterValidProviderAssetID(upstream.Data.ProviderAssetID) {
			result.UpstreamGroupID = upstream.Data.ProviderAssetID
			result.Status = model.AssetStatusActive
			return result, nil
		}
		// Keep the management ID pollable so a temporarily incomplete Active
		// response cannot cause a second, non-idempotent upload.
		result.Status = model.AssetStatusProcessing
		return result, nil
	case "failed", "blocked", "offline", "deleting":
		result.Status = model.AssetStatusFailed
		return result, nil
	default:
		// Unknown creation states are polled by management ID rather than
		// starting a duplicate upload.
		result.Status = model.AssetStatusProcessing
		return result, nil
	}
}

func virtualCharacterGetResult(upstream virtualCharacterAssetResponse, managementID, expectedType string) (AssetMaterializeResult, error) {
	if !upstream.Success || upstream.Data.SourceType != "volc_aigc" || (expectedType != "" && upstream.Data.AssetType != expectedType) {
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
	result := AssetMaterializeResult{UpstreamAssetID: managementID}
	switch upstream.Data.Status {
	case "creating":
		result.Status = model.AssetStatusProcessing
		return result, nil
	case "active":
		if !virtualCharacterValidProviderAssetID(upstream.Data.ProviderAssetID) {
			return AssetMaterializeResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
		}
		result.UpstreamGroupID = upstream.Data.ProviderAssetID // provider-specific generation reference, not a group
		result.Status = model.AssetStatusActive
		return result, nil
	case "failed", "blocked", "offline", "deleting":
		result.Status = model.AssetStatusFailed
		return result, nil
	default:
		return AssetMaterializeResult{}, virtualCharacterProcessingFailure(http.StatusOK, errVirtualCharacterProtocol)
	}
}

func readVirtualCharacterResponse(response *http.Response) (virtualCharacterAssetResponse, error) {
	if response == nil || response.Body == nil {
		return virtualCharacterAssetResponse{}, virtualCharacterProcessingFailure(0, errVirtualCharacterProtocol)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, virtualCharacterAssetResponseMaxSize+1))
	if err != nil || len(body) > virtualCharacterAssetResponseMaxSize {
		return virtualCharacterAssetResponse{}, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	var upstream virtualCharacterAssetResponse
	if err := common.Unmarshal(body, &upstream); err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return virtualCharacterAssetResponse{}, virtualCharacterHTTPFailure(response, upstream)
		}
		return virtualCharacterAssetResponse{}, virtualCharacterProcessingFailure(response.StatusCode, errVirtualCharacterProtocol)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return virtualCharacterAssetResponse{}, virtualCharacterHTTPFailure(response, upstream)
	}
	return upstream, nil
}

func virtualCharacterHTTPFailure(response *http.Response, upstream virtualCharacterAssetResponse) error {
	status := 0
	var headers http.Header
	if response != nil {
		status = response.StatusCode
		headers = response.Header
	}
	return newAssetMaterializeFailure(
		assetMaterializeClassForHTTPStatus(status, strings.TrimSpace(upstream.Error.Code)),
		status,
		strings.TrimSpace(upstream.Error.Code),
		parseAssetMaterializeRetryAfter(headers.Get("Retry-After"), time.Now()),
		"",
		nil,
	)
}

func virtualCharacterProcessingFailure(status int, cause error) error {
	return newAssetMaterializeFailure(AssetMaterializeErrorProcessing, status, "", 0, "", cause)
}

func virtualCharacterDefinitiveFailure(cause error) error {
	return newAssetMaterializeFailure(AssetMaterializeErrorDefinitive, 0, "", 0, "", cause)
}

func virtualCharacterSourceMetadata(response *http.Response, asset model.Asset, assetType string) (string, string, error) {
	filename := path.Base(strings.ReplaceAll(strings.TrimSpace(asset.ObjectKey), "\\", "/"))
	if filename == "" || filename == "." || filename == "/" || strings.ContainsAny(filename, "\r\n") {
		filename = "asset"
	}
	extension := strings.ToLower(path.Ext(filename))
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = strings.TrimSpace(asset.ContentType)
	}
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = strings.ToLower(parsed)
	} else if contentType != "" {
		return "", "", errors.New("virtual character source content type invalid")
	}
	if contentType == "" || contentType == "application/octet-stream" {
		var ok bool
		contentType, ok = virtualCharacterContentTypeForExtension(assetType, extension)
		if !ok {
			return "", "", errors.New("virtual character source format unsupported")
		}
	}
	canonicalExtension, compatibleExtensions, ok := virtualCharacterFormat(assetType, contentType)
	if !ok {
		return "", "", errors.New("virtual character source format unsupported")
	}
	if !virtualCharacterExtensionAllowed(extension, compatibleExtensions) {
		filename = strings.TrimSuffix(filename, path.Ext(filename)) + canonicalExtension
	}
	return contentType, filename, nil
}

func virtualCharacterFormat(assetType, contentType string) (string, []string, bool) {
	switch assetType + "\x00" + contentType {
	case "Image\x00image/jpeg":
		return ".jpg", []string{".jpg", ".jpeg"}, true
	case "Image\x00image/png":
		return ".png", []string{".png"}, true
	case "Image\x00image/webp":
		return ".webp", []string{".webp"}, true
	case "Image\x00image/gif":
		return ".gif", []string{".gif"}, true
	case "Image\x00image/heic":
		return ".heic", []string{".heic"}, true
	case "Video\x00video/mp4":
		return ".mp4", []string{".mp4"}, true
	case "Video\x00video/quicktime":
		return ".mov", []string{".mov"}, true
	case "Audio\x00audio/mpeg":
		return ".mp3", []string{".mp3"}, true
	case "Audio\x00audio/wav", "Audio\x00audio/x-wav":
		return ".wav", []string{".wav"}, true
	default:
		return "", nil, false
	}
}

func virtualCharacterContentTypeForExtension(assetType, extension string) (string, bool) {
	switch assetType + "\x00" + extension {
	case "Image\x00.jpg", "Image\x00.jpeg":
		return "image/jpeg", true
	case "Image\x00.png":
		return "image/png", true
	case "Image\x00.webp":
		return "image/webp", true
	case "Image\x00.gif":
		return "image/gif", true
	case "Image\x00.heic":
		return "image/heic", true
	case "Video\x00.mp4":
		return "video/mp4", true
	case "Video\x00.mov":
		return "video/quicktime", true
	case "Audio\x00.mp3":
		return "audio/mpeg", true
	case "Audio\x00.wav":
		return "audio/wav", true
	default:
		return "", false
	}
}

func virtualCharacterExtensionAllowed(extension string, allowed []string) bool {
	for _, candidate := range allowed {
		if extension == candidate {
			return true
		}
	}
	return false
}

func writeVirtualCharacterMultipart(pipeWriter *io.PipeWriter, writer *multipart.Writer, name, assetType, filename, contentType string, source io.Reader, maxSize int64) error {
	closeWithError := func(err error) error {
		_ = pipeWriter.CloseWithError(err)
		return err
	}
	if err := writer.WriteField("name", name); err != nil {
		return closeWithError(err)
	}
	if err := writer.WriteField("asset_type", assetType); err != nil {
		return closeWithError(err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
	header.Set("Content-Type", contentType)
	filePart, err := writer.CreatePart(header)
	if err != nil {
		return closeWithError(err)
	}
	limited := &io.LimitedReader{R: source, N: maxSize + 1}
	written, err := io.Copy(filePart, limited)
	if err != nil {
		return closeWithError(err)
	}
	if written > maxSize {
		return closeWithError(errVirtualCharacterTooLarge)
	}
	if err := writer.Close(); err != nil {
		return closeWithError(err)
	}
	return pipeWriter.Close()
}
