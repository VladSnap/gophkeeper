package repository

import (
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/google/uuid"
)

// UserRepositoryInterface defines the interface for user repository operations
type UserRepositoryInterface interface {
	Create(user *storage.User) error
	GetByID(userID uuid.UUID) (*storage.User, error)
	GetByLogin(login string) (*storage.User, error)
	Update(user *storage.User) error
	Delete(userID uuid.UUID) error
}

// SessionRepositoryInterface defines the interface for session repository operations
type SessionRepositoryInterface interface {
	Create(session *storage.Session) error
	GetByID(sessionID uuid.UUID) (*storage.Session, error)
	GetByUserID(userID uuid.UUID) ([]*storage.Session, error)
	Update(session *storage.Session) error
	Delete(sessionID uuid.UUID) error
	GetActiveSessions(userID uuid.UUID) ([]*storage.Session, error)
}

// SecretRepositoryInterface defines the interface for secret repository operations
type SecretRepositoryInterface interface {
	Create(secret *storage.Secret) error
	GetByID(secretID uuid.UUID) (*storage.Secret, error)
	GetByUserID(userID uuid.UUID) ([]*storage.Secret, error)
	Update(secret *storage.Secret) error
	Delete(secretID uuid.UUID) error
	GetChangedSince(userID uuid.UUID, since time.Time) ([]*storage.Secret, error)
}

// MetadataRepositoryInterface defines the interface for metadata repository operations
type MetadataRepositoryInterface interface {
	Create(metadata *storage.Metadata) error
	GetByID(metadataID uuid.UUID) (*storage.Metadata, error)
	GetBySecretID(secretID uuid.UUID) ([]*storage.Metadata, error)
	Update(metadata *storage.Metadata) error
	Delete(metadataID uuid.UUID) error
	DeleteBySecretID(secretID uuid.UUID) error
	GetChangedSince(userID uuid.UUID, since time.Time) ([]*storage.Metadata, error)
}
