-- Create sessions table
CREATE TABLE sessions (
    session_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    is_active BOOLEAN NOT NULL,
    last_sync_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

-- Create index on last_sync_date for faster queries
CREATE INDEX idx_sessions_last_sync_date ON sessions(last_sync_date);

-- Create index on user_id for faster lookups
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- Create index on expires_at for cleanup queries
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
