package service

import "testing"

func TestValidateModelMarketplaceEffectiveRate(t *testing.T) {
	valid := 0.75
	if err := validateModelMarketplaceEffectiveRate(&valid); err != nil {
		t.Fatalf("validate positive effective rate: %v", err)
	}
	if err := validateModelMarketplaceEffectiveRate(nil); err != nil {
		t.Fatalf("validate nil effective rate: %v", err)
	}

	zero := 0.0
	if err := validateModelMarketplaceEffectiveRate(&zero); err != ErrModelMarketplaceMonitorInvalidEffectiveRate {
		t.Fatalf("validate zero effective rate = %v, want %v", err, ErrModelMarketplaceMonitorInvalidEffectiveRate)
	}

	negative := -0.1
	if err := validateModelMarketplaceEffectiveRate(&negative); err != ErrModelMarketplaceMonitorInvalidEffectiveRate {
		t.Fatalf("validate negative effective rate = %v, want %v", err, ErrModelMarketplaceMonitorInvalidEffectiveRate)
	}
}

func TestNormalizeModelMarketplaceEffectiveRate(t *testing.T) {
	if got := normalizeModelMarketplaceEffectiveRate(nil); got != 1 {
		t.Fatalf("normalize nil effective rate = %v, want 1", got)
	}
	zero := 0.0
	if got := normalizeModelMarketplaceEffectiveRate(&zero); got != 1 {
		t.Fatalf("normalize zero effective rate = %v, want 1", got)
	}
	custom := 0.62
	if got := normalizeModelMarketplaceEffectiveRate(&custom); got != custom {
		t.Fatalf("normalize custom effective rate = %v, want %v", got, custom)
	}
}

func TestModelMarketplacePricingOverride(t *testing.T) {
	input := 0.15
	output := 0.6
	cfgs := map[string]ModelMarketplaceModelCallConfig{
		"missing-default-price-model": {
			Pricing: &ModelMarketplaceModelPricingOverride{
				InputPricePerMillion:  &input,
				OutputPricePerMillion: &output,
			},
		},
	}
	pricing := modelMarketplacePricingOverrideFor(cfgs, "missing-default-price-model")
	if pricing == nil {
		t.Fatal("pricing override = nil")
	}
	if pricing.InputPrice == nil || *pricing.InputPrice != input/1_000_000 {
		t.Fatalf("input price = %v, want %v", pricing.InputPrice, input/1_000_000)
	}
	if pricing.OutputPrice == nil || *pricing.OutputPrice != output/1_000_000 {
		t.Fatalf("output price = %v, want %v", pricing.OutputPrice, output/1_000_000)
	}

	negative := -0.1
	if err := validateModelMarketplacePricingOverride(&ModelMarketplaceModelPricingOverride{InputPricePerMillion: &negative}); err != ErrModelMarketplaceMonitorInvalidPricing {
		t.Fatalf("validate negative pricing = %v, want %v", err, ErrModelMarketplaceMonitorInvalidPricing)
	}
}
