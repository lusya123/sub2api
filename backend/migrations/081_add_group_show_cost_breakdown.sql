-- 为分组增加控制普通用户是否显示 usage 费用明细表的开关
ALTER TABLE groups
ADD COLUMN IF NOT EXISTS show_cost_breakdown BOOLEAN NOT NULL DEFAULT TRUE;
