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

// ClientService provides business logic for the client application
type ClientService struct {
	secretRepo   repository.SecretRepositoryInterface
	metadataRepo repository.MetadataRepositoryInterface
}

// NewClientService creates a new client service
func NewClientService(secretRepo repository.SecretRepositoryInterface, metadataRepo repository.MetadataRepositoryInterface) *ClientService {
	return &ClientService{
		secretRepo:   secretRepo,
		metadataRepo: metadataRepo,
	}
}

// CreateSecret creates a new secret with associated metadata
func (s *ClientService) CreateSecret(encrypted string,
	metadata map[string]string) (*models.Secret, error) {
	secret := &models.Secret{
		SecretID:  uuid.New(),
		Encrypted: encrypted,
	}

	// Create the secret
	if err := s.secretRepo.Create(secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	log.Zap.Info("Secret created",
		zap.String("secret_id", secret.SecretID.String()))

	// Create metadata entries
	for key, value := range metadata {
		meta := &models.Metadata{
			MetadataID:     uuid.New(),
			SecretID:       secret.SecretID,
			Key:            key,
			ValueHash:      generateHash(value), // You'll need to implement this
			ValueEncrypted: encryptValue(value), // You'll need to implement this
		}

		if err := s.metadataRepo.Create(meta); err != nil {
			log.Zap.Error("Failed to create metadata",
				zap.String("secret_id", secret.SecretID.String()),
				zap.String("key", key),
				zap.Error(err))
			// Continue with other metadata entries
		}
	}

	return secret, nil
}

// GetSecret retrieves a secret with its metadata
func (s *ClientService) GetSecret(secretID uuid.UUID) (*models.Secret, []*models.Metadata, error) {
	secret, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret: %w", err)
	}

	metadata, err := s.metadataRepo.GetBySecretID(secretID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	return secret, metadata, nil
}

// UpdateSecret updates an existing secret
func (s *ClientService) UpdateSecret(secretID uuid.UUID, encrypted string) error {
	secret, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("failed to find secret: %w", err)
	}

	secret.Encrypted = encrypted
	if err := s.secretRepo.Update(secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	log.Zap.Info("Secret updated",
		zap.String("secret_id", secretID.String()))

	return nil
}

// DeleteSecret removes a secret and its metadata
func (s *ClientService) DeleteSecret(secretID uuid.UUID) error {
	// Delete metadata first (due to foreign key constraint)
	if err := s.metadataRepo.DeleteBySecretID(secretID); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	// Delete the secret
	if err := s.secretRepo.Delete(secretID); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	log.Zap.Info("Secret deleted",
		zap.String("secret_id", secretID.String()))

	return nil
}

// GetChangedDataSince retrieves all data changed since the specified time
func (s *ClientService) GetChangedDataSince(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	secrets, err := s.secretRepo.GetChangedSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	metadata, err := s.metadataRepo.GetChangedSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}

	log.Zap.Info("Retrieved changed data",
		zap.Time("since", since),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	return secrets, metadata, nil
}

// AddMetadata adds metadata to an existing secret
func (s *ClientService) AddMetadata(secretID uuid.UUID, key, value string) error {
	// Verify secret exists
	_, err := s.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}

	meta := &models.Metadata{
		MetadataID:     uuid.New(),
		SecretID:       secretID,
		Key:            key,
		ValueHash:      generateHash(value),
		ValueEncrypted: encryptValue(value),
	}

	if err := s.metadataRepo.Create(meta); err != nil {
		return fmt.Errorf("failed to create metadata: %w", err)
	}

	log.Zap.Info("Metadata added",
		zap.String("secret_id", secretID.String()),
		zap.String("key", key))

	return nil
}

// UpdateMetadata updates existing metadata
func (s *ClientService) UpdateMetadata(metadataID uuid.UUID, key, value string) error {
	metadata, err := s.metadataRepo.GetByID(metadataID)
	if err != nil {
		return fmt.Errorf("metadata not found: %w", err)
	}

	metadata.Key = key
	metadata.ValueHash = generateHash(value)
	metadata.ValueEncrypted = encryptValue(value)

	if err := s.metadataRepo.Update(metadata); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	log.Zap.Info("Metadata updated",
		zap.String("metadata_id", metadataID.String()),
		zap.String("key", key))

	return nil
}

// DeleteMetadata removes a metadata entry
func (s *ClientService) DeleteMetadata(metadataID uuid.UUID) error {
	if err := s.metadataRepo.Delete(metadataID); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	log.Zap.Info("Metadata deleted",
		zap.String("metadata_id", metadataID.String()))

	return nil
}

// GetAllSecrets retrieves all secrets from the local database
func (s *ClientService) GetAllSecrets() ([]*models.Secret, error) {
	secrets, err := s.secretRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get all secrets: %w", err)
	}

	log.Zap.Info("Retrieved all secrets",
		zap.Int("secrets_count", len(secrets)))

	return secrets, nil
}

// GetMetadataBySecretID retrieves all metadata for a specific secret
func (s *ClientService) GetMetadataBySecretID(secretID uuid.UUID) ([]*models.Metadata, error) {
	metadata, err := s.metadataRepo.GetBySecretID(secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for secret: %w", err)
	}

	return metadata, nil
}

// TODO: Implement these utility functions
func generateHash(value string) string {
	// Placeholder - implement proper hashing
	return fmt.Sprintf("hash_%s", value)
}

func encryptValue(value string) string {
	// Placeholder - implement proper encryption
	return fmt.Sprintf("encrypted_%s", value)
}
