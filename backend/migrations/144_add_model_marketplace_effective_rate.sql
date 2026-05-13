-- 独立配置模型广场渠道倍率，不再依赖分组管理倍率。

ALTER TABLE IF EXISTS model_marketplace_monitors
  ADD COLUMN IF NOT EXISTS effective_rate DECIMAL(20,8) NOT NULL DEFAULT 1;

ALTER TABLE IF EXISTS model_marketplace_monitors
  DROP CONSTRAINT IF EXISTS model_marketplace_monitors_effective_rate_positive;

ALTER TABLE IF EXISTS model_marketplace_monitors
  ADD CONSTRAINT model_marketplace_monitors_effective_rate_positive CHECK (effective_rate > 0);
