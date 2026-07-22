package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenAIModelList_IncludesGPT55AndInstantModelID(t *testing.T) {
	assert.Contains(t, ModelList, "gpt-5.5")
	assert.Contains(t, ModelList, "chat-latest")
}

func TestOpenAIModelList_DoesNotRegisterGPT55InstantAlias(t *testing.T) {
	assert.NotContains(t, ModelList, "gpt-5.5-instant")
}
