package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const promptLibraryImportMaxItems = 100
const promptLibraryListDefaultLimit = 600
const promptLibraryListMaxLimit = 1000

type promptLibraryImportRequest struct {
	Items []promptLibraryImportItem `json:"items"`
}

type promptLibraryImportItem struct {
	Artifact map[string]any    `json:"artifact"`
	Category string            `json:"category"`
	Model    string            `json:"model"`
	Output   map[string]any    `json:"output"`
	Prompt   string            `json:"prompt"`
	Slug     string            `json:"slug"`
	Source   promptSourceInput `json:"source"`
	Summary  map[string]string `json:"summary"`
	Tags     []string          `json:"tags"`
	Title    map[string]string `json:"title"`
}

type promptSourceInput struct {
	Label      string `json:"label"`
	Platform   string `json:"platform"`
	URL        string `json:"url"`
	CapturedAt string `json:"captured_at"`
}

type promptLibraryImportResult struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type promptLibraryPublicSource struct {
	CapturedAt string `json:"capturedAt"`
	Label      string `json:"label"`
	Platform   string `json:"platform"`
	URL        string `json:"url"`
}

type promptLibraryStoredSource struct {
	CapturedAt      string `json:"capturedAt"`
	CapturedAtSnake string `json:"captured_at"`
	Label           string `json:"label"`
	Platform        string `json:"platform"`
	URL             string `json:"url"`
}

type promptLibraryPublicItem struct {
	Artifact  map[string]any            `json:"artifact"`
	Category  string                    `json:"category"`
	Model     string                    `json:"model"`
	Output    map[string]any            `json:"output"`
	Prompt    string                    `json:"prompt"`
	Slug      string                    `json:"slug"`
	Source    promptLibraryPublicSource `json:"source"`
	Summary   map[string]string         `json:"summary"`
	Tags      []string                  `json:"tags"`
	Title     map[string]string         `json:"title"`
	UpdatedAt string                    `json:"updatedAt"`
}

func GetWebsitePromptLibrary(c *gin.Context) {
	category := strings.TrimSpace(c.Query("category"))
	if category != "" && !model.IsPromptLibraryCategoryAllowed(category) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "category is invalid"})
		return
	}
	query := model.PromptLibraryListQuery{
		Category: category,
		Limit:    promptLibraryIntQuery(c, "limit", promptLibraryListDefaultLimit, promptLibraryListMaxLimit),
		Offset:   promptLibraryIntQuery(c, "offset", 0, 0),
		Search:   strings.TrimSpace(c.Query("q")),
	}
	total, err := model.CountPromptLibraryItems(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := model.ListPromptLibraryItems(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	publicItems := make([]promptLibraryPublicItem, 0, len(items))
	for _, item := range items {
		publicItem, err := promptLibraryPublicItemFromModel(item)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		publicItems = append(publicItems, publicItem)
	}
	common.ApiSuccess(c, gin.H{
		"items":  publicItems,
		"limit":  query.Limit,
		"offset": query.Offset,
		"total":  total,
	})
}

func ImportPromptLibrary(c *gin.Context) {
	var request promptLibraryImportRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid JSON body"})
		return
	}
	if len(request.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "items is required"})
		return
	}
	if len(request.Items) > promptLibraryImportMaxItems {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "too many items"})
		return
	}

	items := make([]model.PromptLibraryItem, 0, len(request.Items))
	results := make([]promptLibraryImportResult, 0, len(request.Items))
	for _, input := range request.Items {
		item, err := normalizePromptLibraryImportItem(input)
		if err != nil {
			results = append(results, promptLibraryImportResult{Slug: strings.TrimSpace(input.Slug), Status: "rejected", Reason: err.Error()})
			continue
		}
		allowed, err := model.IsPromptLibraryModelAllowed(item.Model)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if !allowed {
			results = append(results, promptLibraryImportResult{Slug: item.Slug, Status: "rejected", Reason: "model is not available in Flatkey"})
			continue
		}
		items = append(items, item)
		results = append(results, promptLibraryImportResult{Slug: item.Slug, Status: "imported"})
	}

	if err := model.UpsertPromptLibraryItems(items); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"imported": len(items),
		"rejected": len(request.Items) - len(items),
		"items":    results,
	})
}

func normalizePromptLibraryImportItem(input promptLibraryImportItem) (model.PromptLibraryItem, error) {
	slug := strings.TrimSpace(input.Slug)
	category := strings.TrimSpace(input.Category)
	modelName := strings.TrimSpace(input.Model)
	prompt := strings.TrimSpace(input.Prompt)
	if slug == "" {
		return model.PromptLibraryItem{}, errors.New("slug is required")
	}
	if !model.IsPromptLibraryCategoryAllowed(category) {
		return model.PromptLibraryItem{}, errors.New("category is invalid")
	}
	if modelName == "" {
		return model.PromptLibraryItem{}, errors.New("model is required")
	}
	if prompt == "" {
		return model.PromptLibraryItem{}, errors.New("prompt is required")
	}
	if err := validatePromptArtifact(input.Artifact); err != nil {
		return model.PromptLibraryItem{}, err
	}
	sourcePlatform := strings.TrimSpace(input.Source.Platform)
	sourceURL := strings.TrimSpace(input.Source.URL)
	if sourcePlatform == "" {
		return model.PromptLibraryItem{}, errors.New("source.platform is required")
	}
	if sourceURL == "" {
		return model.PromptLibraryItem{}, errors.New("source.url is required")
	}
	titleJSON, err := marshalPromptLibraryField(input.Title)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}
	artifactJSON, err := marshalPromptLibraryField(input.Artifact)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}
	sourceJSON, err := marshalPromptLibraryField(input.Source)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}
	summaryJSON, err := marshalPromptLibraryField(input.Summary)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}
	tagsJSON, err := marshalPromptLibraryField(input.Tags)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}
	outputJSON, err := marshalPromptLibraryField(input.Output)
	if err != nil {
		return model.PromptLibraryItem{}, err
	}

	return model.PromptLibraryItem{
		Slug:           slug,
		Category:       category,
		Model:          modelName,
		Prompt:         prompt,
		TitleJSON:      titleJSON,
		SummaryJSON:    summaryJSON,
		TagsJSON:       tagsJSON,
		OutputJSON:     outputJSON,
		ArtifactJSON:   artifactJSON,
		SourceJSON:     sourceJSON,
		SourcePlatform: sourcePlatform,
		SourceURL:      sourceURL,
		CapturedAt:     strings.TrimSpace(input.Source.CapturedAt),
	}, nil
}

func validatePromptArtifact(artifact map[string]any) error {
	kindValue, ok := artifact["kind"].(string)
	if !ok || strings.TrimSpace(kindValue) == "" {
		return errors.New("artifact.kind is required")
	}
	switch strings.TrimSpace(kindValue) {
	case "image":
		if strings.TrimSpace(stringField(artifact, "url")) == "" {
			return errors.New("artifact.url is required")
		}
	case "video":
		if strings.TrimSpace(stringField(artifact, "url")) == "" {
			return errors.New("artifact.url is required")
		}
	case "text":
		if strings.TrimSpace(stringField(artifact, "body")) == "" {
			return errors.New("artifact.body is required")
		}
	case "code":
		if strings.TrimSpace(stringField(artifact, "code")) == "" {
			return errors.New("artifact.code is required")
		}
	case "storyboard":
		frames, ok := artifact["frames"].([]any)
		if !ok || len(frames) == 0 {
			return errors.New("artifact.frames is required")
		}
	default:
		return errors.New("artifact.kind is invalid")
	}
	return nil
}

func promptLibraryPublicItemFromModel(item model.PromptLibraryItem) (promptLibraryPublicItem, error) {
	var title map[string]string
	var summary map[string]string
	var tags []string
	var output map[string]any
	var artifact map[string]any
	var source promptLibraryStoredSource
	if err := unmarshalPromptLibraryField(item.TitleJSON, &title); err != nil {
		return promptLibraryPublicItem{}, err
	}
	if err := unmarshalPromptLibraryField(item.SummaryJSON, &summary); err != nil {
		return promptLibraryPublicItem{}, err
	}
	if err := unmarshalPromptLibraryField(item.TagsJSON, &tags); err != nil {
		return promptLibraryPublicItem{}, err
	}
	if err := unmarshalPromptLibraryField(item.OutputJSON, &output); err != nil {
		return promptLibraryPublicItem{}, err
	}
	if err := unmarshalPromptLibraryField(item.ArtifactJSON, &artifact); err != nil {
		return promptLibraryPublicItem{}, err
	}
	if err := unmarshalPromptLibraryField(item.SourceJSON, &source); err != nil {
		return promptLibraryPublicItem{}, err
	}
	capturedAt := firstPromptLibraryValue(source.CapturedAt, source.CapturedAtSnake, item.CapturedAt, promptLibraryTimestampDate(item.UpdatedTime))
	return promptLibraryPublicItem{
		Artifact: artifact,
		Category: item.Category,
		Model:    item.Model,
		Output:   output,
		Prompt:   item.Prompt,
		Slug:     item.Slug,
		Source: promptLibraryPublicSource{
			CapturedAt: capturedAt,
			Label:      source.Label,
			Platform:   source.Platform,
			URL:        source.URL,
		},
		Summary:   summary,
		Tags:      tags,
		Title:     title,
		UpdatedAt: firstPromptLibraryValue(capturedAt, promptLibraryTimestampDate(item.UpdatedTime)),
	}, nil
}

func unmarshalPromptLibraryField(value string, target any) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return common.UnmarshalJsonStr(value, target)
}

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func marshalPromptLibraryField(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func promptLibraryIntQuery(c *gin.Context, key string, fallback int, max int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func promptLibraryTimestampDate(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).UTC().Format("2006-01-02")
}

func firstPromptLibraryValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
