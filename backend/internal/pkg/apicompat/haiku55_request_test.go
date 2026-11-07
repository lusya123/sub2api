package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHaiku55ChatThinkingAndCacheConversion(t *testing.T) {
	for _, model := range []string{"claude-haiku-5-5", "claude-haiku-5.5-thinking[1m]", "claude-haiku-5-5-thinking-1m"} {
		for _, effort := range []string{"", "none", "low", "medium", "high", "xhigh", "max"} {
			t.Run(model+"/"+effort, func(t *testing.T) {
				temperature := 0.7
				req := &ChatCompletionsRequest{Model: model, ReasoningEffort: effort, Temperature: &temperature, Messages: []ChatMessage{
					{Role: "developer", Content: json.RawMessage(`[{"type":"text","text":"instructions","cache_control":{"type":"ephemeral","ttl":"1h"}}]`)},
					{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"reference","cache_control":{"type":"ephemeral","ttl":"5m"}},{"type":"text","text":"question"}]`)},
				}}
				responses, err := ChatCompletionsToResponses(req)
				require.NoError(t, err)
				got, err := ResponsesToAnthropicRequest(responses)
				require.NoError(t, err)
				require.NotNil(t, got.Thinking)
				assert.Zero(t, got.Thinking.BudgetTokens)
				assert.Nil(t, got.Temperature)
				if effort == "none" {
					assert.Equal(t, "disabled", got.Thinking.Type)
					assert.Nil(t, got.OutputConfig)
				} else {
					assert.Equal(t, "adaptive", got.Thinking.Type)
					require.NotNil(t, got.OutputConfig)
					want := effort
					if want == "" {
						want = "medium"
					}
					assert.Equal(t, want, got.OutputConfig.Effort)
				}
				var system []AnthropicContentBlock
				require.NoError(t, json.Unmarshal(got.System, &system))
				require.Len(t, system, 1)
				require.NotNil(t, system[0].CacheControl)
				assert.Equal(t, "1h", system[0].CacheControl.TTL)
				require.Len(t, got.Messages, 1)
				assert.Equal(t, "user", got.Messages[0].Role)
				var content []AnthropicContentBlock
				require.NoError(t, json.Unmarshal(got.Messages[0].Content, &content))
				require.Len(t, content, 2)
				require.NotNil(t, content[0].CacheControl)
				assert.Equal(t, "5m", content[0].CacheControl.TTL)
				assert.Nil(t, content[1].CacheControl)
			})
		}
	}
}
