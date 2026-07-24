-- ============================================================
-- MyBasics-Expenses – Definitive Schema
-- ============================================================
-- Single source of truth for structure and base seed data.
-- The database starts clean: base categories and one baseline
-- income-config row, but NO sample movements.
-- ============================================================

CREATE DATABASE IF NOT EXISTS mybasics_expenses
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE mybasics_expenses;

-- ------------------------------------------------------------
-- Table: categories
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS categories (
    id          BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    name        VARCHAR(100)     NOT NULL,
    description VARCHAR(255)     NOT NULL DEFAULT '',
    color       VARCHAR(7)       NOT NULL DEFAULT '#6B7280', -- hex colour for UI
    created_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_categories_name (name)
) ENGINE=InnoDB;

-- ------------------------------------------------------------
-- Table: movements
-- type: 'I' = ingreso (income), 'E' = egreso (expense)
-- hour: optional time of day for the movement (HH:MM:SS)
-- The user creates every movement manually via the API.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS movements (
    id          BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    category_id BIGINT UNSIGNED  NOT NULL,
    user_id     BIGINT UNSIGNED  NULL,               -- owner; no FK by design (orphans tolerated)
    type        CHAR(1)          NOT NULL DEFAULT 'E',
    amount      DECIMAL(12, 2)   NOT NULL,
    description TEXT             NOT NULL,
    date        DATE             NOT NULL,
    hour        TIME             NULL,
    created_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    CONSTRAINT chk_movements_type CHECK (type IN ('I', 'E')),
    INDEX idx_movements_date        (date),
    INDEX idx_movements_category_id (category_id),
    INDEX idx_movements_user_id     (user_id),
    INDEX idx_movements_type        (type),
    CONSTRAINT fk_movements_category
        FOREIGN KEY (category_id) REFERENCES categories (id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
) ENGINE=InnoDB;

-- ------------------------------------------------------------
-- Table: income_config_history
-- Versioned fixed monthly income. Each row is valid from its
-- year_month forward until a newer row exists.
-- Query: WHERE `year_month` <= ? ORDER BY `year_month` DESC LIMIT 1
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS income_config_history (
    id           INT UNSIGNED     NOT NULL AUTO_INCREMENT,
    `year_month` DATE             NOT NULL,          -- stored as first day of month
    amount       DECIMAL(12, 2)   NOT NULL,
    cut_day      TINYINT UNSIGNED NOT NULL DEFAULT 24,
    description  VARCHAR(255)     NOT NULL DEFAULT 'Ingreso fijo',
    created_at   DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_year_month (`year_month`),
    CONSTRAINT chk_ich_cut_day CHECK (cut_day BETWEEN 1 AND 28)
) ENGINE=InnoDB;

-- ------------------------------------------------------------
-- Table: users
-- App accounts. Passwords are stored as bcrypt hashes (60 chars),
-- produced by User.Normalize before insert — never plaintext.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id              BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    username        VARCHAR(255)     NOT NULL,
    name            VARCHAR(255)     NOT NULL,
    email           VARCHAR(255)     NOT NULL,
    hashed_password CHAR(60)         NOT NULL,          -- bcrypt hash
    created_at      DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_username (username),
    UNIQUE KEY uq_users_email    (email)
) ENGINE=InnoDB;

-- ------------------------------------------------------------
-- Table: sessions
-- Server-side session store for alexedwards/scs (mysqlstore). Schema is
-- dictated by the library: token PK, gob-encoded data, expiry for GC.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sessions (
    token  CHAR(43)     PRIMARY KEY,
    data   BLOB         NOT NULL,
    expiry TIMESTAMP(6) NOT NULL
) ENGINE=InnoDB;

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- ------------------------------------------------------------
-- Seed: base categories (Spanish). No movements are seeded.
-- ------------------------------------------------------------
INSERT INTO categories (name, description, color) VALUES
    ('Alimentacion',        'Supermercado, comida y bebidas',          '#16A34A'),
    ('Restaurante',         'Restaurantes y comidas fuera',            '#EA580C'),
    ('Transporte',          'Combustible, transporte y viajes cortos', '#3B82F6'),
    ('Vivienda',            'Arriendo, servicios y hogar',             '#8B5CF6'),
    ('Salud',               'Farmacia, medico y seguros de salud',     '#10B981'),
    ('Entretenimiento',     'Cine, streaming, hobbies',                '#EC4899'),
    ('Educacion',           'Cursos, libros y suscripciones',          '#06B6D4'),
    ('Ropa',                'Ropa y accesorios',                       '#F97316'),
    ('Viajes',              'Hoteles, vuelos y vacaciones',            '#EF4444'),
    ('Ahorros e Inversiones', 'Fondo de emergencia e inversiones',     '#14B8A6'),
    ('Otros',               'Gastos varios',                           '#6B7280')
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- ------------------------------------------------------------
-- Seed: baseline income config so GET /income always returns a row.
-- Neutral starting value (0). Update it via the API.
-- ------------------------------------------------------------
INSERT INTO income_config_history (`year_month`, amount, cut_day, description)
VALUES (DATE_FORMAT(NOW(), '%Y-%m-01'), 0.00, 24, 'Ingreso fijo')
ON DUPLICATE KEY UPDATE amount = VALUES(amount);
