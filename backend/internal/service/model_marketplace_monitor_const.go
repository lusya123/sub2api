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
	modelMarketplaceProviderMiniMaxPath        = "/v1/text/chatcompletion_v2"
	modelMarketplaceProviderZhipuPathTemplate  = "/api/paas/v3/model-api/%s/invoke"
	modelMarketplaceProviderZhipuV4Path        = "/api/paas/v4/chat/completions"

	ModelMarketplaceProviderOpenAI         = "openai"
	ModelMarketplaceProviderOpenAIMax      = "openai_max"
	ModelMarketplaceProviderOhMyGPT        = "ohmygpt"
	ModelMarketplaceProviderCustom         = "custom"
	ModelMarketplaceProviderAILS           = "ails"
	ModelMarketplaceProviderAIProxy        = "ai_proxy"
	ModelMarketplaceProviderAPI2GPT        = "api2gpt"
	ModelMarketplaceProviderAIGC2D         = "aigc2d"
	ModelMarketplaceProviderAnthropic      = "anthropic"
	ModelMarketplaceProviderAWS            = "aws"
	ModelMarketplaceProviderGemini         = "gemini"
	ModelMarketplaceProviderDeepSeek       = "deepseek"
	ModelMarketplaceProviderAzure          = "azure"
	ModelMarketplaceProviderVertexAI       = "vertex_ai"
	ModelMarketplaceProviderXAI            = "xai"
	ModelMarketplaceProviderMistral        = "mistral"
	ModelMarketplaceProviderCohere         = "cohere"
	ModelMarketplaceProviderOpenRouter     = "openrouter"
	ModelMarketplaceProviderOllama         = "ollama"
	ModelMarketplaceProviderSiliconFlow    = "siliconflow"
	ModelMarketplaceProviderPerplexity     = "perplexity"
	ModelMarketplaceProviderMoonshot       = "moonshot"
	ModelMarketplaceProviderAli            = "ali"
	ModelMarketplaceProviderZhipu          = "zhipu"
	ModelMarketplaceProviderZhipuV4        = "zhipu_v4"
	ModelMarketplaceProviderBaidu          = "baidu"
	ModelMarketplaceProviderBaiduV2        = "baidu_v2"
	ModelMarketplaceProviderTencent        = "tencent"
	ModelMarketplaceProviderXunfei         = "xunfei"
	ModelMarketplaceProviderVolcEngine     = "volcengine"
	ModelMarketplaceProviderLingYiWanWu    = "lingyiwanwu"
	ModelMarketplaceProviderMiniMax        = "minimax"
	ModelMarketplaceProviderCoze           = "coze"
	ModelMarketplaceProviderAI360          = "ai360"
	ModelMarketplaceProviderXinference     = "xinference"
	ModelMarketplaceProviderDify           = "dify"
	ModelMarketplaceProviderJina           = "jina"
	ModelMarketplaceProviderCloudflare     = "cloudflare"
	ModelMarketplaceProviderPaLM           = "palm"
	ModelMarketplaceProviderCodex          = "codex"
	ModelMarketplaceProviderFastGPT        = "fastgpt"
	ModelMarketplaceProviderAIProxyLibrary = "ai_proxy_library"
	ModelMarketplaceProviderMokaAI         = "mokaai"
	ModelMarketplaceProviderMidjourney     = "midjourney"
	ModelMarketplaceProviderMidjourneyPlus = "midjourney_plus"
	ModelMarketplaceProviderSunoAPI        = "sunoapi"
	ModelMarketplaceProviderKling          = "kling"
	ModelMarketplaceProviderJimeng         = "jimeng"
	ModelMarketplaceProviderVidu           = "vidu"
	ModelMarketplaceProviderSubmodel       = "submodel"
	ModelMarketplaceProviderDoubaoVideo    = "doubao_video"
	ModelMarketplaceProviderSora           = "sora"
	ModelMarketplaceProviderReplicate      = "replicate"

	modelMarketplaceProtocolOpenAICompatible = "openai-compatible"
	modelMarketplaceProtocolAnthropic        = "anthropic"
	modelMarketplaceProtocolGemini           = "gemini"
	modelMarketplaceProtocolZhipu            = "zhipu"

	ModelMarketplaceStatusOperational = "operational"
	ModelMarketplaceStatusDegraded    = "degraded"
	ModelMarketplaceStatusFailed      = "failed"
	ModelMarketplaceStatusError       = "error"

	modelMarketplaceAvailability7Days  = 7
	modelMarketplaceAvailability15Days = 15
	modelMarketplaceAvailability30Days = 30

	ModelMarketplaceMonitorHistoryDefaultLimit = 100
	ModelMarketplaceMonitorHistoryMaxLimit     = 1000

	modelMarketplaceTimelineMaxPoints = 60

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
