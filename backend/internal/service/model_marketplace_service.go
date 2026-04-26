package service

import (
	"context"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

type ModelMarketplaceService struct {
	accountRepo    AccountRepository
	billingService *BillingService
}

type ModelMarketplacePrice struct {
	InputPricePerToken  *float64 `json:"input_price_per_token"`
	OutputPricePerToken *float64 `json:"output_price_per_token"`
	InputPricePerMTok   *float64 `json:"input_price_per_mtok"`
	OutputPricePerMTok  *float64 `json:"output_price_per_mtok"`
	TotalPricePerMTok   *float64 `json:"total_price_per_mtok"`
	SourceModelID       string   `json:"source_model_id,omitempty"`
	Available           bool     `json:"available"`
}

type ModelMarketplaceItem struct {
	ModelID       string                `json:"model_id"`
	DisplayName   string                `json:"display_name"`
	Platforms     []string              `json:"platforms"`
	AccountCount  int                   `json:"account_count"`
	UpstreamNames []string              `json:"upstream_names,omitempty"`
	Price         ModelMarketplacePrice `json:"price"`
}

type ModelMarketplaceResponse struct {
	Models        []ModelMarketplaceItem `json:"models"`
	TotalModels   int                    `json:"total_models"`
	TotalAccounts int                    `json:"total_accounts"`
}

func NewModelMarketplaceService(accountRepo AccountRepository, billingService *BillingService) *ModelMarketplaceService {
	return &ModelMarketplaceService{accountRepo: accountRepo, billingService: billingService}
}

func (s *ModelMarketplaceService) ListModels(ctx context.Context) (*ModelMarketplaceResponse, error) {
	if s == nil || s.accountRepo == nil {
		return &ModelMarketplaceResponse{Models: []ModelMarketplaceItem{}}, nil
	}
	accounts, err := s.accountRepo.ListSchedulable(ctx)
	if err != nil {
		return nil, err
	}

	byModel := make(map[string]*modelMarketplaceAccumulator)
	for i := range accounts {
		acc := &accounts[i]
		for _, model := range supportedMarketplaceModels(acc) {
			if model.modelID == "" {
				continue
			}
			item := byModel[model.modelID]
			if item == nil {
				item = &modelMarketplaceAccumulator{
					modelID:     model.modelID,
					displayName: model.displayName,
					platforms:   map[string]struct{}{},
					accounts:    map[int64]struct{}{},
					upstreams:   map[string]struct{}{},
				}
				byModel[model.modelID] = item
			}
			item.platforms[acc.Platform] = struct{}{}
			item.accounts[acc.ID] = struct{}{}
			if model.upstreamModel != "" && model.upstreamModel != model.modelID {
				item.upstreams[model.upstreamModel] = struct{}{}
			}
			if item.displayName == "" {
				item.displayName = model.displayName
			}
		}
	}

	models := make([]ModelMarketplaceItem, 0, len(byModel))
	for _, acc := range byModel {
		models = append(models, ModelMarketplaceItem{
			ModelID:       acc.modelID,
			DisplayName:   displayNameOrID(acc.displayName, acc.modelID),
			Platforms:     sortedMarketplaceKeys(acc.platforms),
			AccountCount:  len(acc.accounts),
			UpstreamNames: sortedMarketplaceKeys(acc.upstreams),
			Price:         s.priceForModel(acc.modelID, sortedMarketplaceKeys(acc.upstreams)),
		})
	}
	sort.Slice(models, func(i, j int) bool {
		leftPlatform := firstMarketplacePlatform(models[i].Platforms)
		rightPlatform := firstMarketplacePlatform(models[j].Platforms)
		if leftPlatform != rightPlatform {
			return leftPlatform < rightPlatform
		}
		return models[i].ModelID < models[j].ModelID
	})

	return &ModelMarketplaceResponse{
		Models:        models,
		TotalModels:   len(models),
		TotalAccounts: len(accounts),
	}, nil
}

func (s *ModelMarketplaceService) priceForModel(modelID string, upstreams []string) ModelMarketplacePrice {
	candidates := append([]string{modelID}, upstreams...)
	if normalized := claude.NormalizeModelID(modelID); normalized != modelID {
		candidates = append(candidates, normalized)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if s == nil || s.billingService == nil {
			break
		}
		pricing, sourceModelID := s.marketplacePricing(candidate)
		if pricing == nil {
			continue
		}
		input := pricing.InputPricePerToken
		output := pricing.OutputPricePerToken
		inputMTok := input * 1_000_000
		outputMTok := output * 1_000_000
		totalMTok := inputMTok + outputMTok
		return ModelMarketplacePrice{
			InputPricePerToken:  &input,
			OutputPricePerToken: &output,
			InputPricePerMTok:   &inputMTok,
			OutputPricePerMTok:  &outputMTok,
			TotalPricePerMTok:   &totalMTok,
			SourceModelID:       sourceModelID,
			Available:           true,
		}
	}
	return ModelMarketplacePrice{Available: false}
}

func (s *ModelMarketplaceService) marketplacePricing(modelName string) (*ModelPricing, string) {
	if s == nil || s.billingService == nil {
		return nil, ""
	}
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	if modelLower == "" {
		return nil, ""
	}

	if pricing, source := exactPricingFromService(s.billingService.pricingService, modelLower); pricing != nil {
		return s.billingService.applyModelSpecificPricingPolicy(modelLower, pricing), source
	}

	if pricing := s.billingService.getFallbackPricing(modelLower); pricing != nil {
		return s.billingService.applyModelSpecificPricingPolicy(modelLower, pricing), modelLower
	}

	return nil, ""
}

func exactPricingFromService(pricingService *PricingService, modelName string) (*ModelPricing, string) {
	if pricingService == nil {
		return nil, ""
	}
	lookupCandidates := pricingService.buildModelLookupCandidates(modelName)
	pricingService.mu.RLock()
	defer pricingService.mu.RUnlock()

	for _, candidate := range lookupCandidates {
		if pricing, ok := pricingService.pricingData[candidate]; ok {
			return modelPricingFromLiteLLM(pricing), candidate
		}
	}

	for _, candidate := range lookupCandidates {
		normalized := strings.ReplaceAll(candidate, "-4-5-", "-4.5-")
		if pricing, ok := pricingService.pricingData[normalized]; ok {
			return modelPricingFromLiteLLM(pricing), normalized
		}
	}

	return nil, ""
}

func modelPricingFromLiteLLM(pricing *LiteLLMModelPricing) *ModelPricing {
	if pricing == nil {
		return nil
	}
	price5m := pricing.CacheCreationInputTokenCost
	price1h := pricing.CacheCreationInputTokenCostAbove1hr
	return &ModelPricing{
		InputPricePerToken:             pricing.InputCostPerToken,
		InputPricePerTokenPriority:     pricing.InputCostPerTokenPriority,
		OutputPricePerToken:            pricing.OutputCostPerToken,
		OutputPricePerTokenPriority:    pricing.OutputCostPerTokenPriority,
		CacheCreationPricePerToken:     pricing.CacheCreationInputTokenCost,
		CacheReadPricePerToken:         pricing.CacheReadInputTokenCost,
		CacheReadPricePerTokenPriority: pricing.CacheReadInputTokenCostPriority,
		CacheCreation5mPrice:           price5m,
		CacheCreation1hPrice:           price1h,
		SupportsCacheBreakdown:         price1h > 0 && price1h > price5m,
		LongContextInputThreshold:      pricing.LongContextInputTokenThreshold,
		LongContextInputMultiplier:     pricing.LongContextInputCostMultiplier,
		LongContextOutputMultiplier:    pricing.LongContextOutputCostMultiplier,
	}
}

type modelMarketplaceAccumulator struct {
	modelID     string
	displayName string
	platforms   map[string]struct{}
	accounts    map[int64]struct{}
	upstreams   map[string]struct{}
}

type marketplaceModelCandidate struct {
	modelID       string
	displayName   string
	upstreamModel string
}

func supportedMarketplaceModels(account *Account) []marketplaceModelCandidate {
	if account == nil {
		return nil
	}
	switch account.Platform {
	case PlatformOpenAI:
		if account.IsOpenAIPassthroughEnabled() {
			return openAIMarketplaceModels(nil)
		}
		return modelsFromMappingOrDefault(account.GetModelMapping(), openAIMarketplaceModels(nil))
	case PlatformGemini:
		if account.IsOAuth() {
			return geminiMarketplaceModels(nil)
		}
		return modelsFromMappingOrDefault(account.GetModelMapping(), geminiMarketplaceModels(nil))
	case PlatformAntigravity:
		return modelsFromMappingOrDefault(account.GetModelMapping(), antigravityMarketplaceModels(nil))
	case PlatformSora:
		return openAIModelsToMarketplace(DefaultSoraModels(nil), nil)
	case PlatformAnthropic:
		if account.IsOAuth() {
			return claudeMarketplaceModels(nil)
		}
		return modelsFromMappingOrDefault(account.GetModelMapping(), claudeMarketplaceModels(nil))
	default:
		if account.IsBedrock() {
			return modelsFromMappingOrDefault(account.GetModelMapping(), mappingKeysToMarketplace(domain.DefaultBedrockModelMapping, claudeMarketplaceModels(nil)))
		}
		return modelsFromMappingOrDefault(account.GetModelMapping(), nil)
	}
}

func modelsFromMappingOrDefault(mapping map[string]string, defaults []marketplaceModelCandidate) []marketplaceModelCandidate {
	if len(mapping) == 0 {
		return defaults
	}
	defaultByID := make(map[string]marketplaceModelCandidate, len(defaults))
	for _, model := range defaults {
		defaultByID[model.modelID] = model
	}
	outByID := make(map[string]marketplaceModelCandidate)
	for pattern, upstream := range mapping {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || pattern == "*" {
			continue
		}
		if strings.Contains(pattern, "*") {
			for _, model := range defaults {
				if matchWildcard(pattern, model.modelID) {
					candidate := model
					if strings.TrimSpace(upstream) != "" {
						candidate.upstreamModel = upstream
					}
					outByID[candidate.modelID] = candidate
				}
			}
			continue
		}
		candidate := marketplaceModelCandidate{modelID: pattern, displayName: pattern, upstreamModel: strings.TrimSpace(upstream)}
		if known, ok := defaultByID[pattern]; ok {
			candidate.displayName = known.displayName
			if candidate.upstreamModel == "" {
				candidate.upstreamModel = known.upstreamModel
			}
		}
		outByID[pattern] = candidate
	}
	out := make([]marketplaceModelCandidate, 0, len(outByID))
	for _, model := range outByID {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modelID < out[j].modelID })
	return out
}

func openAIMarketplaceModels(_ map[string]string) []marketplaceModelCandidate {
	return openAIModelsToMarketplace(openai.DefaultModels, nil)
}

func claudeMarketplaceModels(_ map[string]string) []marketplaceModelCandidate {
	out := make([]marketplaceModelCandidate, 0, len(claude.DefaultModels))
	for _, model := range claude.DefaultModels {
		out = append(out, marketplaceModelCandidate{modelID: model.ID, displayName: model.DisplayName})
	}
	return out
}

func geminiMarketplaceModels(_ map[string]string) []marketplaceModelCandidate {
	out := make([]marketplaceModelCandidate, 0, len(geminicli.DefaultModels))
	for _, model := range geminicli.DefaultModels {
		out = append(out, marketplaceModelCandidate{modelID: model.ID, displayName: model.DisplayName})
	}
	return out
}

func antigravityMarketplaceModels(_ map[string]string) []marketplaceModelCandidate {
	models := antigravity.DefaultModels()
	out := make([]marketplaceModelCandidate, 0, len(models))
	for _, model := range models {
		out = append(out, marketplaceModelCandidate{modelID: model.ID, displayName: model.DisplayName})
	}
	return out
}

func openAIModelsToMarketplace(models []openai.Model, upstreams map[string]string) []marketplaceModelCandidate {
	out := make([]marketplaceModelCandidate, 0, len(models))
	for _, model := range models {
		out = append(out, marketplaceModelCandidate{
			modelID:       model.ID,
			displayName:   model.DisplayName,
			upstreamModel: upstreams[model.ID],
		})
	}
	return out
}

func mappingKeysToMarketplace(mapping map[string]string, defaults []marketplaceModelCandidate) []marketplaceModelCandidate {
	return modelsFromMappingOrDefault(mapping, defaults)
}

func sortedMarketplaceKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func firstMarketplacePlatform(platforms []string) string {
	if len(platforms) == 0 {
		return ""
	}
	return platforms[0]
}

func displayNameOrID(displayName, id string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return id
}
