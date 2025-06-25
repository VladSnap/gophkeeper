package main

import (
	"fmt"
	"os"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/config"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/internal/client/service"
	"github.com/VladSnap/gophkeeper/internal/client/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Printf("Failed to close logger: %v\n", err)
		}
	}()

	log.Zap.Info("Starting gophkeeper client application")

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Zap.Error("Failed to load configuration",
			zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Configuration loaded",
		zap.String("database_path", cfg.DatabasePath),
		zap.String("data_dir", cfg.DataDir))

	// Initialize database
	db, err := storage.NewDatabaseClient(cfg.DatabasePath)
	if err != nil {
		log.Zap.Error("Failed to initialize database",
			zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Zap.Error("Failed to close database",
				zap.Error(err))
		}
	}()

	log.Zap.Info("Database initialized successfully")

	// Initialize repositories
	secretRepo := repository.NewSecretRepository(db.DB)
	metadataRepo := repository.NewMetadataRepository(db.DB)

	// Initialize service
	clientService := service.NewClientService(secretRepo, metadataRepo)

	log.Zap.Info("Services initialized successfully")

	// For now, just test that everything works
	// TODO: Add CLI interface later
	log.Zap.Info("Gophkeeper client is ready")

	// Test services (just for verification)
	if err := testServices(clientService); err != nil {
		log.Zap.Error("Service test failed",
			zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Application completed successfully")
}

// testServices performs basic tests to verify services work
func testServices(clientService *service.ClientService) error {
	log.Zap.Info("Testing services...")

	// Test getting changed data (should be empty on first run)
	secrets, metadata, err := clientService.GetChangedDataSince(time.Time{})
	if err != nil {
		return fmt.Errorf("failed to get changed data: %w", err)
	}

	log.Zap.Info("Retrieved changed data on startup",
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	// Test creating a sample secret
	userID := uuid.New()
	sampleMetadata := map[string]string{
		"type":        "login",
		"website":     "example.com",
		"description": "Sample secret for testing",
	}

	secret, err := clientService.CreateSecret(userID, "encrypted_sample_data", sampleMetadata)
	if err != nil {
		return fmt.Errorf("failed to create sample secret: %w", err)
	}

	log.Zap.Info("Created sample secret",
		zap.String("secret_id", secret.SecretID.String()))

	// Test retrieving the secret
	retrievedSecret, retrievedMetadata, err := clientService.GetSecret(secret.SecretID)
	if err != nil {
		return fmt.Errorf("failed to retrieve secret: %w", err)
	}

	log.Zap.Info("Retrieved secret successfully",
		zap.String("secret_id", retrievedSecret.SecretID.String()),
		zap.Int("metadata_count", len(retrievedMetadata)))

	// Test updating the secret
	if err := clientService.UpdateSecret(secret.SecretID, "updated_encrypted_data"); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	log.Zap.Info("Updated secret successfully")

	// Test getting user secrets
	userSecrets, err := clientService.GetUserSecrets(userID)
	if err != nil {
		return fmt.Errorf("failed to get user secrets: %w", err)
	}

	log.Zap.Info("Retrieved user secrets",
		zap.String("user_id", userID.String()),
		zap.Int("secrets_count", len(userSecrets)))

	// Clean up - delete the test secret
	if err := clientService.DeleteSecret(secret.SecretID); err != nil {
		return fmt.Errorf("failed to delete test secret: %w", err)
	}

	log.Zap.Info("Deleted test secret successfully")

	log.Zap.Info("Service tests completed successfully")
	return nil
}
