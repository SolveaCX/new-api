package controller

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type websiteFeaturedModelRequest struct {
	ModelNames []string                          `json:"model_names"`
	Items      []websiteFeaturedModelRequestItem `json:"items"`
}

type websiteFeaturedModelRequestItem struct {
	ModelName               string `json:"model_name"`
	DisplayName             string `json:"display_name"`
	Description             string `json:"description"`
	Tags                    string `json:"tags"`
	BackgroundImageURL      string `json:"background_image_url"`
	BackgroundImage         string `json:"background_image"`
	FallbackBackgroundImage string `json:"fallback_background_image"`
	Video                   string `json:"video"`
}

type websiteFeaturedModelResponse struct {
	ModelName               string `json:"model_name"`
	SortOrder               int    `json:"sort_order"`
	VendorName              string `json:"vendor_name,omitempty"`
	Tags                    string `json:"tags,omitempty"`
	DisplayName             string `json:"display_name,omitempty"`
	Description             string `json:"description,omitempty"`
	BackgroundImageURL      string `json:"background_image_url,omitempty"`
	BackgroundImage         string `json:"background_image,omitempty"`
	FallbackBackgroundImage string `json:"fallback_background_image,omitempty"`
	Video                   string `json:"video,omitempty"`
	Available               bool   `json:"available"`
}

type websiteFeaturedCandidateResponse struct {
	ModelName  string `json:"model_name"`
	VendorName string `json:"vendor_name,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Tags       string `json:"tags,omitempty"`
	Available  bool   `json:"available"`
}

// GetWebsiteFeaturedModels returns current featured rows and the public model
// candidates that can be selected by an administrator.
func GetWebsiteFeaturedModels(c *gin.Context) {
	rows, err := model.ListWebsiteFeaturedModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	publicModels := publicWebsitePricingModels()
	vendorNames := websiteVendorNames(model.GetVendors())
	available := make(map[string]model.Pricing, len(publicModels))
	candidates := make([]websiteFeaturedCandidateResponse, 0, len(publicModels))
	for _, item := range publicModels {
		available[item.ModelName] = item
		candidates = append(candidates, websiteFeaturedCandidateResponse{
			ModelName:  item.ModelName,
			VendorName: vendorNames[item.VendorID],
			Icon:       item.Icon,
			Tags:       item.Tags,
			Available:  true,
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ModelName < candidates[j].ModelName })

	featured := make([]websiteFeaturedModelResponse, 0, len(rows))
	for _, row := range rows {
		item, ok := available[row.ModelName]
		featured = append(featured, websiteFeaturedModelResponse{
			ModelName:               row.ModelName,
			SortOrder:               row.SortOrder,
			VendorName:              vendorNames[item.VendorID],
			Tags:                    firstNonEmptyWebsiteFeatured(row.Tags, item.Tags),
			DisplayName:             row.DisplayName,
			Description:             row.Description,
			BackgroundImageURL:      row.BackgroundImageURL,
			BackgroundImage:         row.BackgroundImage,
			FallbackBackgroundImage: row.FallbackBackgroundImage,
			Video:                   row.Video,
			Available:               ok,
		})
	}

	common.ApiSuccess(c, gin.H{"featured": featured, "candidates": candidates})
}

// UpdateWebsiteFeaturedModels replaces the complete public website featured
// order in one transaction.
func UpdateWebsiteFeaturedModels(c *gin.Context) {
	var request websiteFeaturedModelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	items, err := normalizeWebsiteFeaturedModelItems(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	available := make(map[string]struct{})
	for _, item := range publicWebsitePricingModels() {
		available[item.ModelName] = struct{}{}
	}
	for _, item := range items {
		if _, ok := available[item.ModelName]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model is not available on the public website: " + item.ModelName})
			return
		}
	}

	if err := model.ReplaceWebsiteFeaturedModelsWithConfig(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	InvalidateWebsitePricingCache()
	modelNames := make([]string, len(items))
	for i, item := range items {
		modelNames[i] = item.ModelName
	}
	common.ApiSuccess(c, gin.H{"model_names": modelNames})
}

func normalizeWebsiteFeaturedModelItems(request websiteFeaturedModelRequest) ([]model.WebsiteFeaturedModelInput, error) {
	if request.Items == nil {
		modelNames, err := normalizeWebsiteFeaturedModelNames(request.ModelNames)
		if err != nil {
			return nil, err
		}
		existing := make(map[string]model.WebsiteFeaturedModel)
		if model.DB != nil {
			if rows, loadErr := model.ListWebsiteFeaturedModels(); loadErr == nil {
				for _, row := range rows {
					existing[row.ModelName] = row
				}
			}
		}
		items := make([]model.WebsiteFeaturedModelInput, len(modelNames))
		for i, modelName := range modelNames {
			row := existing[modelName]
			items[i] = model.WebsiteFeaturedModelInput{
				ModelName:               modelName,
				DisplayName:             row.DisplayName,
				Description:             row.Description,
				Tags:                    row.Tags,
				BackgroundImageURL:      row.BackgroundImageURL,
				BackgroundImage:         row.BackgroundImage,
				FallbackBackgroundImage: row.FallbackBackgroundImage,
				Video:                   row.Video,
			}
		}
		return items, nil
	}
	seen := make(map[string]struct{}, len(request.Items))
	items := make([]model.WebsiteFeaturedModelInput, 0, len(request.Items))
	for _, raw := range request.Items {
		modelName := strings.TrimSpace(raw.ModelName)
		if modelName == "" {
			return nil, errors.New("model names must not be empty")
		}
		if _, ok := seen[modelName]; ok {
			return nil, errors.New("model names must be unique")
		}
		seen[modelName] = struct{}{}
		backgroundURL, err := normalizeWebsiteFeaturedURL(raw.BackgroundImageURL)
		if err != nil {
			return nil, errors.New("background image URL must use http(s) or a relative path")
		}
		fallbackURL, err := normalizeWebsiteFeaturedURL(raw.FallbackBackgroundImage)
		if err != nil {
			return nil, errors.New("fallback background image URL must use http(s) or a relative path")
		}
		videoURL, err := normalizeWebsiteFeaturedURL(raw.Video)
		if err != nil {
			return nil, errors.New("video URL must use http(s) or a relative path")
		}
		if len(raw.BackgroundImage) > 12<<20 {
			return nil, errors.New("background image upload must be smaller than 8 MB")
		}
		items = append(items, model.WebsiteFeaturedModelInput{
			ModelName:               modelName,
			DisplayName:             strings.TrimSpace(raw.DisplayName),
			Description:             strings.TrimSpace(raw.Description),
			Tags:                    strings.TrimSpace(raw.Tags),
			BackgroundImageURL:      backgroundURL,
			BackgroundImage:         strings.TrimSpace(raw.BackgroundImage),
			FallbackBackgroundImage: fallbackURL,
			Video:                   videoURL,
		})
	}
	return items, nil
}

func normalizeWebsiteFeaturedURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("invalid URL")
	}
	return value, nil
}

func firstNonEmptyWebsiteFeatured(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeWebsiteFeaturedModelNames(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	modelNames := make([]string, 0, len(raw))
	for _, rawName := range raw {
		modelName := strings.TrimSpace(rawName)
		if modelName == "" {
			return nil, errors.New("model names must not be empty")
		}
		if _, ok := seen[modelName]; ok {
			return nil, errors.New("model names must be unique")
		}
		seen[modelName] = struct{}{}
		modelNames = append(modelNames, modelName)
	}
	return modelNames, nil
}

func getWebsiteFeaturedModelNames() []string {
	if model.DB == nil {
		return nil
	}
	rows, err := model.ListWebsiteFeaturedModels()
	if err != nil {
		common.SysLog("failed to load website featured model order: " + err.Error())
		return nil
	}
	modelNames := make([]string, 0, len(rows))
	for _, row := range rows {
		modelNames = append(modelNames, row.ModelName)
	}
	return modelNames
}

func attachWebsiteFeaturedConfig(pricing []model.Pricing) []model.Pricing {
	if len(pricing) == 0 || model.DB == nil {
		return pricing
	}
	rows, err := model.ListWebsiteFeaturedModels()
	if err != nil {
		common.SysLog("failed to load website featured model config: " + err.Error())
		return pricing
	}
	configs := make(map[string]model.WebsiteFeaturedModel, len(rows))
	for _, row := range rows {
		configs[row.ModelName] = row
	}
	for index := range pricing {
		row, ok := configs[pricing[index].ModelName]
		if !ok {
			continue
		}
		pricing[index].WebsiteFeaturedConfig = &model.WebsiteFeaturedModelConfig{
			DisplayName:             row.DisplayName,
			Description:             row.Description,
			Tags:                    firstNonEmptyWebsiteFeatured(row.Tags, pricing[index].Tags),
			BackgroundImageURL:      row.BackgroundImageURL,
			BackgroundImage:         row.BackgroundImage,
			FallbackBackgroundImage: row.FallbackBackgroundImage,
			Video:                   row.Video,
		}
	}
	return pricing
}

func applyWebsiteFeaturedOrder(pricing []model.Pricing, featuredNames []string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	featuredOrder := make(map[string]int, len(featuredNames))
	for order, modelName := range featuredNames {
		if _, exists := featuredOrder[modelName]; !exists {
			featuredOrder[modelName] = order
		}
	}

	featured := make([]model.Pricing, 0, len(featuredNames))
	remaining := make([]model.Pricing, 0, len(pricing))
	byName := make(map[string]model.Pricing, len(pricing))
	for _, item := range pricing {
		item.WebsiteFeaturedOrder = nil
		byName[item.ModelName] = item
	}
	for _, modelName := range featuredNames {
		item, ok := byName[modelName]
		if !ok {
			continue
		}
		order := featuredOrder[modelName]
		item.WebsiteFeaturedOrder = &order
		featured = append(featured, item)
		delete(byName, modelName)
	}
	for _, item := range pricing {
		if remainingItem, ok := byName[item.ModelName]; ok {
			remaining = append(remaining, remainingItem)
			delete(byName, item.ModelName)
		}
	}
	return append(featured, remaining...)
}

func publicWebsitePricingModels() []model.Pricing {
	if _, ok := ratio_setting.GetGroupRatioCopy()[websitePublicGroup]; !ok {
		return nil
	}
	description := setting.GetUsableGroupDescription(websitePublicGroup)
	if strings.TrimSpace(description) == "" {
		description = websitePublicGroup
	}
	return filterPricingByUsableGroups(model.GetPricing(), map[string]string{websitePublicGroup: description})
}

func websiteVendorNames(vendors []model.PricingVendor) map[int]string {
	result := make(map[int]string, len(vendors))
	for _, vendor := range vendors {
		result[vendor.ID] = vendor.Name
	}
	return result
}
