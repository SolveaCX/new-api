package dto

import "testing"

func TestImageRequestGPTImagePriceRatioUsesOfficialQualityAndSizeTiers(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		size    string
		want    float64
	}{
		{name: "low square", quality: "low", size: "1024x1024", want: 1},
		{name: "low portrait", quality: "low", size: "1024x1536", want: 0.016 / 0.011},
		{name: "medium square", quality: "medium", size: "1024x1024", want: 0.042 / 0.011},
		{name: "high landscape", quality: "high", size: "1536x1024", want: 0.25 / 0.011},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &ImageRequest{Model: "gpt-image-2", Quality: tt.quality, Size: tt.size}
			meta := request.GetTokenCountMeta()
			if diff := meta.ImagePriceRatio - tt.want; diff < -1e-9 || diff > 1e-9 {
				t.Fatalf("ImagePriceRatio = %v, want %v", meta.ImagePriceRatio, tt.want)
			}
		})
	}
}

func TestImageRequestUnknownGPTImageOptionsUseLowestTier(t *testing.T) {
	meta := (&ImageRequest{Model: "gpt-image-2", Quality: "auto", Size: "auto"}).GetTokenCountMeta()
	if meta.ImagePriceRatio != 1 {
		t.Fatalf("ImagePriceRatio = %v, want lowest-tier fallback 1", meta.ImagePriceRatio)
	}
}

func TestImageRequestDallEImagePriceRatioRemainsCompatible(t *testing.T) {
	meta := (&ImageRequest{Model: "dall-e-3", Quality: "hd", Size: "1792x1024"}).GetTokenCountMeta()
	if meta.ImagePriceRatio != 3 {
		t.Fatalf("ImagePriceRatio = %v, want 3", meta.ImagePriceRatio)
	}
}

func TestImageRequestGrokImagePriceRatioUsesOfficialResolutionTiers(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		quality string
		size    string
		want    float64
	}{
		{name: "base 2k", model: "grok-imagine-image", size: "2048x2048", want: 1},
		{name: "2.0 low 2k", model: "grok-imagine-image-2.0", quality: "low", size: "2048x2048", want: 0.06 / 0.04},
		{name: "2.0 medium 1k", model: "grok-imagine-image-2.0", quality: "medium", size: "1024x1024", want: 0.06 / 0.04},
		{name: "2.0 medium 2k", model: "grok-imagine-image-2.0", quality: "medium", size: "2048x2048", want: 0.08 / 0.04},
		{name: "pro 2k", model: "grok-imagine-image-pro", size: "2048x2048", want: 0.07 / 0.05},
		{name: "quality 2k", model: "grok-imagine-image-quality", size: "2048x2048", want: 0.07 / 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := (&ImageRequest{Model: tt.model, Quality: tt.quality, Size: tt.size}).GetTokenCountMeta()
			if diff := meta.ImagePriceRatio - tt.want; diff < -1e-9 || diff > 1e-9 {
				t.Fatalf("ImagePriceRatio = %v, want %v", meta.ImagePriceRatio, tt.want)
			}
		})
	}
}

func TestImageRequestUnknownGrokImageOptionsUseLowestTier(t *testing.T) {
	for _, model := range []string{"grok-imagine-image-2.0", "grok-imagine-image-pro", "grok-imagine-image-quality"} {
		meta := (&ImageRequest{Model: model, Quality: "auto", Size: "auto"}).GetTokenCountMeta()
		if meta.ImagePriceRatio != 1 {
			t.Fatalf("%s ImagePriceRatio = %v, want lowest-tier fallback 1", model, meta.ImagePriceRatio)
		}
	}
}
