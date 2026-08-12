package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPromptLibraryImportTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ability{}, &model.Model{}, &model.PromptLibraryItem{}))
	model.DB = db
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-image-2",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	t.Setenv(middleware.PromptLibraryImportTokenEnv, "secret")
	r := gin.New()
	g := r.Group("/api/prompt-library")
	g.Use(middleware.PromptLibraryImportAuth())
	g.POST("/import", ImportPromptLibrary)
	r.GET("/api/website/prompt-library", GetWebsitePromptLibrary)
	return r
}

func TestImportPromptLibraryPersistsOnlyCompleteAllowedItems(t *testing.T) {
	r := setupPromptLibraryImportTest(t)
	body := []byte(`{
		"items": [
			{
				"slug": "valid-image",
				"category": "image",
				"model": "gpt-image-2",
				"prompt": "Create a product image",
				"title": {"en": "Valid image"},
				"summary": {"en": "A complete imported prompt"},
				"tags": ["product"],
				"output": {"ratio": "1:1"},
				"artifact": {"kind": "image", "url": "https://example.com/output.png", "alt": "output"},
				"source": {"label": "GitHub", "platform": "GitHub", "url": "https://github.com/flatkey-ai/example", "captured_at": "2026-08-05"}
			},
			{
				"slug": "unknown-model",
				"category": "image",
				"model": "seedance-2.0",
				"prompt": "Create a product image",
				"title": {"en": "Unknown model"},
				"artifact": {"kind": "image", "url": "https://example.com/output.png"},
				"source": {"label": "X", "platform": "Social", "url": "https://x.com/example/status/1"}
			},
			{
				"slug": "missing-artifact",
				"category": "image",
				"model": "gpt-image-2",
				"prompt": "Create a product image",
				"title": {"en": "Missing artifact"},
				"source": {"label": "GitHub", "platform": "GitHub", "url": "https://github.com/flatkey-ai/example"}
			}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/prompt-library/import", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Imported int `json:"imported"`
			Rejected int `json:"rejected"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 1, response.Data.Imported)
	require.Equal(t, 2, response.Data.Rejected)

	var count int64
	require.NoError(t, model.DB.Model(&model.PromptLibraryItem{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var item model.PromptLibraryItem
	require.NoError(t, model.DB.First(&item, "slug = ?", "valid-image").Error)
	require.Equal(t, "gpt-image-2", item.Model)
	require.Equal(t, "GitHub", item.SourcePlatform)
}

func TestImportPromptLibraryRequiresBearerToken(t *testing.T) {
	r := setupPromptLibraryImportTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-library/import", bytes.NewReader([]byte(`{"items":[]}`)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestImportPromptLibraryClosedWhenTokenUnset(t *testing.T) {
	r := setupPromptLibraryImportTest(t)
	os.Unsetenv(middleware.PromptLibraryImportTokenEnv)
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-library/import", bytes.NewReader([]byte(`{"items":[]}`)))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestGetWebsitePromptLibraryReturnsPublicItems(t *testing.T) {
	r := setupPromptLibraryImportTest(t)
	item := model.PromptLibraryItem{
		Slug:           "valid-image",
		Category:       "image",
		Model:          "gpt-image-2",
		Prompt:         "Create a product image",
		TitleJSON:      `{"en":"Valid image","zh":"有效图片"}`,
		SummaryJSON:    `{"en":"A complete imported prompt","zh":"一条完整的导入提示词"}`,
		TagsJSON:       `["product","image"]`,
		OutputJSON:     `{"ratio":"1:1","label":{"en":"Generated image"}}`,
		ArtifactJSON:   `{"kind":"image","url":"/assets/prompts/example.jpg","alt":"output"}`,
		SourceJSON:     `{"label":"DiffusionDB","platform":"Hugging Face","url":"https://huggingface.co/datasets/example","captured_at":"2026-08-11"}`,
		SourcePlatform: "Hugging Face",
		SourceURL:      "https://huggingface.co/datasets/example",
		CapturedAt:     "2026-08-11",
		CreatedTime:    1786446000,
		UpdatedTime:    1786446000,
	}
	require.NoError(t, model.DB.Create(&item).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/website/prompt-library?limit=10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				Slug   string `json:"slug"`
				Source struct {
					CapturedAt string `json:"capturedAt"`
					Platform   string `json:"platform"`
				} `json:"source"`
				Artifact struct {
					Kind string `json:"kind"`
					URL  string `json:"url"`
				} `json:"artifact"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, "valid-image", response.Data.Items[0].Slug)
	require.Equal(t, "Hugging Face", response.Data.Items[0].Source.Platform)
	require.Equal(t, "2026-08-11", response.Data.Items[0].Source.CapturedAt)
	require.Equal(t, "image", response.Data.Items[0].Artifact.Kind)
	require.Equal(t, "/assets/prompts/example.jpg", response.Data.Items[0].Artifact.URL)
}
