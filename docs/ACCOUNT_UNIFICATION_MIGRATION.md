# Main / Shop historical account unification

This tool links existing ordinary-user accounts that already exist in both
PostgreSQL databases. It does not merge orders, balances, API keys, roles, or
any other business data, and it never needs plaintext passwords.

## What moves

For one unique, verified, active, non-TOTP ordinary user with the same
normalized email in both systems:

1. The existing Shop bcrypt verifier is copied to Main's
   `users.legacy_shop_password_hash` when it differs from Main's primary
   verifier.
2. Main's trigger advances `credential_version` and emits the normal Shop
   credential event.
3. The Shop row stores Main's stable user ID, switches `auth_authority` to
   `sub2api`, mirrors Main's original verifier in
   `legacy_sub2api_password_hash`, advances its local token epoch, and records
   Main's credential watermark.

After this transition Shop asks Main to verify logins. Main accepts its primary
password and the preserved Shop password. A later password reset on Main clears
the compatibility verifier, so both old passwords stop working and the new
password becomes the only password on both sites.

The two database writes cannot be one distributed transaction. The Main phase
therefore runs first. If the Shop phase fails, rerunning the same plan safely
finishes it; the Main update is compare-and-swap and idempotent.

## Accounts that are never changed automatically

The plan reports but does not mutate:

- Main-only or Shop-only accounts;
- duplicate normalized emails in either database;
- Main administrators or operators;
- disabled or soft-deleted accounts;
- Main accounts with TOTP enabled (Shop has no Main TOTP step-up UI yet);
- unverified Shop email accounts;
- non-bcrypt password verifiers;
- existing ID bindings, legacy verifiers, or credential versions that conflict.

Single-sided account creation should be a later migration cohort after the
matched-pair rollout is proven safe.

## Prerequisites

- Main migrations 193, 194, and 195 are applied.
- Shop account-unification columns and `sub2api_credential_watermarks` exist.
- Both DSNs point to the same explicit environment named by
  `ACCOUNT_UNIFICATION_TARGET`.
- Plan and result paths are new paths. The tool creates them with mode `0600`
  and refuses to overwrite an existing file.

Never put either DSN in command-line flags, logs, a plan, or a Git commit.

## Read-only plan

```bash
export ACCOUNT_UNIFICATION_TARGET=staging
export ACCOUNT_UNIFICATION_MAIN_DSN='...'
export ACCOUNT_UNIFICATION_SHOP_DSN='...'

go run ./cmd/account-unification-migrate plan \
  --out /secure/path/account-unification-staging.json
```

The command prints the plan SHA-256 and category counts. The plan contains
email addresses, IDs, versions, and one-way SHA-256 fingerprints of the bcrypt
strings, but never reusable bcrypt verifiers.

Review every `manual_review` reason before applying anything.

## Bounded apply

Start with one dedicated staging test account:

```bash
go run ./cmd/account-unification-migrate apply \
  --plan /secure/path/account-unification-staging.json \
  --plan-sha256 '<exact digest printed by plan>' \
  --max-users 1 \
  --confirm APPLY_MATCHED_ACCOUNTS_TO_STAGING \
  --result /secure/path/account-unification-staging-result.json
```

`apply` re-reads and locks both rows, validates the plan fingerprints and all
eligibility rules, and stops on the first drift or conflict. Recreate the plan
after any password, email, role, status, TOTP, binding, or authority change.

An all-account apply additionally requires `--all`. A production-labeled plan
also requires `--allow-production` and the exact confirmation
`APPLY_MATCHED_ACCOUNTS_TO_PRODUCTION`. Those gates do not replace a reviewed
backup, staged rollout, production fingerprint checks, or explicit operational
approval.

## Required verification per cohort

For each migrated staging cohort verify all of the following:

- Main's old password logs in to Main and Shop.
- Shop's old password logs in to Main and Shop.
- Changing the password on Main invalidates both old passwords.
- The new password logs in to both sites.
- Previously issued Shop sessions are rejected.
- Disabling the Main account blocks Shop login.
- Duplicate and out-of-order credential events remain idempotent.
