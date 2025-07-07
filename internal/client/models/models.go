package models

import (
	"time"

	"github.com/google/uuid"
)

// Secret represents an encrypted secret
type Secret struct {
	SecretID        uuid.UUID `json:"secret_id" db:"secret_id"`
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
