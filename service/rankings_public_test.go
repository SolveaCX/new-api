package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildWebsiteRankingsResponseHidesAbsoluteUsage(t *testing.T) {
	previousRank := 3
	source := &RankingsResponse{
		Models: []RankedModel{
			{
				Rank:           1,
				PreviousRank:   &previousRank,
				ModelName:      "gpt-4.1-mini",
				Vendor:         "OpenAI",
				VendorIcon:     "OpenAI",
				TotalTokens:    123456789,
				PreviousTokens: 100000000,
				Share:          0.42,
				GrowthPct:      12.5,
			},
		},
		Vendors: []RankedVendor{
			{
				Rank:           1,
				Vendor:         "OpenAI",
				VendorIcon:     "OpenAI",
				TotalTokens:    987654321,
				PreviousTokens: 900000000,
				Share:          0.42,
				GrowthPct:      9.5,
				ModelsCount:    4,
				TopModel:       "gpt-4.1-mini",
			},
		},
		TopMovers: []RankingMover{
			{
				ModelName:   "gpt-4.1-mini",
				Vendor:      "OpenAI",
				RankDelta:   2,
				CurrentRank: 1,
				GrowthPct:   12.5,
			},
		},
	}

	public := buildWebsiteRankingsResponseWithPublicModels("month", source, map[string]struct{}{"gpt-4.1-mini": {}})
	body, err := common.Marshal(public)
	require.NoError(t, err)

	require.Equal(t, "month", public.Period)
	require.Equal(t, "gpt-4.1-mini", public.Models[0].ModelName)
	require.Equal(t, 1.0, public.Models[0].Share)
	require.NotContains(t, string(body), "total_tokens")
	require.NotContains(t, string(body), "123456789")
	require.NotContains(t, string(body), "987654321")
	require.False(t, strings.Contains(string(body), "tokens"), "public rankings JSON should not expose absolute usage fields: %s", string(body))
}

func TestBuildWebsiteRankingsResponseSuppressesLowVolumeSignals(t *testing.T) {
	previousRank := 8
	source := &RankingsResponse{
		Models: []RankedModel{
			{
				Rank:           1,
				PreviousRank:   &previousRank,
				ModelName:      "trusted-model",
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens,
				PreviousTokens: 1,
				Share:          0.51,
				GrowthPct:      100,
			},
			{
				Rank:        2,
				ModelName:   "tiny-model",
				Vendor:      "Tiny",
				TotalTokens: websiteRankingMinimumTokens - 1,
				Share:       0.49,
				GrowthPct:   100,
			},
		},
		Vendors: []RankedVendor{
			{
				Rank:           1,
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens,
				PreviousTokens: 1,
				Share:          0.51,
				GrowthPct:      100,
				ModelsCount:    1,
				TopModel:       "trusted-model",
			},
		},
	}

	public := buildWebsiteRankingsResponseWithPublicModels("week", source, map[string]struct{}{"trusted-model": {}})
	body, err := common.Marshal(public)
	require.NoError(t, err)

	require.Len(t, public.Models, 1)
	require.Equal(t, "trusted-model", public.Models[0].ModelName)
	require.Nil(t, public.Models[0].GrowthPct)
	require.Empty(t, public.TopMovers)
	require.Empty(t, public.TopDroppers)
	require.NotContains(t, string(body), "tiny-model")
	require.NotContains(t, string(body), "top_model")
}

func TestBuildWebsiteRankingsResponseExcludesUnknownModels(t *testing.T) {
	source := &RankingsResponse{
		Models: []RankedModel{
			{
				Rank:           1,
				ModelName:      "internal/customer-only-model",
				Vendor:         rankingUnknownVendor,
				TotalTokens:    websiteRankingMinimumTokens * 100,
				PreviousTokens: websiteRankingMinimumTokens * 100,
				Share:          0.9,
			},
			{
				Rank:           2,
				ModelName:      "public-model",
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens * 10,
				PreviousTokens: websiteRankingMinimumTokens * 10,
				Share:          0.1,
			},
		},
		Vendors: []RankedVendor{
			{
				Rank:           1,
				Vendor:         rankingUnknownVendor,
				TotalTokens:    websiteRankingMinimumTokens * 100,
				PreviousTokens: websiteRankingMinimumTokens * 100,
				Share:          0.9,
				ModelsCount:    1,
			},
			{
				Rank:           2,
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens * 10,
				PreviousTokens: websiteRankingMinimumTokens * 10,
				Share:          0.1,
				ModelsCount:    1,
			},
		},
	}

	public := buildWebsiteRankingsResponseWithPublicModels("week", source, map[string]struct{}{"public-model": {}})
	body, err := common.Marshal(public)
	require.NoError(t, err)

	require.Len(t, public.Models, 1)
	require.Equal(t, "public-model", public.Models[0].ModelName)
	require.Equal(t, 1.0, public.Models[0].Share)
	require.Len(t, public.Vendors, 1)
	require.Equal(t, "OpenAI", public.Vendors[0].Vendor)
	require.Equal(t, 1.0, public.Vendors[0].Share)
	require.NotContains(t, string(body), "internal/customer-only-model")
	require.NotContains(t, string(body), rankingUnknownVendor)
}

func TestFilterPricingByUsableGroupsExcludesPrivateOnlyModels(t *testing.T) {
	filtered := FilterPricingByUsableGroups([]model.Pricing{
		{ModelName: "public-model", EnableGroup: []string{"default"}},
		{ModelName: "private-model", EnableGroup: []string{"internal"}},
		{ModelName: "all-model", EnableGroup: []string{"all"}},
	}, map[string]string{"default": "Default"})

	require.Equal(t, []model.Pricing{
		{ModelName: "public-model", EnableGroup: []string{"default"}},
		{ModelName: "all-model", EnableGroup: []string{"default"}},
	}, filtered)
}

func TestBuildWebsiteRankingsResponseComputesVendorSignalsFromPublicModelsOnly(t *testing.T) {
	source := &RankingsResponse{
		Models: []RankedModel{
			{
				Rank:           1,
				ModelName:      "hidden-internal-model",
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens * 100,
				PreviousTokens: websiteRankingMinimumTokens * 100,
				Share:          0.9,
				GrowthPct:      0,
			},
			{
				Rank:           2,
				ModelName:      "public-openai-model",
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens,
				PreviousTokens: 1,
				Share:          0.01,
				GrowthPct:      100,
			},
			{
				Rank:           3,
				ModelName:      "public-google-model",
				Vendor:         "Google",
				TotalTokens:    websiteRankingMinimumTokens,
				PreviousTokens: websiteRankingMinimumTokens,
				Share:          0.01,
				GrowthPct:      0,
			},
		},
		Vendors: []RankedVendor{
			{
				Rank:           1,
				Vendor:         "OpenAI",
				TotalTokens:    websiteRankingMinimumTokens*101 + 1,
				PreviousTokens: websiteRankingMinimumTokens * 100,
				Share:          0.91,
				GrowthPct:      1,
				ModelsCount:    2,
				TopModel:       "hidden-internal-model",
			},
			{
				Rank:           2,
				Vendor:         "Google",
				TotalTokens:    websiteRankingMinimumTokens,
				PreviousTokens: websiteRankingMinimumTokens,
				Share:          0.01,
				GrowthPct:      0,
				ModelsCount:    1,
				TopModel:       "public-google-model",
			},
		},
	}

	public := buildWebsiteRankingsResponseWithPublicModels("week", source, map[string]struct{}{
		"public-openai-model": {},
		"public-google-model": {},
	})

	require.Len(t, public.Vendors, 2)
	require.Equal(t, "OpenAI", public.Vendors[0].Vendor)
	require.Equal(t, 0.5, public.Vendors[0].Share)
	require.Equal(t, 1, public.Vendors[0].ModelsCount)
	require.Nil(t, public.Vendors[0].GrowthPct)
	require.Empty(t, public.Vendors[0].TopModel)
}

func TestWebsiteRankingConfigOnlyAllowsPublicPeriods(t *testing.T) {
	week, err := websiteRankingConfig("")
	require.NoError(t, err)
	require.Equal(t, "week", week.id)

	month, err := websiteRankingConfig("month")
	require.NoError(t, err)
	require.Equal(t, "month", month.id)

	_, err = websiteRankingConfig("all")
	require.Error(t, err)
}
