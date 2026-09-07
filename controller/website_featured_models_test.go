package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyWebsiteFeaturedOrderKeepsMultipleFeaturedModelsInConfiguredOrder(t *testing.T) {
	rows := []model.Pricing{
		{ModelName: "fallback"},
		{ModelName: "claude-opus-4.7"},
		{ModelName: "gpt-5.5"},
	}

	ordered := applyWebsiteFeaturedOrder(rows, []string{"gpt-5.5", "claude-opus-4.7"})
	require.Equal(t, []string{"gpt-5.5", "claude-opus-4.7", "fallback"}, pricingModelNames(ordered))
	require.NotNil(t, ordered[0].WebsiteFeaturedOrder)
	require.Equal(t, 0, *ordered[0].WebsiteFeaturedOrder)
	require.Equal(t, 1, *ordered[1].WebsiteFeaturedOrder)
	require.Nil(t, ordered[2].WebsiteFeaturedOrder)
}

func TestNormalizeWebsiteFeaturedModelNamesRejectsDuplicatesAndEmptyNames(t *testing.T) {
	_, err := normalizeWebsiteFeaturedModelNames([]string{"gpt-5.5", " gpt-5.5"})
	require.Error(t, err)

	_, err = normalizeWebsiteFeaturedModelNames([]string{"gpt-5.5", "   "})
	require.Error(t, err)
}

func TestNormalizeWebsiteFeaturedModelItemsTrimsPresentationAndValidatesImages(t *testing.T) {
	items, err := normalizeWebsiteFeaturedModelItems(websiteFeaturedModelRequest{
		Items: []websiteFeaturedModelRequestItem{{
			ModelName:               " gpt-5.5 ",
			DisplayName:             " Launch ",
			Description:             " Copy ",
			Tags:                    " Coding, Agents ",
			BackgroundImageURL:      " https://cdn.example/banner.png ",
			FallbackBackgroundImage: " /assets/fallback.png ",
		}},
	})
	require.NoError(t, err)
	require.Equal(t, model.WebsiteFeaturedModelInput{
		ModelName:               "gpt-5.5",
		DisplayName:             "Launch",
		Description:             "Copy",
		Tags:                    "Coding, Agents",
		BackgroundImageURL:      "https://cdn.example/banner.png",
		FallbackBackgroundImage: "/assets/fallback.png",
	}, items[0])

	_, err = normalizeWebsiteFeaturedModelItems(websiteFeaturedModelRequest{
		Items: []websiteFeaturedModelRequestItem{{ModelName: "gpt-5.5", BackgroundImageURL: "javascript:alert(1)"}},
	})
	require.Error(t, err)

	_, err = normalizeWebsiteFeaturedModelItems(websiteFeaturedModelRequest{
		Items: []websiteFeaturedModelRequestItem{{
			ModelName:       "gpt-5.5",
			BackgroundImage: strings.Repeat("x", maxLegacyWebsiteFeaturedInlineImageBytes+1),
		}},
	})
	require.ErrorIs(t, err, errWebsiteFeaturedInlineImageTooLarge)
}

func TestUpdateWebsiteFeaturedModelsReturnsExplicitBadRequestForLargeInlineImage(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"items":[{"model_name":"gpt-5.5","background_image":"` +
		strings.Repeat("x", maxLegacyWebsiteFeaturedInlineImageBytes+1) + `"}]}`
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/models/website-featured", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Accept-Language", "zh-CN")

	UpdateWebsiteFeaturedModels(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, "内嵌背景图过大，请改为上传图片文件", response.Message)
}

func pricingModelNames(rows []model.Pricing) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.ModelName)
	}
	return names
}
