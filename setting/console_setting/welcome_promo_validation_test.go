package console_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func marshalWelcomePromoTest(t *testing.T, value any) string {
	t.Helper()
	data, err := common.Marshal(value)
	if err != nil {
		t.Fatalf("marshal welcome promo: %v", err)
	}
	return string(data)
}

func TestValidateWelcomePromo(t *testing.T) {
	valid := []map[string]string{
		{"model_name": "deepseek-v4-pro", "description": "Reasoning", "offer": "55% off"},
		{"model_name": "glm-5.3-flash", "description": "Multimodal", "offer": "55% off"},
		{"model_name": "gpt-5.6-sol", "description": "Frontier", "offer": "55% off"},
	}
	if err := ValidateConsoleSettings(marshalWelcomePromoTest(t, valid), "WelcomePromo"); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	tests := []struct {
		name  string
		value any
	}{
		{"wrong count", valid[:2]},
		{"duplicate models", []map[string]string{valid[0], valid[0], valid[2]}},
		{"missing description", []map[string]string{valid[0], {"model_name": "other", "description": "", "offer": "Offer"}, valid[2]}},
		{"dangerous offer", []map[string]string{valid[0], valid[1], {"model_name": "other", "description": "Safe", "offer": "javascript:alert(1)"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConsoleSettings(marshalWelcomePromoTest(t, tt.value), "WelcomePromo"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
