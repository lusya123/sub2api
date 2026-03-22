//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- calculateVirtualCache 核心逻辑测试 ----------

func TestVirtualCache_TokenCountIdentity(t *testing.T) {
	// I_v + CR_v + CC_v = T (exact)
	tests := []struct {
		name       string
		total      int
		readRatio  float64
	}{
		{"default ratio", 10000, 0.15},
		{"small input", 100, 0.15},
		{"large input", 1000000, 0.15},
		{"max ratio", 10000, maxSafeReadRatio},
		{"min ratio", 10000, 0.01},
		{"zero ratio", 10000, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := calculateVirtualCache(tt.total, tt.readRatio)
			sum := vc.InputTokens + vc.CacheReadInputTokens + vc.CacheCreationInputTokens
			assert.Equal(t, tt.total, sum,
				"token count identity violated: %d + %d + %d = %d, want %d",
				vc.InputTokens, vc.CacheReadInputTokens, vc.CacheCreationInputTokens, sum, tt.total)
		})
	}
}

func TestVirtualCache_CostIdentity(t *testing.T) {
	// I_v + 0.1*CR_v + 1.25*CC_v ≈ T (within ±1 rounding error)
	tests := []struct {
		name       string
		total      int
		readRatio  float64
	}{
		{"default ratio", 10000, 0.15},
		{"small input", 100, 0.15},
		{"large input", 1000000, 0.15},
		{"max ratio", 10000, maxSafeReadRatio},
		{"min ratio", 10000, 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := calculateVirtualCache(tt.total, tt.readRatio)
			costEquiv := float64(vc.InputTokens) + 0.1*float64(vc.CacheReadInputTokens) + 1.25*float64(vc.CacheCreationInputTokens)
			diff := math.Abs(costEquiv - float64(tt.total))
			assert.LessOrEqual(t, diff, 1.0,
				"cost identity violated: %.2f + 0.1*%.0f + 1.25*%.0f = %.2f, want %d (diff=%.2f)",
				float64(vc.InputTokens), float64(vc.CacheReadInputTokens), float64(vc.CacheCreationInputTokens),
				costEquiv, tt.total, diff)
		})
	}
}

func TestVirtualCache_NonNegativeTokens(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		readRatio float64
	}{
		{"default", 10000, 0.15},
		{"max ratio", 10000, maxSafeReadRatio},
		{"over max ratio", 10000, 0.5},
		{"ratio = 1.0", 10000, 1.0},
		{"small total", 3, 0.15},
		{"total = 1", 1, 0.15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := calculateVirtualCache(tt.total, tt.readRatio)
			assert.GreaterOrEqual(t, vc.InputTokens, 0, "InputTokens should be >= 0")
			assert.GreaterOrEqual(t, vc.CacheReadInputTokens, 0, "CacheReadInputTokens should be >= 0")
			assert.GreaterOrEqual(t, vc.CacheCreationInputTokens, 0, "CacheCreationInputTokens should be >= 0")
		})
	}
}

func TestVirtualCache_EdgeCases(t *testing.T) {
	t.Run("zero tokens", func(t *testing.T) {
		vc := calculateVirtualCache(0, 0.15)
		assert.Equal(t, 0, vc.InputTokens)
		assert.Equal(t, 0, vc.CacheReadInputTokens)
		assert.Equal(t, 0, vc.CacheCreationInputTokens)
	})

	t.Run("negative tokens", func(t *testing.T) {
		vc := calculateVirtualCache(-100, 0.15)
		assert.Equal(t, 0, vc.InputTokens)
		assert.Equal(t, 0, vc.CacheReadInputTokens)
		assert.Equal(t, 0, vc.CacheCreationInputTokens)
	})

	t.Run("one token", func(t *testing.T) {
		vc := calculateVirtualCache(1, 0.15)
		sum := vc.InputTokens + vc.CacheReadInputTokens + vc.CacheCreationInputTokens
		assert.Equal(t, 1, sum)
		assert.GreaterOrEqual(t, vc.InputTokens, 0)
	})

	t.Run("zero ratio returns all input", func(t *testing.T) {
		vc := calculateVirtualCache(10000, 0.0)
		assert.Equal(t, 10000, vc.InputTokens)
		assert.Equal(t, 0, vc.CacheReadInputTokens)
		assert.Equal(t, 0, vc.CacheCreationInputTokens)
	})

	t.Run("negative ratio returns all input", func(t *testing.T) {
		vc := calculateVirtualCache(10000, -0.5)
		assert.Equal(t, 10000, vc.InputTokens)
		assert.Equal(t, 0, vc.CacheReadInputTokens)
		assert.Equal(t, 0, vc.CacheCreationInputTokens)
	})
}

func TestVirtualCache_SafetyGuard(t *testing.T) {
	// readRatio > maxSafeReadRatio should be clamped, not cause negative I_v
	vc := calculateVirtualCache(10000, 0.5)
	assert.GreaterOrEqual(t, vc.InputTokens, 0, "safety guard should prevent negative InputTokens")
	sum := vc.InputTokens + vc.CacheReadInputTokens + vc.CacheCreationInputTokens
	assert.Equal(t, 10000, sum, "token count identity must hold even with clamped ratio")

	// Cost identity must also hold after safety guard
	costEquiv := float64(vc.InputTokens) + 0.1*float64(vc.CacheReadInputTokens) + 1.25*float64(vc.CacheCreationInputTokens)
	diff := math.Abs(costEquiv - 10000.0)
	assert.LessOrEqual(t, diff, 1.0, "cost identity must hold after safety guard, diff=%.2f", diff)
}

func TestVirtualCache_ConcreteExample(t *testing.T) {
	// From the plan: T=10000, α=0.15
	// CR_v = floor(10000 * 0.15) = 1500
	// CC_v = floor(3.6 * 1500) = 5400
	// I_v = 10000 - 1500 - 5400 = 3100
	vc := calculateVirtualCache(10000, 0.15)
	assert.Equal(t, 1500, vc.CacheReadInputTokens)
	assert.Equal(t, 5400, vc.CacheCreationInputTokens)
	assert.Equal(t, 3100, vc.InputTokens)
}

func TestVirtualCache_CacheCreationToReadRatio(t *testing.T) {
	// CC_v should be approximately 3.6 * CR_v (floor rounding may differ by 1)
	vc := calculateVirtualCache(100000, 0.15)
	if vc.CacheReadInputTokens > 0 {
		ratio := float64(vc.CacheCreationInputTokens) / float64(vc.CacheReadInputTokens)
		assert.InDelta(t, 3.6, ratio, 0.01,
			"CC/CR ratio should be ~3.6, got %.4f", ratio)
	}
}

// ---------- applyVirtualCacheToUsageJSON 测试 ----------

func TestApplyVirtualCacheToUsageJSON_Basic(t *testing.T) {
	usage := map[string]any{
		"input_tokens":                float64(10000),
		"cache_read_input_tokens":     float64(0),
		"cache_creation_input_tokens": float64(0),
	}

	changed := applyVirtualCacheToUsageJSON(usage, 0.15)
	assert.True(t, changed)
	assert.Equal(t, float64(3100), usage["input_tokens"])
	assert.Equal(t, float64(1500), usage["cache_read_input_tokens"])
	assert.Equal(t, float64(5400), usage["cache_creation_input_tokens"])
}

func TestApplyVirtualCacheToUsageJSON_SkipsRealCache(t *testing.T) {
	// If upstream already has cache tokens, don't override
	usage := map[string]any{
		"input_tokens":                float64(5000),
		"cache_read_input_tokens":     float64(3000),
		"cache_creation_input_tokens": float64(0),
	}

	changed := applyVirtualCacheToUsageJSON(usage, 0.15)
	assert.False(t, changed)
	assert.Equal(t, float64(5000), usage["input_tokens"])
	assert.Equal(t, float64(3000), usage["cache_read_input_tokens"])
}

func TestApplyVirtualCacheToUsageJSON_NilUsage(t *testing.T) {
	assert.False(t, applyVirtualCacheToUsageJSON(nil, 0.15))
}

func TestApplyVirtualCacheToUsageJSON_ZeroInput(t *testing.T) {
	usage := map[string]any{
		"input_tokens":                float64(0),
		"cache_read_input_tokens":     float64(0),
		"cache_creation_input_tokens": float64(0),
	}
	assert.False(t, applyVirtualCacheToUsageJSON(usage, 0.15))
}

// ---------- message_delta 场景测试 ----------

func TestApplyVirtualCacheToUsageJSON_MessageDelta(t *testing.T) {
	// message_delta 可能携带 input_tokens（某些上游），虚拟缓存也应正确处理
	usage := map[string]any{
		"output_tokens":               float64(50),
		"input_tokens":                float64(5000),
		"cache_read_input_tokens":     float64(0),
		"cache_creation_input_tokens": float64(0),
	}

	changed := applyVirtualCacheToUsageJSON(usage, 0.15)
	assert.True(t, changed)

	iv := int(usage["input_tokens"].(float64))
	cr := int(usage["cache_read_input_tokens"].(float64))
	cc := int(usage["cache_creation_input_tokens"].(float64))
	assert.Equal(t, 5000, iv+cr+cc, "token count identity")
	assert.Equal(t, float64(50), usage["output_tokens"], "output_tokens should be untouched")
}

func TestApplyVirtualCacheToUsageJSON_MessageDeltaNoInput(t *testing.T) {
	// message_delta 只有 output_tokens，没有 input_tokens → 不应修改
	usage := map[string]any{
		"output_tokens": float64(100),
	}

	changed := applyVirtualCacheToUsageJSON(usage, 0.15)
	assert.False(t, changed)
	assert.Equal(t, float64(100), usage["output_tokens"])
}

// ---------- 倍率无关性验证 ----------

func TestVirtualCache_RateMultiplierIndependence(t *testing.T) {
	// The virtual cache distribution should be identical regardless of rate multiplier.
	// Rate multiplier only affects the final cost (applied after token-level pricing).
	// Since our formula cancels R, the token split is the same for any R.

	T := 50000
	vc := calculateVirtualCache(T, 0.15)

	// Simulate cost calculation for different multipliers
	inputPrice := 3e-06 // Claude Sonnet input price
	cacheReadPrice := inputPrice * 0.1
	cacheCreationPrice := inputPrice * 1.25

	realCostBase := float64(T) * inputPrice
	virtualCostBase := float64(vc.InputTokens)*inputPrice +
		float64(vc.CacheReadInputTokens)*cacheReadPrice +
		float64(vc.CacheCreationInputTokens)*cacheCreationPrice

	multipliers := []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0}
	for _, R := range multipliers {
		realCost := realCostBase * R
		virtualCost := virtualCostBase * R
		diff := math.Abs(realCost - virtualCost)
		require.InDelta(t, realCost, virtualCost, inputPrice*R*2,
			"cost mismatch at R=%.1f: real=%.10f virtual=%.10f diff=%.10f",
			R, realCost, virtualCost, diff)
	}
}
