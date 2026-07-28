package auto_model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRewriteModelPreservesUnknownAndExplicitZeroValues(t *testing.T) {
	raw := []byte(`{"model":"auto","temperature":0,"stream":false,"unknown":{"nested":"value"},"messages":[{"role":"user","content":"hi"}]}`)
	rewritten, err := RewriteModel(raw, "gpt-real")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, common.Unmarshal(rewritten, &result))
	require.Equal(t, "gpt-real", result["model"])
	require.Equal(t, float64(0), result["temperature"])
	require.Equal(t, false, result["stream"])
	require.Equal(t, map[string]any{"nested": "value"}, result["unknown"])
}
