-- ------------------------------------------------------------
-- Table: tokens
-- Stateful, single-use tokens for out-of-band flows (account activation,
-- password reset, stateful auth). Only the SHA-256 hash of the token is stored
-- (BINARY(32)); the plaintext token is sent to the user and never persisted.
-- scope distinguishes the purpose; expiry drives TTL / cleanup.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tokens (
    hash    BINARY(32)      NOT NULL,          -- SHA-256 of the plaintext token
    user_id BIGINT UNSIGNED NOT NULL,          -- owner
    expiry  DATETIME        NOT NULL,          -- valid until
    scope   VARCHAR(50)     NOT NULL,          -- e.g. 'activation', 'authentication', 'password-reset'
    PRIMARY KEY (hash),
    INDEX idx_tokens_user_id (user_id),
    INDEX idx_tokens_expiry  (expiry),
    CONSTRAINT fk_tokens_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
) ENGINE=InnoDB;
