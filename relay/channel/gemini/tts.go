package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var openAIVoiceToGeminiVoice = map[string]string{
	"alloy":   "Kore",
	"echo":    "Puck",
	"fable":   "Charon",
	"onyx":    "Fenrir",
	"nova":    "Aoede",
	"shimmer": "Leda",
}

type geminiTTSRequestSpeechConfig struct {
	VoiceConfig geminiTTSVoiceConfig `json:"voiceConfig,omitempty"`
}

type geminiTTSVoiceConfig struct {
	PrebuiltVoiceConfig geminiTTSPrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

type geminiTTSPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName,omitempty"`
}

func mapOpenAIVoiceToGeminiVoice(voice string) string {
	trimmed := strings.TrimSpace(voice)
	if trimmed == "" {
		return "Kore"
	}
	if mapped, ok := openAIVoiceToGeminiVoice[strings.ToLower(trimmed)]; ok {
		return mapped
	}
	return trimmed
}

func isGeminiTTSModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "tts")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info != nil && info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}
	if streamFormat := strings.ToLower(strings.TrimSpace(request.StreamFormat)); streamFormat != "" {
		return nil, errors.New("Gemini TTS does not support streaming audio; omit stream_format")
	}
	if strings.TrimSpace(request.Input) == "" {
		return nil, errors.New("input is required")
	}
	if responseFormat := strings.ToLower(strings.TrimSpace(request.ResponseFormat)); responseFormat != "" && responseFormat != "wav" {
		return nil, errors.New("Gemini TTS does not support this response format; use wav")
	}
	if request.Speed != nil {
		speed := *request.Speed
		if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0.25 || speed > 4 {
			return nil, errors.New("speed must be between 0.25 and 4")
		}
		if math.Abs(speed-1) > 0.0001 {
			return nil, errors.New("Gemini TTS does not support numeric speed; use instructions")
		}
	}

	inputText := request.Input
	inputSections := make([]string, 0, 2)
	if instructions := strings.TrimSpace(request.Instructions); instructions != "" {
		inputSections = append(inputSections, instructions)
	}
	if len(inputSections) > 0 {
		inputSections = append(inputSections, inputText)
		inputText = strings.Join(inputSections, "\n\n")
	}

	ttsRequest := dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: inputText},
				},
			},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			ResponseModalities: []string{"AUDIO"},
		},
	}

	speechConfig, err := common.Marshal(geminiTTSRequestSpeechConfig{
		VoiceConfig: geminiTTSVoiceConfig{
			PrebuiltVoiceConfig: geminiTTSPrebuiltVoiceConfig{
				VoiceName: mapOpenAIVoiceToGeminiVoice(request.Voice),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gemini speech config: %w", err)
	}
	ttsRequest.GenerationConfig.SpeechConfig = speechConfig

	jsonData, err := common.Marshal(ttsRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini tts request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

type geminiTTSAudioPayload struct {
	mimeType     string
	decodedBytes []byte
	contentType  string
	duration     float64
}

func GeminiTTSHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var geminiResponse dto.GeminiChatResponse
	if unmarshalErr := common.Unmarshal(responseBody, &geminiResponse); unmarshalErr != nil {
		return nil, types.NewOpenAIError(unmarshalErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	audioPart, ok := findFirstGeminiAudioInlineData(&geminiResponse)
	if !ok {
		return nil, types.NewErrorWithStatusCode(
			errors.New("no audio data in gemini tts response"),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	payload, buildErr := decodeGeminiTTSAudio(c.Request.Context(), audioPart)
	if buildErr != nil {
		return nil, types.NewErrorWithStatusCode(buildErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	c.Header("Content-Type", payload.contentType)
	c.Data(http.StatusOK, payload.contentType, payload.decodedBytes)

	usageValue := buildGeminiTTSUsage(info, geminiResponse.UsageMetadata, payload.duration)
	return usageValue, nil
}

func findFirstGeminiAudioInlineData(response *dto.GeminiChatResponse) (*dto.GeminiInlineData, bool) {
	if response == nil {
		return nil, false
	}
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil {
				continue
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(part.InlineData.MimeType)), "audio/") {
				return part.InlineData, true
			}
		}
	}
	return nil, false
}

func decodeGeminiTTSAudio(ctx context.Context, inlineData *dto.GeminiInlineData) (*geminiTTSAudioPayload, error) {
	if inlineData == nil {
		return nil, errors.New("audio data is missing")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(inlineData.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode gemini tts audio: %w", err)
	}
	if len(decoded) == 0 {
		return nil, errors.New("audio data is empty")
	}

	mimeType := strings.ToLower(strings.TrimSpace(inlineData.MimeType))
	if mimeType == "" {
		mimeType = "audio/wav"
	}

	if isPCMInlineData(mimeType) {
		sampleRate, channels := parsePCMInlineDataParams(mimeType)
		wavData := wrapPCMAsWAV(decoded, sampleRate, channels)
		duration := pcmDurationSeconds(len(decoded), sampleRate, channels)
		return &geminiTTSAudioPayload{
			mimeType:     mimeType,
			decodedBytes: wavData,
			contentType:  "audio/wav",
			duration:     duration,
		}, nil
	}

	contentType := normalizeAudioContentType(mimeType)
	duration, durationErr := estimateAudioDuration(ctx, contentType, decoded)
	if durationErr != nil {
		duration = 0
	}

	return &geminiTTSAudioPayload{
		mimeType:     mimeType,
		decodedBytes: decoded,
		contentType:  contentType,
		duration:     duration,
	}, nil
}

func isPCMInlineData(mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(normalized, "pcm") || strings.HasPrefix(normalized, "audio/l16")
}

func parsePCMInlineDataParams(mimeType string) (sampleRate int, channels int) {
	sampleRate = 24000
	channels = 1

	for _, segment := range strings.Split(mimeType, ";") {
		segment = strings.TrimSpace(segment)
		key, value, ok := strings.Cut(segment, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "rate", "sample_rate":
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
				sampleRate = parsed
			}
		case "channels", "channel":
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
				channels = parsed
			}
		}
	}

	return sampleRate, channels
}

func normalizeAudioContentType(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	switch normalized {
	case "audio/mp3":
		return "audio/mpeg"
	case "audio/mpeg", "audio/wav", "audio/x-wav", "audio/ogg", "audio/opus", "audio/aac", "audio/flac", "audio/webm", "audio/mp4", "audio/x-m4a", "audio/aiff", "audio/x-aiff":
		return normalized
	default:
		// Keep the browser-facing response on an audio media type even when an
		// upstream sends an unfamiliar or malformed audio MIME value.
		return "audio/wav"
	}
}

func estimateAudioDuration(ctx context.Context, contentType string, data []byte) (float64, error) {
	ext := ""
	switch contentType {
	case "audio/mpeg":
		ext = ".mp3"
	case "audio/wav", "audio/x-wav":
		ext = ".wav"
	case "audio/ogg":
		ext = ".ogg"
	case "audio/opus":
		ext = ".opus"
	case "audio/aac":
		ext = ".aac"
	case "audio/flac":
		ext = ".flac"
	case "audio/mp4", "audio/x-m4a":
		ext = ".m4a"
	case "audio/aiff", "audio/x-aiff":
		ext = ".aiff"
	case "audio/webm":
		ext = ".webm"
	default:
		return 0, fmt.Errorf("unsupported audio content type: %s", contentType)
	}

	duration, err := common.GetAudioDuration(ctx, bytes.NewReader(data), ext)
	if err != nil {
		return 0, err
	}
	return duration, nil
}

func pcmDurationSeconds(dataLen int, sampleRate int, channels int) float64 {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	if channels <= 0 {
		channels = 1
	}
	return float64(dataLen) / float64(sampleRate*channels*2)
}

func wrapPCMAsWAV(pcm []byte, sampleRate int, channels int) []byte {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	if channels <= 0 {
		channels = 1
	}

	const bitsPerSample = 16
	blockAlign := channels * bitsPerSample / 8
	byteRate := sampleRate * blockAlign
	dataSize := len(pcm)
	chunkSize := 36 + dataSize

	buf := make([]byte, 0, 44+dataSize)
	buf = append(buf, 'R', 'I', 'F', 'F')
	buf = appendUint32LE(buf, uint32(chunkSize))
	buf = append(buf, 'W', 'A', 'V', 'E')
	buf = append(buf, 'f', 'm', 't', ' ')
	buf = appendUint32LE(buf, 16)
	buf = appendUint16LE(buf, 1)
	buf = appendUint16LE(buf, uint16(channels))
	buf = appendUint32LE(buf, uint32(sampleRate))
	buf = appendUint32LE(buf, uint32(byteRate))
	buf = appendUint16LE(buf, uint16(blockAlign))
	buf = appendUint16LE(buf, bitsPerSample)
	buf = append(buf, 'd', 'a', 't', 'a')
	buf = appendUint32LE(buf, uint32(dataSize))
	buf = append(buf, pcm...)
	return buf
}

func appendUint16LE(dst []byte, value uint16) []byte {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], value)
	return append(dst, buf[:]...)
}

func appendUint32LE(dst []byte, value uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	return append(dst, buf[:]...)
}

func buildGeminiTTSUsage(info *relaycommon.RelayInfo, metadata dto.GeminiUsageMetadata, duration float64) *dto.Usage {
	fallbackPromptTokens := 0
	if info != nil {
		fallbackPromptTokens = info.GetEstimatePromptTokens()
	}
	if metadata.TotalTokenCount > 0 {
		usage := buildUsageFromGeminiMetadata(metadata, fallbackPromptTokens)
		return &usage
	}

	promptTokens := fallbackPromptTokens
	usage := &dto.Usage{
		PromptTokens: promptTokens,
	}
	usage.PromptTokensDetails.TextTokens = promptTokens
	if duration > 0 {
		completionTokens := int(math.Round(math.Ceil(duration) / 60.0 * 1000))
		usage.CompletionTokens = completionTokens
		usage.CompletionTokenDetails.AudioTokens = completionTokens
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}
