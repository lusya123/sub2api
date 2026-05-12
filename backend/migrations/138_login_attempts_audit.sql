CREATE TABLE IF NOT EXISTS login_attempts (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    email VARCHAR(255) NOT NULL,
    ip VARCHAR(64),
    x_forwarded_for TEXT,
    user_agent TEXT,
    fingerprint VARCHAR(64),
    result VARCHAR(32),
    duration_ms INT
);

CREATE INDEX IF NOT EXISTS login_attempts_email_time_idx
    ON login_attempts (email, created_at DESC);

CREATE INDEX IF NOT EXISTS login_attempts_ip_time_idx
    ON login_attempts (ip, created_at DESC);

CREATE INDEX IF NOT EXISTS login_attempts_time_idx
    ON login_attempts (created_at DESC);

CREATE INDEX IF NOT EXISTS login_attempts_result_time_idx
    ON login_attempts (result, created_at DESC);
