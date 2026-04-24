package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChannelHealthSample 记录 (account × group × model) 每分钟的健康采样,
// 供公开状态页 /status 生成心跳条与可用率。
//
// 采样来源:
//   - "passive":  gateway 请求完成钩子被动记录真实流量
//   - "active_probe": 后台稀疏主动探针(Haiku + max_tokens=1)补空白桶
//
// 保留策略: 24 小时,由清理任务基于 created_at 扫描删除。
type ChannelHealthSample struct {
	ent.Schema
}

// Annotations 指定数据库表名。
func (ChannelHealthSample) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channel_health_samples"},
	}
}

// Fields 定义采样实体字段。
func (ChannelHealthSample) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),

		// bucket_ts: 采样所属 1 分钟桶起点 (UTC),用于 upsert 去重与时间轴聚合。
		field.Time("bucket_ts").
			Comment("采样所属 1 分钟桶起点 (UTC)").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		// account_id: 对应 accounts.id
		field.Int64("account_id"),

		// group_id: 对应 groups.id;0 表示"无分组"即原生 anthropic 路由
		field.Int64("group_id").
			Comment("分组 id;0 表示无分组(原生 anthropic 路由)"),

		// model: 请求模型名
		field.String("model").
			MaxLen(128),

		// 桶内计数:成功 / 失败 / 429 / 529
		field.Int("success_count").Default(0),
		field.Int("error_count").Default(0),
		field.Int("rate_limited_count").
			Default(0).
			Comment("429 计数"),
		field.Int("overloaded_count").
			Default(0).
			Comment("529 计数"),

		// latency_p50_ms: 成功请求的 p50 延迟(毫秒)
		field.Int("latency_p50_ms").Default(0),

		// source: 采样来源
		field.String("source").
			MaxLen(16).
			Default("passive").
			Comment(`采样来源: "passive" | "active_probe"`),

		// created_at: 行创建时间,用于保留策略扫描
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义索引:
//   - 唯一索引 (bucket_ts, account_id, group_id, model) 用于 upsert 同桶去重
//   - 查询索引 (model, bucket_ts) 状态页主查询(按模型取最近 90 分钟)
//   - 清理索引 (created_at) 保留策略扫描
func (ChannelHealthSample) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("bucket_ts", "account_id", "group_id", "model").Unique(),
		index.Fields("model", "bucket_ts"),
		index.Fields("created_at"),
	}
}
