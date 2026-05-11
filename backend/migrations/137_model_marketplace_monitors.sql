-- 独立模型广场探测配置。
-- 这三张表刻意不复用 channel_monitors / channel_monitor_histories，
-- 方便模型广场后续独立演进，不被渠道监控 schema 和业务逻辑牵连。

CREATE TABLE IF NOT EXISTS model_marketplace_request_templates (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(20) NOT NULL CHECK (provider IN ('openai', 'anthropic', 'gemini')),
    description VARCHAR(500) NOT NULL DEFAULT '',
    extra_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    body_override_mode VARCHAR(10) NOT NULL DEFAULT 'off',
    body_override JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT model_marketplace_request_templates_provider_name_key UNIQUE (provider, name)
);

CREATE TABLE IF NOT EXISTS model_marketplace_monitors (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(20) NOT NULL CHECK (provider IN ('openai', 'anthropic', 'gemini')),
    endpoint VARCHAR(500) NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    primary_model VARCHAR(200) NOT NULL,
    extra_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    group_name VARCHAR(100) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    interval_seconds INT NOT NULL CHECK (interval_seconds BETWEEN 15 AND 3600),
    last_checked_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL,
    template_id BIGINT REFERENCES model_marketplace_request_templates(id) ON DELETE SET NULL,
    extra_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    body_override_mode VARCHAR(10) NOT NULL DEFAULT 'off',
    body_override JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitors_enabled_checked
    ON model_marketplace_monitors(enabled, last_checked_at);
CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitors_provider
    ON model_marketplace_monitors(provider);
CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitors_group_name
    ON model_marketplace_monitors(group_name);
CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitors_template_id
    ON model_marketplace_monitors(template_id);

CREATE TABLE IF NOT EXISTS model_marketplace_monitor_histories (
    id BIGSERIAL PRIMARY KEY,
    monitor_id BIGINT NOT NULL REFERENCES model_marketplace_monitors(id) ON DELETE CASCADE,
    model VARCHAR(200) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('operational', 'degraded', 'failed', 'error')),
    latency_ms INT,
    ping_latency_ms INT,
    message VARCHAR(500) NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitor_histories_monitor_model_checked
    ON model_marketplace_monitor_histories(monitor_id, model, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_marketplace_monitor_histories_checked
    ON model_marketplace_monitor_histories(checked_at);
