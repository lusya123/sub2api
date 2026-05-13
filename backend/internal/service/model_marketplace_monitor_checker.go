package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tidwall/gjson"
)

// modelMarketplaceHTTPClient 共享一个 http.Client，避免每次检测重建 transport。
// 自定义 Transport 在 dial 时强制再次校验 IP，防止 DNS rebinding 绕过 validateEndpoint。
var modelMarketplaceHTTPClient = newModelMarketplaceSSRFSafeHTTPClient(modelMarketplaceRequestTimeout)

// modelMarketplacePingHTTPClient 用于 endpoint origin 的 HEAD ping，超时更短。
var modelMarketplacePingHTTPClient = newModelMarketplaceSSRFSafeHTTPClient(modelMarketplacePingTimeout)

// newModelMarketplaceSSRFSafeHTTPClient 返回一个使用 modelMarketplaceSafeDialContext 的 http.Client。
// 仅供监控模块对外发起请求使用——所有目标都应是公网 endpoint。
func newModelMarketplaceSSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DialContext:           modelMarketplaceSafeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       modelMarketplaceIdleConnTimeout,
		TLSHandshakeTimeout:   modelMarketplaceTLSHandshakeTimeout,
		ResponseHeaderTimeout: modelMarketplaceResponseHeaderTimeout,
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

// ModelMarketplaceCheckOptions 承载一次检测的自定义入参。
// 所有字段都是可选（零值即等价于"用默认行为"）。
type ModelMarketplaceCheckOptions struct {
	// ExtraHeaders 用户自定义 HTTP 头（merge 到 adapter 默认 headers，用户优先）。
	ExtraHeaders map[string]string
	// BodyOverrideMode: off | merge | replace
	BodyOverrideMode string
	// BodyOverride 在 merge 模式下做浅合并（key 命中黑名单时静默丢弃），
	// 在 replace 模式下直接当作完整 body。
	BodyOverride map[string]any
	// RequestURL 是某个模型配置的完整请求地址。非空时检测会直接请求它，
	// provider 仍用于展示品牌，但协议会优先从 URL path 推断。
	RequestURL string
}

// runModelMarketplaceCheckForModel 对单个 (provider, model) 做一次完整检测。
// 不返回 error：所有失败都包装进 CheckResult.Status=error/failed。
//
// opts 承载模板 / 监控快照带来的自定义配置。nil 等同于 "off + 无 extra headers"。
func runModelMarketplaceCheckForModel(ctx context.Context, provider, endpoint, apiKey, model string, opts *ModelMarketplaceCheckOptions) *ModelMarketplaceCheckResult {
	res := &ModelMarketplaceCheckResult{
		Model:     model,
		Status:    ModelMarketplaceStatusError,
		CheckedAt: time.Now(),
	}

	challenge := generateModelMarketplaceChallenge()
	mode := modelMarketplaceBodyOverrideMode(opts)

	start := time.Now()
	respText, rawBody, statusCode, err := callModelMarketplaceProvider(ctx, provider, endpoint, apiKey, model, challenge.Prompt, opts)
	latency := time.Since(start)
	latencyMs := int(latency / time.Millisecond)
	res.LatencyMs = &latencyMs

	if err != nil {
		res.Status = ModelMarketplaceStatusError
		res.Message = truncateModelMarketplaceMessage(sanitizeModelMarketplaceErrorMessage(err.Error()))
		return res
	}
	if statusCode < 200 || statusCode >= 300 {
		// 错误路径：用 rawBody 而非 respText（gjson textPath 抽取在错误响应里通常为空，
		// 会丢掉真正的上游错误信息，例如 `{"error":{"message":"No available accounts ..."}}`）。
		res.Status = ModelMarketplaceStatusError
		bodySnippet := truncateModelMarketplaceErrorBody(rawBody)
		res.Message = truncateModelMarketplaceMessage(sanitizeModelMarketplaceErrorMessage(fmt.Sprintf("upstream HTTP %d: %s", statusCode, bodySnippet)))
		return res
	}

	// Replace 模式：跳过 challenge 校验（用户 body 是静态的，challenge 没法嵌入）。
	// 改用「HTTP 2xx + 响应文本（adapter.textPath 抽取）非空」作为 operational 判定。
	// 响应文本为空则降级为 failed（视为上游回了 200 但没实际内容）。
	if mode == ModelMarketplaceBodyOverrideModeReplace {
		if strings.TrimSpace(respText) == "" {
			res.Status = ModelMarketplaceStatusFailed
			res.Message = truncateModelMarketplaceMessage("replace-mode: upstream returned 2xx with empty text")
			return res
		}
		return finalizeModelMarketplaceOperationalOrDegraded(res, latency, latencyMs)
	}

	if !validateModelMarketplaceChallenge(respText, challenge.Expected) {
		res.Status = ModelMarketplaceStatusFailed
		res.Message = truncateModelMarketplaceMessage(sanitizeModelMarketplaceErrorMessage(fmt.Sprintf("challenge mismatch (expected %s, got %q)", challenge.Expected, respText)))
		return res
	}

	return finalizeModelMarketplaceOperationalOrDegraded(res, latency, latencyMs)
}

// finalizeModelMarketplaceOperationalOrDegraded 负责走到最后一步的 operational/degraded 判定。
// 拆出来是为了让 runModelMarketplaceCheckForModel 不超过 30 行。
func finalizeModelMarketplaceOperationalOrDegraded(res *ModelMarketplaceCheckResult, latency time.Duration, latencyMs int) *ModelMarketplaceCheckResult {
	if latency >= modelMarketplaceDegradedThreshold {
		res.Status = ModelMarketplaceStatusDegraded
		res.Message = truncateModelMarketplaceMessage(fmt.Sprintf("slow response: %dms", latencyMs))
		return res
	}
	res.Status = ModelMarketplaceStatusOperational
	return res
}

// modelMarketplaceBodyOverrideMode 归一取 opts.BodyOverrideMode，nil opts / 空串都视为 off。
func modelMarketplaceBodyOverrideMode(opts *ModelMarketplaceCheckOptions) string {
	if opts == nil || opts.BodyOverrideMode == "" {
		return ModelMarketplaceBodyOverrideModeOff
	}
	return opts.BodyOverrideMode
}

// pingModelMarketplaceEndpointOrigin 对 endpoint 的 origin (scheme://host) 发起 HEAD 请求，返回耗时。
// 失败时返回 nil（不影响主状态判定）。
func pingModelMarketplaceEndpointOrigin(ctx context.Context, endpoint string) *int {
	origin, err := extractModelMarketplaceOrigin(endpoint)
	if err != nil || origin == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, origin, nil)
	if err != nil {
		return nil
	}
	start := time.Now()
	resp, err := modelMarketplacePingHTTPClient.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, modelMarketplacePingDiscardMaxBytes))
	ms := int(time.Since(start) / time.Millisecond)
	return &ms
}

// modelMarketplaceProviderAdapter 描述某个 provider 在 challenge 检测中需要的 4 件事：
//   - 拼出请求路径（含 model 占位）
//   - 序列化请求体
//   - 构造鉴权头
//   - 从响应 JSON 中按 path 提取文本（gjson path）
//
// 加新检测协议只需要在 modelMarketplaceProtocolAdapters 里增加一个条目；
// 加 New API 渠道只需要在 modelMarketplaceProviderProtocols 里映射到协议。
type modelMarketplaceProviderAdapter struct {
	buildPath    func(model string) string
	buildBody    func(model, prompt string) ([]byte, error)
	buildHeaders func(apiKey string) map[string]string
	textPath     string // gjson 提取响应文本的 path
}

// modelMarketplaceProviderProtocols mirrors New API's channel list while keeping
// the health checker implementation compact.
//
//nolint:gochecknoglobals // 静态查表，初始化后不变。
var modelMarketplaceProviderProtocols = map[string]string{
	ModelMarketplaceProviderOpenAI:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderOpenAIMax:      modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderOhMyGPT:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderCustom:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAILS:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAIProxy:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAPI2GPT:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAIGC2D:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAnthropic:      modelMarketplaceProtocolAnthropic,
	ModelMarketplaceProviderAWS:            modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderGemini:         modelMarketplaceProtocolGemini,
	ModelMarketplaceProviderDeepSeek:       modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAzure:          modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderVertexAI:       modelMarketplaceProtocolGemini,
	ModelMarketplaceProviderXAI:            modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMistral:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderCohere:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderOpenRouter:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderOllama:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderSiliconFlow:    modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderPerplexity:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMoonshot:       modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAli:            modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderZhipu:          modelMarketplaceProtocolZhipu,
	ModelMarketplaceProviderZhipuV4:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderBaidu:          modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderBaiduV2:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderTencent:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderXunfei:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderVolcEngine:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderLingYiWanWu:    modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMiniMax:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderCoze:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAI360:          modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderXinference:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderDify:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderJina:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderCloudflare:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderPaLM:           modelMarketplaceProtocolGemini,
	ModelMarketplaceProviderCodex:          modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderFastGPT:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderAIProxyLibrary: modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMokaAI:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMidjourney:     modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderMidjourneyPlus: modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderSunoAPI:        modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderKling:          modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderJimeng:         modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderVidu:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderSubmodel:       modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderDoubaoVideo:    modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderSora:           modelMarketplaceProtocolOpenAICompatible,
	ModelMarketplaceProviderReplicate:      modelMarketplaceProtocolOpenAICompatible,
}

// modelMarketplaceProtocolAdapters 全部已支持的检测协议。
//
//nolint:gochecknoglobals // 适配器表是只读静态数据，初始化后不变更。
var modelMarketplaceProtocolAdapters = map[string]modelMarketplaceProviderAdapter{
	modelMarketplaceProtocolOpenAICompatible: {
		buildPath: func(string) string { return modelMarketplaceProviderOpenAIPath },
		buildBody: func(model, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"model":      model,
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"max_tokens": modelMarketplaceChallengeMaxTokens,
				"stream":     false,
			})
		},
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{"Authorization": "Bearer " + apiKey}
		},
		textPath: "choices.0.message.content",
	},
	modelMarketplaceProtocolAnthropic: {
		buildPath: func(string) string { return modelMarketplaceProviderAnthropicPath },
		buildBody: func(model, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"model":      model,
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"max_tokens": modelMarketplaceChallengeMaxTokens,
			})
		},
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{
				"x-api-key":         apiKey,
				"anthropic-version": modelMarketplaceAnthropicAPIVersion,
			}
		},
		textPath: "content.0.text",
	},
	modelMarketplaceProtocolGemini: {
		// Gemini 把 model 名写在 URL path 上：/v1beta/models/{model}:generateContent
		buildPath: func(model string) string { return fmt.Sprintf(modelMarketplaceProviderGeminiPathTemplate, model) },
		buildBody: func(_, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"contents": []map[string]any{
					{"parts": []map[string]any{{"text": prompt}}},
				},
				"generationConfig": map[string]any{"maxOutputTokens": modelMarketplaceChallengeMaxTokens},
			})
		},
		// 使用 x-goog-api-key header 而不是 ?key= query，避免 *url.Error 把 key 回填到错误日志。
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{"x-goog-api-key": apiKey}
		},
		textPath: "candidates.0.content.parts.0.text",
	},
	modelMarketplaceProtocolZhipu: {
		buildPath: func(model string) string { return fmt.Sprintf(modelMarketplaceProviderZhipuPathTemplate, model) },
		buildBody: func(_, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"prompt": []map[string]string{{"role": "user", "content": prompt}},
			})
		},
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{"Authorization": buildModelMarketplaceZhipuToken(apiKey)}
		},
		textPath: "data.choices.0.content",
	},
}

// isModelMarketplaceSupportedProvider 校验 provider 字符串是否在 adapter 表中。
// 供 validate.go 的 validateProvider 复用，避免两份 switch 漂移。
func isModelMarketplaceSupportedProvider(p string) bool {
	_, ok := modelMarketplaceProviderProtocols[p]
	return ok
}

// callModelMarketplaceProvider 通过 provider -> protocol -> adapter 分发到具体实现。
// opts 承载用户的自定义 headers / body 覆盖（可为 nil）。
//
// 返回值：
//   - extractedText: 按 textPath 抽出的成功文本，仅在 status 2xx 时有意义；非 2xx 时通常为空串
//   - rawBody: 完整响应体的字符串形式（已被 modelMarketplaceResponseMaxBytes 截断），用于错误路径保留上游真实回包
//   - status: HTTP 状态码
//   - err: 网络 / 序列化错误
func callModelMarketplaceProvider(ctx context.Context, provider, endpoint, apiKey, model, prompt string, opts *ModelMarketplaceCheckOptions) (extractedText, rawBody string, status int, err error) {
	protocol, adapter, ok := modelMarketplaceAdapterForRequest(provider, opts)
	if !ok {
		return "", "", 0, fmt.Errorf("unsupported provider %q", provider)
	}
	body, err := buildModelMarketplaceRequestBody(adapter, protocol, model, prompt, opts)
	if err != nil {
		return "", "", 0, err
	}
	headers := mergeModelMarketplaceHeaders(adapter.buildHeaders(apiKey), opts)
	full := modelMarketplaceCheckRequestURL(endpoint, adapter.buildPath(model), opts)
	respBytes, status, err := postModelMarketplaceRawJSON(ctx, full, body, headers)
	if err != nil {
		return "", "", status, err
	}
	return gjson.GetBytes(respBytes, adapter.textPath).String(), string(respBytes), status, nil
}

func modelMarketplaceAdapterForRequest(provider string, opts *ModelMarketplaceCheckOptions) (string, modelMarketplaceProviderAdapter, bool) {
	protocol, ok := modelMarketplaceProviderProtocols[provider]
	if !ok {
		return "", modelMarketplaceProviderAdapter{}, false
	}
	if inferred := inferModelMarketplaceProtocolFromRequestURL(modelMarketplaceRequestURLOverride(opts)); inferred != "" {
		protocol = inferred
	}
	adapter, ok := modelMarketplaceProtocolAdapters[protocol]
	if !ok {
		return "", modelMarketplaceProviderAdapter{}, false
	}
	if modelMarketplaceRequestURLOverride(opts) == "" {
		switch provider {
		case ModelMarketplaceProviderMiniMax:
			adapter.buildPath = func(string) string { return modelMarketplaceProviderMiniMaxPath }
		case ModelMarketplaceProviderZhipuV4:
			adapter.buildPath = func(string) string { return modelMarketplaceProviderZhipuV4Path }
		}
	}
	return protocol, adapter, true
}

func modelMarketplaceRequestURLOverride(opts *ModelMarketplaceCheckOptions) string {
	if opts == nil {
		return ""
	}
	return strings.TrimSpace(opts.RequestURL)
}

func modelMarketplaceCheckRequestURL(endpoint, path string, opts *ModelMarketplaceCheckOptions) string {
	if requestURL := modelMarketplaceRequestURLOverride(opts); requestURL != "" {
		return requestURL
	}
	return joinModelMarketplaceURL(endpoint, path)
}

func inferModelMarketplaceProtocolFromRequestURL(requestURL string) string {
	requestURL = strings.TrimSpace(requestURL)
	if requestURL == "" {
		return ""
	}
	u, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	path := strings.ToLower(u.Path)
	switch {
	case strings.Contains(path, "/v1/messages"):
		return modelMarketplaceProtocolAnthropic
	case strings.Contains(path, "/v1beta/models") || strings.Contains(path, ":generatecontent"):
		return modelMarketplaceProtocolGemini
	case strings.Contains(path, "/api/paas/v3/model-api/"):
		return modelMarketplaceProtocolZhipu
	case strings.Contains(path, "/api/paas/v4/chat/completions"), strings.Contains(path, "/chat/completions"):
		return modelMarketplaceProtocolOpenAICompatible
	default:
		return ""
	}
}

// mergeModelMarketplaceHeaders 把用户自定义 headers 合并到 adapter 默认 headers 上。
// 用户值覆盖默认；命中黑名单（hop-by-hop / 由 http.Client 自管的）的 key 静默丢弃。
func mergeModelMarketplaceHeaders(base map[string]string, opts *ModelMarketplaceCheckOptions) map[string]string {
	if opts == nil || len(opts.ExtraHeaders) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(opts.ExtraHeaders))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range opts.ExtraHeaders {
		if isForbiddenModelMarketplaceHeaderName(k) {
			continue
		}
		out[k] = v
	}
	return out
}

// buildModelMarketplaceRequestBody 根据 body_override_mode 构造请求 body。
//
//   - off:     adapter 默认 body
//   - merge:   adapter 默认 body 与 BodyOverride 浅合并；BodyOverride 中命中
//     modelMarketplaceBodyMergeKeyDenyList[protocol] 的 key 会被静默丢弃，避免破坏 challenge / model 路由
//   - replace: 直接 marshal BodyOverride 作为完整 body
//
// 任何 mode 返回的 []byte 都已经是合法 JSON，可直接送入 postModelMarketplaceRawJSON。
func buildModelMarketplaceRequestBody(adapter modelMarketplaceProviderAdapter, protocol, model, prompt string, opts *ModelMarketplaceCheckOptions) ([]byte, error) {
	mode := modelMarketplaceBodyOverrideMode(opts)

	if mode == ModelMarketplaceBodyOverrideModeReplace {
		if opts == nil || len(opts.BodyOverride) == 0 {
			return nil, fmt.Errorf("replace mode: body_override is empty")
		}
		body, err := json.Marshal(opts.BodyOverride)
		if err != nil {
			return nil, fmt.Errorf("marshal body_override (replace): %w", err)
		}
		return body, nil
	}

	defaultBody, err := adapter.buildBody(model, prompt)
	if err != nil {
		return nil, fmt.Errorf("marshal default body: %w", err)
	}
	if mode != ModelMarketplaceBodyOverrideModeMerge || opts == nil || len(opts.BodyOverride) == 0 {
		return defaultBody, nil
	}

	var defaultMap map[string]any
	if err := json.Unmarshal(defaultBody, &defaultMap); err != nil {
		return nil, fmt.Errorf("unmarshal default body for merge: %w", err)
	}
	deny := modelMarketplaceBodyMergeKeyDenyList[protocol]
	for k, v := range opts.BodyOverride {
		if deny[k] {
			continue
		}
		defaultMap[k] = v
	}
	merged, err := json.Marshal(defaultMap)
	if err != nil {
		return nil, fmt.Errorf("marshal merged body: %w", err)
	}
	return merged, nil
}

// modelMarketplaceBodyMergeKeyDenyList 在 merge 模式下，禁止用户覆盖这些 provider-specific 的关键字段。
// 思路抄 check-cx 的 EXCLUDED_METADATA_KEYS：保护 challenge / model 路由不被用户误伤。
// 用户想动这些字段就用 replace 模式（已知会跳 challenge 校验）。
//
//nolint:gochecknoglobals // 静态查表，初始化后不变。
var modelMarketplaceBodyMergeKeyDenyList = map[string]map[string]bool{
	modelMarketplaceProtocolOpenAICompatible: {"model": true, "messages": true, "stream": true},
	modelMarketplaceProtocolAnthropic:        {"model": true, "messages": true},
	modelMarketplaceProtocolGemini:           {"contents": true},
	modelMarketplaceProtocolZhipu:            {"prompt": true, "incremental": true},
}

func buildModelMarketplaceZhipuToken(apiKey string) string {
	parts := strings.Split(apiKey, ".")
	if len(parts) != 2 {
		return ""
	}
	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	expMs := time.Now().Add(24*time.Hour).UnixNano() / int64(time.Millisecond)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"api_key":   parts[0],
		"exp":       expMs,
		"timestamp": nowMs,
	})
	token.Header["alg"] = "HS256"
	token.Header["sign_type"] = "SIGN"
	signed, err := token.SignedString([]byte(parts[1]))
	if err != nil {
		return ""
	}
	return signed
}

// postModelMarketplaceRawJSON 发送 POST + 已序列化好的 JSON 字节，限制响应体大小，返回响应字节、HTTP status、错误。
// adapter 自行 marshal 是为了精确控制字段顺序与类型，所以这里直接收 []byte 而不是 any。
func postModelMarketplaceRawJSON(ctx context.Context, fullURL string, payload []byte, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := modelMarketplaceHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, modelMarketplaceResponseMaxBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// joinModelMarketplaceURL 把 base origin 与 path 拼成完整 URL。
// 容忍 base 末尾有/无斜杠，path 必带前导斜杠。
func joinModelMarketplaceURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// extractModelMarketplaceOrigin 从一个 endpoint URL 中提取 scheme://host[:port] 部分。
func extractModelMarketplaceOrigin(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("endpoint missing scheme or host")
	}
	return u.Scheme + "://" + u.Host, nil
}

// modelMarketplaceSensitiveQueryParamRegex 匹配 URL query 中可能泄露凭证的参数：
// key / api_key / api-key / access_token / token / authorization / x-api-key。
// 大小写不敏感，匹配 `?name=value` 或 `&name=value` 形式（value 截到 & 或字符串末尾）。
var modelMarketplaceSensitiveQueryParamRegex = regexp.MustCompile(`(?i)([?&](?:key|api[_-]?key|access[_-]?token|token|authorization|x-api-key)=)[^&\s"']+`)

// modelMarketplaceAPIKeyPatterns 匹配常见 provider 的 API key 字面量。
// 顺序敏感：sk-ant- 必须放在 sk- 之前，否则会被通用 sk- 模式先消费。
var modelMarketplaceAPIKeyPatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// Anthropic（带前缀，必须先匹配）：sk-ant-xxxxxxx
	{regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`), "sk-ant-***REDACTED***"},
	// OpenAI / Anthropic 通用 sk-: sk-xxxxxxx
	{regexp.MustCompile(`sk-[A-Za-z0-9-]{20,}`), "sk-***REDACTED***"},
	// Gemini / Google API Key：固定前缀 + 35 位
	{regexp.MustCompile(`AIza[A-Za-z0-9_-]{35}`), "AIza***REDACTED***"},
	// JWT 三段式（Bearer 后常出现）：eyJxxx.eyJxxx.signature
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), "eyJ***REDACTED.JWT***"},
}

// sanitizeModelMarketplaceErrorMessage 擦除错误/响应文本中可能泄露的 API key。
// 处理两类来源：
//  1. URL query 中的 ?key= / ?api_key= 等（Go *url.Error 会回填完整 URL）
//  2. 上游 HTTP body 文本里直接出现的 sk-* / AIza* / JWT 等密钥碎片
//
// 注意：与 gemini_messages_compat_service.go 的 sanitizeUpstreamErrorMessage 关注点类似但参数集更广，
// 监控模块独立维护，避免互相耦合。
func sanitizeModelMarketplaceErrorMessage(msg string) string {
	if msg == "" {
		return msg
	}
	msg = modelMarketplaceSensitiveQueryParamRegex.ReplaceAllString(msg, `${1}REDACTED`)
	for _, p := range modelMarketplaceAPIKeyPatterns {
		msg = p.pattern.ReplaceAllString(msg, p.replace)
	}
	return msg
}

// truncateModelMarketplaceMessage 把消息按 modelMarketplaceMessageMaxBytes 截断，避免 DB 列溢出与日志过长。
func truncateModelMarketplaceMessage(msg string) string {
	if len(msg) <= modelMarketplaceMessageMaxBytes {
		return msg
	}
	const ellipsis = "...(truncated)"
	cutoff := modelMarketplaceMessageMaxBytes - len(ellipsis)
	if cutoff < 0 {
		cutoff = 0
	}
	return msg[:cutoff] + ellipsis
}

// truncateModelMarketplaceErrorBody 把上游错误响应 body 压到 modelMarketplaceErrorBodySnippetMaxBytes 以内，
// 并顺手把连续空白折成一个空格：上游 HTML 错误页常含大量缩进/换行，保留会浪费预算。
// 被 truncateModelMarketplaceMessage 做最终总截断兜底，所以这里只负责 body 自身的精简。
func truncateModelMarketplaceErrorBody(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= modelMarketplaceErrorBodySnippetMaxBytes {
		return body
	}
	const ellipsis = "...(body truncated)"
	cutoff := modelMarketplaceErrorBodySnippetMaxBytes - len(ellipsis)
	if cutoff < 0 {
		cutoff = 0
	}
	return body[:cutoff] + ellipsis
}
