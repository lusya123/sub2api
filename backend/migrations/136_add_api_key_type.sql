ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS type VARCHAR(16) NOT NULL DEFAULT 'user';

UPDATE api_keys SET type = 'user' WHERE type IS NULL OR type = '';

CREATE INDEX IF NOT EXISTS idx_api_keys_user_type ON api_keys(user_id, type);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_chat_user_group
    ON api_keys(user_id, group_id, type)
    WHERE type = 'chat' AND group_id IS NOT NULL AND deleted_at IS NULL;
