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

// ClientService provides business logic for the client application
type ClientService struct {
	secretsService    *SecretsService
	metadataService   *MetadataService
	cryptoService     *CryptoService
	clientSyncService *ClientSyncService
}

// NewClientService creates a new client service
func NewClientService(secretRepo repository.SecretRepositoryInterface, metadataRepo repository.MetadataRepositoryInterface, masterPasswordManager *crypto.MasterPasswordManager) *ClientService {
	// Create specialized services
	cryptoService := NewCryptoService(masterPasswordManager)
	secretsService := NewSecretsService(secretRepo, cryptoService)
	metadataService := NewMetadataService(metadataRepo, secretRepo, cryptoService)
	clientSyncService := NewClientSyncService(secretsService, metadataService, cryptoService)

	return &ClientService{
		secretsService:    secretsService,
		metadataService:   metadataService,
		cryptoService:     cryptoService,
		clientSyncService: clientSyncService,
	}
}

// CreateSecret creates a new secret with associated metadata
func (s *ClientService) CreateSecret(plaintext string, metadata map[string]string) (*models.Secret, error) {
	// Create the secret using secrets service
	secret, err := s.secretsService.CreateSecret(plaintext)
	if err != nil {
		return nil, err
	}

	// Create metadata using metadata service
	if err := s.metadataService.CreateMetadata(secret.SecretID, metadata); err != nil {
		log.Zap.Error("Failed to create metadata for secret",
			zap.String("secret_id", secret.SecretID.String()),
			zap.Error(err))
		// Continue - secret was created successfully
	}

	return secret, nil
}

// GetSecret retrieves a secret with its metadata
func (s *ClientService) GetSecret(secretID uuid.UUID) (*models.Secret, []*models.Metadata, error) {
	secret, err := s.secretsService.GetSecret(secretID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret: %w", err)
	}

	metadata, err := s.metadataService.GetMetadataBySecretID(secretID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	return secret, metadata, nil
}

// UpdateSecret updates an existing secret
func (s *ClientService) UpdateSecret(secretID uuid.UUID, encrypted string) error {
	return s.secretsService.UpdateSecretEncrypted(secretID, encrypted)
}

// DeleteSecret removes a secret and its metadata
func (s *ClientService) DeleteSecret(secretID uuid.UUID) error {
	// Delete metadata first (due to foreign key constraint)
	if err := s.metadataService.DeleteMetadataBySecretID(secretID); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	// Delete the secret
	if err := s.secretsService.DeleteSecret(secretID); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// GetChangedDataSince retrieves all data changed since the specified time
func (s *ClientService) GetChangedDataSince(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	secrets, err := s.secretsService.GetChangedSecretsSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	metadata, err := s.metadataService.GetChangedMetadataSince(since)
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
	return s.metadataService.AddMetadata(secretID, key, value)
}

// UpdateMetadata updates existing metadata
func (s *ClientService) UpdateMetadata(metadataID uuid.UUID, key, value string) error {
	return s.metadataService.UpdateMetadata(metadataID, key, value)
}

// DeleteMetadata removes a metadata entry
func (s *ClientService) DeleteMetadata(metadataID uuid.UUID) error {
	return s.metadataService.DeleteMetadata(metadataID)
}

// GetAllSecrets retrieves all secrets from the local database
func (s *ClientService) GetAllSecrets() ([]*models.Secret, error) {
	return s.secretsService.GetAllSecrets()
}

// GetMetadataBySecretID retrieves all metadata for a specific secret
func (s *ClientService) GetMetadataBySecretID(secretID uuid.UUID) ([]*models.Metadata, error) {
	return s.metadataService.GetMetadataBySecretID(secretID)
}

// DecryptSecretData расшифровывает данные секрета (публичный метод)
func (s *ClientService) DecryptSecretData(encryptedData string) (string, error) {
	return s.cryptoService.DecryptSecretData(encryptedData)
}

// DecryptMetadataValue расшифровывает значение метаданных (публичный метод)
func (s *ClientService) DecryptMetadataValue(encryptedValue string) (string, error) {
	decryptedValue := s.cryptoService.DecryptMetadataValue(encryptedValue)
	// DecryptMetadataValue не возвращает ошибку, но для консистентности API возвращаем nil
	return decryptedValue, nil
}

// SyncFromServer применяет изменения, полученные с сервера
func (s *ClientService) SyncFromServer(secrets []*models.Secret, metadata []*models.Metadata) error {
	return s.clientSyncService.SyncFromServer(secrets, metadata)
}

// GetLocalChangesForPush получает локальные изменения для отправки на сервер
func (s *ClientService) GetLocalChangesForPush(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	return s.clientSyncService.GetLocalChangesForPush(since)
}

// SyncType определяет тип синхронизации
type SyncType int

const (
	// SyncTypeIncremental - инкрементальная синхронизация (изменения за период)
	SyncTypeIncremental SyncType = iota
	// SyncTypeFull - полная синхронизация (все данные)
	SyncTypeFull
)

// PerformSync выполняет синхронизацию указанного типа
func (s *ClientService) PerformSync(syncService *SyncService, syncType SyncType) error {
	return s.clientSyncService.PerformSync(syncService, syncType)
}

// PerformFullSync выполняет полную синхронизацию: pull + push
// Deprecated: use PerformSync(syncService, SyncTypeFull) instead
func (s *ClientService) PerformFullSync(syncService *SyncService, since time.Time) error {
	return s.clientSyncService.PerformFullSync(syncService, since)
}

// PerformSyncSince выполняет синхронизацию изменений с указанного времени
func (s *ClientService) PerformSyncSince(syncService *SyncService, since time.Time) error {
	return s.clientSyncService.PerformSyncSince(syncService, since)
}
