package service

import "math"

// VirtualCacheResult holds the simulated cache token distribution.
// The three fields satisfy BOTH identities:
//
//	Cost:  InputTokens + 0.1*CacheReadInputTokens + 1.25*CacheCreationInputTokens = original total
//	Count: InputTokens + CacheReadInputTokens + CacheCreationInputTokens = original total
//
// The ratio CacheCreationInputTokens = 3.6 * CacheReadInputTokens is fixed by
// the Claude pricing structure (cache_read = 10% of input, cache_creation = 125% of input).
type VirtualCacheResult struct {
	InputTokens              int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// cacheCreationToReadRatio is the fixed ratio CC/CR derived from Claude pricing:
//
//	0.9 * CR = 0.25 * CC  →  CC = 3.6 * CR
//
// This comes from requiring both cost identity and token count identity simultaneously.
const cacheCreationToReadRatio = 3.6

// maxSafeReadRatio is the maximum α before I_v goes negative: 1 / (1 + 3.6) ≈ 0.2174
const maxSafeReadRatio = 1.0 / (1.0 + cacheCreationToReadRatio)

// defaultVirtualCacheReadRatio is the default α when not configured.
const defaultVirtualCacheReadRatio = 0.15

// calculateVirtualCache converts real input tokens (all regular, no cache) into a
// virtual cache distribution that preserves both the cost identity and the token
// count identity.
//
// Mathematical basis:
//
//	Given T total input tokens and Claude pricing ratios:
//	  cache_read_price  = 0.1  × input_price
//	  cache_creation_price = 1.25 × input_price
//
//	We need:
//	  (1) I_v + 0.1*CR_v + 1.25*CC_v = T   (cost identity — rate multiplier cancels)
//	  (2) I_v + CR_v + CC_v = T              (token count identity)
//
//	Solving: CC_v = 3.6 * CR_v, I_v = T - 4.6 * CR_v
//
// readRatio (α) controls what fraction of T becomes cache_read. Valid range: [0, ~0.217].
// Values above maxSafeReadRatio are clamped automatically.
func calculateVirtualCache(totalInputTokens int, readRatio float64) VirtualCacheResult {
	if totalInputTokens <= 0 {
		return VirtualCacheResult{}
	}

	// Clamp readRatio to safe range
	if readRatio <= 0 {
		return VirtualCacheResult{InputTokens: totalInputTokens}
	}
	if readRatio > maxSafeReadRatio {
		readRatio = maxSafeReadRatio
	}

	T := float64(totalInputTokens)

	// CR_v = floor(T × α)
	crv := int(math.Floor(T * readRatio))
	if crv < 0 {
		crv = 0
	}

	// CC_v = floor(3.6 × CR_v)
	ccv := int(math.Floor(cacheCreationToReadRatio * float64(crv)))
	if ccv < 0 {
		ccv = 0
	}

	// I_v = T - CR_v - CC_v (exact integer, preserves token count identity)
	iv := totalInputTokens - crv - ccv

	// Safety: if I_v went negative (shouldn't happen with clamped ratio, but be safe).
	// Recalculate using the maximum safe ratio to guarantee I_v >= 0.
	if iv < 0 {
		crv = int(T / (1.0 + cacheCreationToReadRatio))
		ccv = int(math.Floor(cacheCreationToReadRatio * float64(crv)))
		iv = totalInputTokens - crv - ccv
		// Final guard: if still negative due to integer rounding, absorb into I_v
		if iv < 0 {
			ccv += iv // reduce ccv by the deficit
			iv = 0
		}
	}

	return VirtualCacheResult{
		InputTokens:              iv,
		CacheReadInputTokens:     crv,
		CacheCreationInputTokens: ccv,
	}
}

// applyVirtualCacheToUsageJSON rewrites the usage map (from SSE event JSON) with
// virtual cache tokens. Returns true if the map was modified.
func applyVirtualCacheToUsageJSON(usage map[string]any, readRatio float64) bool {
	if usage == nil {
		return false
	}

	// Only apply when upstream returned no cache tokens
	cacheRead, _ := usage["cache_read_input_tokens"].(float64)
	cacheCreation, _ := usage["cache_creation_input_tokens"].(float64)
	if cacheRead > 0 || cacheCreation > 0 {
		return false // real cache data exists, don't override
	}

	inputTokens, _ := usage["input_tokens"].(float64)
	if inputTokens <= 0 {
		return false
	}

	vc := calculateVirtualCache(int(inputTokens), readRatio)
	usage["input_tokens"] = float64(vc.InputTokens)
	usage["cache_read_input_tokens"] = float64(vc.CacheReadInputTokens)
	usage["cache_creation_input_tokens"] = float64(vc.CacheCreationInputTokens)
	return true
}

// applyVirtualCacheToOpenAIUsage rewrites OpenAI-style usage with virtual cache tokens.
//
// OpenAI usage semantics differ from Claude:
//   - input_tokens includes cached read tokens
//   - cache_read lives under input_tokens_details.cached_tokens
//
// To preserve the OpenAI invariant, we store:
//
//	input_tokens = virtual_input_tokens + virtual_cache_read_tokens
//
// while separately exposing cache_read / cache_creation for billing and usage logs.
func applyVirtualCacheToOpenAIUsage(usage *OpenAIUsage, readRatio float64) bool {
	if usage == nil {
		return false
	}
	if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 || usage.InputTokens <= 0 {
		return false
	}

	vc := calculateVirtualCache(usage.InputTokens, readRatio)
	usage.InputTokens = vc.InputTokens + vc.CacheReadInputTokens
	usage.CacheReadInputTokens = vc.CacheReadInputTokens
	usage.CacheCreationInputTokens = vc.CacheCreationInputTokens
	return true
}
