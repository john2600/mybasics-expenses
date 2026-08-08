-- ============================================================
-- Legacy data import: myexpenses (backup) -> mybasics_expenses
-- ------------------------------------------------------------
-- Brings the real financial data from the old myexpenses instance into the
-- current database, attaching everything to a single user (johntest), and
-- preserving the mail_uid / mail_message_id columns for future email ingestion.
--
-- This script NEVER touches the old myexpenses app. All legacy data comes from a
-- throwaway staging schema named `legacy_import`, loaded from the backup .sql
-- file on the mybasics MySQL server (see docs/PLAN_data_migration.md).
--
-- Prereqs (see the runbook in docs/PLAN_data_migration.md):
--   0. mybasics_expenses has been backed up with mysqldump.
--   1. The legacy backup is loaded into the `legacy_import` staging schema.
--
-- Run:  docker exec -i mybasics_expenses_db sh -c \
--         'mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < scripts/import_legacy_data.sql
-- ============================================================

USE mybasics_expenses;

-- --- Resolve the target user by username (robust against id changes) --------
SET @user_id := (SELECT id FROM users WHERE username = 'johntest');
-- Safety: this must print a real id. If it is NULL, stop — the user is missing.
SELECT @user_id AS target_user_id;

-- --- Step A: add the mail columns + dedup key to the current movements -------
-- Idempotent: only ALTER when the columns are missing. This handles both the
-- existing instance (created before the schema change -> columns absent) and a
-- fresh volume where 001_init.sql already created them (-> skip). DDL auto-commits.
SET @needs_mail_cols := (
    SELECT COUNT(*) = 0
    FROM information_schema.columns
    WHERE table_schema = 'mybasics_expenses'
      AND table_name   = 'movements'
      AND column_name  = 'mail_uid'
);
SET @ddl := IF(@needs_mail_cols,
    'ALTER TABLE movements
        ADD COLUMN mail_uid        BIGINT UNSIGNED NULL AFTER hour,
        ADD COLUMN mail_message_id VARCHAR(255)    NULL AFTER mail_uid,
        ADD UNIQUE KEY uq_mail_source (mail_uid, mail_message_id)',
    'SELECT ''mail columns already present, skipping ALTER'' AS note');
PREPARE alter_stmt FROM @ddl;
EXECUTE alter_stmt;
DEALLOCATE PREPARE alter_stmt;

-- --- Step B: the data import, all-or-nothing --------------------------------
START TRANSACTION;

-- B1. Categories (Option A): adopt legacy categories verbatim, keeping their ids
--     so movements.category_id maps 1:1. movements is empty -> clearing
--     categories violates no foreign key.
DELETE FROM movements;    -- ensure empty (defensive)
DELETE FROM categories;   -- drop the current seed
INSERT INTO categories (id, name, description, color, created_at, updated_at)
SELECT id, name, description, color, created_at, updated_at
FROM legacy_import.categories;

-- B2. Movements: copy every legacy row; stamp user_id; KEEP mail columns.
--     Legacy movement ids are preserved (table is empty -> no collisions).
INSERT INTO movements
    (id, user_id, category_id, type, amount, description, date, hour,
     mail_uid, mail_message_id, created_at, updated_at)
SELECT
    id, @user_id, category_id, type, amount, description, date, hour,
    mail_uid, mail_message_id, created_at, updated_at
FROM legacy_import.movements;

-- B3. Income config history: copy rows; stamp user_id (let id regenerate).
--     ON DUPLICATE guards against a config johntest may already have set.
--     The legacy single-row `income_config` table is intentionally ignored:
--     the latest history row is the effective config.
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
-- Expected: categories = 18, movements ≈ 655, income_config_history = 3.
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

-- Sanity: every movement is owned by the target user (must be 0).
SELECT COUNT(*) AS movements_without_owner
FROM movements
WHERE user_id IS NULL OR user_id <> @user_id;

-- ------------------------------------------------------------
-- After verifying, drop the staging schema (removes only the temporary copy,
-- not the old app):
--   DROP DATABASE legacy_import;
-- ------------------------------------------------------------
