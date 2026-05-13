ALTER TABLE IF EXISTS model_marketplace_monitors
  ADD COLUMN IF NOT EXISTS model_call_configs JSONB NOT NULL DEFAULT '{}'::jsonb;
