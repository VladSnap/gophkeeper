package service

import (
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/google/uuid"
)

// SyncServiceInterface defines the interface for sync service operations
type SyncServiceInterface interface {
	PullChanges(userID uuid.UUID, since time.Time) ([]*storage.Secret, []*storage.Metadata, error)
	PushChanges(userID uuid.UUID, secrets []*ClientSecret, metadata []*ClientMetadata) (*PushResult, error)
}

// ClientSecret represents a secret from client (without user_id)
type ClientSecret struct {
	SecretID        uuid.UUID `json:"secret_id"`
	Encrypted       string    `json:"encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}

// ClientMetadata represents metadata from client
type ClientMetadata struct {
	MetadataID      uuid.UUID `json:"metadata_id"`
	SecretID        uuid.UUID `json:"secret_id"`
	Key             string    `json:"key"`
	ValueHash       string    `json:"value_hash"`
	ValueEncrypted  string    `json:"value_encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}

// PushResult represents the result of push operation
type PushResult struct {
	Success           bool
	Message           string
	SecretsProcessed  int
	SecretsErrors     int
	MetadataProcessed int
	MetadataErrors    int
}
