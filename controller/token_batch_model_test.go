package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestTokenListEndpointsFilterByGroupBackport(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedToken(t, db, 1, "default-key", "default-key-value")
	premium := seedToken(t, db, 1, "premium-key", "premium-key-value")
	if err := db.Model(premium).Update("group", "premium").Error; err != nil {
		t.Fatalf("failed to update token group: %v", err)
	}

	for _, target := range []string{
		"/api/token/?p=1&size=20&group=premium",
		"/api/token/search?p=1&size=20&group=premium",
	} {
		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, target, nil, 1)
		if target == "/api/token/?p=1&size=20&group=premium" {
			GetAllTokens(ctx)
		} else {
			SearchTokens(ctx)
		}

		response := decodeAPIResponse(t, recorder)
		if !response.Success {
			t.Fatalf("expected successful response for %s: %s", target, response.Message)
		}
		var page struct {
			Items []tokenResponseItem `json:"items"`
			Total int                 `json:"total"`
		}
		if err := common.Unmarshal(response.Data, &page); err != nil {
			t.Fatalf("failed to decode token page: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "premium-key" {
			t.Fatalf("unexpected filtered items for %s: %+v", target, page.Items)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=20", nil, 1)
	GetAllTokens(ctx)
	response := decodeAPIResponse(t, recorder)
	var unfilteredPage struct {
		Total int `json:"total"`
	}
	if err := common.Unmarshal(response.Data, &unfilteredPage); err != nil {
		t.Fatalf("failed to decode unfiltered token page: %v", err)
	}
	if unfilteredPage.Total != 2 {
		t.Fatalf("expected account total 2 without group filter, got %d", unfilteredPage.Total)
	}
}

func TestUpdateTokenBatchUpdatesAndDisablesModelLimitsBackport(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "batch-key", "batch-key-value")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/batch", map[string]any{
		"ids":                  []int{token.Id},
		"model_limits_enabled": true,
		"model_limits":         "gpt-4o,gpt-5",
	}, 1)
	UpdateTokenBatch(ctx)
	if response := decodeAPIResponse(t, recorder); !response.Success {
		t.Fatalf("expected batch enable to succeed: %s", response.Message)
	}

	var updated model.Token
	if err := db.First(&updated, token.Id).Error; err != nil {
		t.Fatalf("failed to load updated token: %v", err)
	}
	if !updated.ModelLimitsEnabled || updated.ModelLimits != "gpt-4o,gpt-5" {
		t.Fatalf("unexpected enabled model limits: enabled=%v limits=%q", updated.ModelLimitsEnabled, updated.ModelLimits)
	}

	disableCtx, disableRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/batch", map[string]any{
		"ids":                  []int{token.Id},
		"model_limits_enabled": false,
		"model_limits":         "ignored",
	}, 1)
	UpdateTokenBatch(disableCtx)
	if response := decodeAPIResponse(t, disableRecorder); !response.Success {
		t.Fatalf("expected batch disable to succeed: %s", response.Message)
	}
	if err := db.First(&updated, token.Id).Error; err != nil {
		t.Fatalf("failed to reload updated token: %v", err)
	}
	if updated.ModelLimitsEnabled || updated.ModelLimits != "" {
		t.Fatalf("unexpected disabled model limits: enabled=%v limits=%q", updated.ModelLimitsEnabled, updated.ModelLimits)
	}
}

func TestUpdateTokenBatchRollsBackForForeignTokenBackport(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	owned := seedToken(t, db, 1, "owned-key", "owned-key-value")
	foreign := seedToken(t, db, 2, "foreign-key", "foreign-key-value")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/batch", map[string]any{
		"ids":                  []int{owned.Id, foreign.Id},
		"model_limits_enabled": true,
		"model_limits":         "gpt-5",
	}, 1)
	UpdateTokenBatch(ctx)
	if response := decodeAPIResponse(t, recorder); response.Success {
		t.Fatal("expected a mixed-ownership batch to fail")
	}

	var unchanged model.Token
	if err := db.First(&unchanged, owned.Id).Error; err != nil {
		t.Fatalf("failed to reload owned token: %v", err)
	}
	if unchanged.ModelLimitsEnabled || unchanged.ModelLimits != "" {
		t.Fatalf("owned token changed despite rollback: enabled=%v limits=%q", unchanged.ModelLimitsEnabled, unchanged.ModelLimits)
	}
}
