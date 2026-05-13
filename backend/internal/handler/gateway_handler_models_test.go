package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExpandListableModelIDsOpenAISkipsWildcardPatterns(t *testing.T) {
	got := expandListableModelIDs(service.PlatformOpenAI, []string{
		"gpt-5.4",
		"gpt-5.*",
		"gpt-image-*",
		"*",
	})

	require.NotContains(t, got, "gpt-5.*")
	require.NotContains(t, got, "gpt-image-*")
	require.NotContains(t, got, "*")
	require.Contains(t, got, "gpt-5.4")
	require.Contains(t, got, "gpt-image-2")

	for _, model := range openai.DefaultModelIDs() {
		require.Contains(t, got, model)
	}
}

func TestExpandListableModelIDsOnlyReturnsDirectRequestNames(t *testing.T) {
	got := expandListableModelIDs(service.PlatformOpenAI, []string{
		"custom-alias",
		"gpt-5.4",
	})

	require.Equal(t, []string{"custom-alias", "gpt-5.4"}, got)
}

func TestExpandListableModelIDsUnknownPlatformUsesAllDefaultCatalogs(t *testing.T) {
	got := expandListableModelIDs("", []string{
		"gpt-*",
		"claude-*",
	})

	require.Contains(t, got, "gpt-5.4")
	require.Contains(t, got, "claude-sonnet-4-6")
	require.NotContains(t, got, "gpt-*")
	require.NotContains(t, got, "claude-*")
}

func TestModelListPatternMatches(t *testing.T) {
	require.True(t, modelListPatternMatches("gpt-*", "gpt-5.4"))
	require.True(t, modelListPatternMatches("*", "gpt-5.4"))
	require.False(t, modelListPatternMatches("gpt-*", "claude-sonnet-4-6"))
	require.False(t, modelListPatternMatches("gpt-*-mini", "gpt-5.4-mini"))
	require.False(t, modelListPatternMatches("", "gpt-5.4"))
}
