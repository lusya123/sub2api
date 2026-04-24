package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
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
//   - 若 (bucket_ts, account_id, group_id, model) 已存在则对应的 *_count 字段 +1,
//     latency_p50_ms 取 MAX(old, new) —— 暂时用 MAX 近似 p50,后续替换为真实
//     滑动 p50 估计器 (TODO: t-digest / HDR histogram)。
//   - 否则插入一行,目标 count=1,其它 count=0。
//
// 实现走"事务内 SELECT -> 分支 UPDATE/INSERT",原因:
//   - ent 的 OnConflictColumns 能支持 Add() 方便累加 count,但 latency_p50_ms
//     要写 MAX() 在 sqlite/postgres 两套方言下表达不同
//     (postgres: GREATEST(excluded, table); sqlite: MAX(excluded, table))。
//   - 事务 + 读改写写法两种方言都原生支持,行为更容易预测,并且生产 QPS
//     对单桶并发很低 (一个账号一分钟) —— 冲突窗口小,收益低于可移植性。
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

	// 开启事务确保 "SELECT -> UPDATE/INSERT" 原子,避免两个并发 Record 双插入
	// (唯一索引会兜底但事务能减少错误路径)。
	tx, err := r.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("channel_health_recorder: begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	existing, err := tx.ChannelHealthSample.Query().
		Where(
			channelhealthsample.BucketTsEQ(bucket),
			channelhealthsample.AccountIDEQ(e.AccountID),
			channelhealthsample.GroupIDEQ(e.GroupID),
			channelhealthsample.ModelEQ(e.Model),
		).
		Only(ctx)

	switch {
	case err == nil:
		// 行已存在: 累加对应的 count,latency 取 max。
		upd := existing.Update()
		switch e.Outcome {
		case OutcomeSuccess:
			upd = upd.AddSuccessCount(1)
		case OutcomeError:
			upd = upd.AddErrorCount(1)
		case OutcomeRateLimited:
			upd = upd.AddRateLimitedCount(1)
		case OutcomeOverloaded:
			upd = upd.AddOverloadedCount(1)
		default:
			return fmt.Errorf("channel_health_recorder: unknown outcome %d", e.Outcome)
		}
		if e.LatencyMs > existing.LatencyP50Ms {
			upd = upd.SetLatencyP50Ms(e.LatencyMs)
		}
		if _, err := upd.Save(ctx); err != nil {
			return fmt.Errorf("channel_health_recorder: update: %w", err)
		}
	case dbent.IsNotFound(err):
		// 新桶: 插入初始行,目标 count=1。
		builder := tx.ChannelHealthSample.Create().
			SetBucketTs(bucket).
			SetAccountID(e.AccountID).
			SetGroupID(e.GroupID).
			SetModel(e.Model).
			SetLatencyP50Ms(e.LatencyMs).
			SetSource(string(e.Source))
		switch e.Outcome {
		case OutcomeSuccess:
			builder = builder.SetSuccessCount(1)
		case OutcomeError:
			builder = builder.SetErrorCount(1)
		case OutcomeRateLimited:
			builder = builder.SetRateLimitedCount(1)
		case OutcomeOverloaded:
			builder = builder.SetOverloadedCount(1)
		default:
			return fmt.Errorf("channel_health_recorder: unknown outcome %d", e.Outcome)
		}
		if _, err := builder.Save(ctx); err != nil {
			return fmt.Errorf("channel_health_recorder: insert: %w", err)
		}
	default:
		return fmt.Errorf("channel_health_recorder: query existing: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("channel_health_recorder: commit: %w", err)
	}
	committed = true
	return nil
}
