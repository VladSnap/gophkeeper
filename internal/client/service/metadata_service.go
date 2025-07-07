package service

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/crypto"
	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MetadataService handles metadata operations
type MetadataService struct {
	metadataRepo  repository.MetadataRepositoryInterface
	secretRepo    repository.SecretRepositoryInterface
	cryptoService *CryptoService
}

// NewMetadataService creates a new metadata service
func NewMetadataService(
	metadataRepo repository.MetadataRepositoryInterface,
	secretRepo repository.SecretRepositoryInterface,
	cryptoService *CryptoService,
) *MetadataService {
	return &MetadataService{
		metadataRepo:  metadataRepo,
		secretRepo:    secretRepo,
		cryptoService: cryptoService,
	}
}

// CreateMetadata creates metadata for a secret
func (ms *MetadataService) CreateMetadata(secretID uuid.UUID, metadata map[string]string) error {
	// Verify secret exists
	_, err := ms.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}

	// Create metadata entries
	for key, value := range metadata {
		meta := &models.Metadata{
			MetadataID:     uuid.New(),
			SecretID:       secretID,
			Key:            key,
			ValueHash:      crypto.HashValue(value),
			ValueEncrypted: ms.cryptoService.EncryptMetadataValue(value),
		}

		if err := ms.metadataRepo.Create(meta); err != nil {
			log.Zap.Error("Failed to create metadata",
				zap.String("secret_id", secretID.String()),
				zap.String("key", key),
				zap.Error(err))
			// Continue with other metadata entries
		}
	}

	return nil
}

// GetMetadataBySecretID retrieves all metadata for a specific secret
func (ms *MetadataService) GetMetadataBySecretID(secretID uuid.UUID) ([]*models.Metadata, error) {
	metadata, err := ms.metadataRepo.GetBySecretID(secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for secret: %w", err)
	}

	return metadata, nil
}

// GetMetadataDecrypted retrieves and decrypts metadata for a secret
func (ms *MetadataService) GetMetadataDecrypted(secretID uuid.UUID) (map[string]string, error) {
	metadata, err := ms.GetMetadataBySecretID(secretID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, meta := range metadata {
		decryptedValue := ms.cryptoService.DecryptMetadataValue(meta.ValueEncrypted)
		result[meta.Key] = decryptedValue
	}

	return result, nil
}

// AddMetadata adds metadata to an existing secret
func (ms *MetadataService) AddMetadata(secretID uuid.UUID, key, value string) error {
	// Verify secret exists
	_, err := ms.secretRepo.GetByID(secretID)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}

	meta := &models.Metadata{
		MetadataID:     uuid.New(),
		SecretID:       secretID,
		Key:            key,
		ValueHash:      crypto.HashValue(value),
		ValueEncrypted: ms.cryptoService.EncryptMetadataValue(value),
	}

	if err := ms.metadataRepo.Create(meta); err != nil {
		return fmt.Errorf("failed to create metadata: %w", err)
	}

	log.Zap.Info("Metadata added",
		zap.String("secret_id", secretID.String()),
		zap.String("key", key))

	return nil
}

// UpdateMetadata updates existing metadata
func (ms *MetadataService) UpdateMetadata(metadataID uuid.UUID, key, value string) error {
	metadata, err := ms.metadataRepo.GetByID(metadataID)
	if err != nil {
		return fmt.Errorf("metadata not found: %w", err)
	}

	metadata.Key = key
	metadata.ValueHash = crypto.HashValue(value)
	metadata.ValueEncrypted = ms.cryptoService.EncryptMetadataValue(value)

	if err := ms.metadataRepo.Update(metadata); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	log.Zap.Info("Metadata updated",
		zap.String("metadata_id", metadataID.String()),
		zap.String("key", key))

	return nil
}

// DeleteMetadata removes a metadata entry
func (ms *MetadataService) DeleteMetadata(metadataID uuid.UUID) error {
	if err := ms.metadataRepo.Delete(metadataID); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	log.Zap.Info("Metadata deleted",
		zap.String("metadata_id", metadataID.String()))

	return nil
}

// DeleteMetadataBySecretID removes all metadata for a secret
func (ms *MetadataService) DeleteMetadataBySecretID(secretID uuid.UUID) error {
	if err := ms.metadataRepo.DeleteBySecretID(secretID); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	log.Zap.Info("All metadata deleted for secret",
		zap.String("secret_id", secretID.String()))

	return nil
}

// GetChangedMetadataSince retrieves metadata changed since the specified time
func (ms *MetadataService) GetChangedMetadataSince(since time.Time) ([]*models.Metadata, error) {
	metadata, err := ms.metadataRepo.GetChangedSince(since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}

	log.Zap.Debug("Retrieved changed metadata",
		zap.Time("since", since),
		zap.Int("metadata_count", len(metadata)))

	return metadata, nil
}

// UpsertMetadata creates or updates metadata (used for sync)
func (ms *MetadataService) UpsertMetadata(metadata *models.Metadata) (bool, error) {
	existingMeta, err := ms.metadataRepo.GetByID(metadata.MetadataID)
	if err != nil {
		// Metadata doesn't exist locally, create it
		if err := ms.metadataRepo.Create(metadata); err != nil {
			return false, fmt.Errorf("failed to create metadata: %w", err)
		}
		log.Zap.Debug("Metadata created from sync", zap.String("metadata_id", metadata.MetadataID.String()))
		return true, nil // Created
	}

	// Metadata exists, check if we need to update
	// Rule: Last Write Wins
	if metadata.LastUpdatedDate.After(existingMeta.LastUpdatedDate) {
		// Server version is newer, update local
		if err := ms.metadataRepo.Update(metadata); err != nil {
			return false, fmt.Errorf("failed to update metadata: %w", err)
		}
		log.Zap.Debug("Metadata updated from sync",
			zap.String("metadata_id", metadata.MetadataID.String()),
			zap.Time("old_date", existingMeta.LastUpdatedDate),
			zap.Time("new_date", metadata.LastUpdatedDate))
		return true, nil // Updated
	}

	log.Zap.Debug("Local metadata is newer or same, skipping",
		zap.String("metadata_id", metadata.MetadataID.String()),
		zap.Time("local_date", existingMeta.LastUpdatedDate),
		zap.Time("server_date", metadata.LastUpdatedDate))

	return false, nil // Not changed
}
