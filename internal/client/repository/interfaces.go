package repository

import (
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/google/uuid"
)

// SecretRepositoryInterface defines the interface for secret repository operations
type SecretRepositoryInterface interface {
	Create(secret *models.Secret) error
	GetByID(secretID uuid.UUID) (*models.Secret, error)
	Update(secret *models.Secret) error
	Delete(secretID uuid.UUID) error
	GetChangedSince(since time.Time) ([]*models.Secret, error)
}

// MetadataRepositoryInterface defines the interface for metadata repository operations
type MetadataRepositoryInterface interface {
	Create(metadata *models.Metadata) error
	GetByID(metadataID uuid.UUID) (*models.Metadata, error)
	GetBySecretID(secretID uuid.UUID) ([]*models.Metadata, error)
	Update(metadata *models.Metadata) error
	Delete(metadataID uuid.UUID) error
	DeleteBySecretID(secretID uuid.UUID) error
	GetChangedSince(since time.Time) ([]*models.Metadata, error)
}
