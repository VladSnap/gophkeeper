package service

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/repository"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SyncService handles synchronization operations
type SyncService struct {
	secretRepo   repository.SecretRepositoryInterface
	metadataRepo repository.MetadataRepositoryInterface
}

// NewSyncService creates a new sync service
func NewSyncService(
	secretRepo repository.SecretRepositoryInterface,
	metadataRepo repository.MetadataRepositoryInterface,
) *SyncService {
	return &SyncService{
		secretRepo:   secretRepo,
		metadataRepo: metadataRepo,
	}
}

// PullChanges retrieves changes since the specified time
func (s *SyncService) PullChanges(userID uuid.UUID, since time.Time) ([]*storage.Secret, []*storage.Metadata, error) {
	log.Zap.Info("Pulling changes from repositories",
		zap.String("user_id", userID.String()),
		zap.Time("since", since))

	// Get changed secrets since the specified time
	secrets, err := s.secretRepo.GetChangedSince(userID, since)
	if err != nil {
		log.Zap.Error("Failed to get changed secrets", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to retrieve secrets: %w", err)
	}

	// Get changed metadata since the specified time
	metadata, err := s.metadataRepo.GetChangedSince(userID, since)
	if err != nil {
		log.Zap.Error("Failed to get changed metadata", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to retrieve metadata: %w", err)
	}

	log.Zap.Info("Changes retrieved successfully",
		zap.String("user_id", userID.String()),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	return secrets, metadata, nil
}

// PushChanges processes changes from client
func (s *SyncService) PushChanges(userID uuid.UUID, secrets []*ClientSecret, metadata []*ClientMetadata) (*PushResult, error) {
	log.Zap.Info("Processing push changes",
		zap.String("user_id", userID.String()),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	result := &PushResult{}

	// Process secrets
	result.SecretsProcessed, result.SecretsErrors = s.processSecrets(userID, secrets)

	// Process metadata
	result.MetadataProcessed, result.MetadataErrors = s.processMetadata(userID, metadata)

	// Set success status and message
	result.Success = result.SecretsErrors == 0 && result.MetadataErrors == 0
	if result.Success {
		result.Message = fmt.Sprintf("All changes processed successfully. Secrets: %d processed, Metadata: %d processed",
			result.SecretsProcessed, result.MetadataProcessed)
	} else {
		result.Message = fmt.Sprintf("Processed with errors. Secrets: %d processed, %d errors. Metadata: %d processed, %d errors",
			result.SecretsProcessed, result.SecretsErrors, result.MetadataProcessed, result.MetadataErrors)
	}

	log.Zap.Info("Push changes processed",
		zap.String("user_id", userID.String()),
		zap.Bool("success", result.Success),
		zap.String("message", result.Message))

	return result, nil
}

// processSecrets processes client secrets
func (s *SyncService) processSecrets(userID uuid.UUID, secrets []*ClientSecret) (processed, errors int) {
	for _, clientSecret := range secrets {
		log.Zap.Debug("Processing secret",
			zap.String("secret_id", clientSecret.SecretID.String()),
			zap.String("user_id", userID.String()))

		// Convert client secret to server secret with user ID
		secret := s.convertClientSecretToServer(clientSecret, userID)

		// Try to get existing secret
		existingSecret, err := s.secretRepo.GetByID(secret.SecretID)
		if err != nil {
			// Secret doesn't exist, create it
			if err := s.secretRepo.Create(secret); err != nil {
				log.Zap.Error("Failed to create secret",
					zap.String("secret_id", secret.SecretID.String()),
					zap.Error(err))
				errors++
				continue
			}
			log.Zap.Debug("Secret created", zap.String("secret_id", secret.SecretID.String()))
		} else {
			// Secret exists, check if update is needed
			// Last Write Wins: сравниваем даты последнего изменения
			if existingSecret.LastUpdatedDate.Before(secret.LastUpdatedDate) {
				if err := s.secretRepo.Update(secret); err != nil {
					log.Zap.Error("Failed to update secret",
						zap.String("secret_id", secret.SecretID.String()),
						zap.Error(err))
					errors++
					continue
				}
				log.Zap.Debug("Secret updated (client version newer)",
					zap.String("secret_id", secret.SecretID.String()),
					zap.Time("server_date", existingSecret.LastUpdatedDate),
					zap.Time("client_date", secret.LastUpdatedDate))
			} else if existingSecret.LastUpdatedDate.After(secret.LastUpdatedDate) {
				log.Zap.Debug("Secret not updated (server version newer)",
					zap.String("secret_id", secret.SecretID.String()),
					zap.Time("server_date", existingSecret.LastUpdatedDate),
					zap.Time("client_date", secret.LastUpdatedDate))
			} else {
				log.Zap.Debug("Secret timestamps equal, no update needed",
					zap.String("secret_id", secret.SecretID.String()))
			}
		}
		processed++
	}

	return processed, errors
}

// processMetadata processes client metadata
func (s *SyncService) processMetadata(userID uuid.UUID, metadata []*ClientMetadata) (processed, errors int) {
	for _, clientMeta := range metadata {
		log.Zap.Debug("Processing metadata",
			zap.String("metadata_id", clientMeta.MetadataID.String()),
			zap.String("secret_id", clientMeta.SecretID.String()))

		// Convert client metadata to server metadata
		meta := s.convertClientMetadataToServer(clientMeta)

		// Verify the secret exists and belongs to the user
		secret, err := s.secretRepo.GetByID(meta.SecretID)
		if err != nil {
			log.Zap.Warn("Metadata references non-existent secret",
				zap.String("metadata_id", meta.MetadataID.String()),
				zap.String("secret_id", meta.SecretID.String()))
			errors++
			continue
		}

		if secret.UserID != userID {
			log.Zap.Warn("Metadata secret user ID mismatch",
				zap.String("secret_user_id", secret.UserID.String()),
				zap.String("authenticated_user_id", userID.String()))
			errors++
			continue
		}

		// Try to get existing metadata
		existingMeta, err := s.metadataRepo.GetByID(meta.MetadataID)
		if err != nil {
			// Metadata doesn't exist, create it
			if err := s.metadataRepo.Create(meta); err != nil {
				log.Zap.Error("Failed to create metadata",
					zap.String("metadata_id", meta.MetadataID.String()),
					zap.Error(err))
				errors++
				continue
			}
			log.Zap.Debug("Metadata created", zap.String("metadata_id", meta.MetadataID.String()))
		} else {
			// Metadata exists, check if update is needed
			// Last Write Wins: сравниваем даты последнего изменения
			if existingMeta.LastUpdatedDate.Before(meta.LastUpdatedDate) {
				if err := s.metadataRepo.Update(meta); err != nil {
					log.Zap.Error("Failed to update metadata",
						zap.String("metadata_id", meta.MetadataID.String()),
						zap.Error(err))
					errors++
					continue
				}
				log.Zap.Debug("Metadata updated (client version newer)",
					zap.String("metadata_id", meta.MetadataID.String()),
					zap.Time("server_date", existingMeta.LastUpdatedDate),
					zap.Time("client_date", meta.LastUpdatedDate))
			} else if existingMeta.LastUpdatedDate.After(meta.LastUpdatedDate) {
				log.Zap.Debug("Metadata not updated (server version newer)",
					zap.String("metadata_id", meta.MetadataID.String()),
					zap.Time("server_date", existingMeta.LastUpdatedDate),
					zap.Time("client_date", meta.LastUpdatedDate))
			} else {
				log.Zap.Debug("Metadata timestamps equal, no update needed",
					zap.String("metadata_id", meta.MetadataID.String()))
			}
		}
		processed++
	}

	return processed, errors
}

// convertClientSecretToServer converts client secret to server secret
func (s *SyncService) convertClientSecretToServer(clientSecret *ClientSecret, userID uuid.UUID) *storage.Secret {
	return &storage.Secret{
		SecretID:        clientSecret.SecretID,
		UserID:          userID,
		Encrypted:       clientSecret.Encrypted,
		CreatedDate:     clientSecret.CreatedDate,
		LastUpdatedDate: clientSecret.LastUpdatedDate,
	}
}

// convertClientMetadataToServer converts client metadata to server metadata
func (s *SyncService) convertClientMetadataToServer(clientMeta *ClientMetadata) *storage.Metadata {
	return &storage.Metadata{
		MetadataID:      clientMeta.MetadataID,
		SecretID:        clientMeta.SecretID,
		Key:             clientMeta.Key,
		ValueHash:       clientMeta.ValueHash,
		ValueEncrypted:  clientMeta.ValueEncrypted,
		CreatedDate:     clientMeta.CreatedDate,
		LastUpdatedDate: clientMeta.LastUpdatedDate,
	}
}
