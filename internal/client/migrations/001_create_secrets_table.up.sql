CREATE TABLE IF NOT EXISTS secrets (
    secret_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    encrypted TEXT NOT NULL,
    created_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_updated_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_secrets_user_id ON secrets(user_id);
CREATE INDEX IF NOT EXISTS idx_secrets_last_updated ON secrets(last_updated_date);
