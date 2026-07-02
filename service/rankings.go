package service

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const (
	rankingCacheTTL         = 5 * time.Minute
	rankingLeaderboardLimit = 20
	rankingHistoryLimit     = 10
	rankingVendorLimit      = 5
	rankingMoverLimit       = 6
	rankingOthersLabel      = "Others"
	rankingUnknownVendor    = "Unknown"

	websiteRankingModelLimit       = 20
	websiteRankingVendorLimit      = 5
	websiteRankingMoverLimit       = 4
	websiteRankingMinimumTokens    = int64(100000)
	websiteRankingMinimumShare     = 0.005
	websiteRankingMinimumRankDelta = 2
	websiteRankingGrowthBucketPct  = 5.0
)

type RankingsResponse struct {
	Models             []RankedModel      `json:"models"`
	Vendors            []RankedVendor     `json:"vendors"`
	TopMovers          []RankingMover     `json:"top_movers"`
	TopDroppers        []RankingMover     `json:"top_droppers"`
	ModelsHistory      ModelHistorySeries `json:"models_history"`
	VendorShareHistory VendorShareSeries  `json:"vendor_share_history"`
}

type WebsiteRankingsResponse struct {
	Period      string                `json:"period"`
	Models      []WebsiteRankedModel  `json:"models"`
	Vendors     []WebsiteRankedVendor `json:"vendors"`
	TopMovers   []WebsiteRankingMover `json:"top_movers"`
	TopDroppers []WebsiteRankingMover `json:"top_droppers"`
}

type WebsiteRankedModel struct {
	Rank       int      `json:"rank"`
	ModelName  string   `json:"model_name"`
	Vendor     string   `json:"vendor"`
	VendorIcon string   `json:"vendor_icon,omitempty"`
	Category   string   `json:"category"`
	Share      float64  `json:"share"`
	GrowthPct  *float64 `json:"growth_pct,omitempty"`
}

type WebsiteRankedVendor struct {
	Rank        int      `json:"rank"`
	Vendor      string   `json:"vendor"`
	VendorIcon  string   `json:"vendor_icon,omitempty"`
	Share       float64  `json:"share"`
	GrowthPct   *float64 `json:"growth_pct,omitempty"`
	ModelsCount int      `json:"models_count"`
	TopModel    string   `json:"top_model,omitempty"`
}

type WebsiteRankingMover struct {
	ModelName   string   `json:"model_name"`
	Vendor      string   `json:"vendor"`
	VendorIcon  string   `json:"vendor_icon,omitempty"`
	RankDelta   int      `json:"rank_delta"`
	CurrentRank int      `json:"current_rank"`
	GrowthPct   *float64 `json:"growth_pct,omitempty"`
}

type RankedModel struct {
	Rank           int     `json:"rank"`
	PreviousRank   *int    `json:"previous_rank,omitempty"`
	ModelName      string  `json:"model_name"`
	Vendor         string  `json:"vendor"`
	VendorIcon     string  `json:"vendor_icon,omitempty"`
	Category       string  `json:"category"`
	TotalTokens    int64   `json:"total_tokens"`
	PreviousTokens int64   `json:"-"`
	Share          float64 `json:"share"`
	GrowthPct      float64 `json:"growth_pct"`
}

type RankedVendor struct {
	Rank           int     `json:"rank"`
	Vendor         string  `json:"vendor"`
	VendorIcon     string  `json:"vendor_icon,omitempty"`
	TotalTokens    int64   `json:"total_tokens"`
	PreviousTokens int64   `json:"-"`
	Share          float64 `json:"share"`
	GrowthPct      float64 `json:"growth_pct"`
	ModelsCount    int     `json:"models_count"`
	TopModel       string  `json:"top_model"`
}

type RankingMover struct {
	ModelName   string  `json:"model_name"`
	Vendor      string  `json:"vendor"`
	VendorIcon  string  `json:"vendor_icon,omitempty"`
	RankDelta   int     `json:"rank_delta"`
	CurrentRank int     `json:"current_rank"`
	GrowthPct   float64 `json:"growth_pct"`
}

type ModelHistoryPoint struct {
	Ts     string `json:"ts"`
	Label  string `json:"label"`
	Model  string `json:"model"`
	Vendor string `json:"vendor"`
	Tokens int64  `json:"tokens"`
}

type ModelHistoryModel struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	Total  int64  `json:"total"`
}

type ModelHistorySeries struct {
	Points  []ModelHistoryPoint `json:"points"`
	Models  []ModelHistoryModel `json:"models"`
	Buckets int                 `json:"buckets"`
}

type VendorSharePoint struct {
	Ts     string  `json:"ts"`
	Label  string  `json:"label"`
	Vendor string  `json:"vendor"`
	Share  float64 `json:"share"`
	Tokens int64   `json:"tokens"`
}

type VendorShareVendor struct {
	Name  string  `json:"name"`
	Total int64   `json:"total"`
	Share float64 `json:"share"`
}

type VendorShareSeries struct {
	Points  []VendorSharePoint  `json:"points"`
	Vendors []VendorShareVendor `json:"vendors"`
	Buckets int                 `json:"buckets"`
}

type rankingPeriodConfig struct {
	id          string
	duration    time.Duration
	bucketSize  int64
	labelLayout string
	hasPrevious bool
}

type rankingCacheItem struct {
	expiresAt time.Time
	data      *RankingsResponse
}

type websiteRankingCacheItem struct {
	expiresAt time.Time
	data      *WebsiteRankingsResponse
}

type rankingInflightCall struct {
	done chan struct{}
	data *RankingsResponse
	err  error
}

type rankingModelMeta struct {
	vendor     string
	vendorIcon string
}

type vendorAggregate struct {
	name           string
	icon           string
	totalTokens    int64
	previousTokens int64
	models         map[string]struct{}
	topModel       string
	topModelTokens int64
}

var (
	rankingCacheMu            sync.Mutex
	rankingCache              = map[string]rankingCacheItem{}
	websiteRankingCache       = map[string]websiteRankingCacheItem{}
	rankingInflight           = map[string]*rankingInflightCall{}
	buildRankingsSnapshotFunc = buildRankingsSnapshot
)

func GetRankingsSnapshot(period string) (*RankingsResponse, error) {
	config, err := rankingConfig(period)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	rankingCacheMu.Lock()
	if item, ok := rankingCache[config.id]; ok && now.Before(item.expiresAt) {
		rankingCacheMu.Unlock()
		return item.data, nil
	}
	if call, ok := rankingInflight[config.id]; ok {
		rankingCacheMu.Unlock()
		<-call.done
		return call.data, call.err
	}
	call := &rankingInflightCall{done: make(chan struct{})}
	rankingInflight[config.id] = call
	rankingCacheMu.Unlock()

	data, err := buildRankingsSnapshotFunc(config, now)
	if err != nil {
		rankingCacheMu.Lock()
		call.err = err
		delete(rankingInflight, config.id)
		close(call.done)
		rankingCacheMu.Unlock()
		return nil, err
	}

	rankingCacheMu.Lock()
	rankingCache[config.id] = rankingCacheItem{
		expiresAt: now.Add(rankingCacheTTL),
		data:      data,
	}
	call.data = data
	delete(rankingInflight, config.id)
	close(call.done)
	rankingCacheMu.Unlock()

	return data, nil
}

func GetWebsiteRankingsSnapshot(period string) (*WebsiteRankingsResponse, error) {
	config, err := websiteRankingConfig(period)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	rankingCacheMu.Lock()
	if item, ok := websiteRankingCache[config.id]; ok && now.Before(item.expiresAt) {
		rankingCacheMu.Unlock()
		return item.data, nil
	}
	rankingCacheMu.Unlock()

	data, err := GetRankingsSnapshot(config.id)
	if err != nil {
		return nil, err
	}
	publicData := buildWebsiteRankingsResponse(config.id, data)

	rankingCacheMu.Lock()
	websiteRankingCache[config.id] = websiteRankingCacheItem{
		expiresAt: now.Add(rankingCacheTTL),
		data:      publicData,
	}
	rankingCacheMu.Unlock()

	return publicData, nil
}

func buildWebsiteRankingsResponse(period string, source *RankingsResponse) *WebsiteRankingsResponse {
	return buildWebsiteRankingsResponseWithPublicModels(period, source, buildWebsitePublicModelSet())
}

func buildWebsiteRankingsResponseWithPublicModels(period string, source *RankingsResponse, publicModels map[string]struct{}) *WebsiteRankingsResponse {
	if source == nil {
		return &WebsiteRankingsResponse{Period: period}
	}

	out := &WebsiteRankingsResponse{
		Period:      period,
		Models:      make([]WebsiteRankedModel, 0, len(source.Models)),
		Vendors:     make([]WebsiteRankedVendor, 0, len(source.Vendors)),
		TopMovers:   make([]WebsiteRankingMover, 0, websiteRankingMoverLimit),
		TopDroppers: make([]WebsiteRankingMover, 0, websiteRankingMoverLimit),
	}

	publicModelTotalTokens := sumPublicRankingModelTokens(source.Models, publicModels)
	publicVendorTokens := publicRankingVendorTokens(source.Models, publicModels)
	publicVendorPreviousTokens := publicRankingVendorPreviousTokens(source.Models, publicModels)
	publicVendorModelCounts := publicRankingVendorModelCounts(source.Models, publicModels)
	publicModelRows := make([]WebsiteRankedModel, 0, len(source.Models))
	for _, item := range source.Models {
		if !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		publicModelRows = append(publicModelRows, WebsiteRankedModel{
			Rank:       len(publicModelRows) + 1,
			ModelName:  item.ModelName,
			Vendor:     item.Vendor,
			VendorIcon: item.VendorIcon,
			Category:   item.Category,
			Share:      coarseWebsiteShare(rankingShare(item.TotalTokens, publicModelTotalTokens)),
			GrowthPct:  websiteRankingGrowth(item.GrowthPct, item.TotalTokens, item.PreviousTokens),
		})
		if len(publicModelRows) >= websiteRankingModelLimit {
			break
		}
	}
	out.Models = publicModelRows

	for _, item := range source.Vendors {
		publicTokens := publicVendorTokens[item.Vendor]
		publicPreviousTokens := publicVendorPreviousTokens[item.Vendor]
		publicVendorShare := rankingShare(publicTokens, publicModelTotalTokens)
		publicVendorGrowth := rankingGrowthPct(publicTokens, publicPreviousTokens)
		if !websiteRankingVendorIsPublic(item, publicTokens, publicVendorShare) {
			continue
		}
		out.Vendors = append(out.Vendors, WebsiteRankedVendor{
			Rank:        len(out.Vendors) + 1,
			Vendor:      item.Vendor,
			VendorIcon:  item.VendorIcon,
			Share:       coarseWebsiteShare(publicVendorShare),
			GrowthPct:   websiteRankingGrowth(publicVendorGrowth, publicTokens, publicPreviousTokens),
			ModelsCount: publicVendorModelCounts[item.Vendor],
			TopModel:    "",
		})
		if len(out.Vendors) >= websiteRankingVendorLimit {
			break
		}
	}

	out.TopMovers, out.TopDroppers = buildWebsiteRankingMovers(source.Models, publicModels)
	return out
}

func buildWebsitePublicModelSet() map[string]struct{} {
	publicModels := make(map[string]struct{})
	metaByModel := buildRankingModelMeta()
	for _, pricing := range FilterPricingByUsableGroups(model.GetPricing(), GetUserUsableGroups("")) {
		meta := metaByModel[pricing.ModelName]
		if meta.vendor == "" || meta.vendor == rankingUnknownVendor {
			continue
		}
		publicModels[pricing.ModelName] = struct{}{}
	}
	return publicModels
}

func sumPublicRankingModelTokens(models []RankedModel, publicModels map[string]struct{}) int64 {
	total := int64(0)
	for _, item := range models {
		if !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		total += item.TotalTokens
	}
	return total
}

func publicRankingVendorTokens(models []RankedModel, publicModels map[string]struct{}) map[string]int64 {
	tokens := make(map[string]int64)
	for _, item := range models {
		if !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		tokens[item.Vendor] += item.TotalTokens
	}
	return tokens
}

func publicRankingVendorPreviousTokens(models []RankedModel, publicModels map[string]struct{}) map[string]int64 {
	tokens := make(map[string]int64)
	for _, item := range models {
		if !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		tokens[item.Vendor] += item.PreviousTokens
	}
	return tokens
}

func publicRankingVendorModelCounts(models []RankedModel, publicModels map[string]struct{}) map[string]int {
	counts := make(map[string]int)
	for _, item := range models {
		if !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		counts[item.Vendor]++
	}
	return counts
}

func websiteRankingConfig(period string) (rankingPeriodConfig, error) {
	switch period {
	case "", "week":
		return rankingConfig("week")
	case "month":
		return rankingConfig("month")
	default:
		return rankingPeriodConfig{}, fmt.Errorf("invalid public ranking period: %s", period)
	}
}

func websiteRankingModelIsPublic(item RankedModel, publicModels map[string]struct{}) bool {
	if _, ok := publicModels[item.ModelName]; !ok {
		return false
	}
	if item.Vendor == "" || item.Vendor == rankingUnknownVendor {
		return false
	}
	return item.TotalTokens >= websiteRankingMinimumTokens && item.Share >= websiteRankingMinimumShare
}

func websiteRankingVendorIsPublic(item RankedVendor, publicTokens int64, publicShare float64) bool {
	if item.Vendor == "" || item.Vendor == rankingUnknownVendor {
		return false
	}
	return publicTokens >= websiteRankingMinimumTokens && publicShare >= websiteRankingMinimumShare
}

func websiteRankingGrowth(growth float64, currentTokens int64, previousTokens int64) *float64 {
	if currentTokens < websiteRankingMinimumTokens || previousTokens < websiteRankingMinimumTokens {
		return nil
	}
	coarse := math.Round(growth/websiteRankingGrowthBucketPct) * websiteRankingGrowthBucketPct
	if coarse > 200 {
		coarse = 200
	}
	if coarse < -200 {
		coarse = -200
	}
	return &coarse
}

func coarseWebsiteShare(share float64) float64 {
	if share <= 0 {
		return 0
	}
	return math.Round(share*100) / 100
}

func buildWebsiteRankingMovers(models []RankedModel, publicModels map[string]struct{}) ([]WebsiteRankingMover, []WebsiteRankingMover) {
	movers := make([]WebsiteRankingMover, 0, websiteRankingMoverLimit)
	droppers := make([]WebsiteRankingMover, 0, websiteRankingMoverLimit)
	for _, item := range models {
		if item.PreviousRank == nil || !websiteRankingModelIsPublic(item, publicModels) {
			continue
		}
		delta := *item.PreviousRank - item.Rank
		if absInt(delta) < websiteRankingMinimumRankDelta {
			continue
		}
		growth := websiteRankingGrowth(item.GrowthPct, item.TotalTokens, item.PreviousTokens)
		if growth == nil {
			continue
		}
		row := WebsiteRankingMover{
			ModelName:   item.ModelName,
			Vendor:      item.Vendor,
			VendorIcon:  item.VendorIcon,
			RankDelta:   delta,
			CurrentRank: item.Rank,
			GrowthPct:   growth,
		}
		if delta > 0 {
			movers = append(movers, row)
		} else {
			droppers = append(droppers, row)
		}
	}
	sort.Slice(movers, func(i, j int) bool {
		if movers[i].RankDelta == movers[j].RankDelta {
			return growthValue(movers[i].GrowthPct) > growthValue(movers[j].GrowthPct)
		}
		return movers[i].RankDelta > movers[j].RankDelta
	})
	sort.Slice(droppers, func(i, j int) bool {
		if droppers[i].RankDelta == droppers[j].RankDelta {
			return growthValue(droppers[i].GrowthPct) < growthValue(droppers[j].GrowthPct)
		}
		return droppers[i].RankDelta < droppers[j].RankDelta
	})
	return limitWebsiteRankingMovers(movers, websiteRankingMoverLimit), limitWebsiteRankingMovers(droppers, websiteRankingMoverLimit)
}

func growthValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func rankingConfig(period string) (rankingPeriodConfig, error) {
	switch period {
	case "", "week":
		return rankingPeriodConfig{id: "week", duration: 7 * 24 * time.Hour, bucketSize: 24 * 3600, labelLayout: "Jan 2", hasPrevious: true}, nil
	case "today":
		return rankingPeriodConfig{id: "today", duration: 24 * time.Hour, bucketSize: 3600, labelLayout: "15:04", hasPrevious: true}, nil
	case "month":
		return rankingPeriodConfig{id: "month", duration: 30 * 24 * time.Hour, bucketSize: 24 * 3600, labelLayout: "Jan 2", hasPrevious: true}, nil
	case "year":
		return rankingPeriodConfig{id: "year", duration: 365 * 24 * time.Hour, bucketSize: 7 * 24 * 3600, labelLayout: "Jan 2", hasPrevious: true}, nil
	case "all":
		return rankingPeriodConfig{id: "all", bucketSize: 30 * 24 * 3600, labelLayout: "Jan 2006"}, nil
	default:
		return rankingPeriodConfig{}, fmt.Errorf("invalid ranking period: %s", period)
	}
}

func buildRankingsSnapshot(config rankingPeriodConfig, now time.Time) (*RankingsResponse, error) {
	startTime, endTime := rankingTimeRange(config, now)
	currentTotals, err := model.GetRankingQuotaTotals(startTime, endTime)
	if err != nil {
		return nil, err
	}
	currentBuckets, err := model.GetRankingQuotaBuckets(startTime, endTime, config.bucketSize)
	if err != nil {
		return nil, err
	}

	var previousTotals []model.RankingQuotaTotal
	if config.hasPrevious {
		previousStart, previousEnd := previousRankingTimeRange(config, startTime)
		previousTotals, err = model.GetRankingQuotaTotals(previousStart, previousEnd)
		if err != nil {
			return nil, err
		}
	}

	meta := buildRankingModelMeta()
	totalTokens := sumRankingTokens(currentTotals)
	previousRankByModel := rankingRankMap(previousTotals)
	previousTokensByModel := rankingTokenMap(previousTotals)

	rankedModels := buildRankedModels(currentTotals, totalTokens, previousRankByModel, previousTokensByModel, meta, config.hasPrevious)
	vendors := buildRankedVendors(currentTotals, previousTotals, totalTokens, meta, config.hasPrevious)
	modelHistory := buildModelHistory(currentBuckets, currentTotals, meta, config)
	vendorHistory := buildVendorShareHistory(currentBuckets, vendors, totalTokens, meta, config)
	movers, droppers := buildRankingMovers(rankedModels)

	return &RankingsResponse{
		Models:             limitRankedModels(rankedModels, rankingLeaderboardLimit),
		Vendors:            vendors,
		TopMovers:          movers,
		TopDroppers:        droppers,
		ModelsHistory:      modelHistory,
		VendorShareHistory: vendorHistory,
	}, nil
}

func rankingTimeRange(config rankingPeriodConfig, now time.Time) (int64, int64) {
	endTime := now.Unix()
	if config.duration <= 0 {
		return 0, endTime
	}
	return now.Add(-config.duration).Unix(), endTime
}

func previousRankingTimeRange(config rankingPeriodConfig, currentStart int64) (int64, int64) {
	previousEnd := currentStart - 1
	previousStart := time.Unix(currentStart, 0).Add(-config.duration).Unix()
	return previousStart, previousEnd
}

func buildRankingModelMeta() map[string]rankingModelMeta {
	vendorByID := make(map[int]model.PricingVendor)
	for _, vendor := range model.GetVendors() {
		vendorByID[vendor.ID] = vendor
	}

	meta := make(map[string]rankingModelMeta)
	for _, pricing := range model.GetPricing() {
		item := rankingModelMeta{vendor: rankingUnknownVendor}
		if vendor, ok := vendorByID[pricing.VendorID]; ok {
			item.vendor = vendor.Name
			item.vendorIcon = vendor.Icon
		} else if pricing.OwnerBy != "" {
			item.vendor = pricing.OwnerBy
		}
		meta[pricing.ModelName] = item
	}
	return meta
}

func modelMeta(modelName string, meta map[string]rankingModelMeta) rankingModelMeta {
	if item, ok := meta[modelName]; ok && item.vendor != "" {
		return item
	}
	return rankingModelMeta{vendor: rankingUnknownVendor}
}

func buildRankedModels(totals []model.RankingQuotaTotal, totalTokens int64, previousRanks map[string]int, previousTokens map[string]int64, meta map[string]rankingModelMeta, showGrowth bool) []RankedModel {
	rows := make([]RankedModel, 0, len(totals))
	for idx, item := range totals {
		modelMeta := modelMeta(item.ModelName, meta)
		var previousRank *int
		if rank, ok := previousRanks[item.ModelName]; ok {
			rankCopy := rank
			previousRank = &rankCopy
		}
		growth := 0.0
		if showGrowth {
			growth = rankingGrowthPct(item.TotalTokens, previousTokens[item.ModelName])
		}
		rows = append(rows, RankedModel{
			Rank:           idx + 1,
			PreviousRank:   previousRank,
			ModelName:      item.ModelName,
			Vendor:         modelMeta.vendor,
			VendorIcon:     modelMeta.vendorIcon,
			Category:       "all",
			TotalTokens:    item.TotalTokens,
			PreviousTokens: previousTokens[item.ModelName],
			Share:          rankingShare(item.TotalTokens, totalTokens),
			GrowthPct:      growth,
		})
	}
	return rows
}

func buildRankedVendors(currentTotals []model.RankingQuotaTotal, previousTotals []model.RankingQuotaTotal, totalTokens int64, meta map[string]rankingModelMeta, showGrowth bool) []RankedVendor {
	aggregates := make(map[string]*vendorAggregate)
	for _, item := range currentTotals {
		modelMeta := modelMeta(item.ModelName, meta)
		agg := ensureVendorAggregate(aggregates, modelMeta)
		agg.totalTokens += item.TotalTokens
		agg.models[item.ModelName] = struct{}{}
		if item.TotalTokens > agg.topModelTokens {
			agg.topModel = item.ModelName
			agg.topModelTokens = item.TotalTokens
		}
	}
	for _, item := range previousTotals {
		modelMeta := modelMeta(item.ModelName, meta)
		agg := ensureVendorAggregate(aggregates, modelMeta)
		agg.previousTokens += item.TotalTokens
	}

	rows := make([]RankedVendor, 0, len(aggregates))
	for _, agg := range aggregates {
		if agg.totalTokens <= 0 {
			continue
		}
		growth := 0.0
		if showGrowth {
			growth = rankingGrowthPct(agg.totalTokens, agg.previousTokens)
		}
		rows = append(rows, RankedVendor{
			Vendor:         agg.name,
			VendorIcon:     agg.icon,
			TotalTokens:    agg.totalTokens,
			PreviousTokens: agg.previousTokens,
			Share:          rankingShare(agg.totalTokens, totalTokens),
			GrowthPct:      growth,
			ModelsCount:    len(agg.models),
			TopModel:       agg.topModel,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalTokens == rows[j].TotalTokens {
			return rows[i].Vendor < rows[j].Vendor
		}
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	for idx := range rows {
		rows[idx].Rank = idx + 1
	}
	return rows
}

func ensureVendorAggregate(aggregates map[string]*vendorAggregate, meta rankingModelMeta) *vendorAggregate {
	name := meta.vendor
	if name == "" {
		name = rankingUnknownVendor
	}
	agg, ok := aggregates[name]
	if !ok {
		agg = &vendorAggregate{
			name:   name,
			icon:   meta.vendorIcon,
			models: make(map[string]struct{}),
		}
		aggregates[name] = agg
	}
	if agg.icon == "" && meta.vendorIcon != "" {
		agg.icon = meta.vendorIcon
	}
	return agg
}

func buildModelHistory(buckets []model.RankingQuotaBucket, totals []model.RankingQuotaTotal, meta map[string]rankingModelMeta, config rankingPeriodConfig) ModelHistorySeries {
	topModels := make(map[string]struct{})
	models := make([]ModelHistoryModel, 0, minInt(len(totals), rankingHistoryLimit)+1)
	otherTotal := int64(0)
	for idx, item := range totals {
		if idx < rankingHistoryLimit {
			topModels[item.ModelName] = struct{}{}
			modelMeta := modelMeta(item.ModelName, meta)
			models = append(models, ModelHistoryModel{Name: item.ModelName, Vendor: modelMeta.vendor, Total: item.TotalTokens})
			continue
		}
		otherTotal += item.TotalTokens
	}
	if otherTotal > 0 {
		models = append(models, ModelHistoryModel{Name: rankingOthersLabel, Vendor: "Various", Total: otherTotal})
	}

	bucketSet := make(map[int64]struct{})
	tokensByBucketAndModel := make(map[int64]map[string]int64)
	for _, item := range buckets {
		modelName := item.ModelName
		if _, ok := topModels[modelName]; !ok {
			modelName = rankingOthersLabel
		}
		bucketSet[item.Bucket] = struct{}{}
		if _, ok := tokensByBucketAndModel[item.Bucket]; !ok {
			tokensByBucketAndModel[item.Bucket] = make(map[string]int64)
		}
		tokensByBucketAndModel[item.Bucket][modelName] += item.Tokens
	}

	sortedBuckets := sortedRankingBuckets(bucketSet)
	points := make([]ModelHistoryPoint, 0, len(sortedBuckets)*len(models))
	for _, bucket := range sortedBuckets {
		for _, historyModel := range models {
			tokens := tokensByBucketAndModel[bucket][historyModel.Name]
			if tokens <= 0 {
				continue
			}
			points = append(points, ModelHistoryPoint{
				Ts:     rankingBucketTs(bucket),
				Label:  rankingBucketLabel(bucket, config),
				Model:  historyModel.Name,
				Vendor: historyModel.Vendor,
				Tokens: tokens,
			})
		}
	}

	return ModelHistorySeries{
		Points:  points,
		Models:  models,
		Buckets: len(sortedBuckets),
	}
}

func buildVendorShareHistory(buckets []model.RankingQuotaBucket, vendors []RankedVendor, totalTokens int64, meta map[string]rankingModelMeta, config rankingPeriodConfig) VendorShareSeries {
	topVendors := make(map[string]struct{})
	vendorRows := make([]VendorShareVendor, 0, minInt(len(vendors), rankingVendorLimit)+1)
	otherTotal := int64(0)
	for idx, vendor := range vendors {
		if idx < rankingVendorLimit {
			topVendors[vendor.Vendor] = struct{}{}
			vendorRows = append(vendorRows, VendorShareVendor{Name: vendor.Vendor, Total: vendor.TotalTokens, Share: vendor.Share})
			continue
		}
		otherTotal += vendor.TotalTokens
	}
	if otherTotal > 0 {
		vendorRows = append(vendorRows, VendorShareVendor{Name: rankingOthersLabel, Total: otherTotal, Share: rankingShare(otherTotal, totalTokens)})
	}

	bucketSet := make(map[int64]struct{})
	tokensByBucketAndVendor := make(map[int64]map[string]int64)
	totalsByBucket := make(map[int64]int64)
	for _, item := range buckets {
		modelMeta := modelMeta(item.ModelName, meta)
		vendorName := modelMeta.vendor
		if _, ok := topVendors[vendorName]; !ok {
			vendorName = rankingOthersLabel
		}
		bucketSet[item.Bucket] = struct{}{}
		if _, ok := tokensByBucketAndVendor[item.Bucket]; !ok {
			tokensByBucketAndVendor[item.Bucket] = make(map[string]int64)
		}
		tokensByBucketAndVendor[item.Bucket][vendorName] += item.Tokens
		totalsByBucket[item.Bucket] += item.Tokens
	}

	sortedBuckets := sortedRankingBuckets(bucketSet)
	points := make([]VendorSharePoint, 0, len(sortedBuckets)*len(vendorRows))
	for _, bucket := range sortedBuckets {
		for _, vendor := range vendorRows {
			tokens := tokensByBucketAndVendor[bucket][vendor.Name]
			if tokens <= 0 {
				continue
			}
			points = append(points, VendorSharePoint{
				Ts:     rankingBucketTs(bucket),
				Label:  rankingBucketLabel(bucket, config),
				Vendor: vendor.Name,
				Share:  rankingShare(tokens, totalsByBucket[bucket]),
				Tokens: tokens,
			})
		}
	}

	return VendorShareSeries{
		Points:  points,
		Vendors: vendorRows,
		Buckets: len(sortedBuckets),
	}
}

func buildRankingMovers(models []RankedModel) ([]RankingMover, []RankingMover) {
	movers := make([]RankingMover, 0)
	droppers := make([]RankingMover, 0)
	for _, item := range models {
		if item.PreviousRank == nil {
			continue
		}
		delta := *item.PreviousRank - item.Rank
		if delta == 0 {
			continue
		}
		row := RankingMover{
			ModelName:   item.ModelName,
			Vendor:      item.Vendor,
			VendorIcon:  item.VendorIcon,
			RankDelta:   delta,
			CurrentRank: item.Rank,
			GrowthPct:   item.GrowthPct,
		}
		if delta > 0 {
			movers = append(movers, row)
		} else {
			droppers = append(droppers, row)
		}
	}
	sort.Slice(movers, func(i, j int) bool {
		if movers[i].RankDelta == movers[j].RankDelta {
			return movers[i].GrowthPct > movers[j].GrowthPct
		}
		return movers[i].RankDelta > movers[j].RankDelta
	})
	sort.Slice(droppers, func(i, j int) bool {
		if droppers[i].RankDelta == droppers[j].RankDelta {
			return droppers[i].GrowthPct < droppers[j].GrowthPct
		}
		return droppers[i].RankDelta < droppers[j].RankDelta
	})
	return limitRankingMovers(movers, rankingMoverLimit), limitRankingMovers(droppers, rankingMoverLimit)
}

func sortedRankingBuckets(bucketSet map[int64]struct{}) []int64 {
	buckets := make([]int64, 0, len(bucketSet))
	for bucket := range bucketSet {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i] < buckets[j]
	})
	return buckets
}

func rankingBucketTs(bucket int64) string {
	return time.Unix(bucket, 0).UTC().Format(time.RFC3339)
}

func rankingBucketLabel(bucket int64, config rankingPeriodConfig) string {
	return time.Unix(bucket, 0).Format(config.labelLayout)
}

func rankingRankMap(totals []model.RankingQuotaTotal) map[string]int {
	ranks := make(map[string]int, len(totals))
	for idx, item := range totals {
		ranks[item.ModelName] = idx + 1
	}
	return ranks
}

func rankingTokenMap(totals []model.RankingQuotaTotal) map[string]int64 {
	tokens := make(map[string]int64, len(totals))
	for _, item := range totals {
		tokens[item.ModelName] = item.TotalTokens
	}
	return tokens
}

func sumRankingTokens(totals []model.RankingQuotaTotal) int64 {
	total := int64(0)
	for _, item := range totals {
		total += item.TotalTokens
	}
	return total
}

func rankingShare(value int64, total int64) float64 {
	if total <= 0 || value <= 0 {
		return 0
	}
	return roundRankingFloat(float64(value) / float64(total))
}

func rankingGrowthPct(current int64, previous int64) float64 {
	if previous <= 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return roundRankingFloat((float64(current-previous) / float64(previous)) * 100)
}

func roundRankingFloat(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func limitRankedModels(rows []RankedModel, limit int) []RankedModel {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func limitRankingMovers(rows []RankingMover, limit int) []RankingMover {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func limitWebsiteRankingMovers(rows []WebsiteRankingMover, limit int) []WebsiteRankingMover {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
