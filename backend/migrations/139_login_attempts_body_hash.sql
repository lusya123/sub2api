ALTER TABLE login_attempts
    ADD COLUMN IF NOT EXISTS body_hash VARCHAR(16);

CREATE INDEX IF NOT EXISTS login_attempts_body_hash_time_idx
    ON login_attempts (body_hash, created_at DESC);
