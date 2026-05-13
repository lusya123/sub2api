package service

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

func validateModelMarketplaceProvider(p string) error {
	if !isModelMarketplaceSupportedProvider(p) {
		return ErrModelMarketplaceMonitorInvalidProvider
	}
	return nil
}

func validateModelMarketplaceInterval(sec int) error {
	if sec < modelMarketplaceMinIntervalSeconds || sec > modelMarketplaceMaxIntervalSeconds {
		return ErrModelMarketplaceMonitorInvalidInterval
	}
	return nil
}

func validateModelMarketplaceEffectiveRate(rate *float64) error {
	if rate == nil {
		return nil
	}
	if *rate <= 0 {
		return ErrModelMarketplaceMonitorInvalidEffectiveRate
	}
	return nil
}

func normalizeModelMarketplaceEffectiveRate(rate *float64) float64 {
	if rate == nil || *rate <= 0 {
		return 1
	}
	return *rate
}

func validateModelMarketplaceEndpoint(ep string) error {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return ErrModelMarketplaceMonitorInvalidEndpoint
	}
	u, err := url.Parse(ep)
	if err != nil {
		return ErrModelMarketplaceMonitorInvalidEndpoint
	}
	if u.Scheme != "https" {
		return ErrModelMarketplaceMonitorEndpointScheme
	}
	if u.Host == "" {
		return ErrModelMarketplaceMonitorInvalidEndpoint
	}
	if u.Path != "" && u.Path != "/" {
		return ErrModelMarketplaceMonitorEndpointPath
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return ErrModelMarketplaceMonitorEndpointPath
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelMarketplaceEndpointResolveTimeout)
	defer cancel()
	blocked, err := isModelMarketplacePrivateOrLoopbackHost(ctx, u.Hostname())
	if err != nil {
		return ErrModelMarketplaceMonitorEndpointUnreachable
	}
	if blocked {
		return ErrModelMarketplaceMonitorEndpointPrivate
	}
	return nil
}

func normalizeModelMarketplaceEndpoint(ep string) string {
	ep = strings.TrimSpace(ep)
	return strings.TrimRight(ep, "/")
}

func normalizeModelMarketplaceModels(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

func normalizeModelMarketplaceDisplayNames(in map[string]ModelMarketplaceModelDisplayName) map[string]ModelMarketplaceModelDisplayName {
	if len(in) == 0 {
		return map[string]ModelMarketplaceModelDisplayName{}
	}
	out := make(map[string]ModelMarketplaceModelDisplayName, len(in))
	for model, names := range in {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		item := ModelMarketplaceModelDisplayName{
			Zh: strings.TrimSpace(names.Zh),
			En: strings.TrimSpace(names.En),
		}
		if item.Zh == "" && item.En == "" {
			continue
		}
		out[model] = item
	}
	return out
}

func normalizeModelMarketplaceCallConfigs(in map[string]ModelMarketplaceModelCallConfig) map[string]ModelMarketplaceModelCallConfig {
	if len(in) == 0 {
		return map[string]ModelMarketplaceModelCallConfig{}
	}
	out := make(map[string]ModelMarketplaceModelCallConfig, len(in))
	for model, cfg := range in {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		item := ModelMarketplaceModelCallConfig{
			Model:      strings.TrimSpace(cfg.Model),
			RequestURL: strings.TrimSpace(cfg.RequestURL),
			Pricing:    normalizeModelMarketplacePricingOverride(cfg.Pricing),
		}
		if item.Model == "" && item.RequestURL == "" && item.Pricing == nil {
			continue
		}
		out[model] = item
	}
	return out
}

func validateModelMarketplaceCallConfigs(in map[string]ModelMarketplaceModelCallConfig) error {
	for _, cfg := range in {
		if err := validateModelMarketplaceRequestURL(cfg.RequestURL); err != nil {
			return err
		}
		if err := validateModelMarketplacePricingOverride(cfg.Pricing); err != nil {
			return err
		}
	}
	return nil
}

func normalizeModelMarketplacePricingOverride(in *ModelMarketplaceModelPricingOverride) *ModelMarketplaceModelPricingOverride {
	if in == nil {
		return nil
	}
	out := &ModelMarketplaceModelPricingOverride{}
	if in.InputPricePerMillion != nil {
		v := *in.InputPricePerMillion
		out.InputPricePerMillion = &v
	}
	if in.OutputPricePerMillion != nil {
		v := *in.OutputPricePerMillion
		out.OutputPricePerMillion = &v
	}
	if out.InputPricePerMillion == nil && out.OutputPricePerMillion == nil {
		return nil
	}
	return out
}

func validateModelMarketplacePricingOverride(in *ModelMarketplaceModelPricingOverride) error {
	if in == nil {
		return nil
	}
	if in.InputPricePerMillion != nil && *in.InputPricePerMillion < 0 {
		return ErrModelMarketplaceMonitorInvalidPricing
	}
	if in.OutputPricePerMillion != nil && *in.OutputPricePerMillion < 0 {
		return ErrModelMarketplaceMonitorInvalidPricing
	}
	return nil
}

func validateModelMarketplaceRequestURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ErrModelMarketplaceMonitorInvalidEndpoint
	}
	if u.Scheme != "https" {
		return ErrModelMarketplaceMonitorEndpointScheme
	}
	if u.Host == "" {
		return ErrModelMarketplaceMonitorInvalidEndpoint
	}
	if u.Path == "" || u.Path == "/" || u.Fragment != "" {
		return ErrModelMarketplaceMonitorEndpointPath
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelMarketplaceEndpointResolveTimeout)
	defer cancel()
	blocked, err := isModelMarketplacePrivateOrLoopbackHost(ctx, u.Hostname())
	if err != nil {
		return ErrModelMarketplaceMonitorEndpointUnreachable
	}
	if blocked {
		return ErrModelMarketplaceMonitorEndpointPrivate
	}
	return nil
}

func validateModelMarketplaceBodyModeParams(mode string, body map[string]any) error {
	switch mode {
	case "", ModelMarketplaceBodyOverrideModeOff:
		return nil
	case ModelMarketplaceBodyOverrideModeMerge, ModelMarketplaceBodyOverrideModeReplace:
		if len(body) == 0 {
			return ErrModelMarketplaceTemplateBodyRequired
		}
		return nil
	default:
		return ErrModelMarketplaceTemplateInvalidBodyMode
	}
}

var modelMarketplaceHeaderNameRegex = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+\-.^_` + "`" + `|~]+$`)

var modelMarketplaceForbiddenHeaderNames = map[string]bool{
	"host":              true,
	"content-length":    true,
	"content-encoding":  true,
	"transfer-encoding": true,
	"connection":        true,
}

func isForbiddenModelMarketplaceHeaderName(name string) bool {
	return modelMarketplaceForbiddenHeaderNames[strings.ToLower(strings.TrimSpace(name))]
}

func validateModelMarketplaceExtraHeaders(h map[string]string) error {
	for k := range h {
		if !modelMarketplaceHeaderNameRegex.MatchString(k) {
			return ErrModelMarketplaceTemplateHeaderInvalidName
		}
		if isForbiddenModelMarketplaceHeaderName(k) {
			return ErrModelMarketplaceTemplateHeaderForbidden
		}
	}
	return nil
}

func emptyModelMarketplaceHeadersIfNil(h map[string]string) map[string]string {
	if h == nil {
		return map[string]string{}
	}
	return h
}

func defaultModelMarketplaceBodyMode(mode string) string {
	if mode == "" {
		return ModelMarketplaceBodyOverrideModeOff
	}
	return mode
}
