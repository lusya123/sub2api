-- Durable, transactionally-enqueued Shop credential-version notifications.
--
-- Passwords and password hashes never enter this table. The outbox contains
-- only the monotonic credential/authorization version required to revoke Shop
-- sessions after password, email-identity, account-status, or TOTP-policy
-- changes.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS credential_version BIGINT NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_credential_version_positive'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_credential_version_positive
            CHECK (credential_version > 0);
    END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS shop_credential_event_outbox (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL CHECK (user_id > 0),
    credential_version BIGINT NOT NULL CHECK (credential_version > 0),
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts           INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error         TEXT,
    claimed_at         TIMESTAMPTZ,
    claimed_by         TEXT,
    CONSTRAINT shop_credential_event_outbox_user_version_unique
        UNIQUE (user_id, credential_version)
);

CREATE INDEX IF NOT EXISTS idx_shop_credential_event_outbox_available
    ON shop_credential_event_outbox (available_at, id)
    WHERE claimed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_shop_credential_event_outbox_lease
    ON shop_credential_event_outbox (claimed_at)
    WHERE claimed_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shop_credential_event_outbox_created_at
    ON shop_credential_event_outbox (created_at);

CREATE OR REPLACE FUNCTION bump_user_credential_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.password_hash IS DISTINCT FROM NEW.password_hash
       OR OLD.legacy_shop_password_hash IS DISTINCT FROM NEW.legacy_shop_password_hash
       OR OLD.email IS DISTINCT FROM NEW.email
       OR OLD.status IS DISTINCT FROM NEW.status
       OR OLD.totp_enabled IS DISTINCT FROM NEW.totp_enabled THEN
        IF OLD.credential_version = 9223372036854775807 THEN
            RAISE EXCEPTION 'users.credential_version exhausted for user %', OLD.id;
        END IF;
        -- Ignore any caller-supplied value. Hash changes advance exactly once.
        NEW.credential_version := OLD.credential_version + 1;
    ELSIF OLD.credential_version IS DISTINCT FROM NEW.credential_version THEN
        RAISE EXCEPTION 'users.credential_version is trigger-managed';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_bump_credential_version ON users;
CREATE TRIGGER trg_users_bump_credential_version
BEFORE UPDATE OF password_hash, legacy_shop_password_hash, email, status, totp_enabled, credential_version ON users
FOR EACH ROW EXECUTE FUNCTION bump_user_credential_version();

CREATE OR REPLACE FUNCTION enqueue_shop_credential_event()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.credential_version IS DISTINCT FROM NEW.credential_version THEN
        INSERT INTO shop_credential_event_outbox (user_id, credential_version, occurred_at)
        VALUES (NEW.id, NEW.credential_version, NOW())
        ON CONFLICT (user_id, credential_version) DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_enqueue_shop_credential_event ON users;
CREATE TRIGGER trg_users_enqueue_shop_credential_event
AFTER UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION enqueue_shop_credential_event();

COMMENT ON COLUMN users.credential_version IS
    'Monotonic trigger-managed credential/authorization version; starts at 1';

COMMENT ON TABLE shop_credential_event_outbox IS
    'Durable Shop session-revocation events; contains no password or password hash';
