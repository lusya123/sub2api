package service

import (
	"time"

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
// 该函数是 fire-and-forget 语义: TryEnqueue 的 bool 返回值被刻意忽略 (drop
// 监控走 AsyncChannelHealthRecorder.Dropped()),绝对不阻断请求响应路径。
// enqueuer 可能为 nil (wire DI 未接入,或 mock),此时直接 no-op。
//
// groupID 从 gin.Context 中的 "api_key" (*APIKey) 读取,0 表示无分组。
// startTime 用来计算 latency_ms。
//
// 参数签名接受的是 ChannelHealthEnqueuer 接口而非 *ChannelHealthRecorder 本体,
// 这样生产环境走 AsyncChannelHealthRecorder (非阻塞),测试和主动探针走同步的
// *ChannelHealthRecorder——同一套调用点,零分支。
func emitChannelHealthSample(
	c *gin.Context,
	enqueuer ChannelHealthEnqueuer,
	account *Account,
	model string,
	statusCode int,
	startTime time.Time,
) {
	if enqueuer == nil || account == nil {
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

	// Fire-and-forget; the async worker owns its own ctx with timeout, so
	// we don't pass c.Request.Context() here (a cancelled request must not
	// cancel the sample we're already enqueuing).
	_ = enqueuer.TryEnqueue(ChannelHealthEvent{
		AccountID: account.ID,
		GroupID:   groupID,
		Model:     model,
		Outcome:   mapStatusToOutcome(statusCode),
		LatencyMs: latencyMs,
		Source:    SourcePassive,
		At:        time.Now(),
	})
}
