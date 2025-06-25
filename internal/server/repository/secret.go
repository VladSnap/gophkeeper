package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SecretRepository handles database operations for secrets
type SecretRepository struct {
	db *sqlx.DB
}

// NewSecretRepository creates a new secret repository
func NewSecretRepository(db *sqlx.DB) *SecretRepository {
	return &SecretRepository{db: db}
}

// Create inserts a new secret into the database
func (r *SecretRepository) Create(secret *storage.Secret) error {
	if secret.SecretID == uuid.Nil {
		secret.SecretID = uuid.New()
	}

	now := time.Now()
	secret.CreatedDate = now
	secret.LastUpdatedDate = now

	query := `
		INSERT INTO secrets (secret_id, user_id, encrypted, created_date, last_updated_date)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Exec(query,
		secret.SecretID,
		secret.UserID,
		secret.Encrypted,
		secret.CreatedDate,
		secret.LastUpdatedDate,
	)

	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// GetByID retrieves a secret by its ID
func (r *SecretRepository) GetByID(secretID uuid.UUID) (*storage.Secret, error) {
	query := `
		SELECT secret_id, user_id, encrypted, created_date, last_updated_date
		FROM secrets
		WHERE secret_id = $1
	`

	secret := &storage.Secret{}

	err := r.db.QueryRow(query, secretID).Scan(
		&secret.SecretID,
		&secret.UserID,
		&secret.Encrypted,
		&secret.CreatedDate,
		&secret.LastUpdatedDate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("secret not found")
		}
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// GetByUserID retrieves all secrets for a specific user
func (r *SecretRepository) GetByUserID(userID uuid.UUID) ([]*storage.Secret, error) {
	query := `
		SELECT secret_id, user_id, encrypted, created_date, last_updated_date
		FROM secrets
		WHERE user_id = $1
		ORDER BY created_date DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets: %w", err)
	}
	defer rows.Close()

	var secrets []*storage.Secret
	for rows.Next() {
		secret := &storage.Secret{}

		err := rows.Scan(
			&secret.SecretID,
			&secret.UserID,
			&secret.Encrypted,
			&secret.CreatedDate,
			&secret.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}

		secrets = append(secrets, secret)
	}

	return secrets, rows.Err()
}

// Update updates an existing secret
func (r *SecretRepository) Update(secret *storage.Secret) error {
	secret.LastUpdatedDate = time.Now()

	query := `
		UPDATE secrets
		SET encrypted = $2, last_updated_date = $3
		WHERE secret_id = $1
	`

	result, err := r.db.Exec(query,
		secret.SecretID,
		secret.Encrypted,
		secret.LastUpdatedDate,
	)

	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("secret not found")
	}

	return nil
}

// Delete removes a secret from the database
func (r *SecretRepository) Delete(secretID uuid.UUID) error {
	query := `DELETE FROM secrets WHERE secret_id = $1`

	result, err := r.db.Exec(query, secretID)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("secret not found")
	}

	return nil
}

// GetChangedSince returns all secrets modified since the given date for a specific user
func (r *SecretRepository) GetChangedSince(userID uuid.UUID, since time.Time) ([]*storage.Secret, error) {
	query := `
		SELECT secret_id, user_id, encrypted, created_date, last_updated_date
		FROM secrets
		WHERE user_id = $1 AND last_updated_date > $2
		ORDER BY last_updated_date ASC
	`

	rows, err := r.db.Query(query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}
	defer rows.Close()

	var secrets []*storage.Secret
	for rows.Next() {
		secret := &storage.Secret{}

		err := rows.Scan(
			&secret.SecretID,
			&secret.UserID,
			&secret.Encrypted,
			&secret.CreatedDate,
			&secret.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}

		secrets = append(secrets, secret)
	}

	return secrets, rows.Err()
}
