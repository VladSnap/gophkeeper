package handlers

import (
	"time"

	"github.com/google/uuid"
)

// PullRequest represents a request to pull changes from server
type PullRequest struct {
	Since time.Time `json:"since"`
}

// PullResponse represents a response with changes from server
type PullResponse struct {
	Secrets   []*SecretResponse   `json:"secrets"`
	Metadata  []*MetadataResponse `json:"metadata"`
	Timestamp time.Time           `json:"timestamp"`
}

// PushRequest represents a request to push changes to server
type PushRequest struct {
	Secrets  []*SecretRequest   `json:"secrets"`
	Metadata []*MetadataRequest `json:"metadata"`
}

// PushResponse represents a response after pushing changes
type PushResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SecretRequest represents a secret from client (without user_id)
type SecretRequest struct {
	SecretID        uuid.UUID `json:"secret_id"`
	Encrypted       string    `json:"encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}

// SecretResponse represents a secret in response (without user_id)
type SecretResponse struct {
	SecretID        uuid.UUID `json:"secret_id"`
	Encrypted       string    `json:"encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}

// MetadataRequest represents metadata from client
type MetadataRequest struct {
	MetadataID      uuid.UUID `json:"metadata_id"`
	SecretID        uuid.UUID `json:"secret_id"`
	Key             string    `json:"key"`
	ValueHash       string    `json:"value_hash"`
	ValueEncrypted  string    `json:"value_encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}

// MetadataResponse represents metadata in response
type MetadataResponse struct {
	MetadataID      uuid.UUID `json:"metadata_id"`
	SecretID        uuid.UUID `json:"secret_id"`
	Key             string    `json:"key"`
	ValueHash       string    `json:"value_hash"`
	ValueEncrypted  string    `json:"value_encrypted"`
	CreatedDate     time.Time `json:"created_date"`
	LastUpdatedDate time.Time `json:"last_updated_date"`
}
