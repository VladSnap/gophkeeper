-- Create secrets table
CREATE TABLE secrets (
    secret_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    encrypted TEXT NOT NULL,
    created_date TIMESTAMP WITH TIME ZONE NOT NULL,
    last_updated_date TIMESTAMP WITH TIME ZONE NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

-- Create index on last_updated_date for faster queries
CREATE INDEX idx_secrets_last_updated_date ON secrets(last_updated_date);

-- Create index on user_id for faster lookups
CREATE INDEX idx_secrets_user_id ON secrets(user_id);
