package config

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the client application configuration
type Config struct {
	DatabasePath string
	DataDir      string
	UserDataDir  string // Directory specific to current user
	LogLevel     string
	ServerURL    string
	Username     string // Current logged in username
}

// NewConfig creates a new configuration with default values
func NewConfig() (*Config, error) {
	dataDir, err := getDataDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to get data directory: %w", err)
	}

	config := &Config{
		DataDir:   dataDir,
		LogLevel:  "info",
		ServerURL: "http://localhost:8080", // Default server URL
	}

	// Override with environment variables if present
	if serverURL := os.Getenv("GOPHKEEPER_SERVER_URL"); serverURL != "" {
		config.ServerURL = serverURL
	}

	if logLevel := os.Getenv("GOPHKEEPER_LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	if dbPath := os.Getenv("GOPHKEEPER_DB_PATH"); dbPath != "" {
		config.DatabasePath = dbPath
	}

	return config, nil
}

// getDataDirectory returns the data directory for the application
func getDataDirectory() (string, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create application data directory
	dataDir := filepath.Join(homeDir, ".gophkeeper")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	return dataDir, nil
}

// SetUser configures the config for a specific user
func (c *Config) SetUser(username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	c.Username = username

	// Create user-specific directory using username hash for security
	userDirName := getUserDirName(username)
	c.UserDataDir = filepath.Join(c.DataDir, "users", userDirName)

	// Create user directory if it doesn't exist
	if err := os.MkdirAll(c.UserDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create user data directory: %w", err)
	}

	// Set user-specific database path
	c.DatabasePath = filepath.Join(c.UserDataDir, "user.db")

	return nil
}

// GetUserDataDir returns the data directory for the current user
func (c *Config) GetUserDataDir() string {
	return c.UserDataDir
}

// GetMasterPasswordFile returns the path to the master password file for current user
func (c *Config) GetMasterPasswordFile() string {
	if c.UserDataDir == "" {
		return ""
	}
	return filepath.Join(c.UserDataDir, "master.key")
}

// GetTokenFile returns the path to the token file for current user
func (c *Config) GetTokenFile() string {
	if c.UserDataDir == "" {
		return ""
	}
	return filepath.Join(c.UserDataDir, "token.enc")
}

// getUserDirName creates a safe directory name from username
// Uses MD5 hash to avoid filesystem issues with special characters
func getUserDirName(username string) string {
	hash := md5.Sum([]byte(username))
	return fmt.Sprintf("%x", hash)
}

// GetUsernameFromDirName retrieves username from user directory name
func (c *Config) GetUsernameFromDirName(dirName string) (string, error) {
	// Read username from user.info file in the directory
	userInfoPath := filepath.Join(c.DataDir, "users", dirName, "user.info")
	data, err := os.ReadFile(userInfoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read user info: %w", err)
	}
	return string(data), nil
}

// SaveUserInfo saves username to user.info file
func (c *Config) SaveUserInfo(username string) error {
	if c.UserDataDir == "" {
		return fmt.Errorf("user data directory not set")
	}

	userInfoPath := filepath.Join(c.UserDataDir, "user.info")
	if err := os.WriteFile(userInfoPath, []byte(username), 0644); err != nil {
		return fmt.Errorf("failed to save user info: %w", err)
	}
	return nil
}

// ListUsers returns a list of users that have data stored locally
func (c *Config) ListUsers() ([]string, error) {
	usersDir := filepath.Join(c.DataDir, "users")

	// Check if users directory exists
	if _, err := os.Stat(usersDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read users directory: %w", err)
	}

	var users []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if this directory has a user database
			dbPath := filepath.Join(usersDir, entry.Name(), "user.db")
			if _, err := os.Stat(dbPath); err == nil {
				// Get username from user.info file
				if username, err := c.GetUsernameFromDirName(entry.Name()); err == nil {
					users = append(users, username)
				}
			}
		}
	}

	return users, nil
}

// UserExists checks if a user already exists locally
func (c *Config) UserExists(username string) bool {
	userDirName := getUserDirName(username)
	userDir := filepath.Join(c.DataDir, "users", userDirName)
	dbPath := filepath.Join(userDir, "user.db")

	_, err := os.Stat(dbPath)
	return err == nil
}
