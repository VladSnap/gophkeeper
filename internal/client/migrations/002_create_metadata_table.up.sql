CREATE TABLE IF NOT EXISTS metadata (
    metadata_id TEXT PRIMARY KEY,
    secret_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value_hash TEXT NOT NULL,
    value_encrypted TEXT NOT NULL,
    created_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_updated_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (secret_id) REFERENCES secrets(secret_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_metadata_secret_id ON metadata(secret_id);
CREATE INDEX IF NOT EXISTS idx_metadata_last_updated ON metadata(last_updated_date);
