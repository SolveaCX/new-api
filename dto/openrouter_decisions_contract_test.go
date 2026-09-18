package dto

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// These fixtures mirror OpenRouter's /api/alpha/decisions wire contract.  The
// endpoint is deliberately not treated as chat/completions: a request carries
// state plus named, typed questions and the response carries named answers.
const openRouterDecisionsRequestFixture = `{
  "model": "typesafe/jev-1.13",
  "state": {"text": "route this request", "priority": 2},
  "questions": {
    "is_safe": {
      "type": "noul",
      "instructions": "Is the request safe?",
      "criteria": {"true": "safe", "false": "unsafe"}
    },
    "route": {
      "type": "choice",
      "instructions": "Choose the best route.",
      "criteria": {"fast": "latency is most important", "cheap": "cost is most important"}
    },
    "score": {
      "type": "score",
      "instructions": "Score the request.",
      "criteria": ["poor", "acceptable", "excellent"]
    }
  }
}`

const openRouterDecisionsResponseFixture = `{
  "id": "dec_123",
  "model": "typesafe/jev-1.13",
  "provider": "TypeSafe",
  "answers": {
    "is_safe": {"type": "noul", "noul": 0.98},
    "route": {"type": "choice", "choice": "fast", "confidence": 0.91, "probabilities": {"fast": 0.91, "cheap": 0.09}},
    "score": {"type": "score", "score": 0.8, "confidence": 0.88, "legend": {"0": "poor", "1": "excellent"}}
  },
  "usage": {"input_tokens": 32, "output_tokens": 3, "cost": 0.000001}
}`

func TestOpenRouterDecisionsRequestFixture(t *testing.T) {
	var request struct {
		Model     string                     `json:"model"`
		State     json.RawMessage            `json:"state"`
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := common.Unmarshal([]byte(openRouterDecisionsRequestFixture), &request); err != nil {
		t.Fatalf("request fixture is not valid JSON: %v", err)
	}
	if request.Model != "typesafe/jev-1.13" {
		t.Fatalf("model = %q, want typesafe/jev-1.13", request.Model)
	}
	if len(request.State) == 0 || string(request.State) == "null" {
		t.Fatal("state is required")
	}
	if len(request.Questions) != 3 {
		t.Fatalf("questions count = %d, want 3", len(request.Questions))
	}
	for name, raw := range request.Questions {
		var question struct {
			Type string `json:"type"`
		}
		if err := common.Unmarshal(raw, &question); err != nil {
			t.Fatalf("question %q is not valid JSON: %v", name, err)
		}
		switch question.Type {
		case "noul", "choice", "score":
		default:
			t.Fatalf("question %q has unsupported type %q", name, question.Type)
		}
	}
}

func TestOpenRouterDecisionsResponseFixture(t *testing.T) {
	var response struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal([]byte(openRouterDecisionsResponseFixture), &response); err != nil {
		t.Fatalf("response fixture is not valid JSON: %v", err)
	}
	if response.Model != "typesafe/jev-1.13" {
		t.Fatalf("model = %q, want typesafe/jev-1.13", response.Model)
	}
	if len(response.Answers) != 3 {
		t.Fatalf("answers count = %d, want 3", len(response.Answers))
	}
	if response.Usage.InputTokens <= 0 || response.Usage.OutputTokens < 0 {
		t.Fatalf("unexpected usage: input=%d output=%d", response.Usage.InputTokens, response.Usage.OutputTokens)
	}
	for name, raw := range response.Answers {
		var answer struct {
			Type string `json:"type"`
		}
		if err := common.Unmarshal(raw, &answer); err != nil {
			t.Fatalf("answer %q is not valid JSON: %v", name, err)
		}
		switch answer.Type {
		case "noul", "choice", "score":
		default:
			t.Fatalf("answer %q has unsupported type %q", name, answer.Type)
		}
	}
}
