package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type modelMarketplaceAccountRepo struct {
	AccountRepository
	accounts []Account
	err      error
}

func (r *modelMarketplaceAccountRepo) ListSchedulable(ctx context.Context) ([]Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.IsSchedulable() {
			out = append(out, account)
		}
	}
	return out, nil
}

func TestModelMarketplaceServiceListModelsUsesSchedulableAccountMappingsAndPrices(t *testing.T) {
	svc := NewModelMarketplaceService(
		&modelMarketplaceAccountRepo{accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"gpt-5.4":     "gpt-5.4",
					"gpt-unknown": "gpt-5.4",
					"*":           "gpt-5.4",
					"gpt-*":       "gpt-5.4",
				}},
			},
			{
				ID:          2,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: true,
			},
			{
				ID:          3,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusDisabled,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"disabled-model": "gpt-5.4"}},
			},
		}},
		NewBillingService(&config.Config{}, nil),
	)

	resp, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, resp.TotalAccounts)

	require.Nil(t, findMarketplaceModel(resp.Models, "*"))
	require.Nil(t, findMarketplaceModel(resp.Models, "gpt-*"))
	require.Nil(t, findMarketplaceModel(resp.Models, "disabled-model"))

	gpt54 := requireMarketplaceModel(t, resp.Models, "gpt-5.4")
	require.Equal(t, []string{PlatformOpenAI}, gpt54.Platforms)
	require.Equal(t, 1, gpt54.AccountCount)
	require.True(t, gpt54.Price.Available)
	require.NotNil(t, gpt54.Price.InputPricePerMTok)
	require.NotNil(t, gpt54.Price.OutputPricePerMTok)
	require.InDelta(t, 2.5, *gpt54.Price.InputPricePerMTok, 1e-12)
	require.InDelta(t, 15.0, *gpt54.Price.OutputPricePerMTok, 1e-12)
	require.Equal(t, "gpt-5.4", gpt54.Price.SourceModelID)

	unknown := requireMarketplaceModel(t, resp.Models, "gpt-unknown")
	require.True(t, unknown.Price.Available)
	require.Equal(t, "gpt-5.4", unknown.Price.SourceModelID)

	require.True(t, hasMarketplacePlatform(resp.Models, PlatformAnthropic))
}

func TestModelsFromMappingOrDefaultExpandsWildcardsOnlyAgainstDefaults(t *testing.T) {
	defaults := []marketplaceModelCandidate{
		{modelID: "foo-a", displayName: "Foo A"},
		{modelID: "bar-b", displayName: "Bar B"},
	}

	models := modelsFromMappingOrDefault(map[string]string{
		"foo-*": "foo-upstream",
		"*":     "bar-b",
	}, defaults)

	require.Len(t, models, 1)
	require.Equal(t, "foo-a", models[0].modelID)
	require.Equal(t, "Foo A", models[0].displayName)
	require.Equal(t, "foo-upstream", models[0].upstreamModel)
}

func TestOpenAIMarketplaceModelsIncludeOpenAICompatibleVendors(t *testing.T) {
	models := openAIMarketplaceModels(nil)
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.modelID)
	}

	require.Contains(t, names, "glm-5")
	require.Contains(t, names, "minimax-m2.5")
}

func TestModelMarketplacePriceDoesNotUseFuzzyFamilyFallback(t *testing.T) {
	svc := NewModelMarketplaceService(nil, NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"claude-sonnet-4-5-20250929": {
				InputCostPerToken:  0.000003,
				OutputCostPerToken: 0.000015,
			},
			"gpt-5.1-codex": {
				InputCostPerToken:  0.00000125,
				OutputCostPerToken: 0.00001,
			},
		},
	}))

	claude35 := svc.priceForModel("claude-3-5-sonnet-20240620", nil)
	require.True(t, claude35.Available)
	require.Equal(t, "claude-3-5-sonnet-20240620", claude35.SourceModelID)

	unknownGPT := svc.priceForModel("gpt-unknown", nil)
	require.False(t, unknownGPT.Available)

	explicitUpstream := svc.priceForModel("custom-model", []string{"claude-sonnet-4-5-20250929"})
	require.True(t, explicitUpstream.Available)
	require.Equal(t, "claude-sonnet-4-5-20250929", explicitUpstream.SourceModelID)
	require.NotNil(t, explicitUpstream.InputPricePerMTok)
	require.InDelta(t, 3.0, *explicitUpstream.InputPricePerMTok, 1e-12)
}

func TestModelMarketplaceServiceListModelsHandlesEmptyPlatform(t *testing.T) {
	svc := NewModelMarketplaceService(
		&modelMarketplaceAccountRepo{accounts: []Account{
			{
				ID:          1,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4"}},
			},
		}},
		NewBillingService(&config.Config{}, nil),
	)

	resp, err := svc.ListModels(context.Background())
	require.NoError(t, err)

	gpt54 := requireMarketplaceModel(t, resp.Models, "gpt-5.4")
	require.Empty(t, gpt54.Platforms)
	require.True(t, gpt54.Price.Available)
}

func requireMarketplaceModel(t *testing.T, models []ModelMarketplaceItem, modelID string) ModelMarketplaceItem {
	t.Helper()
	model := findMarketplaceModel(models, modelID)
	require.NotNil(t, model, "model %s should be listed", modelID)
	return *model
}

func findMarketplaceModel(models []ModelMarketplaceItem, modelID string) *ModelMarketplaceItem {
	for i := range models {
		if models[i].ModelID == modelID {
			return &models[i]
		}
	}
	return nil
}

func hasMarketplacePlatform(models []ModelMarketplaceItem, platform string) bool {
	for _, model := range models {
		for _, item := range model.Platforms {
			if item == platform {
				return true
			}
		}
	}
	return false
}
