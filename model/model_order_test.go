package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortModelsByFamilyAndCreatedTimeSortsOnlyWithinFamily(t *testing.T) {
	input := []string{"gpt-5.4", "claude-sonnet-4", "gpt-5.3", "gpt-4o"}
	created := map[string]int64{
		"gpt-5.4":         100,
		"gpt-5.3":         200,
		"claude-sonnet-4": 300,
		"gpt-4o":          400,
	}

	got := SortModelsByFamilyAndCreatedTime(input, created)

	require.Equal(t, []string{"gpt-5.3", "claude-sonnet-4", "gpt-5.4", "gpt-4o"}, got)
	require.Equal(t, []string{"gpt-5.4", "claude-sonnet-4", "gpt-5.3", "gpt-4o"}, input)
}

func TestSortModelsByFamilyAndCreatedTimeKeepsUnknownEntriesAfterKnownEntries(t *testing.T) {
	input := []string{"claude-sonnet-4-custom", "claude-sonnet-4", "claude-sonnet-4-preview"}
	created := map[string]int64{"claude-sonnet-4": 10}

	got := SortModelsByFamilyAndCreatedTime(input, created)

	require.Equal(t, []string{"claude-sonnet-4", "claude-sonnet-4-custom", "claude-sonnet-4-preview"}, got)
}

func TestSortModelsByFamilyAndCreatedTimeKeepsStableOrderForTiesAndDifferentFamilies(t *testing.T) {
	input := []string{"gpt-5.4-a", "gpt-5.3", "gpt-5.4-b", "gemini-2.5-pro"}
	created := map[string]int64{
		"gpt-5.4-a":      50,
		"gpt-5.3":        80,
		"gpt-5.4-b":      50,
		"gemini-2.5-pro": 999,
	}

	got := SortModelsByFamilyAndCreatedTime(input, created)

	require.Equal(t, []string{"gpt-5.3", "gpt-5.4-a", "gpt-5.4-b", "gemini-2.5-pro"}, got)
}

func TestSortModelsByFamilyAndCreatedTimeReturnsCopyWhenMetadataIsUnavailable(t *testing.T) {
	input := []string{"gpt-5.4", "gpt-5.3"}

	got := SortModelsByFamilyAndCreatedTime(input, nil)

	require.Equal(t, input, got)
	require.NotSame(t, &input[0], &got[0])
}
