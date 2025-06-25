package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
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
func (r *SecretRepository) Create(secret *models.Secret) error {
	if secret.SecretID == uuid.Nil {
		secret.SecretID = uuid.New()
	}

	now := time.Now()
	secret.CreatedDate = now
	secret.LastUpdatedDate = now

	query := `
		INSERT INTO secrets (secret_id, encrypted, created_date, last_updated_date)
		VALUES (?, ?, ?, ?)
	`

	_, err := r.db.Exec(query,
		secret.SecretID.String(),
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
func (r *SecretRepository) GetByID(secretID uuid.UUID) (*models.Secret, error) {
	query := `
		SELECT secret_id, encrypted, created_date, last_updated_date
		FROM secrets
		WHERE secret_id = ?
	`

	secret := &models.Secret{}
	var secretIDStr string

	err := r.db.QueryRow(query, secretID.String()).Scan(
		&secretIDStr,
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

	secret.SecretID, err = uuid.Parse(secretIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret ID: %w", err)
	}

	return secret, nil
}

// Update updates an existing secret
func (r *SecretRepository) Update(secret *models.Secret) error {
	secret.LastUpdatedDate = time.Now()

	query := `
		UPDATE secrets
		SET encrypted = ?, last_updated_date = ?
		WHERE secret_id = ?
	`

	result, err := r.db.Exec(query,
		secret.Encrypted,
		secret.LastUpdatedDate,
		secret.SecretID.String(),
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
	query := `DELETE FROM secrets WHERE secret_id = ?`

	result, err := r.db.Exec(query, secretID.String())
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

// GetChangedSince returns all secrets modified since the given date
func (r *SecretRepository) GetChangedSince(since time.Time) ([]*models.Secret, error) {
	query := `
		SELECT secret_id, encrypted, created_date, last_updated_date
		FROM secrets
		WHERE last_updated_date > ?
		ORDER BY last_updated_date ASC
	`

	rows, err := r.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed secrets: %w", err)
	}
	defer rows.Close()

	var secrets []*models.Secret
	for rows.Next() {
		secret := &models.Secret{}
		var secretIDStr string

		err := rows.Scan(
			&secretIDStr,
			&secret.Encrypted,
			&secret.CreatedDate,
			&secret.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}

		secret.SecretID, err = uuid.Parse(secretIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse secret ID: %w", err)
		}

		secrets = append(secrets, secret)
	}

	return secrets, rows.Err()
}
