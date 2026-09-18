package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestGetRequestURLOpenRouterDecisions(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeDecisions,
		RequestURLPath: "/api/alpha/decisions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			ChannelBaseUrl:    "https://openrouter.ai/api",
			UpstreamModelName: "typesafe/jev-1.13",
		},
	}

	got, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	if want := "https://openrouter.ai/api/alpha/decisions"; got != want {
		t.Fatalf("GetRequestURL = %q, want %q", got, want)
	}
}

func TestConvertOpenAIRequestDecisionsPreservesNativeFields(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:     "typesafe/jev-1.13",
		State:     json.RawMessage(`{"text":"hello"}`),
		Questions: json.RawMessage(`{"route":{"type":"choice"}}`),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeDecisions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok || got != request {
		t.Fatalf("decisions request should be returned unchanged, got %T/%p", converted, converted)
	}
	if string(got.State) != `{"text":"hello"}` || string(got.Questions) != `{"route":{"type":"choice"}}` {
		t.Fatalf("native fields changed: state=%s questions=%s", got.State, got.Questions)
	}
	if len(got.Usage) != 0 || len(got.Reasoning) != 0 {
		t.Fatalf("chat-specific OpenRouter fields should not be injected: usage=%s reasoning=%s", got.Usage, got.Reasoning)
	}
}

func TestGetRequestURLOpenRouterDecisionsNormalizesV1Base(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeDecisions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenRouter,
			ChannelBaseUrl: "https://openrouter.ai/api/v1/",
		},
	}

	got, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	if want := "https://openrouter.ai/api/alpha/decisions"; got != want {
		t.Fatalf("GetRequestURL = %q, want %q", got, want)
	}
}
