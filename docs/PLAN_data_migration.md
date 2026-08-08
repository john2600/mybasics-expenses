# Plan — Import legacy myexpenses data into mybasics-expenses

> Status: **Ready. Script validated end-to-end in a disposable container; not yet
> run against the live instance.**
> Goal: bring the real financial data from the old **myexpenses** instance
> (categories, ~655 movements, income config) into the current
> **mybasics-expenses** database, attaching everything to a single user.

---

## Decisions (agreed)

| Topic | Decision |
|---|---|
| Target user | **`johntest`** (resolved by username, currently id `2`) |
| Categories | **Option A** — adopt the legacy categories verbatim (keep their ids), so `movements.category_id` maps 1:1 with no remapping |
| Mail columns | **Keep** `mail_uid` + `mail_message_id` in the current schema (nullable). They power future email-ingestion dedup ("if the message already exists, don't duplicate"). Manual movements leave them `NULL` |
| Source data | `myexpenses/backups/myexpenses_backup_20260807_001257.sql` |

---

## Context — why the tables diverged

The two apps evolved separately: the **old** schema had no concept of a user, and
the movements table carried email-ingestion columns. The **current** schema added
`user_id` (per-user scoping) but has no data yet.

| Table | Old (myexpenses) | Current (mybasics) | Gap to close |
|---|---|---|---|
| `categories` | 18 rows, ids 1–18, no owner | 11 seed rows, ids 1–11, shared | different ids/names → **replace with legacy (Option A)** |
| `movements` | ~655 rows, `mail_uid`+`mail_message_id`, **no `user_id`** | empty, has `user_id`, **no mail cols** | add `user_id`, **add mail cols to current** |
| `income_config_history` | ~3 rows, `UNIQUE(year_month)` | empty, `UNIQUE(user_id, year_month)` | add `user_id` |
| `income_config` (single row) | exists | does not exist | ignored — the history's latest row is the effective config |

---

## Schema change required (before the data import)

The current `movements` table must gain the two mail columns and the dedup key.
Apply it in **two places**:

1. **`migrations/001_init.sql`** — so fresh installs get the columns. Inside the
   `movements` table definition, add:
   ```sql
   mail_uid        BIGINT UNSIGNED  NULL,
   mail_message_id VARCHAR(255)     NULL,
   -- ...and in the keys section:
   UNIQUE KEY uq_mail_source (mail_uid, mail_message_id),
   ```
   > Note: MySQL treats `NULL`s as distinct in a UNIQUE key, so many manual
   > movements with `(NULL, NULL)` coexist fine; only real mail ids dedup.

2. **The running instance** — its `movements` table already exists (empty), so it
   won't pick up the change from `001_init.sql`. The migration script below runs
   an `ALTER TABLE` to add the columns (safe on an empty table). This respects the
   data-safety rule: **no volume wipe.**

---

## Migration steps (runbook)

Nothing here recreates the volume; the only writes are the import itself.

> **The old `myexpenses` app is never touched.** It runs in its own container
> (`myexpenses_db`, volume `myexpenses_db_data`, port 3308/8081) and stays fully
> intact — we never connect to it. All legacy data comes from the **backup `.sql`
> file**, which we load into a **throwaway staging schema** named `legacy_import`
> *inside the mybasics server*. "Dropping" that staging schema removes only this
> temporary copy, not the old app.

- **Step 0 — Back up the current DB first** (data-safety rule):
  ```bash
  docker exec mybasics_expenses_db sh -c \
    'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --routines --triggers \
     --databases mybasics_expenses' > ~/mybasics_pre_import_$(date +%Y%m%d_%H%M%S).sql
  ```

- **Step 1 — Load the legacy backup into a throwaway staging schema**
  (`legacy_import`) on the mybasics server. We strip any `CREATE DATABASE`/`USE`
  lines from the dump so it lands in `legacy_import` regardless of the dump's
  original database name — never as the live `myexpenses`:
  ```bash
  docker exec mybasics_expenses_db sh -c \
    'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "CREATE DATABASE IF NOT EXISTS legacy_import"'

  grep -vE '^(CREATE DATABASE|USE )' \
    myexpenses/backups/myexpenses_backup_20260807_001257.sql \
    | docker exec -i mybasics_expenses_db sh -c \
        'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" legacy_import'
  ```

- **Step 2 — Run the import script** (below), which does the ALTER + the
  `INSERT…SELECT`s inside a transaction, then verifies counts.

- **Step 3 — Drop the staging schema** once verified (this removes only the
  temporary copy created in Step 1, not the old app):
  ```sql
  DROP DATABASE legacy_import;
  ```

---

## The SQL script (annotated)

**Canonical, runnable copy: [`scripts/import_legacy_data.sql`](../scripts/import_legacy_data.sql)** —
the block below mirrors it for reference. Validated end-to-end against the real
backup in a disposable MySQL container: categories = 18, movements = 655
(635 with a `mail_uid`, 20 manual), 0 orphans, 0 without an owner.

```sql
-- ============================================================
-- Legacy data import: myexpenses (staging) -> mybasics_expenses
-- Attaches all data to a single user (johntest) and preserves the
-- mail_uid / mail_message_id columns for future email ingestion.
--
-- Prereqs:
--   1. mybasics_expenses has been backed up (Step 0).
--   2. The old backup is loaded into the `legacy_import` staging schema (Step 1).
-- ============================================================

USE mybasics_expenses;

-- --- Resolve the target user by username (robust against id changes) --------
SET @user_id := (SELECT id FROM users WHERE username = 'johntest');
-- Safety: verify this printed a real id before continuing.
SELECT @user_id AS target_user_id;   -- must NOT be NULL

-- --- Step A: add the mail columns + dedup key to the current movements -------
-- Idempotent: only ALTER when the columns are missing (handles both the existing
-- instance and a fresh volume where 001_init.sql already added them). DDL auto-commits.
SET @needs_mail_cols := (
    SELECT COUNT(*) = 0 FROM information_schema.columns
    WHERE table_schema = 'mybasics_expenses'
      AND table_name = 'movements' AND column_name = 'mail_uid');
SET @ddl := IF(@needs_mail_cols,
    'ALTER TABLE movements
        ADD COLUMN mail_uid        BIGINT UNSIGNED NULL AFTER hour,
        ADD COLUMN mail_message_id VARCHAR(255)    NULL AFTER mail_uid,
        ADD UNIQUE KEY uq_mail_source (mail_uid, mail_message_id)',
    'SELECT ''mail columns already present, skipping ALTER'' AS note');
PREPARE alter_stmt FROM @ddl; EXECUTE alter_stmt; DEALLOCATE PREPARE alter_stmt;

-- --- Step B: the data import, all-or-nothing --------------------------------
START TRANSACTION;

-- B1. Categories (Option A): adopt legacy categories verbatim, keeping ids.
--     movements is empty, so clearing categories violates no FK.
DELETE FROM movements;    -- ensure empty (defensive)
DELETE FROM categories;   -- drop the current seed
INSERT INTO categories (id, name, description, color, created_at, updated_at)
SELECT id, name, description, color, created_at, updated_at
FROM legacy_import.categories;

-- B2. Movements: copy every legacy row; add user_id; KEEP mail columns.
--     category_id is preserved as-is (1:1 thanks to B1).
INSERT INTO movements
    (id, user_id, category_id, type, amount, description, date, hour,
     mail_uid, mail_message_id, created_at, updated_at)
SELECT
    id, @user_id, category_id, type, amount, description, date, hour,
    mail_uid, mail_message_id, created_at, updated_at
FROM legacy_import.movements;

-- B3. Income config history: copy rows; add user_id (let id regenerate).
--     ON DUPLICATE guards against a config johntest may already have set.
INSERT INTO income_config_history
    (user_id, `year_month`, amount, cut_day, description, created_at)
SELECT
    @user_id, `year_month`, amount, cut_day, description, created_at
FROM legacy_import.income_config_history
ON DUPLICATE KEY UPDATE
    amount      = VALUES(amount),
    cut_day     = VALUES(cut_day),
    description = VALUES(description);

COMMIT;

-- --- Step C: verification ---------------------------------------------------
SELECT 'categories'            AS tbl, COUNT(*) AS rows_now FROM categories
UNION ALL
SELECT 'movements (this user)', COUNT(*) FROM movements WHERE user_id = @user_id
UNION ALL
SELECT 'income_config_history', COUNT(*) FROM income_config_history WHERE user_id = @user_id;

-- Sanity: no movement should point to a missing category (must be 0).
SELECT COUNT(*) AS orphan_movements
FROM movements m
LEFT JOIN categories c ON c.id = m.category_id
WHERE c.id IS NULL;

-- Sanity: every imported movement is owned by the target user (must be 0).
SELECT COUNT(*) AS movements_without_owner
FROM movements
WHERE user_id IS NULL OR user_id <> @user_id;
```

### What each `INSERT…SELECT` does (plain language)
- **B1** copies the 18 legacy categories *with their original ids* → so a movement
  whose `category_id = 7` still means the same category it meant in the old app.
- **B2** copies all ~655 movements in one statement, stamping `@user_id` (johntest)
  into the new `user_id` column and carrying the mail columns across unchanged.
- **B3** copies the income config history, stamping the same `user_id`.

---

## Verification expectations (confirmed by the dry-run)
- `categories` = 18
- `movements (this user)` = 655 — of which 635 carry a `mail_uid` and 20 are manual (`NULL`)
- `income_config_history (this user)` = whatever the source holds (1 in this backup)
- `orphan_movements` = 0
- `movements_without_owner` = 0

---

## Rollback
If anything looks wrong, restore the Step 0 backup:
```bash
docker exec -i mybasics_expenses_db sh -c \
  'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" mybasics_expenses' < ~/mybasics_pre_import_<ts>.sql
```
The import writes (B1–B3) are wrapped in a single transaction, so a failure mid-way
rolls back on its own; the Step 0 dump covers the DDL (`ALTER`) too.

---

## Decisions (confirmed)
1. **`income_config` (single-row)** — **ignored**; the history's latest row is
   the effective config.
2. **Keep legacy movement ids** — **yes** (preserves references; the table is
   empty so no collisions; auto_increment continues from the max).
3. **`001_init.sql`** — mail columns + `uq_mail_source` **added** so fresh
   installs get them (validated by loading the migration in a clean container).

## Still to do before the real run
- **Back up** the running mybasics DB (Step 0) — mandatory per the data-safety rule.
- Confirm `johntest` exists in the target DB (the script resolves the id by
  username and prints it — it must not be `NULL`).
- Keep produced dumps out of git (they go to `$HOME`, outside the repo).
