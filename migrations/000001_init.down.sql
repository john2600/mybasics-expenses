-- Reverse of 000001_init.up.sql. Dropped in FK-safe order: movements (child)
-- before categories (parent). DESTRUCTIVE — only runs on an explicit
-- `migrate down`, never automatically.
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS income_config_history;
DROP TABLE IF EXISTS movements;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;
