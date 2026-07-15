package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func upstreamModelSyncTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestBuildV1ModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1/models"))
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", buildV1ModelsURL("https://gateway.example.com/antigravity/"))
}

func TestBuildGeminiModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta/models"))
}

func TestExtractUpstreamModelIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "openai and anthropic data array",
			body: `{"data":[{"id":"claude-sonnet-4-5"},{"id":"gpt-5"},{"id":"gpt-5"},{"id":""}]}`,
			want: []string{"claude-sonnet-4-5", "gpt-5"},
		},
		{
			name: "gemini models array strips prefix",
			body: `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"gemini-2.5-flash"}]}`,
			want: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
		},
		{
			name: "top level array",
			body: `[{"id":"z-model"},{"name":"models/a-model"}]`,
			want: []string{"a-model", "z-model"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractUpstreamModelIDs([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildUpstreamModelsRequestsForAPIKeyAccounts(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	ctx := context.Background()

	anthropicReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://anthropic.example.com/v1/models", anthropicReq.URL.String())
	require.Equal(t, "anthropic-key", anthropicReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicReq.Header.Get("anthropic-version"))

	openAIReq, err := svc.buildOpenAIUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://openai.example.com/v1/models", openAIReq.URL.String())
	require.Equal(t, "Bearer openai-key", openAIReq.Header.Get("Authorization"))

	geminiReq, err := svc.buildGeminiUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com/v1beta",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", geminiReq.URL.String())
	require.Equal(t, "gemini-key", geminiReq.Header.Get("x-goog-api-key"))

	antigravityReq, err := svc.buildAntigravityAPIKeyModelsRequest(ctx, &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://gateway.example.com/antigravity",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", antigravityReq.URL.String())
	require.Equal(t, "antigravity-key", antigravityReq.Header.Get("x-api-key"))
}

func TestBuildAntigravityAPIKeyModelsRequestRejectsOfficialCloudCodeBase(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAntigravityAPIKeyModelsRequest(context.Background(), &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://cloudcode-pa.googleapis.com",
		},
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "compatible gateway")
}

func TestBuildAnthropicUpstreamModelsRequestRejectsBedrock(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAnthropicUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
}

func TestFetchUpstreamSupportedModelsParsesOpenAIResponse(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"},{"id":"gpt-5"},{"name":"o3"}]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5", "o3"}, models)
	require.Equal(t, "https://openai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer openai-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsDoesNotExposeUpstreamBody(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"SECRET_TOKEN should not be exposed"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	_, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET_TOKEN")

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUpstream, syncErr.Kind)
	require.NotContains(t, syncErr.SafeMessage(), "SECRET_TOKEN")
	require.Contains(t, syncErr.SafeMessage(), "HTTP 502")
}

func TestMergeUpstreamModelsIntoCredentialsPreservesAliases(t *testing.T) {
	t.Parallel()

	originalMapping := map[string]any{
		"claude-alias":      "claude-sonnet-4-6",
		"claude-sonnet-4-6": "custom-upstream-model",
		"glm-5":             nil,
	}
	original := map[string]any{
		"api_key":       "secret",
		"model_mapping": originalMapping,
	}

	merged, added := MergeUpstreamModelsIntoCredentials(original, []string{
		" claude-sonnet-4-6 ",
		"models/claude-sonnet-4-6-thinking",
		"claude-sonnet-4-6-thinking",
		"glm-5",
		"",
	})

	require.Equal(t, []string{"claude-sonnet-4-6-thinking", "glm-5"}, added)
	require.Equal(t, "secret", merged["api_key"])
	require.Equal(t, map[string]any{
		"claude-alias":               "claude-sonnet-4-6",
		"claude-sonnet-4-6":          "custom-upstream-model",
		"claude-sonnet-4-6-thinking": "claude-sonnet-4-6-thinking",
		"glm-5":                      "glm-5",
	}, merged["model_mapping"])

	// The caller may still be using the account snapshot concurrently.
	require.Equal(t, map[string]any{
		"claude-alias":      "claude-sonnet-4-6",
		"claude-sonnet-4-6": "custom-upstream-model",
		"glm-5":             nil,
	}, originalMapping)
}

func TestMergeUpstreamModelsIntoCredentialsNoopWhenAllModelsExist(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"model_mapping": map[string]string{
			"claude-sonnet-4-6": "custom-target",
		},
	}

	merged, added := MergeUpstreamModelsIntoCredentials(original, []string{"claude-sonnet-4-6"})

	require.Empty(t, added)
	require.Equal(t, original, merged)
}

type upstreamModelsAccountRepo struct {
	AccountRepository
	account            *Account
	getErr             error
	updateErr          error
	updatedID          int64
	updatedCredentials map[string]any
	mergedModelMapping map[string]string
}

func (r *upstreamModelsAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, r.getErr
}

func (r *upstreamModelsAccountRepo) UpdateCredentials(_ context.Context, id int64, credentials map[string]any) error {
	r.updatedID = id
	r.updatedCredentials = cloneCredentials(credentials)
	return r.updateErr
}

func (r *upstreamModelsAccountRepo) MergeModelMapping(_ context.Context, id int64, modelMapping map[string]string) error {
	r.updatedID = id
	r.mergedModelMapping = make(map[string]string, len(modelMapping))
	for model, target := range modelMapping {
		r.mergedModelMapping[model] = target
	}
	return r.updateErr
}

func TestSyncUpstreamModelMappingPersistsOnlyMissingModels(t *testing.T) {
	t.Parallel()

	repo := &upstreamModelsAccountRepo{account: &Account{
		ID:       2412,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com",
			"model_mapping": map[string]any{
				"claude-alias":    "claude-sonnet-4-6",
				"claude-opus-4-6": "claude-opus-4-6",
			},
		},
	}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"id":"claude-opus-4-6"},
			{"id":"claude-opus-4-6-thinking"},
			{"id":"glm-5"}
		]}`)),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, added, err := svc.SyncUpstreamModelMapping(context.Background(), 2412)

	require.NoError(t, err)
	require.Equal(t, []string{"claude-opus-4-6", "claude-opus-4-6-thinking", "glm-5"}, models)
	require.Equal(t, []string{"claude-opus-4-6-thinking", "glm-5"}, added)
	require.Equal(t, int64(2412), repo.updatedID)
	require.Equal(t, map[string]string{
		"claude-opus-4-6-thinking": "claude-opus-4-6-thinking",
		"glm-5":                    "glm-5",
	}, repo.mergedModelMapping)
	require.Nil(t, repo.updatedCredentials)
	require.Equal(t, "https://anthropic.example.com/v1/models", upstream.lastReq.URL.String())
}

func TestSyncUpstreamModelMappingSkipsPersistenceWhenUnchanged(t *testing.T) {
	t.Parallel()

	repo := &upstreamModelsAccountRepo{account: &Account{
		ID:       2413,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":       "anthropic-key",
			"base_url":      "https://anthropic.example.com",
			"model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"},
		},
	}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"claude-opus-4-6"}]}`)),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, added, err := svc.SyncUpstreamModelMapping(context.Background(), 2413)

	require.NoError(t, err)
	require.Equal(t, []string{"claude-opus-4-6"}, models)
	require.Empty(t, added)
	require.Zero(t, repo.updatedID)
	require.Nil(t, repo.updatedCredentials)
	require.Nil(t, repo.mergedModelMapping)
}

func TestSyncUpstreamModelMappingRejectsUnsupportedAccount(t *testing.T) {
	t.Parallel()

	repo := &upstreamModelsAccountRepo{account: &Account{
		ID:       10,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}}
	svc := &AccountTestService{accountRepo: repo}

	_, _, err := svc.SyncUpstreamModelMapping(context.Background(), 10)

	require.Error(t, err)
	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
	require.Zero(t, repo.updatedID)
}
