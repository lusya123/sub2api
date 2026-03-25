ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS is_trial BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_is_trial
    ON redeem_codes (is_trial);

CREATE UNIQUE INDEX IF NOT EXISTS uq_redeem_codes_trial_used_by
    ON redeem_codes (used_by)
    WHERE is_trial = TRUE AND status = 'used' AND used_by IS NOT NULL;
