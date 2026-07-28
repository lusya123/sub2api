//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWSPassthroughUsageMeta_InitFromFirstFrame_MappedModelCandidate(t *testing.T) {
	body := []byte(`{"type":"response.create","model":"sol","reasoning":{"effort":"max"}}`)

	meta := newOpenAIWSPassthroughUsageMeta("sol", body)
	meta.initFromFirstFrame(body, "gpt-5.6-sol")

	got := meta.reasoningEffort.Load()
	require.NotNil(t, got, "reasoning effort should be set")
	require.Equal(t, "max", *got, "mapped model gpt-5.6-sol should preserve max")
}

func TestWSPassthroughUsageMeta_InitFromFirstFrame_NonGPT56FallsBackToXHigh(t *testing.T) {
	body := []byte(`{"type":"response.create","model":"gpt-5.4","reasoning":{"effort":"max"}}`)

	meta := newOpenAIWSPassthroughUsageMeta("gpt-5.4", body)
	meta.initFromFirstFrame(body, "gpt-5.4")

	got := meta.reasoningEffort.Load()
	require.NotNil(t, got)
	require.Equal(t, "xhigh", *got, "non-5.6 model should normalize max to xhigh")
}

func TestWSPassthroughUsageMeta_UpdateFromResponseCreate_MappedModelCandidate(t *testing.T) {
	body := []byte(`{"type":"response.create","model":"sol","reasoning":{"effort":"max"}}`)

	meta := newOpenAIWSPassthroughUsageMeta("sol", body)
	meta.updateFromResponseCreate(body, "gpt-5.6-sol", "sol")

	got := meta.reasoningEffort.Load()
	require.NotNil(t, got)
	require.Equal(t, "max", *got, "mapped model should preserve max on multi-turn update")
}

func TestWSPassthroughUsageMeta_CurrentTurnSchedulingModelUsesRequestedTurn(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"sol":   "gpt-5.6-sol",
				"terra": "gpt-5.6-terra",
			},
		},
	}
	first := []byte(`{"type":"response.create","model":"gpt-5.6-sol"}`)
	meta := newOpenAIWSPassthroughUsageMeta("sol", first)
	meta.initFromFirstFrame(first, "gpt-5.6-sol")
	require.Equal(t, "gpt-5.6-sol", meta.currentTurnSchedulingModel(account, ""))

	second := []byte(`{"type":"response.create","model":"gpt-5.6-terra"}`)
	meta.updateFromResponseCreate(second, "gpt-5.6-terra", "terra")
	require.Equal(t, "gpt-5.6-terra", meta.currentTurnSchedulingModel(account, ""))

	// A session-level update may arrive while a response is still active. It
	// changes only the fallback for a future turn and must not race with or
	// relabel the active turn's transient-health key.
	meta.updateSessionRequestModel([]byte(`{"type":"session.update","session":{"model":"luna"}}`))
	require.Equal(t, "gpt-5.6-terra", meta.currentTurnSchedulingModel(account, ""))
}
