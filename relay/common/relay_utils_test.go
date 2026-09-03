package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/first.png"}, storedReq.Images)
	require.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestClaudeFable5ModelFamilyPredicate(t *testing.T) {
	for _, tt := range []struct {
		name  string
		model string
		want  bool
	}{
		{name: "canonical", model: "claude-fable-5", want: true},
		{name: "provider revision", model: "anthropic/claude-fable-5.1:stable", want: true},
		{name: "thinking suffix", model: "CLAUDE-FABLE-5.1-thinking", want: true},
		{name: "different major model", model: "claude-fable-50", want: false},
		{name: "different family", model: "claude-opus-4-8", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsClaudeFable5Model(tt.model))
		})
	}
}
