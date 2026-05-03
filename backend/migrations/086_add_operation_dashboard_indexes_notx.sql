-- Indexes for the admin operation analytics dashboard.
-- This migration intentionally runs outside a transaction because PostgreSQL
-- requires CREATE INDEX CONCURRENTLY to be executed as a standalone statement.
-- The migration runner recognizes *_notx.sql and executes each statement one by one.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at_not_deleted
    ON users (created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_user_created_at_actual_cost
    ON usage_logs (user_id, created_at, actual_cost);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_at_user_api_key
    ON usage_logs (created_at, user_id, api_key_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_at_group_user
    ON usage_logs (created_at, group_id, user_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_at_model_user
    ON usage_logs (created_at, requested_model, model, user_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_at_actual_cost
    ON usage_logs (created_at, actual_cost);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_redeem_codes_used_at_used_by
    ON redeem_codes (used_at, used_by)
    WHERE used_at IS NOT NULL AND used_by IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_redeem_codes_used_at_type_value
    ON redeem_codes (used_at, type, value)
    WHERE used_at IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_redeem_codes_used_by_used_at_type
    ON redeem_codes (used_by, used_at, type)
    WHERE used_at IS NOT NULL AND used_by IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_promo_code_usages_used_at
    ON promo_code_usages (used_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_subscriptions_created_at_not_deleted
    ON user_subscriptions (created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_subscriptions_status_expires_at_not_deleted
    ON user_subscriptions (status, expires_at)
    WHERE deleted_at IS NULL;
