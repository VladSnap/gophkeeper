package storage

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Login     string    `json:"login" db:"login"`
	Password  string    `json:"password" db:"password"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Session represents a user session
type Session struct {
	SessionID    uuid.UUID  `json:"session_id" db:"session_id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	LastSyncDate *time.Time `json:"last_sync_date" db:"last_sync_date"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
}

// Secret represents an encrypted secret
type Secret struct {
	SecretID        uuid.UUID `json:"secret_id" db:"secret_id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	Encrypted       string    `json:"encrypted" db:"encrypted"`
	CreatedDate     time.Time `json:"created_date" db:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date" db:"last_updated_date"`
}

// Metadata represents metadata associated with secrets
type Metadata struct {
	MetadataID      uuid.UUID `json:"metadata_id" db:"metadata_id"`
	SecretID        uuid.UUID `json:"secret_id" db:"secret_id"`
	Key             string    `json:"key" db:"key"`
	ValueHash       string    `json:"value_hash" db:"value_hash"`
	ValueEncrypted  string    `json:"value_encrypted" db:"value_encrypted"`
	CreatedDate     time.Time `json:"created_date" db:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date" db:"last_updated_date"`
}
