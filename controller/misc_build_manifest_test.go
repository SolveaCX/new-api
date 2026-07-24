package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func TestGetStatusIncludesEmbeddedBuildManifestWithoutOCIDigest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			BuildManifest common.BuildManifestStatus `json:"build_manifest"`
		} `json:"data"`
	}
	body := recorder.Body.Bytes()
	if err := common.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	want := common.CurrentBuildManifest()
	if response.Data.BuildManifest.ManifestHashPayload != want.ManifestHashPayload || response.Data.BuildManifest.ManifestSHA256 != want.ManifestSHA256 {
		t.Fatal("status manifest does not match the embedded manifest")
	}
	if response.Data.BuildManifest.BuildProvenanceID != want.BuildProvenanceID {
		t.Fatal("status manifest does not expose the embedded provenance ID")
	}
	var raw map[string]any
	if err := common.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	data := raw["data"].(map[string]any)
	manifest := data["build_manifest"].(map[string]any)
	if _, exists := manifest["oci_digest"]; exists {
		t.Fatal("status must never embed an OCI digest")
	}
}
