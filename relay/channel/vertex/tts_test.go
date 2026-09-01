package vertex

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

func TestConvertAudioRequestDelegatesVertexGeminiTTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeAudioSpeech,
		OriginModelName: "gemini-2.5-flash-tts",
	}
	body, err := (&Adaptor{}).ConvertAudioRequest(c, info, dto.AudioRequest{
		Model:          "gemini-2.5-flash-tts",
		Input:          "hello from vertex",
		Voice:          "Kore",
		ResponseFormat: "wav",
	})
	require.NoError(t, err)

	raw, err := io.ReadAll(body)
	require.NoError(t, err)

	var request dto.GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &request))
	require.Len(t, request.Contents, 1)
	require.Equal(t, "hello from vertex", request.Contents[0].Parts[0].Text)
	require.Equal(t, []string{"AUDIO"}, request.GenerationConfig.ResponseModalities)
}

func TestDoResponseRoutesVertexGeminiTTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIAudio,
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-tts",
		},
		OriginModelName: "gemini-2.5-flash-tts",
	}

	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	body, err := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{{
						InlineData: &dto.GeminiInlineData{
							MimeType: "audio/L16;rate=24000",
							Data:     base64.StdEncoding.EncodeToString(pcm),
						},
					}},
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

	usage, apiErr := (&Adaptor{RequestMode: RequestModeGemini}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.IsType(t, &dto.Usage{}, usage)
	require.Equal(t, "audio/wav", recorder.Header().Get("Content-Type"))
	require.Equal(t, []byte("RIFF"), recorder.Body.Bytes()[:4])
	require.Equal(t, uint32(4), binary.LittleEndian.Uint32(recorder.Body.Bytes()[40:44]))
	require.Equal(t, pcm, recorder.Body.Bytes()[44:])
}
