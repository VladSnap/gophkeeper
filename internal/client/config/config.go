package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the client application configuration
type Config struct {
	DatabasePath string
	DataDir      string
	LogLevel     string
	ServerURL    string
}

// NewConfig creates a new configuration with default values
func NewConfig() (*Config, error) {
	dataDir, err := getDataDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to get data directory: %w", err)
	}

	config := &Config{
		DataDir:      dataDir,
		DatabasePath: filepath.Join(dataDir, "gophkeeper.db"),
		LogLevel:     "info",
		ServerURL:    "http://localhost:8080", // Default server URL
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
