-- Snapshot the user-visible cost-breakdown switch onto each usage record so
-- historical usage rows stay stable after group settings change.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS show_cost_breakdown BOOLEAN;

UPDATE usage_logs AS ul
SET show_cost_breakdown = g.show_cost_breakdown
FROM groups AS g
WHERE ul.show_cost_breakdown IS NULL
  AND ul.group_id IS NOT NULL
  AND g.id = ul.group_id;

UPDATE usage_logs
SET show_cost_breakdown = TRUE
WHERE show_cost_breakdown IS NULL;
