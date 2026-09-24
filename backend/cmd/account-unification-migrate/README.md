# Account-unification migration

This tool reads both databases and writes a private (0600) plan containing
identity IDs and one-way password fingerprints, never reusable password hashes.
Run `plan` only after the Main 193–195 and Shop account-unification schema
migrations are present. Rebuild the plan if either site changes materially.

`apply` has two separate cohorts:

- `--cohort matched` (the default) merges existing accounts present at both
  sites. Its confirmation is `APPLY_MATCHED_ACCOUNTS_TO_<TARGET>`.
- `--cohort shop-only` creates a separate Main identity for each eligible
  Shop-only account. Its confirmation is
  `CREATE_SHOP_ONLY_MAIN_ACCOUNTS_IN_<TARGET>`. The Main identity starts with
  zero balance, no subscription or API key, and Main's normal concurrency
  default. Shop's balance, orders, and other Shop data stay in Shop.

The Shop-only cohort is opt-in so an existing matched-account command cannot
accidentally create users. Use `--max-users 1` for the first canary; `--all`
requires a separately reviewed plan. Production additionally requires
`--allow-production`. Both database connection strings and the target label
must be supplied explicitly through environment variables. Do not put them in
the plan, command history, reports, or source control.

Shop-only creation is fail-closed across the two databases: the new Main row
remains disabled until Shop has adopted its Main ID. A failed Shop promotion
or interrupted run can leave a disabled Main mirror. Retrying the same plan
can finish an unchanged mirror; if source credentials or identity changed,
stop and review the exception instead of deleting or overwriting any account.
Neither cohort changes database containers or transfers balances or orders.
