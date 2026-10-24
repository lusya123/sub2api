package service

import "strings"

// Published USD/token prices, verified 2026-10-25:
// https://platform.claude.com/docs/en/about-claude/pricing
// The >100K tier includes cached input and applies to the entire request.
var claudeHaiku55Pricing = LiteLLMModelPricing{
	InputCostPerToken:                   0.10e-6,
	OutputCostPerToken:                  0.50e-6,
	CacheCreationInputTokenCost:         0.125e-6,
	CacheCreationInputTokenCostAbove1hr: 0.20e-6,
	CacheReadInputTokenCost:             0.01e-6,
	LongContextInputTokenThreshold:      100000,
	LongContextInputCostMultiplier:      5,
	LongContextOutputCostMultiplier:     5,
	LiteLLMProvider:                     "anthropic", Mode: "chat", SupportsPromptCaching: true,
}

func isClaudeHaiku55Model(model string) bool {
	model = fallbackModelIdentifier(strings.ToLower(strings.TrimSpace(model)))
	model = strings.TrimSuffix(strings.TrimSuffix(model, "[1m]"), "-1m")
	model = strings.TrimSuffix(model, "-thinking")
	return model == "claude-haiku-5-5" || model == "claude-haiku-5.5"
}
