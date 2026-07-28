package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"golang.org/x/sync/singleflight"
)

// WebSearchEmulationConfig holds the global web search emulation configuration.
type WebSearchEmulationConfig struct {
	Enabled   bool                      `json:"enabled"`
	Providers []WebSearchProviderConfig `json:"providers"`
}

// WebSearchProviderConfig describes a single search provider (Brave or Tavily).
type WebSearchProviderConfig struct {
	Type             string `json:"type"`                    // websearch.ProviderTypeBrave | Tavily
	APIKey           string `json:"api_key,omitempty"`       // secret — omitted in API responses
	APIKeyConfigured bool   `json:"api_key_configured"`      // read-only mask
	QuotaLimit       *int64 `json:"quota_limit"`             // nil = unlimited, >0 = limited
	SubscribedAt     *int64 `json:"subscribed_at,omitempty"` // subscription start (unix seconds); quota resets monthly
	QuotaUsed        int64  `json:"quota_used,omitempty"`    // read-only: current usage from Redis
	ProxyID          *int64 `json:"proxy_id"`                // optional proxy association
	ExpiresAt        *int64 `json:"expires_at,omitempty"`    // optional expiration timestamp
}

// --- Validation ---

const maxWebSearchProviders = 10

var validProviderTypes = map[string]bool{
	websearch.ProviderTypeBrave:  true,
	websearch.ProviderTypeTavily: true,
}

func validateWebSearchConfig(cfg *WebSearchEmulationConfig) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Providers) > maxWebSearchProviders {
		return fmt.Errorf("too many providers (max %d)", maxWebSearchProviders)
	}
	seen := make(map[string]bool, len(cfg.Providers))
	for i, p := range cfg.Providers {
		if !validProviderTypes[p.Type] {
			return fmt.Errorf("provider[%d]: invalid type %q", i, p.Type)
		}
		if p.QuotaLimit != nil && *p.QuotaLimit < 0 {
			return fmt.Errorf("provider[%d]: quota_limit must be > 0 or null", i)
		}
		if seen[p.Type] {
			return fmt.Errorf("provider[%d]: duplicate type %q", i, p.Type)
		}
		seen[p.Type] = true
	}
	return nil
}

// --- In-process cache (same pattern as gateway forwarding settings) ---

const sfKeyWebSearchConfig = "web_search_emulation_config"

type cachedWebSearchEmulationConfig struct {
	config    *WebSearchEmulationConfig
	err       error
	expiresAt int64 // unix nano
}

var webSearchEmulationCache atomic.Value // *cachedWebSearchEmulationConfig
var webSearchEmulationSF singleflight.Group

// webSearchEmulationCacheMu serializes config saves and protects cache stores
// against an older in-flight DB load. DB reads intentionally happen outside
// the lock; the generation check prevents their stale result from winning.
var webSearchEmulationCacheMu sync.Mutex
var webSearchEmulationCacheGeneration uint64

const (
	webSearchEmulationCacheTTL  = 60 * time.Second
	webSearchEmulationErrorTTL  = 5 * time.Second
	webSearchEmulationDBTimeout = 5 * time.Second
)

// GetWebSearchEmulationConfig returns the configuration with in-process cache + singleflight.
func (s *SettingService) GetWebSearchEmulationConfig(ctx context.Context) (*WebSearchEmulationConfig, error) {
	if cached := webSearchEmulationCache.Load(); cached != nil {
		if c, ok := cached.(*cachedWebSearchEmulationConfig); ok && c != nil && time.Now().UnixNano() < c.expiresAt {
			return c.config, c.err
		}
	}
	result, err, _ := webSearchEmulationSF.Do(sfKeyWebSearchConfig, func() (any, error) {
		return s.loadWebSearchConfigFromDB()
	})
	if err != nil {
		return &WebSearchEmulationConfig{}, err
	}
	if cfg, ok := result.(*WebSearchEmulationConfig); ok {
		return cfg, nil
	}
	return &WebSearchEmulationConfig{}, nil
}

func (s *SettingService) loadWebSearchConfigFromDB() (*WebSearchEmulationConfig, error) {
	webSearchEmulationCacheMu.Lock()
	generation := webSearchEmulationCacheGeneration
	webSearchEmulationCacheMu.Unlock()

	dbCtx, cancel := context.WithTimeout(context.Background(), webSearchEmulationDBTimeout)
	defer cancel()

	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyWebSearchEmulationConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			cfg := &WebSearchEmulationConfig{}
			storeWebSearchEmulationCacheIfCurrent(generation, cfg, nil, webSearchEmulationCacheTTL)
			return cfg, nil
		}
		storeWebSearchEmulationCacheIfCurrent(
			generation,
			&WebSearchEmulationConfig{},
			err,
			webSearchEmulationErrorTTL,
		)
		return &WebSearchEmulationConfig{}, err
	}
	cfg := parseWebSearchConfigJSON(raw)
	storeWebSearchEmulationCacheIfCurrent(generation, cfg, nil, webSearchEmulationCacheTTL)
	return cfg, nil
}

func storeWebSearchEmulationCacheIfCurrent(
	generation uint64,
	cfg *WebSearchEmulationConfig,
	cacheErr error,
	ttl time.Duration,
) {
	webSearchEmulationCacheMu.Lock()
	defer webSearchEmulationCacheMu.Unlock()
	if generation != webSearchEmulationCacheGeneration {
		return
	}
	webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
		config:    cfg,
		err:       cacheErr,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func parseWebSearchConfigJSON(raw string) *WebSearchEmulationConfig {
	cfg := &WebSearchEmulationConfig{}
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		slog.Warn("websearch: failed to parse config JSON", "error", err)
		return &WebSearchEmulationConfig{}
	}
	return cfg
}

// SaveWebSearchEmulationConfig validates and persists the configuration.
// Empty API keys in the input are preserved from the existing config.
func (s *SettingService) SaveWebSearchEmulationConfig(ctx context.Context, cfg *WebSearchEmulationConfig) error {
	if err := validateWebSearchConfig(cfg); err != nil {
		return infraerrors.BadRequest("INVALID_WEB_SEARCH_CONFIG", err.Error())
	}

	// Serialize the read-merge-write sequence. Besides preventing concurrent
	// saves from losing updates, the generation bump makes older in-flight
	// loads skip their cache store after this save commits.
	var savedGeneration uint64
	var savedConfig *WebSearchEmulationConfig
	webSearchEmulationCacheMu.Lock()
	err := func() error {
		if err := s.mergeExistingAPIKeys(ctx, cfg); err != nil {
			return err
		}

		// After merge, validate all enabled providers have API keys
		if cfg.Enabled {
			for _, p := range cfg.Providers {
				if p.APIKey == "" {
					return infraerrors.BadRequest("MISSING_API_KEY",
						fmt.Sprintf("provider %s has no API key configured", p.Type))
				}
			}
		}

		data, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("websearch: marshal config: %w", err)
		}
		if err := s.settingRepo.Set(ctx, SettingKeyWebSearchEmulationConfig, string(data)); err != nil {
			return fmt.Errorf("websearch: save config: %w", err)
		}

		savedConfig = cloneWebSearchEmulationConfig(cfg)
		webSearchEmulationCacheGeneration++
		savedGeneration = webSearchEmulationCacheGeneration
		webSearchEmulationSF.Forget(sfKeyWebSearchConfig)
		webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
			config:    savedConfig,
			expiresAt: time.Now().Add(webSearchEmulationCacheTTL).UnixNano(),
		})
		return nil
	}()
	webSearchEmulationCacheMu.Unlock()
	if err != nil {
		return err
	}

	// Hot-reload: rebuild the global Manager with new config
	s.rebuildWebSearchManagerSnapshot(ctx, savedGeneration, savedConfig)
	return nil
}

func cloneWebSearchEmulationConfig(cfg *WebSearchEmulationConfig) *WebSearchEmulationConfig {
	if cfg == nil {
		return nil
	}
	cloneInt64 := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		cloned := *value
		return &cloned
	}
	out := *cfg
	out.Providers = make([]WebSearchProviderConfig, len(cfg.Providers))
	for i, provider := range cfg.Providers {
		out.Providers[i] = provider
		out.Providers[i].QuotaLimit = cloneInt64(provider.QuotaLimit)
		out.Providers[i].SubscribedAt = cloneInt64(provider.SubscribedAt)
		out.Providers[i].ProxyID = cloneInt64(provider.ProxyID)
		out.Providers[i].ExpiresAt = cloneInt64(provider.ExpiresAt)
	}
	return &out
}

// mergeExistingAPIKeys preserves API keys from the current config when incoming value is empty.
func (s *SettingService) mergeExistingAPIKeys(ctx context.Context, cfg *WebSearchEmulationConfig) error {
	if cfg == nil {
		return nil
	}
	existing, err := s.getWebSearchEmulationConfigRaw(ctx)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil
		}
		return fmt.Errorf("websearch: load existing config before save: %w", err)
	}
	if existing == nil {
		return nil
	}
	existingByType := make(map[string]string, len(existing.Providers))
	for _, p := range existing.Providers {
		if p.APIKey != "" {
			existingByType[p.Type] = p.APIKey
		}
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].APIKey == "" {
			if key, ok := existingByType[cfg.Providers[i].Type]; ok {
				cfg.Providers[i].APIKey = key
			}
		}
	}
	return nil
}

func (s *SettingService) getWebSearchEmulationConfigRaw(ctx context.Context) (*WebSearchEmulationConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyWebSearchEmulationConfig)
	if err != nil {
		return nil, err
	}
	return parseWebSearchConfigJSON(raw), nil
}

// IsWebSearchEmulationEnabled is a quick check for whether the global switch is on.
func (s *SettingService) IsWebSearchEmulationEnabled(ctx context.Context) bool {
	cfg, err := s.GetWebSearchEmulationConfig(ctx)
	if err != nil {
		return false
	}
	return cfg.Enabled && len(cfg.Providers) > 0
}

// SetWebSearchManagerBuilder injects a callback that creates and wires a websearch.Manager.
// The infra layer (main/wire) provides this builder, keeping redis out of the service layer.
// Triggers initial build.
func (s *SettingService) SetWebSearchManagerBuilder(ctx context.Context, builder WebSearchManagerBuilder) {
	s.webSearchManagerBuilder = builder
	s.rebuildWebSearchManager(ctx)
}

// rebuildWebSearchManager reads the current config, resolves proxy URLs, and invokes the builder.
func (s *SettingService) rebuildWebSearchManager(ctx context.Context) {
	if s.webSearchManagerBuilder == nil {
		return
	}
	webSearchEmulationCacheMu.Lock()
	generation := webSearchEmulationCacheGeneration
	webSearchEmulationCacheMu.Unlock()
	cfg, err := s.GetWebSearchEmulationConfig(ctx)
	if err != nil {
		webSearchEmulationCacheMu.Lock()
		if generation == webSearchEmulationCacheGeneration {
			SetWebSearchManager(nil)
		}
		webSearchEmulationCacheMu.Unlock()
		return
	}
	s.rebuildWebSearchManagerSnapshot(ctx, generation, cloneWebSearchEmulationConfig(cfg))
}

func (s *SettingService) rebuildWebSearchManagerSnapshot(
	ctx context.Context,
	generation uint64,
	cfg *WebSearchEmulationConfig,
) {
	if s.webSearchManagerBuilder == nil {
		return
	}
	proxyURLs := s.resolveProviderProxyURLs(ctx, cfg)

	// Proxy resolution can be slow and runs outside the commit lock. The final
	// generation check prevents an older save from installing its manager after
	// a newer save. Holding the lock through builder invocation also means a
	// newer save waits if the older builder has already begun; it will then
	// persist and build last.
	webSearchEmulationCacheMu.Lock()
	defer webSearchEmulationCacheMu.Unlock()
	if generation != webSearchEmulationCacheGeneration || s.webSearchManagerBuilder == nil {
		return
	}
	s.webSearchManagerBuilder(cfg, proxyURLs)
}

// resolveProviderProxyURLs collects proxy IDs from providers and resolves them to URLs.
func (s *SettingService) resolveProviderProxyURLs(ctx context.Context, cfg *WebSearchEmulationConfig) map[int64]string {
	if cfg == nil || s.proxyRepo == nil {
		return nil
	}
	var ids []int64
	for _, p := range cfg.Providers {
		if p.ProxyID != nil && *p.ProxyID > 0 {
			ids = append(ids, *p.ProxyID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	proxies, err := s.proxyRepo.ListByIDs(ctx, ids)
	if err != nil {
		slog.Warn("websearch: failed to resolve proxy URLs", "error", err)
		return nil
	}
	result := make(map[int64]string, len(proxies))
	for _, px := range proxies {
		result[px.ID] = px.URL()
	}
	return result
}

// WebSearchTestResult holds the result of a search test.
type WebSearchTestResult struct {
	Provider string                   `json:"provider"`
	Results  []websearch.SearchResult `json:"results"`
	Query    string                   `json:"query"`
}

// TestWebSearch executes a test search using the currently configured Manager.
// Uses Manager.TestSearch which bypasses quota tracking.
const testSearchTimeout = 15 * time.Second

func TestWebSearch(ctx context.Context, query string) (*WebSearchTestResult, error) {
	mgr := getWebSearchManager()
	if mgr == nil {
		return nil, fmt.Errorf("web search: manager not initialized, save config first")
	}
	testCtx, cancel := context.WithTimeout(ctx, testSearchTimeout)
	defer cancel()
	resp, providerName, err := mgr.TestSearch(testCtx, websearch.SearchRequest{
		Query:      query,
		MaxResults: webSearchDefaultMaxResults,
	})
	if err != nil {
		return nil, err
	}
	return &WebSearchTestResult{
		Provider: providerName,
		Results:  resp.Results,
		Query:    resp.Query,
	}, nil
}

// PopulateWebSearchUsage returns a copy with quota usage populated from Redis (api_key kept as-is).
func PopulateWebSearchUsage(ctx context.Context, cfg *WebSearchEmulationConfig) *WebSearchEmulationConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.Providers = make([]WebSearchProviderConfig, len(cfg.Providers))

	mgr := getWebSearchManager()

	for i, p := range cfg.Providers {
		out.Providers[i] = p
		out.Providers[i].APIKeyConfigured = p.APIKey != ""

		if mgr != nil {
			used, _ := mgr.GetUsage(ctx, p.Type)
			out.Providers[i].QuotaUsed = used
		}
	}
	return &out
}

// ResetWebSearchUsage deletes the Redis quota key for the given provider type.
func ResetWebSearchUsage(ctx context.Context, providerType string) error {
	mgr := getWebSearchManager()
	if mgr == nil {
		return fmt.Errorf("web search manager not initialized")
	}
	return mgr.ResetUsage(ctx, providerType)
}

// SanitizeWebSearchConfig returns a copy with api_key fields masked and quota usage populated.
func SanitizeWebSearchConfig(ctx context.Context, cfg *WebSearchEmulationConfig) *WebSearchEmulationConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.Providers = make([]WebSearchProviderConfig, len(cfg.Providers))

	// Load usage from the global Manager (reads from Redis)
	mgr := getWebSearchManager()

	for i, p := range cfg.Providers {
		out.Providers[i] = p
		out.Providers[i].APIKeyConfigured = p.APIKey != ""
		out.Providers[i].APIKey = "" // never return the secret

		// Populate quota usage from Redis
		if mgr != nil {
			used, _ := mgr.GetUsage(ctx, p.Type)
			out.Providers[i].QuotaUsed = used
		}
	}
	return &out
}
