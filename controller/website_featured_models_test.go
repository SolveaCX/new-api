package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
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
}

func pricingModelNames(rows []model.Pricing) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.ModelName)
	}
	return names
}
