-- Store per-model display names for the model marketplace.
-- Keys are real callable model IDs; values can contain zh/en names.

ALTER TABLE IF EXISTS model_marketplace_monitors
  ADD COLUMN IF NOT EXISTS model_display_names JSONB NOT NULL DEFAULT '{}'::jsonb;
