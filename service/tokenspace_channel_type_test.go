package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestTokenSpaceDoesNotAdvertiseOpenAIResponses(t *testing.T) {
	if channelSupportsOpenAIResponses(constant.ChannelTypeTokenSpace) {
		t.Fatal("TokenSpace async video channel must not be selected for OpenAI Responses")
	}
}
