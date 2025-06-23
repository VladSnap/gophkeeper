-- Create metadata table
CREATE TABLE metadata (
    metadata_id UUID PRIMARY KEY,
    secret_id UUID NOT NULL,
    key VARCHAR(255) NOT NULL,
    value_hash VARCHAR(32) NOT NULL,
    value_encrypted TEXT NOT NULL,
    created_date TIMESTAMP WITH TIME ZONE NOT NULL,
    last_updated_date TIMESTAMP WITH TIME ZONE NOT NULL,
    FOREIGN KEY (secret_id) REFERENCES secrets(secret_id) ON DELETE CASCADE
);

-- Create index on value_hash for faster queries
CREATE INDEX idx_metadata_value_hash ON metadata(value_hash);

-- Create index on last_updated_date for faster queries
CREATE INDEX idx_metadata_last_updated_date ON metadata(last_updated_date);

-- Create index on secret_id for faster lookups
CREATE INDEX idx_metadata_secret_id ON metadata(secret_id);

-- Create composite index on secret_id and key for faster metadata lookups
CREATE INDEX idx_metadata_secret_key ON metadata(secret_id, key);
