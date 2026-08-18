-- 0002_add_activated_and_version.down.sql
ALTER TABLE users
    DROP COLUMN version,
    DROP COLUMN activated;