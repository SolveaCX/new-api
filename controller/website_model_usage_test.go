package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newModelUsageRequest(t *testing.T, key string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/website/model-usage?model=seedance-2.5", nil)
	if key != "" {
		c.Request.Header.Set("X-Website-Metrics-Key", key)
	}
	return recorder, c
}

// An unconfigured deployment must not serve usage, and must not advertise that
// the route exists.
func TestGetWebsiteModelUsageDisabledWithoutKey(t *testing.T) {
	t.Setenv(websiteMetricsKeyEnv, "")
	recorder, c := newModelUsageRequest(t, "anything")

	GetWebsiteModelUsage(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when %s is unset, got %d", websiteMetricsKeyEnv, recorder.Code)
	}
}

func TestGetWebsiteModelUsageRejectsWrongKey(t *testing.T) {
	t.Setenv(websiteMetricsKeyEnv, "correct-key")

	for _, presented := range []string{"", "wrong-key", "correct-key-with-suffix", "correct-ke"} {
		recorder, c := newModelUsageRequest(t, presented)
		GetWebsiteModelUsage(c)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for presented key %q, got %d", presented, recorder.Code)
		}
	}
}

// A correct key still requires a model: without one the handler would otherwise
// scan every model's buckets.
func TestGetWebsiteModelUsageRequiresModel(t *testing.T) {
	t.Setenv(websiteMetricsKeyEnv, "correct-key")
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/website/model-usage", nil)
	c.Request.Header.Set("X-Website-Metrics-Key", "correct-key")

	GetWebsiteModelUsage(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without a model, got %d", recorder.Code)
	}
	var payload struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if payload.Success {
		t.Fatal("expected success=false")
	}
}
