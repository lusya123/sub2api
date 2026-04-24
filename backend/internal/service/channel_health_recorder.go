package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// HealthOutcome 描述一次请求 / 探针结果在健康指标意义上的分类。
type HealthOutcome int

const (
	// OutcomeSuccess 计入 success_count。
	OutcomeSuccess HealthOutcome = iota
	// OutcomeError 计入 error_count (非 429/529 的失败)。
	OutcomeError
	// OutcomeRateLimited 对应 HTTP 429。
	OutcomeRateLimited
	// OutcomeOverloaded 对应 HTTP 529 (Anthropic overloaded)。
	OutcomeOverloaded
)

// HealthSource 标识一个样本的采集来源,写入 channel_health_samples.source。
type HealthSource string

const (
	// SourcePassive: gateway 请求钩子被动记录的真实流量样本。
	SourcePassive HealthSource = "passive"
	// SourceActiveProbe: 后台主动探针 (Haiku max_tokens=1) 补空白桶的样本。
	SourceActiveProbe HealthSource = "active_probe"
)

// ChannelHealthEvent 是一次健康采样事件。调用方(gateway 钩子 / 主动探针)
// 填好字段后交给 ChannelHealthRecorder.Record upsert 到 1 分钟桶。
type ChannelHealthEvent struct {
	AccountID int64
	GroupID   int64 // 0 表示无分组 (原生 anthropic 路由)
	Model     string
	Outcome   HealthOutcome
	LatencyMs int
	Source    HealthSource
	At        time.Time
}

// ChannelHealthRecorder 把 ChannelHealthEvent upsert 到 channel_health_samples
// 表的 1 分钟桶里。该 Recorder 是公开状态页 /status 的数据入口。
//
// 依赖 *dbent.Client,与 AdminService / UsageService 等邻近 service 的 DI 风格
// 一致。后续的 gateway 钩子和主动探针都会通过 wire 拿到这个 Recorder 实例。
type ChannelHealthRecorder struct {
	entClient *dbent.Client
}

// NewChannelHealthRecorder 构造 Recorder。Wire 会在 ProviderSet 里注入 *dbent.Client。
func NewChannelHealthRecorder(entClient *dbent.Client) *ChannelHealthRecorder {
	return &ChannelHealthRecorder{entClient: entClient}
}

// floorToMinute 把时间向下取整到分钟边界 (UTC),用作 bucket_ts。
func floorToMinute(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), utc.Minute(), 0, 0, time.UTC)
}

// Record 把一个 ChannelHealthEvent upsert 到对应的 1 分钟桶。
//
// 语义:
//   - bucket_ts = floor(e.At, 1min)
//   - 若 (bucket_ts, account_id, group_id, model) 已存在,对应的 *_count 字段
//     累加,latency_p50_ms 取 MAX(old, new)。
//   - 否则插入一行,目标 count=1,其它 count=0。
//
// 实现:原生 SQL `INSERT ... ON CONFLICT DO UPDATE`,一次网络往返就能原子
// upsert。解决了旧版"事务 SELECT → UPDATE/INSERT"在高 QPS 同桶并发下踩到
// 唯一索引冲突导致样本丢失的问题(事务回滚,error 被调用方 log 吞掉)。
//
// 方言适配:
//   - Postgres:`GREATEST(EXCLUDED.col, table.col)` 取最大延迟
//   - SQLite  :`MAX(EXCLUDED.col, table.col)` —— SQLite 3.24+ 也支持 ON CONFLICT
//     和 EXCLUDED 伪表,只是 `GREATEST` 是 Postgres 方言扩展,SQLite 下标量 `MAX()`
//     等价。
func (r *ChannelHealthRecorder) Record(ctx context.Context, e ChannelHealthEvent) error {
	if r == nil || r.entClient == nil {
		return errors.New("channel_health_recorder: entClient is nil")
	}
	if e.Model == "" {
		return errors.New("channel_health_recorder: Model is required")
	}
	if e.Source == "" {
		e.Source = SourcePassive
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	bucket := floorToMinute(e.At)

	// success / error / ratelimited / overloaded 初始值:只有本次事件对应的
	// 那一列 = 1,其它 = 0。冲突路径下由 SQL 表达式把这些整体加到旧值。
	var sc, ec, rc, oc int
	switch e.Outcome {
	case OutcomeSuccess:
		sc = 1
	case OutcomeError:
		ec = 1
	case OutcomeRateLimited:
		rc = 1
	case OutcomeOverloaded:
		oc = 1
	default:
		return fmt.Errorf("channel_health_recorder: unknown outcome %d", e.Outcome)
	}

	drv, ok := r.entClient.Driver().(*entsql.Driver)
	if !ok {
		return errors.New("channel_health_recorder: driver is not *entsql.Driver")
	}
	db := drv.DB()
	if db == nil {
		return errors.New("channel_health_recorder: driver DB() is nil")
	}
	dialectName := drv.Dialect()

	query, args := buildUpsertSQL(dialectName, bucket, e, sc, ec, rc, oc)
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("channel_health_recorder: upsert: %w", err)
	}
	return nil
}

// buildUpsertSQL crafts the dialect-specific ON CONFLICT upsert. Kept as a
// pure function so the test suite can reason about the emitted SQL without
// hitting the database.
func buildUpsertSQL(
	dialectName string,
	bucket time.Time,
	e ChannelHealthEvent,
	sc, ec, rc, oc int,
) (string, []interface{}) {
	// Column order MUST match the placeholders below.
	args := []interface{}{
		bucket,            // $1 bucket_ts
		e.AccountID,       // $2 account_id
		e.GroupID,         // $3 group_id
		e.Model,           // $4 model
		sc,                // $5 success_count
		ec,                // $6 error_count
		rc,                // $7 rate_limited_count
		oc,                // $8 overloaded_count
		e.LatencyMs,       // $9 latency_p50_ms
		string(e.Source),  // $10 source
		time.Now().UTC(),  // $11 created_at (immutable; only used on INSERT)
	}

	// Latency expression differs per dialect: Postgres has GREATEST, SQLite
	// does not but `MAX(a, b)` in scalar position is equivalent.
	latencyExpr := "GREATEST(EXCLUDED.latency_p50_ms, channel_health_samples.latency_p50_ms)"
	if dialectName == dialect.SQLite {
		latencyExpr = "MAX(EXCLUDED.latency_p50_ms, channel_health_samples.latency_p50_ms)"
	}

	q := `
INSERT INTO channel_health_samples (
  bucket_ts, account_id, group_id, model,
  success_count, error_count, rate_limited_count, overloaded_count,
  latency_p50_ms, source, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (bucket_ts, account_id, group_id, model) DO UPDATE SET
  success_count      = channel_health_samples.success_count      + EXCLUDED.success_count,
  error_count        = channel_health_samples.error_count        + EXCLUDED.error_count,
  rate_limited_count = channel_health_samples.rate_limited_count + EXCLUDED.rate_limited_count,
  overloaded_count   = channel_health_samples.overloaded_count   + EXCLUDED.overloaded_count,
  latency_p50_ms     = ` + latencyExpr

	return q, args
}
