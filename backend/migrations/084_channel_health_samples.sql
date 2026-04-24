-- channel_health_samples: (account × group × model) 每分钟健康采样,
-- 供公开状态页 /status 生成心跳条与可用率。
--
-- 数据源:
--   - source = 'passive':     gateway 请求完成钩子被动记录真实流量
--   - source = 'active_probe': 后台稀疏主动探针补空白桶
-- 保留策略: 24 小时,由清理任务基于 created_at 扫描删除。
--
-- 项目迁移为 forward-only(见 migrations/README.md),
-- 如需回滚请新建 NNN_drop_channel_health_samples.sql 迁移。
-- 参考回滚 DDL(不执行):
--   DROP INDEX IF EXISTS channelhealthsample_created_at;
--   DROP INDEX IF EXISTS channelhealthsample_model_bucket_ts;
--   DROP INDEX IF EXISTS channelhealthsample_bucket_ts_account_id_group_id_model;
--   DROP TABLE IF EXISTS channel_health_samples;

CREATE TABLE IF NOT EXISTS channel_health_samples (
    id                  BIGSERIAL PRIMARY KEY,
    bucket_ts           TIMESTAMPTZ NOT NULL,
    account_id          BIGINT      NOT NULL,
    group_id            BIGINT      NOT NULL,
    model               VARCHAR(128) NOT NULL,
    success_count       INTEGER     NOT NULL DEFAULT 0,
    error_count         INTEGER     NOT NULL DEFAULT 0,
    rate_limited_count  INTEGER     NOT NULL DEFAULT 0,
    overloaded_count    INTEGER     NOT NULL DEFAULT 0,
    latency_p50_ms      INTEGER     NOT NULL DEFAULT 0,
    source              VARCHAR(16) NOT NULL DEFAULT 'passive',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN channel_health_samples.bucket_ts          IS '采样所属 1 分钟桶起点 (UTC)';
COMMENT ON COLUMN channel_health_samples.group_id           IS '分组 id;0 表示无分组(原生 anthropic 路由)';
COMMENT ON COLUMN channel_health_samples.rate_limited_count IS '429 计数';
COMMENT ON COLUMN channel_health_samples.overloaded_count   IS '529 计数';
COMMENT ON COLUMN channel_health_samples.source             IS '采样来源: "passive" | "active_probe"';

-- upsert 去重:同一分钟桶 × 账号 × 组 × 模型 只保留一行
CREATE UNIQUE INDEX IF NOT EXISTS channelhealthsample_bucket_ts_account_id_group_id_model
    ON channel_health_samples (bucket_ts, account_id, group_id, model);

-- 状态页主查询:按模型取最近 N 分钟
CREATE INDEX IF NOT EXISTS channelhealthsample_model_bucket_ts
    ON channel_health_samples (model, bucket_ts);

-- 保留策略清理:按 created_at 扫
CREATE INDEX IF NOT EXISTS channelhealthsample_created_at
    ON channel_health_samples (created_at);
