package service

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SecretsService handles secret operations
type SecretsService struct {
	secretRepo    repository.SecretRepositoryInterface
	cryptoService *CryptoService
}

// NewSecretsService creates a new secrets service
func NewSecretsService(secretRepo repository.SecretRepositoryInterface, cryptoService *CryptoService) *SecretsService {
	return &SecretsService{
		secretRepo:    secretRepo,
		cryptoService: cryptoService,
	}
}

// CreateSecret creates a new secret
func (ss *SecretsService) CreateSecret(plaintext string) (*models.Secret, error) {
	if !ss.cryptoService.IsUnlocked() {
		return nil, fmt.Errorf("master password is locked")
	}

	// Encrypt the secret data
	encryptedData, err := ss.cryptoService.EncryptSecretData(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret data: %w", err)
	}

	secret := &models.Secret{
		SecretID:  uuid.New(),
		Encrypted: encryptedData,
	}

	// Create the secret
	if err := ss.secretRepo.Create(secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	log.Zap.Info("Secret created",
		zap.String("secret_id", secret.SecretID.String()))

	return secret, nil
}

// GetSecret retrieves a secret by ID
func (ss *SecretsService) GetSecret(secretID uuid.UUID) (*models.Secret, error) {
	secret, err := ss.secretRepo.GetByID(secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// GetSecretDecrypted retrieves and decrypts a secret
func (ss *SecretsService) GetSecretDecrypted(secretID uuid.UUID) (string, error) {
	secret, err := ss.GetSecret(secretID)
	if err != nil {
		return "", err
	}

	decryptedData, err := ss.cryptoService.DecryptSecretData(secret.Encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}

	return decryptedData, nil
}

// UpdateSecret updates an existing secret
func (ss *SecretsService) UpdateSecret(secretID uuid.UUID, plaintext string) error {
	if !ss.cryptoService.IsUnlocked() {
		return fmt.Errorf("master password is locked")
	}

	// Encrypt the new data
	encryptedData, err := ss.cryptoService.EncryptSecretData(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret data: %w", err)
	}

	secret, err := ss.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("failed to find secret: %w", err)
	}

	secret.Encrypted = encryptedData
	if err := ss.secretRepo.Update(secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	log.Zap.Info("Secret updated",
		zap.String("secret_id", secretID.String()))

	return nil
}

// UpdateSecretEncrypted updates an existing secret with already encrypted data
func (ss *SecretsService) UpdateSecretEncrypted(secretID uuid.UUID, encrypted string) error {
	secret, err := ss.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("failed to find secret: %w", err)
	}

	secret.Encrypted = encrypted
	if err := ss.secretRepo.Update(secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	log.Zap.Info("Secret updated",
		zap.String("secret_id", secretID.String()))

	return nil
}

// DeleteSecret removes a secret
func (ss *SecretsService) DeleteSecret(secretID uuid.UUID) error {
	if err := ss.secretRepo.Delete(secretID); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	log.Zap.Info("Secret deleted",
		zap.String("secret_id", secretID.String()))

	return nil
}

// GetAllSecrets retrieves all secrets from the local database
func (ss *SecretsService) GetAllSecrets() ([]*models.Secret, error) {
	secrets, err := ss.secretRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get all secrets: %w", err)
	}

	log.Zap.Info("Retrieved all secrets",
		zap.Int("secrets_count", len(secrets)))

	return secrets, nil
}

// GetChangedSecretsSince retrieves secrets changed since the specified time
func (ss *SecretsService) GetChangedSecretsSince(since time.Time) ([]*models.Secret, error) {
	secrets, err := ss.secretRepo.GetChangedSince(since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	log.Zap.Debug("Retrieved changed secrets",
		zap.Time("since", since),
		zap.Int("secrets_count", len(secrets)))

	return secrets, nil
}

// UpsertSecret creates or updates a secret (used for sync)
func (ss *SecretsService) UpsertSecret(secret *models.Secret) (bool, error) {
	existingSecret, err := ss.secretRepo.GetByID(secret.SecretID)
	if err != nil {
		// Secret doesn't exist locally, create it
		if err := ss.secretRepo.Create(secret); err != nil {
			return false, fmt.Errorf("failed to create secret: %w", err)
		}
		log.Zap.Debug("Secret created from sync", zap.String("secret_id", secret.SecretID.String()))
		return true, nil // Created
	}

	// Secret exists, check if we need to update
	// Rule: Last Write Wins
	if secret.LastUpdatedDate.After(existingSecret.LastUpdatedDate) {
		// Server version is newer, update local
		if err := ss.secretRepo.Update(secret); err != nil {
			return false, fmt.Errorf("failed to update secret: %w", err)
		}
		log.Zap.Debug("Secret updated from sync",
			zap.String("secret_id", secret.SecretID.String()),
			zap.Time("old_date", existingSecret.LastUpdatedDate),
			zap.Time("new_date", secret.LastUpdatedDate))
		return true, nil // Updated
	}

	log.Zap.Debug("Local secret is newer or same, skipping",
		zap.String("secret_id", secret.SecretID.String()),
		zap.Time("local_date", existingSecret.LastUpdatedDate),
		zap.Time("server_date", secret.LastUpdatedDate))

	return false, nil // Not changed
}
