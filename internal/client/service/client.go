package service

import (
	"encoding/json"
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
	secretRepo            repository.SecretRepositoryInterface
	metadataRepo          repository.MetadataRepositoryInterface
	masterPasswordManager *crypto.MasterPasswordManager
}

// NewClientService creates a new client service
func NewClientService(secretRepo repository.SecretRepositoryInterface, metadataRepo repository.MetadataRepositoryInterface, masterPasswordManager *crypto.MasterPasswordManager) *ClientService {
	return &ClientService{
		secretRepo:            secretRepo,
		metadataRepo:          metadataRepo,
		masterPasswordManager: masterPasswordManager,
	}
}

// CreateSecret creates a new secret with associated metadata
func (s *ClientService) CreateSecret(plaintext string, metadata map[string]string) (*models.Secret, error) {
	if !s.masterPasswordManager.IsUnlocked() {
		return nil, fmt.Errorf("master password is locked")
	}

	// Шифруем данные секрета
	encryptedData, salt, err := s.masterPasswordManager.EncryptData([]byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret data: %w", err)
	}

	// Сохраняем зашифрованные данные + соль в поле Encrypted (как JSON)
	secretData := struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	encryptedJSON, err := json.Marshal(secretData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal encrypted secret data: %w", err)
	}

	secret := &models.Secret{
		SecretID:  uuid.New(),
		Encrypted: string(encryptedJSON),
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
			ValueHash:      crypto.HashValue(value),
			ValueEncrypted: s.encryptMetadataValue(value),
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
		ValueHash:      crypto.HashValue(value),
		ValueEncrypted: s.encryptMetadataValue(value),
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
	metadata.ValueHash = crypto.HashValue(value)
	metadata.ValueEncrypted = s.encryptMetadataValue(value)

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

// encryptMetadataValue шифрует значение метаданных
func (s *ClientService) encryptMetadataValue(value string) string {
	if !s.masterPasswordManager.IsUnlocked() {
		log.Zap.Warn("Master password is locked, cannot encrypt metadata value")
		return value // Fallback to plaintext (не рекомендуется в продакшене)
	}

	encryptedData, salt, err := s.masterPasswordManager.EncryptData([]byte(value))
	if err != nil {
		log.Zap.Error("Failed to encrypt metadata value", zap.Error(err))
		return value // Fallback to plaintext
	}

	// Сохраняем как JSON с солью
	metaData := struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	jsonData, err := json.Marshal(metaData)
	if err != nil {
		log.Zap.Error("Failed to marshal encrypted metadata", zap.Error(err))
		return value // Fallback to plaintext
	}

	return string(jsonData)
}

// decryptMetadataValue расшифровывает значение метаданных
func (s *ClientService) decryptMetadataValue(encryptedValue string) string {
	if !s.masterPasswordManager.IsUnlocked() {
		return encryptedValue // Return as-is if locked
	}

	// Попытаемся десериализовать как JSON
	var metaData struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}

	if err := json.Unmarshal([]byte(encryptedValue), &metaData); err != nil {
		// Если не JSON, значит это plaintext
		return encryptedValue
	}

	// Расшифровываем
	decryptedData, err := s.masterPasswordManager.DecryptData(metaData.EncryptedData, metaData.Salt)
	if err != nil {
		log.Zap.Error("Failed to decrypt metadata value", zap.Error(err))
		return encryptedValue // Return encrypted value as fallback
	}

	return string(decryptedData)
}

// decryptSecretData расшифровывает данные секрета
func (s *ClientService) decryptSecretData(encryptedData string) (string, error) {
	if !s.masterPasswordManager.IsUnlocked() {
		return "", fmt.Errorf("master password is locked")
	}

	// Десериализуем JSON
	var secretData struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}

	if err := json.Unmarshal([]byte(encryptedData), &secretData); err != nil {
		return "", fmt.Errorf("failed to unmarshal secret data: %w", err)
	}

	// Расшифровываем
	decryptedData, err := s.masterPasswordManager.DecryptData(secretData.EncryptedData, secretData.Salt)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret data: %w", err)
	}

	return string(decryptedData), nil
}

// DecryptSecretData расшифровывает данные секрета (публичный метод)
func (s *ClientService) DecryptSecretData(encryptedData string) (string, error) {
	return s.decryptSecretData(encryptedData)
}

// DecryptMetadataValue расшифровывает значение метаданных (публичный метод)
func (s *ClientService) DecryptMetadataValue(encryptedValue string) (string, error) {
	decryptedValue := s.decryptMetadataValue(encryptedValue)
	// decryptMetadataValue не возвращает ошибку, но для консистентности API возвращаем nil
	return decryptedValue, nil
}

// SyncFromServer применяет изменения, полученные с сервера
func (s *ClientService) SyncFromServer(secrets []*models.Secret, metadata []*models.Metadata) error {
	if !s.masterPasswordManager.IsUnlocked() {
		return fmt.Errorf("master password is locked")
	}

	log.Zap.Info("Starting sync from server",
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	secretsUpdated := 0
	secretsCreated := 0
	secretsErrors := 0

	// Обрабатываем секреты
	for _, serverSecret := range secrets {
		log.Zap.Debug("Processing secret from server",
			zap.String("secret_id", serverSecret.SecretID.String()),
			zap.Time("last_updated", serverSecret.LastUpdatedDate))

		// Пытаемся получить существующий секрет
		existingSecret, err := s.secretRepo.GetByID(serverSecret.SecretID)
		if err != nil {
			// Секрет не существует локально, создаем его
			if err := s.secretRepo.Create(serverSecret); err != nil {
				log.Zap.Error("Failed to create secret from server",
					zap.String("secret_id", serverSecret.SecretID.String()),
					zap.Error(err))
				secretsErrors++
				continue
			}
			log.Zap.Debug("Secret created from server", zap.String("secret_id", serverSecret.SecretID.String()))
			secretsCreated++
		} else {
			// Секрет существует, проверяем нужно ли обновление
			// Правило: побеждает самая последняя дата изменения (Last Write Wins)
			if serverSecret.LastUpdatedDate.After(existingSecret.LastUpdatedDate) {
				// Серверная версия новее, обновляем локальную
				if err := s.secretRepo.Update(serverSecret); err != nil {
					log.Zap.Error("Failed to update secret from server",
						zap.String("secret_id", serverSecret.SecretID.String()),
						zap.Error(err))
					secretsErrors++
					continue
				}
				log.Zap.Debug("Secret updated from server",
					zap.String("secret_id", serverSecret.SecretID.String()),
					zap.Time("old_date", existingSecret.LastUpdatedDate),
					zap.Time("new_date", serverSecret.LastUpdatedDate))
				secretsUpdated++
			} else {
				log.Zap.Debug("Local secret is newer or same, skipping",
					zap.String("secret_id", serverSecret.SecretID.String()),
					zap.Time("local_date", existingSecret.LastUpdatedDate),
					zap.Time("server_date", serverSecret.LastUpdatedDate))
			}
		}
	}

	metadataUpdated := 0
	metadataCreated := 0
	metadataErrors := 0

	// Обрабатываем метаданные
	for _, serverMeta := range metadata {
		log.Zap.Debug("Processing metadata from server",
			zap.String("metadata_id", serverMeta.MetadataID.String()),
			zap.String("secret_id", serverMeta.SecretID.String()),
			zap.Time("last_updated", serverMeta.LastUpdatedDate))

		// Пытаемся получить существующие метаданные
		existingMeta, err := s.metadataRepo.GetByID(serverMeta.MetadataID)
		if err != nil {
			// Метаданные не существуют локально, создаем их
			if err := s.metadataRepo.Create(serverMeta); err != nil {
				log.Zap.Error("Failed to create metadata from server",
					zap.String("metadata_id", serverMeta.MetadataID.String()),
					zap.Error(err))
				metadataErrors++
				continue
			}
			log.Zap.Debug("Metadata created from server", zap.String("metadata_id", serverMeta.MetadataID.String()))
			metadataCreated++
		} else {
			// Метаданные существуют, проверяем нужно ли обновление
			// Правило: побеждает самая последняя дата изменения (Last Write Wins)
			if serverMeta.LastUpdatedDate.After(existingMeta.LastUpdatedDate) {
				// Серверная версия новее, обновляем локальную
				if err := s.metadataRepo.Update(serverMeta); err != nil {
					log.Zap.Error("Failed to update metadata from server",
						zap.String("metadata_id", serverMeta.MetadataID.String()),
						zap.Error(err))
					metadataErrors++
					continue
				}
				log.Zap.Debug("Metadata updated from server",
					zap.String("metadata_id", serverMeta.MetadataID.String()),
					zap.Time("old_date", existingMeta.LastUpdatedDate),
					zap.Time("new_date", serverMeta.LastUpdatedDate))
				metadataUpdated++
			} else {
				log.Zap.Debug("Local metadata is newer or same, skipping",
					zap.String("metadata_id", serverMeta.MetadataID.String()),
					zap.Time("local_date", existingMeta.LastUpdatedDate),
					zap.Time("server_date", serverMeta.LastUpdatedDate))
			}
		}
	}

	log.Zap.Info("Sync from server completed",
		zap.Int("secrets_created", secretsCreated),
		zap.Int("secrets_updated", secretsUpdated),
		zap.Int("secrets_errors", secretsErrors),
		zap.Int("metadata_created", metadataCreated),
		zap.Int("metadata_updated", metadataUpdated),
		zap.Int("metadata_errors", metadataErrors))

	if secretsErrors > 0 || metadataErrors > 0 {
		return fmt.Errorf("sync completed with errors: %d secret errors, %d metadata errors",
			secretsErrors, metadataErrors)
	}

	return nil
}

// GetLocalChangesForPush получает локальные изменения для отправки на сервер
func (s *ClientService) GetLocalChangesForPush(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	// Получаем все локальные секреты, измененные после указанной даты
	secrets, err := s.secretRepo.GetChangedSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	// Получаем все локальные метаданные, измененные после указанной даты
	metadata, err := s.metadataRepo.GetChangedSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}

	log.Zap.Info("Retrieved local changes for push",
		zap.Time("since", since),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	return secrets, metadata, nil
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
	var since time.Time
	var syncDescription string

	switch syncType {
	case SyncTypeIncremental:
		// Синхронизация изменений за последние 24 часа
		since = time.Now().Add(-24 * time.Hour)
		syncDescription = "incremental sync (last 24 hours)"
	case SyncTypeFull:
		// Полная синхронизация с начала времён
		since = time.Unix(0, 0)
		syncDescription = "full sync (all data)"
	default:
		return fmt.Errorf("unknown sync type: %d", syncType)
	}

	log.Zap.Info("Starting sync",
		zap.String("type", syncDescription),
		zap.Time("since", since))

	// Шаг 1: Pull - получаем изменения с сервера
	pullResp, err := syncService.Pull(since)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	log.Zap.Info("Pull completed",
		zap.Int("secrets_received", len(pullResp.Secrets)),
		zap.Int("metadata_received", len(pullResp.Metadata)))

	// Шаг 2: Применяем изменения с сервера локально
	if len(pullResp.Secrets) > 0 || len(pullResp.Metadata) > 0 {
		if err := s.SyncFromServer(pullResp.Secrets, pullResp.Metadata); err != nil {
			return fmt.Errorf("failed to apply server changes: %w", err)
		}
	} else {
		log.Zap.Info("No changes received from server")
	}

	// Шаг 3: Получаем локальные изменения для отправки
	localSecrets, localMetadata, err := s.GetLocalChangesForPush(since)
	if err != nil {
		return fmt.Errorf("failed to get local changes: %w", err)
	}

	// Шаг 4: Push - отправляем локальные изменения на сервер
	if len(localSecrets) > 0 || len(localMetadata) > 0 {
		log.Zap.Info("Pushing local changes",
			zap.Int("secrets_to_push", len(localSecrets)),
			zap.Int("metadata_to_push", len(localMetadata)))

		pushResp, err := syncService.Push(localSecrets, localMetadata)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		if !pushResp.Success {
			log.Zap.Warn("Push completed with warnings", zap.String("message", pushResp.Message))
		} else {
			log.Zap.Info("Push completed successfully", zap.String("message", pushResp.Message))
		}
	} else {
		log.Zap.Info("No local changes to push")
	}

	log.Zap.Info("Sync completed successfully", zap.String("type", syncDescription))
	return nil
}

// PerformFullSync выполняет полную синхронизацию: pull + push
// Deprecated: use PerformSync(syncService, SyncTypeFull) instead
func (s *ClientService) PerformFullSync(syncService *SyncService, since time.Time) error {
	log.Zap.Info("Starting full sync", zap.Time("since", since))

	// Шаг 1: Pull - получаем изменения с сервера
	pullResp, err := syncService.Pull(since)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	// Шаг 2: Применяем изменения с сервера локально
	if err := s.SyncFromServer(pullResp.Secrets, pullResp.Metadata); err != nil {
		return fmt.Errorf("failed to apply server changes: %w", err)
	}

	// Шаг 3: Получаем локальные изменения для отправки
	localSecrets, localMetadata, err := s.GetLocalChangesForPush(since)
	if err != nil {
		return fmt.Errorf("failed to get local changes: %w", err)
	}

	// Шаг 4: Push - отправляем локальные изменения на сервер
	if len(localSecrets) > 0 || len(localMetadata) > 0 {
		pushResp, err := syncService.Push(localSecrets, localMetadata)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		if !pushResp.Success {
			log.Zap.Warn("Push completed with warnings", zap.String("message", pushResp.Message))
		}
	} else {
		log.Zap.Info("No local changes to push")
	}

	log.Zap.Info("Full sync completed successfully")
	return nil
}

// PerformSyncSince выполняет синхронизацию изменений с указанного времени
func (s *ClientService) PerformSyncSince(syncService *SyncService, since time.Time) error {
	log.Zap.Debug("Starting sync since specific time", zap.Time("since", since))

	// Шаг 1: Pull - получаем изменения с сервера
	pullResp, err := syncService.Pull(since)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	log.Zap.Debug("Pull completed",
		zap.Int("secrets_received", len(pullResp.Secrets)),
		zap.Int("metadata_received", len(pullResp.Metadata)))

	// Шаг 2: Применяем изменения с сервера локально
	if len(pullResp.Secrets) > 0 || len(pullResp.Metadata) > 0 {
		if err := s.SyncFromServer(pullResp.Secrets, pullResp.Metadata); err != nil {
			return fmt.Errorf("failed to apply server changes: %w", err)
		}
	}

	// Шаг 3: Получаем локальные изменения для отправки
	localSecrets, localMetadata, err := s.GetLocalChangesForPush(since)
	if err != nil {
		return fmt.Errorf("failed to get local changes: %w", err)
	}

	// Шаг 4: Push - отправляем локальные изменения на сервер
	if len(localSecrets) > 0 || len(localMetadata) > 0 {
		log.Zap.Debug("Pushing local changes",
			zap.Int("secrets_to_push", len(localSecrets)),
			zap.Int("metadata_to_push", len(localMetadata)))

		pushResp, err := syncService.Push(localSecrets, localMetadata)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		if !pushResp.Success {
			log.Zap.Warn("Push completed with warnings", zap.String("message", pushResp.Message))
		}
	}

	return nil
}
