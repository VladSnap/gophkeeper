-- Create users table
CREATE TABLE users (
    user_id UUID PRIMARY KEY,
    login VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL
);

-- Create index on login for faster lookups
CREATE INDEX idx_users_login ON users(login);

-- Create composite index on login and password for authentication queries
CREATE INDEX idx_users_login_password ON users(login, password);
