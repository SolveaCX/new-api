package model

import (
	"regexp"
	"sort"
	"strings"
)

var (
	modelOrderDatePattern      = regexp.MustCompile("20[0-9]{2}[-_]?[0-9]{2}[-_]?[0-9]{2}|[0-9]{8}")
	modelOrderSuffixPattern    = regexp.MustCompile("[-_](latest|preview|beta|stable|turbo|instruct|chat|online|fk|br|cc|compact)$")
	modelOrderSeparatorPattern = regexp.MustCompile("[-_]+")

	modelOrderGPT5Pattern       = regexp.MustCompile("^gpt-5([.-][0-9]+)?(-|$)")
	modelOrderGPT4Pattern       = regexp.MustCompile("^gpt-4([.-][0-9]+)?(-|$)")
	modelOrderGPT3Pattern       = regexp.MustCompile("^gpt-3([.-][0-9]+)?(-|$)")
	modelOrderOpenAIOPattern    = regexp.MustCompile("^o[1-4](-|$)")
	modelOrderClaudePattern     = regexp.MustCompile("^claude-(opus|sonnet|haiku|fable)-([0-9]+)")
	modelOrderGeminiPattern     = regexp.MustCompile("^gemini-([0-9]+([.][0-9]+)?)")
	modelOrderQwenPattern       = regexp.MustCompile("^qwen([0-9]+|[-_][0-9]+)?")
	modelOrderDeepSeekRPattern  = regexp.MustCompile("^deepseek[-_]?r")
	modelOrderLlamaPattern      = regexp.MustCompile("^llama[-_]?[0-9]+")
	modelOrderTextEmbedding     = regexp.MustCompile("^text-embedding")
	modelOrderDallEPattern      = regexp.MustCompile("^dall[-_]e")
	modelOrderSimpleFamilyNames = []struct {
		pattern *regexp.Regexp
		key     string
	}{
		{regexp.MustCompile("^deepseek"), "deepseek"},
		{regexp.MustCompile("^doubao"), "doubao"},
		{regexp.MustCompile("^hunyuan"), "hunyuan"},
		{regexp.MustCompile("^glm[-_]?"), "glm"},
		{regexp.MustCompile("^mistral"), "mistral"},
		{regexp.MustCompile("^grok[-_]?"), "grok"},
		{regexp.MustCompile("^kimi[-_]?"), "kimi"},
		{regexp.MustCompile("^abab[-_]?"), "abab"},
		{regexp.MustCompile("^seedance[-_]?"), "seedance"},
		{regexp.MustCompile("^kling[-_]?"), "kling"},
		{regexp.MustCompile("^veo[-_]?"), "veo"},
		{regexp.MustCompile("^imagen[-_]?"), "imagen"},
		{regexp.MustCompile("^sora[-_]?"), "sora"},
	}
)

// SortModelsByFamilyAndCreatedTime sorts only models that resolve to the same
// family. Family slots retain their original positions, so unrelated model
// families keep the order supplied by the ability query.
func SortModelsByFamilyAndCreatedTime(
	modelNames []string,
	createdTimes map[string]int64,
) []string {
	sorted := append([]string(nil), modelNames...)
	if len(sorted) < 2 {
		return sorted
	}

	indexesByFamily := make(map[string][]int)
	for index, modelName := range sorted {
		family := modelOrderFamilyKey(modelName)
		indexesByFamily[family] = append(indexesByFamily[family], index)
	}

	for _, indexes := range indexesByFamily {
		if len(indexes) < 2 {
			continue
		}

		familyModels := make([]string, len(indexes))
		for i, index := range indexes {
			familyModels[i] = sorted[index]
		}

		sort.SliceStable(familyModels, func(i, j int) bool {
			leftTime, leftKnown := createdTimes[familyModels[i]]
			rightTime, rightKnown := createdTimes[familyModels[j]]
			if leftKnown != rightKnown {
				return leftKnown
			}
			if !leftKnown || leftTime == rightTime {
				return false
			}
			return leftTime > rightTime
		})

		for i, index := range indexes {
			sorted[index] = familyModels[i]
		}
	}

	return sorted
}

func modelOrderFamilyKey(modelName string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	normalized = modelOrderDatePattern.ReplaceAllString(normalized, "")
	normalized = modelOrderSuffixPattern.ReplaceAllString(normalized, "")
	normalized = strings.Trim(normalized, "-_")
	normalized = modelOrderSeparatorPattern.ReplaceAllString(normalized, "-")

	switch {
	case modelOrderGPT5Pattern.MatchString(normalized):
		return "gpt-5"
	case modelOrderGPT4Pattern.MatchString(normalized):
		return "gpt-4"
	case modelOrderGPT3Pattern.MatchString(normalized):
		return "gpt-3"
	case modelOrderOpenAIOPattern.MatchString(normalized):
		return "openai-o"
	}

	if match := modelOrderClaudePattern.FindStringSubmatch(normalized); len(match) > 2 {
		return "claude-" + match[1] + "-" + match[2]
	}
	if match := modelOrderGeminiPattern.FindStringSubmatch(normalized); len(match) > 1 {
		return "gemini-" + match[1]
	}
	if modelOrderQwenPattern.MatchString(normalized) {
		return "qwen"
	}
	if modelOrderDeepSeekRPattern.MatchString(normalized) {
		return "deepseek-r"
	}
	if modelOrderLlamaPattern.MatchString(normalized) {
		return "llama"
	}
	if modelOrderTextEmbedding.MatchString(normalized) {
		return "text-embedding"
	}
	if modelOrderDallEPattern.MatchString(normalized) {
		return "dalle"
	}
	for _, family := range modelOrderSimpleFamilyNames {
		if family.pattern.MatchString(normalized) {
			return family.key
		}
	}

	parts := strings.Split(normalized, "-")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, "-")
}
