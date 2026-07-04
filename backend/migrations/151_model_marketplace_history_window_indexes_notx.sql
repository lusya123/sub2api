-- 151_model_marketplace_history_window_indexes_notx.sql
-- 用户侧模型广场按时间窗口计算 7d/15d/30d 可用率时，会按 monitor_id + checked_at 过滤历史明细。
-- 原索引 (monitor_id, model, checked_at DESC) 更适合单模型最新状态/时间线，不适合跨模型时间窗口聚合。
-- 非事务迁移（_notx）：CREATE INDEX CONCURRENTLY 不可在事务内执行。
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_model_marketplace_monitor_histories_monitor_checked_model
    ON model_marketplace_monitor_histories (monitor_id, checked_at DESC, model)
    INCLUDE (status, latency_ms);
