-- Distinguish human JWT sessions from the shared admin API key in the
-- activity audit stream. The masked credential is only a correlation hint;
-- the complete credential is never persisted.
ALTER TABLE admin_audit_logs
  ADD COLUMN IF NOT EXISTS auth_method VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS credential_masked VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_auth_method_created_at
  ON admin_audit_logs (auth_method, created_at DESC);
