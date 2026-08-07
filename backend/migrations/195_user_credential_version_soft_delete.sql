-- Account soft deletion/restoration and role transitions are authorization
-- changes. Advance the Main credential version so Shop revokes any JWT issued
-- before those transitions. A legacy Shop verifier belongs only to an exact
-- ordinary-user identity, so every role transition retires it permanently.
-- This follow-up intentionally preserves migration 194's checksum for
-- environments where that migration has already been applied.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_legacy_shop_password_ordinary_role'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_legacy_shop_password_ordinary_role
            CHECK (role = 'user' OR legacy_shop_password_hash IS NULL);
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION bump_user_credential_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.role IS DISTINCT FROM NEW.role
       OR NEW.role IS DISTINCT FROM 'user' THEN
        NEW.legacy_shop_password_hash := NULL;
    END IF;

    IF OLD.password_hash IS DISTINCT FROM NEW.password_hash
       OR OLD.legacy_shop_password_hash IS DISTINCT FROM NEW.legacy_shop_password_hash
       OR OLD.email IS DISTINCT FROM NEW.email
       OR OLD.status IS DISTINCT FROM NEW.status
       OR OLD.totp_enabled IS DISTINCT FROM NEW.totp_enabled
       OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
       OR OLD.role IS DISTINCT FROM NEW.role THEN
        IF OLD.credential_version = 9223372036854775807 THEN
            RAISE EXCEPTION 'users.credential_version exhausted for user %', OLD.id;
        END IF;
        -- Ignore caller-supplied values. Relevant auth changes advance once.
        NEW.credential_version := OLD.credential_version + 1;
    ELSIF OLD.credential_version IS DISTINCT FROM NEW.credential_version THEN
        RAISE EXCEPTION 'users.credential_version is trigger-managed';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_bump_credential_version ON users;
CREATE TRIGGER trg_users_bump_credential_version
BEFORE UPDATE OF password_hash, legacy_shop_password_hash, email, status, totp_enabled, deleted_at, role, credential_version ON users
FOR EACH ROW EXECUTE FUNCTION bump_user_credential_version();
