package relay

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestNormalizeImageUsageForBillingOnlyFallbacksPerCall(t *testing.T) {
	t.Run("token-priced usage keeps zero prompt tokens", func(t *testing.T) {
		usage := &dto.Usage{CompletionTokens: 196}
		normalizeImageUsageForBilling(usage, false)
		if usage.PromptTokens != 0 || usage.TotalTokens != 0 {
			t.Fatalf("token usage was synthesized: %+v", usage)
		}
	})

	t.Run("per-call empty usage still gets a billable marker", func(t *testing.T) {
		usage := &dto.Usage{}
		normalizeImageUsageForBilling(usage, true)
		if usage.PromptTokens != 1 || usage.TotalTokens != 1 {
			t.Fatalf("per-call fallback missing: %+v", usage)
		}
	})
}

func TestLogImageRequestBodyForDebugRedactsGrokImageData(t *testing.T) {
	const secretMarker = "SECRET_BASE64_MARKER"
	oldDebug := common.DebugEnabled
	common.DebugEnabled = true
	defer func() { common.DebugEnabled = oldDebug }()

	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	defer func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	}()

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesEdits,
		RelayFormat: types.RelayFormatOpenAIImage,
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeGrokSubscription},
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	logImageRequestBodyForDebug(c, info, []byte(`{"model":"grok-imagine-image-2.0","image":{"image_url":{"url":"data:image/png;base64,`+secretMarker+`"}},"images":[{"image_url":{"url":"https://example.com/a.png"}}]}`))

	got := logs.String()
	if got == "" {
		t.Fatalf("expected a debug log entry")
	}
	if bytes.Contains([]byte(got), []byte(secretMarker)) ||
		bytes.Contains([]byte(got), []byte("data:image")) ||
		bytes.Contains([]byte(got), []byte("example.com")) {
		t.Fatalf("debug log leaked image payload: %s", got)
	}
}
