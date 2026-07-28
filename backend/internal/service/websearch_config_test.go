//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/stretchr/testify/require"
)

func expireWebSearchEmulationCacheForTest() {
	webSearchEmulationCacheMu.Lock()
	webSearchEmulationCacheGeneration++
	webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{expiresAt: 0})
	webSearchEmulationCacheMu.Unlock()
	webSearchEmulationSF.Forget(sfKeyWebSearchConfig)
}

type webSearchConfigRepoStub struct {
	mu         sync.Mutex
	value      string
	getCalls   int
	setCalls   int
	getValueFn func(call int, current string) (string, error)
}

func (r *webSearchConfigRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (r *webSearchConfigRepoStub) GetValue(_ context.Context, _ string) (string, error) {
	r.mu.Lock()
	r.getCalls++
	call := r.getCalls
	current := r.value
	fn := r.getValueFn
	r.mu.Unlock()
	if fn != nil {
		return fn(call, current)
	}
	if current == "" {
		return "", ErrSettingNotFound
	}
	return current, nil
}

func (r *webSearchConfigRepoStub) Set(_ context.Context, _ string, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setCalls++
	r.value = value
	return nil
}

func (r *webSearchConfigRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (r *webSearchConfigRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (r *webSearchConfigRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (r *webSearchConfigRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (r *webSearchConfigRepoStub) setCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setCalls
}

func (r *webSearchConfigRepoStub) getCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getCalls
}

func TestGetWebSearchEmulationConfig_MissingSettingUsesDisabledDefault(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	svc := &SettingService{settingRepo: &settingRepoStub{values: map[string]string{}}}
	cfg, err := svc.GetWebSearchEmulationConfig(context.Background())

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}

func TestGetWebSearchEmulationConfig_RepositoryErrorStillFails(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repoErr := errors.New("database unavailable")
	svc := &SettingService{settingRepo: &settingRepoStub{err: repoErr}}
	cfg, err := svc.GetWebSearchEmulationConfig(context.Background())

	require.ErrorIs(t, err, repoErr)
	require.NotNil(t, cfg)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}

func TestGetWebSearchEmulationConfig_CachedRepositoryErrorRemainsAnError(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repoErr := errors.New("database unavailable")
	repo := &webSearchConfigRepoStub{
		getValueFn: func(int, string) (string, error) {
			return "", repoErr
		},
	}
	svc := &SettingService{settingRepo: repo}

	first, firstErr := svc.GetWebSearchEmulationConfig(context.Background())
	second, secondErr := svc.GetWebSearchEmulationConfig(context.Background())

	require.ErrorIs(t, firstErr, repoErr)
	require.ErrorIs(t, secondErr, repoErr)
	require.NotNil(t, first)
	require.NotNil(t, second)
	require.False(t, first.Enabled)
	require.False(t, second.Enabled)
	require.Equal(t, 1, repo.getCallCount(), "the cached error should avoid a second repository read")
}

func TestSaveWebSearchEmulationConfig_EmptyAPIKeyPreservesExistingSecret(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repo := newMockSettingRepo()
	repo.data[SettingKeyWebSearchEmulationConfig] = `{"enabled":true,"providers":[{"type":"brave","api_key":"brave-secret","quota_limit":1000,"proxy_id":null}]}`
	svc := &SettingService{settingRepo: repo}
	incoming := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: websearch.ProviderTypeBrave, QuotaLimit: int64Ptr(1000)},
		},
	}

	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), incoming))

	storedRaw, err := repo.GetValue(context.Background(), SettingKeyWebSearchEmulationConfig)
	require.NoError(t, err)
	stored := parseWebSearchConfigJSON(storedRaw)
	require.Len(t, stored.Providers, 1)
	require.Equal(t, "brave-secret", stored.Providers[0].APIKey)

	responseCfg := SanitizeWebSearchConfig(context.Background(), stored)
	require.Empty(t, responseCfg.Providers[0].APIKey)
	require.True(t, responseCfg.Providers[0].APIKeyConfigured)
}

func TestSaveWebSearchEmulationConfig_ExistingConfigReadErrorDoesNotWrite(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repoErr := errors.New("database read unavailable")
	repo := &webSearchConfigRepoStub{
		getValueFn: func(int, string) (string, error) {
			return "", repoErr
		},
	}
	svc := &SettingService{settingRepo: repo}
	cfg := &WebSearchEmulationConfig{
		Enabled: false,
		Providers: []WebSearchProviderConfig{
			{Type: websearch.ProviderTypeBrave},
		},
	}

	err := svc.SaveWebSearchEmulationConfig(context.Background(), cfg)

	require.ErrorIs(t, err, repoErr)
	require.Zero(t, repo.setCallCount(), "a failed secret-preservation read must not be followed by Set")
}

func TestSaveWebSearchEmulationConfig_MissingExistingConfigAllowsFirstSave(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repo := &webSearchConfigRepoStub{
		getValueFn: func(int, string) (string, error) {
			return "", ErrSettingNotFound
		},
	}
	svc := &SettingService{settingRepo: repo}
	cfg := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: websearch.ProviderTypeBrave, APIKey: "first-secret"},
		},
	}

	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), cfg))
	require.Equal(t, 1, repo.setCallCount())
}

func TestWebSearchEmulationConfig_InFlightOldLoadCannotOverwriteSave(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	firstLoadStarted := make(chan struct{})
	releaseFirstLoad := make(chan struct{})
	repo := &webSearchConfigRepoStub{
		value: `{"enabled":true,"providers":[{"type":"brave","api_key":"old-secret"}]}`,
		getValueFn: func(call int, current string) (string, error) {
			if call == 1 {
				close(firstLoadStarted)
				<-releaseFirstLoad
			}
			return current, nil
		},
	}
	svc := &SettingService{settingRepo: repo}

	type loadResult struct {
		cfg *WebSearchEmulationConfig
		err error
	}
	loadDone := make(chan loadResult, 1)
	go func() {
		cfg, err := svc.GetWebSearchEmulationConfig(context.Background())
		loadDone <- loadResult{cfg: cfg, err: err}
	}()
	<-firstLoadStarted

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
			Enabled: true,
			Providers: []WebSearchProviderConfig{
				{Type: websearch.ProviderTypeBrave, APIKey: "new-secret"},
			},
		})
	}()

	select {
	case err := <-saveDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		close(releaseFirstLoad)
		t.Fatal("save was blocked by an unrelated in-flight config load")
	}
	close(releaseFirstLoad)

	oldLoad := <-loadDone
	require.NoError(t, oldLoad.err)
	require.Equal(t, "old-secret", oldLoad.cfg.Providers[0].APIKey)

	current, err := svc.GetWebSearchEmulationConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "new-secret", current.Providers[0].APIKey,
		"the old in-flight load must not replace the cache written by Save")
}

func TestWebSearchEmulationConfig_StaleSaveGenerationCannotInstallManager(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repo := &webSearchConfigRepoStub{}
	svc := &SettingService{settingRepo: repo}
	first := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: websearch.ProviderTypeBrave, APIKey: "first-secret"},
		},
	}
	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), first))
	webSearchEmulationCacheMu.Lock()
	firstGeneration := webSearchEmulationCacheGeneration
	webSearchEmulationCacheMu.Unlock()

	second := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: websearch.ProviderTypeBrave, APIKey: "second-secret"},
		},
	}
	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), second))
	webSearchEmulationCacheMu.Lock()
	secondGeneration := webSearchEmulationCacheGeneration
	webSearchEmulationCacheMu.Unlock()

	var built []string
	svc.webSearchManagerBuilder = func(cfg *WebSearchEmulationConfig, _ map[int64]string) {
		built = append(built, cfg.Providers[0].APIKey)
	}
	svc.rebuildWebSearchManagerSnapshot(context.Background(), firstGeneration, cloneWebSearchEmulationConfig(first))
	require.Empty(t, built, "a stale save generation must not invoke the manager builder")

	svc.rebuildWebSearchManagerSnapshot(context.Background(), secondGeneration, cloneWebSearchEmulationConfig(second))
	require.Equal(t, []string{"second-secret"}, built)
}

func TestWebSearchEmulationConfig_ConcurrentSavesInstallNewestManagerLast(t *testing.T) {
	expireWebSearchEmulationCacheForTest()
	t.Cleanup(expireWebSearchEmulationCacheForTest)

	repo := &webSearchConfigRepoStub{}
	svc := &SettingService{settingRepo: repo}
	firstBuilderEntered := make(chan struct{})
	releaseFirstBuilder := make(chan struct{})
	var builtMu sync.Mutex
	var built []string
	svc.webSearchManagerBuilder = func(cfg *WebSearchEmulationConfig, _ map[int64]string) {
		builtMu.Lock()
		call := len(built)
		built = append(built, cfg.Providers[0].APIKey)
		if call == 0 {
			close(firstBuilderEntered)
		}
		builtMu.Unlock()
		if call == 0 {
			<-releaseFirstBuilder
		}
	}

	firstSaveDone := make(chan error, 1)
	go func() {
		firstSaveDone <- svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
			Enabled: true,
			Providers: []WebSearchProviderConfig{
				{Type: websearch.ProviderTypeBrave, APIKey: "first-secret"},
			},
		})
	}()
	<-firstBuilderEntered

	// The builder is the manager-commit critical section. If it has begun, a
	// newer save must not persist until the older builder returns.
	if webSearchEmulationCacheMu.TryLock() {
		webSearchEmulationCacheMu.Unlock()
		close(releaseFirstBuilder)
		<-firstSaveDone
		t.Fatal("manager builder ran without holding the config commit mutex")
	}

	secondSaveDone := make(chan error, 1)
	go func() {
		secondSaveDone <- svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
			Enabled: true,
			Providers: []WebSearchProviderConfig{
				{Type: websearch.ProviderTypeBrave, APIKey: "second-secret"},
			},
		})
	}()
	close(releaseFirstBuilder)

	require.NoError(t, <-firstSaveDone)
	require.NoError(t, <-secondSaveDone)
	builtMu.Lock()
	builtSnapshot := append([]string(nil), built...)
	builtMu.Unlock()
	require.Equal(t, []string{"first-secret", "second-secret"}, builtSnapshot)

	current, err := svc.GetWebSearchEmulationConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "second-secret", current.Providers[0].APIKey)
}

// --- validateWebSearchConfig ---

func TestValidateWebSearchConfig_Nil(t *testing.T) {
	require.NoError(t, validateWebSearchConfig(nil))
}

func TestValidateWebSearchConfig_Valid(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: "brave", QuotaLimit: int64Ptr(1000)},
			{Type: "tavily", QuotaLimit: int64Ptr(500)},
		},
	}
	require.NoError(t, validateWebSearchConfig(cfg))
}

func TestValidateWebSearchConfig_TooManyProviders(t *testing.T) {
	cfg := &WebSearchEmulationConfig{Providers: make([]WebSearchProviderConfig, 11)}
	for i := range cfg.Providers {
		cfg.Providers[i] = WebSearchProviderConfig{Type: "brave"}
	}
	err := validateWebSearchConfig(cfg)
	require.ErrorContains(t, err, "too many providers")
}

func TestValidateWebSearchConfig_InvalidType(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "bing"}},
	}
	require.ErrorContains(t, validateWebSearchConfig(cfg), "invalid type")
}

func TestValidateWebSearchConfig_NegativeQuotaLimit(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", QuotaLimit: int64Ptr(-1)}},
	}
	require.ErrorContains(t, validateWebSearchConfig(cfg), "quota_limit must be > 0 or null")
}

func TestValidateWebSearchConfig_DuplicateType(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave"},
			{Type: "brave"},
		},
	}
	require.ErrorContains(t, validateWebSearchConfig(cfg), "duplicate type")
}

func TestValidateWebSearchConfig_NilQuotaLimit(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", QuotaLimit: nil}},
	}
	require.NoError(t, validateWebSearchConfig(cfg))
}

// --- parseWebSearchConfigJSON ---

func TestParseWebSearchConfigJSON_ValidJSON(t *testing.T) {
	raw := `{"enabled":true,"providers":[{"type":"brave","api_key":"sk-xxx"}]}`
	cfg := parseWebSearchConfigJSON(raw)
	require.True(t, cfg.Enabled)
	require.Len(t, cfg.Providers, 1)
	require.Equal(t, "brave", cfg.Providers[0].Type)
}

func TestParseWebSearchConfigJSON_EmptyString(t *testing.T) {
	cfg := parseWebSearchConfigJSON("")
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}

func TestParseWebSearchConfigJSON_InvalidJSON(t *testing.T) {
	cfg := parseWebSearchConfigJSON("not{json")
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}

func TestParseWebSearchConfigJSON_BackwardCompatibility(t *testing.T) {
	// Old config with priority and quota_refresh_interval should parse without error
	raw := `{"enabled":true,"providers":[{"type":"brave","priority":1,"quota_refresh_interval":"monthly","quota_limit":1000}]}`
	cfg := parseWebSearchConfigJSON(raw)
	require.True(t, cfg.Enabled)
	require.Len(t, cfg.Providers, 1)
	require.Equal(t, int64(1000), *cfg.Providers[0].QuotaLimit)
}

// --- SanitizeWebSearchConfig ---

func TestSanitizeWebSearchConfig_MaskAPIKey(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-secret-xxx"},
		},
	}
	out := SanitizeWebSearchConfig(context.Background(), cfg)
	require.Equal(t, "", out.Providers[0].APIKey)
	require.True(t, out.Providers[0].APIKeyConfigured)
}

func TestSanitizeWebSearchConfig_NoAPIKey(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: ""}},
	}
	out := SanitizeWebSearchConfig(context.Background(), cfg)
	require.Equal(t, "", out.Providers[0].APIKey)
	require.False(t, out.Providers[0].APIKeyConfigured)
}

func TestSanitizeWebSearchConfig_Nil(t *testing.T) {
	require.Nil(t, SanitizeWebSearchConfig(context.Background(), nil))
}

func TestSanitizeWebSearchConfig_PreservesOtherFields(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "secret", QuotaLimit: int64Ptr(1000)},
		},
	}
	out := SanitizeWebSearchConfig(context.Background(), cfg)
	require.True(t, out.Enabled)
	require.Equal(t, int64(1000), *out.Providers[0].QuotaLimit)
}

func TestSanitizeWebSearchConfig_DoesNotMutateOriginal(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "secret"}},
	}
	_ = SanitizeWebSearchConfig(context.Background(), cfg)
	require.Equal(t, "secret", cfg.Providers[0].APIKey)
}

// --- PopulateWebSearchUsage ---

func TestPopulateWebSearchUsage_NilInput(t *testing.T) {
	require.Nil(t, PopulateWebSearchUsage(context.Background(), nil))
}

func TestPopulateWebSearchUsage_NoManager_QuotaUsedZero(t *testing.T) {
	// Ensure no global manager is set
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Enabled: true,
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key", QuotaLimit: int64Ptr(1000)},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.NotNil(t, out)
	require.Len(t, out.Providers, 1)
	require.Equal(t, int64(0), out.Providers[0].QuotaUsed)
}

func TestPopulateWebSearchUsage_APIKeyConfigured_True(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key"},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.True(t, out.Providers[0].APIKeyConfigured)
}

func TestPopulateWebSearchUsage_APIKeyConfigured_False(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: ""},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.False(t, out.Providers[0].APIKeyConfigured)
}

func TestPopulateWebSearchUsage_NilQuotaLimit(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key", QuotaLimit: nil},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.Nil(t, out.Providers[0].QuotaLimit)
}

func TestPopulateWebSearchUsage_NonNilQuotaLimit(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key", QuotaLimit: int64Ptr(500)},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.NotNil(t, out.Providers[0].QuotaLimit)
	require.Equal(t, int64(500), *out.Providers[0].QuotaLimit)
}

func TestPopulateWebSearchUsage_WithManager_NilRedis(t *testing.T) {
	// Manager with nil Redis returns 0 usage without error
	mgr := websearch.NewManager([]websearch.ProviderConfig{
		{Type: "brave", APIKey: "k"},
	}, nil)
	SetWebSearchManager(mgr)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key", QuotaLimit: int64Ptr(1000)},
		},
	}
	out := PopulateWebSearchUsage(context.Background(), cfg)
	require.Equal(t, int64(0), out.Providers[0].QuotaUsed)
	require.True(t, out.Providers[0].APIKeyConfigured)
}

func TestPopulateWebSearchUsage_DoesNotMutateOriginal(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "secret", QuotaLimit: int64Ptr(100)},
		},
	}
	_ = PopulateWebSearchUsage(context.Background(), cfg)
	// Original should be unchanged
	require.Equal(t, "secret", cfg.Providers[0].APIKey)
	require.Equal(t, int64(0), cfg.Providers[0].QuotaUsed)
}

// --- ResetWebSearchUsage ---

func TestResetWebSearchUsage_NilManager(t *testing.T) {
	SetWebSearchManager(nil)
	defer SetWebSearchManager(nil)

	err := ResetWebSearchUsage(context.Background(), "brave")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not initialized")
}
