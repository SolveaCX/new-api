package codex

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
)

func TestCodexModelList_IncludesGPT55AndCompactVariant(t *testing.T) {
	assert.Contains(t, ModelList, "gpt-5.5")
	assert.Contains(t, ModelList, ratio_setting.WithCompactModelSuffix("gpt-5.5"))
}

func TestCodexModelList_DoesNotRegisterInstantAlias(t *testing.T) {
	assert.NotContains(t, ModelList, "gpt-5.5-instant")
	assert.NotContains(t, ModelList, "chat-latest")
}
