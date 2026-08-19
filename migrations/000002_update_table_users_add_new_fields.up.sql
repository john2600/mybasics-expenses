-- 0002_add_activated_and_version.up.sql
ALTER TABLE users
    ADD COLUMN activated TINYINT(1)     NOT NULL DEFAULT 0,
    ADD COLUMN version   INT UNSIGNED   NOT NULL DEFAULT 1;