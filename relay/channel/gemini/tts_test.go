package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiSpeechConfigProbe struct {
	VoiceConfig struct {
		PrebuiltVoiceConfig struct {
			VoiceName string `json:"voiceName"`
		} `json:"prebuiltVoiceConfig"`
	} `json:"voiceConfig"`
}

func TestConvertAudioRequestBuildsGeminiTTSPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-preview-tts",
		},
		OriginModelName: "gemini-2.5-flash-preview-tts",
	}

	cases := []struct {
		name      string
		voice     string
		wantVoice string
	}{
		{name: "default", voice: "", wantVoice: "Kore"},
		{name: "alias", voice: "echo", wantVoice: "Puck"},
		{name: "passthrough", voice: "GeminiVoiceX", wantVoice: "GeminiVoiceX"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, err := (&Adaptor{}).ConvertAudioRequest(c, info, dto.AudioRequest{
				Model:          "gemini-2.5-flash-preview-tts",
				Input:          "hello from playground",
				Voice:          tc.voice,
				ResponseFormat: "mp3",
			})
			require.NoError(t, err)
			require.NotNil(t, body)

			raw, err := io.ReadAll(body)
			require.NoError(t, err)

			var req dto.GeminiChatRequest
			require.NoError(t, common.Unmarshal(raw, &req))
			require.Len(t, req.Contents, 1)
			require.Equal(t, "user", req.Contents[0].Role)
			require.Len(t, req.Contents[0].Parts, 1)
			require.Equal(t, "hello from playground", req.Contents[0].Parts[0].Text)
			require.Equal(t, []string{"AUDIO"}, req.GenerationConfig.ResponseModalities)

			var speech geminiSpeechConfigProbe
			require.NoError(t, common.Unmarshal(req.GenerationConfig.SpeechConfig, &speech))
			require.Equal(t, tc.wantVoice, speech.VoiceConfig.PrebuiltVoiceConfig.VoiceName)
		})
	}
}

func TestGeminiDoResponseWrapsPCMInlineDataAsWAV(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIAudio,
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-preview-tts",
		},
		OriginModelName: "gemini-2.5-flash-preview-tts",
	}
	info.SetEstimatePromptTokens(37)

	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	body, err := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "ignore me"},
						{
							InlineData: &dto.GeminiInlineData{
								MimeType: "audio/L16;codec=pcm;rate=24000",
								Data:     base64.StdEncoding.EncodeToString(pcm),
							},
						},
						{
							InlineData: &dto.GeminiInlineData{
								MimeType: "audio/L16;codec=pcm;rate=24000",
								Data:     "not-base64",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}

	usage, newAPIError := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	usageValue, ok := usage.(*dto.Usage)
	require.True(t, ok)

	got := recorder.Body.Bytes()
	require.Equal(t, "audio/wav", recorder.Header().Get("Content-Type"))
	require.Len(t, got, 48)
	require.Equal(t, []byte("RIFF"), got[:4])
	require.Equal(t, []byte("WAVE"), got[8:12])
	require.Equal(t, uint32(40), binary.LittleEndian.Uint32(got[4:8]))
	require.Equal(t, uint32(4), binary.LittleEndian.Uint32(got[40:44]))
	require.Equal(t, pcm, got[44:])

	require.Equal(t, 37, usageValue.PromptTokens)
	require.Equal(t, 17, usageValue.CompletionTokens)
	require.Equal(t, 54, usageValue.TotalTokens)
}

func TestGeminiDoResponseRejectsMissingAudioInlineData(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIAudio,
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-preview-tts",
		},
		OriginModelName: "gemini-2.5-flash-preview-tts",
	}

	body, err := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: "only text"}},
				},
			},
		},
	})
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}

	usage, newAPIError := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
	require.Contains(t, newAPIError.Error(), "audio")
}
