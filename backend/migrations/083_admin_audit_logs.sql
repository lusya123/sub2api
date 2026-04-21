CREATE TABLE IF NOT EXISTS admin_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  actor_user_id BIGINT NOT NULL,
  actor_email VARCHAR(255) NOT NULL DEFAULT '',
  actor_role VARCHAR(20) NOT NULL DEFAULT '',
  method VARCHAR(12) NOT NULL DEFAULT '',
  route_template VARCHAR(255) NOT NULL DEFAULT '',
  path VARCHAR(512) NOT NULL DEFAULT '',
  module VARCHAR(64) NOT NULL DEFAULT '',
  action VARCHAR(128) NOT NULL DEFAULT '',
  action_type VARCHAR(32) NOT NULL DEFAULT '',
  target_type VARCHAR(64) NOT NULL DEFAULT '',
  target_id BIGINT,
  status_code INT NOT NULL DEFAULT 0,
  success BOOLEAN NOT NULL DEFAULT false,
  error_code VARCHAR(128) NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  ip_address VARCHAR(128) NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  query_params JSONB NOT NULL DEFAULT '{}'::jsonb,
  request_body JSONB NOT NULL DEFAULT '{}'::jsonb,
  duration_ms BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_created_at_id
  ON admin_audit_logs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_actor_created_at
  ON admin_audit_logs (actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_actor_role_created_at
  ON admin_audit_logs (actor_role, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_module_action_created_at
  ON admin_audit_logs (module, action_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_target_created_at
  ON admin_audit_logs (target_type, target_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_success_created_at
  ON admin_audit_logs (success, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_text_search
  ON admin_audit_logs USING GIN (
    to_tsvector(
      'simple',
      COALESCE(summary, '') || ' ' ||
      COALESCE(action, '') || ' ' ||
      COALESCE(route_template, '') || ' ' ||
      COALESCE(path, '') || ' ' ||
      COALESCE(error_message, '') || ' ' ||
      COALESCE(target_type, '') || ' ' ||
      COALESCE(request_body::text, '') || ' ' ||
      COALESCE(query_params::text, '')
    )
  );
