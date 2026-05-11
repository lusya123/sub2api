package service

import (
	"time"
)

const (
	modelMarketplaceRequestTimeout             = 45 * time.Second
	modelMarketplacePingTimeout                = 8 * time.Second
	modelMarketplaceDegradedThreshold          = 6 * time.Second
	modelMarketplaceWorkerConcurrency          = 5
	modelMarketplaceStartupLoadTimeout         = 10 * time.Second
	modelMarketplaceMinIntervalSeconds         = 15
	modelMarketplaceMaxIntervalSeconds         = 3600
	modelMarketplaceMessageMaxBytes            = 500
	modelMarketplaceResponseMaxBytes           = 64 * 1024
	modelMarketplaceErrorBodySnippetMaxBytes   = 300
	modelMarketplaceChallengeMin               = 1
	modelMarketplaceChallengeMax               = 50
	modelMarketplaceProviderOpenAIPath         = "/v1/chat/completions"
	modelMarketplaceProviderAnthropicPath      = "/v1/messages"
	modelMarketplaceProviderGeminiPathTemplate = "/v1beta/models/%s:generateContent"

	ModelMarketplaceProviderOpenAI    = "openai"
	ModelMarketplaceProviderAnthropic = "anthropic"
	ModelMarketplaceProviderGemini    = "gemini"

	ModelMarketplaceStatusOperational = "operational"
	ModelMarketplaceStatusDegraded    = "degraded"
	ModelMarketplaceStatusFailed      = "failed"
	ModelMarketplaceStatusError       = "error"

	modelMarketplaceAvailability7Days = 7

	ModelMarketplaceMonitorHistoryDefaultLimit = 100
	ModelMarketplaceMonitorHistoryMaxLimit     = 1000

	modelMarketplaceEndpointResolveTimeout = 5 * time.Second
	modelMarketplaceAnthropicAPIVersion    = "2023-06-01"
	modelMarketplaceChallengeMaxTokens     = 50
	modelMarketplaceRunOneBuffer           = 10 * time.Second
	modelMarketplaceIdleConnTimeout        = 30 * time.Second
	modelMarketplaceTLSHandshakeTimeout    = 10 * time.Second
	modelMarketplaceResponseHeaderTimeout  = 30 * time.Second
	modelMarketplacePingDiscardMaxBytes    = 1024
	modelMarketplaceDialTimeout            = 10 * time.Second
	modelMarketplaceDialKeepAlive          = 30 * time.Second
)

const (
	ModelMarketplaceBodyOverrideModeOff     = "off"
	ModelMarketplaceBodyOverrideModeMerge   = "merge"
	ModelMarketplaceBodyOverrideModeReplace = "replace"
)
