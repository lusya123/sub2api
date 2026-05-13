package service

import "testing"

func TestInferModelMarketplaceProtocolFromRequestURL(t *testing.T) {
	tests := []struct {
		name       string
		requestURL string
		want       string
	}{
		{
			name:       "anthropic messages",
			requestURL: "https://example.com/v1/messages",
			want:       modelMarketplaceProtocolAnthropic,
		},
		{
			name:       "openai compatible chat completions",
			requestURL: "https://example.com/v1/chat/completions",
			want:       modelMarketplaceProtocolOpenAICompatible,
		},
		{
			name:       "gemini generate content",
			requestURL: "https://example.com/v1beta/models/gemini-pro:generateContent",
			want:       modelMarketplaceProtocolGemini,
		},
		{
			name:       "zhipu v3",
			requestURL: "https://example.com/api/paas/v3/model-api/chatglm_turbo/invoke",
			want:       modelMarketplaceProtocolZhipu,
		},
		{
			name:       "zhipu v4 openai compatible",
			requestURL: "https://example.com/api/paas/v4/chat/completions",
			want:       modelMarketplaceProtocolOpenAICompatible,
		},
		{
			name:       "unknown",
			requestURL: "https://example.com/custom/invoke",
			want:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferModelMarketplaceProtocolFromRequestURL(tt.requestURL); got != tt.want {
				t.Fatalf("infer protocol = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModelMarketplaceAdapterForRequestKeepsDisplayProviderButInfersRequestProtocol(t *testing.T) {
	protocol, adapter, ok := modelMarketplaceAdapterForRequest(ModelMarketplaceProviderZhipuV4, &ModelMarketplaceCheckOptions{
		RequestURL: "https://example.com/v1/messages",
	})
	if !ok {
		t.Fatal("expected adapter for zhipu_v4 with custom request url")
	}
	if protocol != modelMarketplaceProtocolAnthropic {
		t.Fatalf("protocol = %q, want %q", protocol, modelMarketplaceProtocolAnthropic)
	}
	if got := adapter.buildPath("glm-5"); got != modelMarketplaceProviderAnthropicPath {
		t.Fatalf("adapter path = %q, want %q", got, modelMarketplaceProviderAnthropicPath)
	}
}

func TestModelMarketplaceCheckRequestURLUsesOverride(t *testing.T) {
	got := modelMarketplaceCheckRequestURL("https://example.com", "/api/paas/v4/chat/completions", &ModelMarketplaceCheckOptions{
		RequestURL: "https://gateway.example.com/v1/messages",
	})
	if got != "https://gateway.example.com/v1/messages" {
		t.Fatalf("request url = %q", got)
	}
}

func TestValidateModelMarketplaceRequestURL(t *testing.T) {
	tests := []struct {
		name       string
		requestURL string
		wantErr    error
	}{
		{
			name:       "empty is optional",
			requestURL: "",
		},
		{
			name:       "https path is accepted",
			requestURL: "https://example.com/v1/messages",
		},
		{
			name:       "http is rejected",
			requestURL: "http://example.com/v1/messages",
			wantErr:    ErrModelMarketplaceMonitorEndpointScheme,
		},
		{
			name:       "origin only is rejected",
			requestURL: "https://example.com",
			wantErr:    ErrModelMarketplaceMonitorEndpointPath,
		},
		{
			name:       "fragment is rejected",
			requestURL: "https://example.com/v1/messages#token",
			wantErr:    ErrModelMarketplaceMonitorEndpointPath,
		},
		{
			name:       "private host is rejected",
			requestURL: "https://127.0.0.1/v1/messages",
			wantErr:    ErrModelMarketplaceMonitorEndpointPrivate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModelMarketplaceRequestURL(tt.requestURL)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validate request url returned error: %v", err)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("validate request url error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
