-- Add hidden billing multipliers for groups, user overrides, and usage log snapshots.
-- groups.actual_rate_multiplier: admin-configurable hidden billing multiplier.
-- user_group_rate_multipliers.actual_rate_multiplier: hidden per-user override multiplier.
-- usage_logs.actual_rate_multiplier: per-request billing multiplier snapshot.

ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS actual_rate_multiplier DECIMAL(10,4);

COMMENT ON COLUMN groups.actual_rate_multiplier IS '实际扣费倍率（空表示回退到展示倍率）';

ALTER TABLE user_group_rate_multipliers
  ADD COLUMN IF NOT EXISTS actual_rate_multiplier DECIMAL(10,4);

COMMENT ON COLUMN user_group_rate_multipliers.actual_rate_multiplier IS '用户专属实际扣费倍率（空表示回退到专属展示倍率）';

ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS actual_rate_multiplier DECIMAL(10,4);

COMMENT ON COLUMN usage_logs.actual_rate_multiplier IS '每条 usage log 的实际扣费倍率快照（空表示历史数据或回退到展示倍率）';

UPDATE groups
SET actual_rate_multiplier = rate_multiplier
WHERE actual_rate_multiplier IS NULL;

UPDATE user_group_rate_multipliers
SET actual_rate_multiplier = rate_multiplier
WHERE actual_rate_multiplier IS NULL;
