package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// mapStatusToOutcome 把上游 HTTP 状态码映射成健康采样的 Outcome 分类。
//
// 这里的分类与 shouldFailoverUpstreamError 互补,但语义略有不同:
//   - 429 单独归为 OutcomeRateLimited (速率限制)
//   - 529 单独归为 OutcomeOverloaded (Anthropic overloaded)
//   - 2xx 为成功
//   - 其它非 2xx 统一为 OutcomeError
//
// 注: 401/403 虽然在 failover 逻辑里被视为可恢复错误,但在健康采样意义上
// 与其它 4xx/5xx 一样都算错误,不需要单独桶。
func mapStatusToOutcome(statusCode int) HealthOutcome {
	switch statusCode {
	case 429:
		return OutcomeRateLimited
	case 529:
		return OutcomeOverloaded
	}
	if statusCode >= 200 && statusCode < 300 {
		return OutcomeSuccess
	}
	return OutcomeError
}

// emitChannelHealthSample 在 gateway 完成点被动采样一次请求的健康数据。
//
// 该函数是 fire-and-forget 语义: Record 返回 error 只写一条 warn log,绝对
// 不阻断请求响应路径。recorder 可能为 nil (wire DI 未接入),此时直接 no-op。
//
// groupID 从 gin.Context 中的 "api_key" (*APIKey) 读取,0 表示无分组。
// startTime 用来计算 latency_ms。
func emitChannelHealthSample(
	c *gin.Context,
	recorder *ChannelHealthRecorder,
	account *Account,
	model string,
	statusCode int,
	startTime time.Time,
) {
	if recorder == nil || account == nil {
		return
	}
	if model == "" {
		return
	}

	var groupID int64
	if c != nil {
		if v, exists := c.Get("api_key"); exists {
			if apiKey, ok := v.(*APIKey); ok && apiKey != nil && apiKey.GroupID != nil {
				groupID = *apiKey.GroupID
			}
		}
	}

	latencyMs := int(time.Since(startTime).Milliseconds())
	if latencyMs < 0 {
		latencyMs = 0
	}

	// 优先使用 gin 请求 ctx; 缺失时退化到 Background,避免因 c/Request 为 nil 崩溃。
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	if err := recorder.Record(ctx, ChannelHealthEvent{
		AccountID: account.ID,
		GroupID:   groupID,
		Model:     model,
		Outcome:   mapStatusToOutcome(statusCode),
		LatencyMs: latencyMs,
		Source:    SourcePassive,
		At:        time.Now(),
	}); err != nil {
		logger.LegacyPrintf("service.channel_health", "passive sample dropped: account=%d group=%d model=%s status=%d err=%v",
			account.ID, groupID, model, statusCode, err)
	}
}
