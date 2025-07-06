package service

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// SyncType определяет тип синхронизации
type SyncType int

const (
	// SyncTypeIncremental - инкрементальная синхронизация (изменения за период)
	SyncTypeIncremental SyncType = iota
	// SyncTypeFull - полная синхронизация (все данные)
	SyncTypeFull
)

// SyncResult представляет результат синхронизации
type SyncResult struct {
	Created int
	Updated int
	Errors  int
}

// SyncReport представляет общий отчет о синхронизации
type SyncReport struct {
	Secrets  SyncResult
	Metadata SyncResult
}

// ClientSyncService handles synchronization operations
type ClientSyncService struct {
	secretsService  *SecretsService
	metadataService *MetadataService
	cryptoService   *CryptoService
}

// NewClientSyncService creates a new client sync service
func NewClientSyncService(
	secretsService *SecretsService,
	metadataService *MetadataService,
	cryptoService *CryptoService,
) *ClientSyncService {
	return &ClientSyncService{
		secretsService:  secretsService,
		metadataService: metadataService,
		cryptoService:   cryptoService,
	}
}

// SyncFromServer applies changes received from the server
func (css *ClientSyncService) SyncFromServer(secrets []*models.Secret, metadata []*models.Metadata) error {
	if !css.cryptoService.IsUnlocked() {
		return fmt.Errorf("master password is locked")
	}

	log.Zap.Info("Starting sync from server",
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	// Sync secrets
	secretsResult := css.syncSecrets(secrets)

	// Sync metadata
	metadataResult := css.syncMetadata(metadata)

	// Generate and log sync report
	report := SyncReport{
		Secrets:  secretsResult,
		Metadata: metadataResult,
	}

	return css.generateSyncReport(report)
}

// syncSecrets синхронизирует секреты с сервера
func (css *ClientSyncService) syncSecrets(secrets []*models.Secret) SyncResult {
	result := SyncResult{}

	for _, serverSecret := range secrets {
		log.Zap.Debug("Processing secret from server",
			zap.String("secret_id", serverSecret.SecretID.String()),
			zap.Time("last_updated", serverSecret.LastUpdatedDate))

		changed, err := css.secretsService.UpsertSecret(serverSecret)
		if err != nil {
			log.Zap.Error("Failed to upsert secret from server",
				zap.String("secret_id", serverSecret.SecretID.String()),
				zap.Error(err))
			result.Errors++
			continue
		}

		if changed {
			result.Updated++
		}
	}

	// Count created items (approximation - all changed items are considered created for simplicity)
	result.Created = result.Updated
	return result
}

// syncMetadata синхронизирует метаданные с сервера
func (css *ClientSyncService) syncMetadata(metadata []*models.Metadata) SyncResult {
	result := SyncResult{}

	for _, serverMeta := range metadata {
		log.Zap.Debug("Processing metadata from server",
			zap.String("metadata_id", serverMeta.MetadataID.String()),
			zap.String("secret_id", serverMeta.SecretID.String()),
			zap.Time("last_updated", serverMeta.LastUpdatedDate))

		changed, err := css.metadataService.UpsertMetadata(serverMeta)
		if err != nil {
			log.Zap.Error("Failed to upsert metadata from server",
				zap.String("metadata_id", serverMeta.MetadataID.String()),
				zap.Error(err))
			result.Errors++
			continue
		}

		if changed {
			result.Updated++
		}
	}

	// Count created items (approximation - all changed items are considered created for simplicity)
	result.Created = result.Updated
	return result
}

// generateSyncReport генерирует отчет о синхронизации и возвращает ошибку если есть проблемы
func (css *ClientSyncService) generateSyncReport(report SyncReport) error {
	log.Zap.Info("Sync from server completed",
		zap.Int("secrets_created", report.Secrets.Created),
		zap.Int("secrets_updated", report.Secrets.Updated),
		zap.Int("secrets_errors", report.Secrets.Errors),
		zap.Int("metadata_created", report.Metadata.Created),
		zap.Int("metadata_updated", report.Metadata.Updated),
		zap.Int("metadata_errors", report.Metadata.Errors))

	if report.Secrets.Errors > 0 || report.Metadata.Errors > 0 {
		return fmt.Errorf("sync completed with errors: %d secret errors, %d metadata errors",
			report.Secrets.Errors, report.Metadata.Errors)
	}

	return nil
}

// GetLocalChangesForPush retrieves local changes for sending to the server
func (css *ClientSyncService) GetLocalChangesForPush(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	// Get all local secrets changed after the specified date
	secrets, err := css.secretsService.GetChangedSecretsSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	// Get all local metadata changed after the specified date
	metadata, err := css.metadataService.GetChangedMetadataSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}

	log.Zap.Info("Retrieved local changes for push",
		zap.Time("since", since),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	return secrets, metadata, nil
}

// GetChangedDataSince retrieves all data changed since the specified time
func (css *ClientSyncService) GetChangedDataSince(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	secrets, err := css.secretsService.GetChangedSecretsSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}

	metadata, err := css.metadataService.GetChangedMetadataSince(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}

	log.Zap.Info("Retrieved changed data",
		zap.Time("since", since),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	return secrets, metadata, nil
}

// PerformSyncSince performs synchronization of changes since the specified time
func (css *ClientSyncService) PerformSyncSince(syncService *SyncServerClientService, since time.Time) error {
	log.Zap.Debug("Starting sync since specific time", zap.Time("since", since))

	// Step 1: Pull changes from server
	pullResp, err := css.performPullStep(syncService, since)
	if err != nil {
		return err
	}

	// Step 2: Apply server changes locally
	if err := css.performApplyServerChangesStep(pullResp); err != nil {
		return err
	}

	// Step 3: Get local changes for push
	localSecrets, localMetadata, err := css.performGetLocalChangesStep(since)
	if err != nil {
		return err
	}

	// Step 4: Push local changes to server
	if err := css.performPushStep(syncService, localSecrets, localMetadata); err != nil {
		return err
	}

	return nil
}

// performPullStep выполняет шаг получения изменений с сервера
func (css *ClientSyncService) performPullStep(syncService *SyncServerClientService, since time.Time) (*PullResponse, error) {
	pullResp, err := syncService.Pull(since)
	if err != nil {
		return nil, fmt.Errorf("pull failed: %w", err)
	}

	log.Zap.Debug("Pull completed",
		zap.Int("secrets_received", len(pullResp.Secrets)),
		zap.Int("metadata_received", len(pullResp.Metadata)))

	return pullResp, nil
}

// performApplyServerChangesStep выполняет шаг применения изменений с сервера локально
func (css *ClientSyncService) performApplyServerChangesStep(pullResp *PullResponse) error {
	if len(pullResp.Secrets) > 0 || len(pullResp.Metadata) > 0 {
		if err := css.SyncFromServer(pullResp.Secrets, pullResp.Metadata); err != nil {
			return fmt.Errorf("failed to apply server changes: %w", err)
		}
	}
	return nil
}

// performGetLocalChangesStep выполняет шаг получения локальных изменений для отправки
func (css *ClientSyncService) performGetLocalChangesStep(since time.Time) ([]*models.Secret, []*models.Metadata, error) {
	localSecrets, localMetadata, err := css.GetLocalChangesForPush(since)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get local changes: %w", err)
	}
	return localSecrets, localMetadata, nil
}

// performPushStep выполняет шаг отправки локальных изменений на сервер
func (css *ClientSyncService) performPushStep(syncService *SyncServerClientService, localSecrets []*models.Secret, localMetadata []*models.Metadata) error {
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
		} else {
			log.Zap.Debug("Push completed successfully", zap.String("message", pushResp.Message))
		}
	}
	return nil
}
