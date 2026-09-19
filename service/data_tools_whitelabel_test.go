package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDataToolWhitelabelIDsRoundTrip(t *testing.T) {
	if got := PublicDataToolID("blockrun.audio.speech"); got != "flatkey.audio.speech" {
		t.Fatalf("public id = %q", got)
	}
	if got := UpstreamDataToolID("flatkey.audio.speech"); got != "blockrun.audio.speech" {
		t.Fatalf("upstream id = %q", got)
	}
	gateway := "gateway:monid:blockrun.ai:/api/v1/exa/answer"
	public := PublicDataToolID(gateway)
	if strings.Contains(strings.ToLower(public), "blockrun") || public != "gateway:monid:flatkey.ai:/api/v1/exa/answer" {
		t.Fatalf("gateway public id = %q", public)
	}
	if c := UpstreamDataToolIDCandidates(public); len(c) != 2 || c[0] != gateway || c[1] != public {
		t.Fatalf("gateway candidates = %v", c)
	}
	if c := UpstreamDataToolIDCandidates("apify.actor.run"); len(c) != 1 {
		t.Fatalf("plain id candidates = %v", c)
	}
	for _, id := range []string{"apify.actor.run", "native.search", ""} {
		if PublicDataToolID(id) != id || UpstreamDataToolID(id) != id {
			t.Fatalf("non-vendor id %q must pass through", id)
		}
	}
	if UpstreamDataToolPlatform("flatkey") != "BlockRun" || UpstreamDataToolPlatform("Apify") != "Apify" {
		t.Fatal("platform filter mapping wrong")
	}
}

func TestDataToolWhitelabelListAndInspection(t *testing.T) {
	list := &DataToolList{
		Tools: []DataToolSummary{{
			ID: "blockrun.chat.completions", Name: "Chat Completions", Provider: "blockrun:chat", Platform: "chat",
			Description: "Access frontier LLMs via BlockRun with an OpenAI-compatible API.", Categories: []string{"BlockRun native"},
		}, {
			ID: "apify.actor.run", Name: "Apify Actor", Provider: "apify", Platform: "Apify", Description: "Run an actor.",
		}},
		Platforms: []DataToolPlatform{{Platform: "BlockRun", Count: 24}, {Platform: "Apify", Count: 900}},
	}
	whitelabelDataToolList(list)
	raw, _ := json.Marshal(list)
	if strings.Contains(strings.ToLower(string(raw)), "blockrun") {
		t.Fatalf("vendor brand leaked: %s", raw)
	}
	if list.Tools[0].ID != "flatkey.chat.completions" || list.Tools[0].Provider != "flatkey:chat" || list.Platforms[0].Platform != "flatkey" {
		t.Fatalf("unexpected rewrite: %+v %+v", list.Tools[0], list.Platforms[0])
	}
	if list.Tools[1].ID != "apify.actor.run" || list.Tools[1].Platform != "Apify" {
		t.Fatalf("non-vendor tool changed: %+v", list.Tools[1])
	}

	ins := &DataToolInspection{ID: "blockrun.audio.speech", Name: "TTS", Provider: "blockrun:audio",
		Description: "Synthesize speech (BlockRun proxy).", Input: DataToolInputSchema{Properties: map[string]DataToolFieldSchema{
			"voice": {Type: "string", Description: "A BlockRun voice alias"}}}}
	whitelabelDataToolInspection(ins)
	raw, _ = json.Marshal(ins)
	if strings.Contains(strings.ToLower(string(raw)), "blockrun") {
		t.Fatalf("vendor brand leaked in inspection: %s", raw)
	}
	if ins.ID != "flatkey.audio.speech" {
		t.Fatalf("inspection id = %q", ins.ID)
	}
}
