package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskSubmitAdaptorsUseBoundedNonStreamingJSONResponses(t *testing.T) {
	adaptors := []string{
		"ali", "blockrunseedance", "blockrunvideo", "doubao", "gemini", "hailuo", "jimeng", "jimengproxy",
		"jimengzhizinan", "kling", "kuaizi", "sora", "suno", "techmobi", "vertex", "vidu",
	}
	require.Len(t, adaptors, 16)
	for _, name := range adaptors {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("channel", "task", name, "adaptor.go")
			body, err := os.ReadFile(path)
			require.NoError(t, err)
			source := string(body)
			require.Contains(t, source, "DoResponse(")
			require.Contains(t, source, "c.JSON(")
			require.NotContains(t, source, "c.Writer.Flush(")
			require.NotContains(t, source, ".Hijack(")
			require.False(t, strings.Contains(source, "text/event-stream"), "task submit response must not stream")
		})
	}
}
