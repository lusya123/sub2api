-- Preserve the Shop password verifier while the two account systems are
-- unified. The column is nullable and backward-compatible: existing users
-- continue to authenticate only against users.password_hash until an explicit
-- backfill supplies a distinct Shop bcrypt verifier.
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS legacy_shop_password_hash VARCHAR(255);
