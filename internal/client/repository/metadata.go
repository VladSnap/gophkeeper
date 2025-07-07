package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// MetadataRepository handles database operations for metadata
type MetadataRepository struct {
	db *sqlx.DB
}

// NewMetadataRepository creates a new metadata repository
func NewMetadataRepository(db *sqlx.DB) *MetadataRepository {
	return &MetadataRepository{db: db}
}

// Create inserts a new metadata entry into the database
func (r *MetadataRepository) Create(metadata *models.Metadata) error {
	if metadata.MetadataID == uuid.Nil {
		metadata.MetadataID = uuid.New()
	}

	now := time.Now()
	metadata.CreatedDate = now
	metadata.LastUpdatedDate = now

	query := `
		INSERT INTO metadata (metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query,
		metadata.MetadataID.String(),
		metadata.SecretID.String(),
		metadata.Key,
		metadata.ValueHash,
		metadata.ValueEncrypted,
		metadata.CreatedDate,
		metadata.LastUpdatedDate,
	)

	if err != nil {
		return fmt.Errorf("failed to create metadata: %w", err)
	}

	return nil
}

// GetByID retrieves metadata by its ID
func (r *MetadataRepository) GetByID(metadataID uuid.UUID) (*models.Metadata, error) {
	query := `
		SELECT metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date
		FROM metadata
		WHERE metadata_id = ?
	`

	metadata := &models.Metadata{}
	var metadataIDStr, secretIDStr string

	err := r.db.QueryRow(query, metadataID.String()).Scan(
		&metadataIDStr,
		&secretIDStr,
		&metadata.Key,
		&metadata.ValueHash,
		&metadata.ValueEncrypted,
		&metadata.CreatedDate,
		&metadata.LastUpdatedDate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metadata not found")
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	metadata.MetadataID, err = uuid.Parse(metadataIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata ID: %w", err)
	}

	metadata.SecretID, err = uuid.Parse(secretIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret ID: %w", err)
	}

	return metadata, nil
}

// GetBySecretID retrieves all metadata for a specific secret
func (r *MetadataRepository) GetBySecretID(secretID uuid.UUID) ([]*models.Metadata, error) {
	query := `
		SELECT metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date
		FROM metadata
		WHERE secret_id = ?
		ORDER BY created_date DESC
	`

	rows, err := r.db.Query(query, secretID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*models.Metadata
	for rows.Next() {
		metadata := &models.Metadata{}
		var metadataIDStr, secretIDStr string

		err := rows.Scan(
			&metadataIDStr,
			&secretIDStr,
			&metadata.Key,
			&metadata.ValueHash,
			&metadata.ValueEncrypted,
			&metadata.CreatedDate,
			&metadata.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}

		metadata.MetadataID, err = uuid.Parse(metadataIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata ID: %w", err)
		}

		metadata.SecretID, err = uuid.Parse(secretIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse secret ID: %w", err)
		}

		metadataList = append(metadataList, metadata)
	}

	return metadataList, rows.Err()
}

// Update updates an existing metadata entry
func (r *MetadataRepository) Update(metadata *models.Metadata) error {
	metadata.LastUpdatedDate = time.Now()

	query := `
		UPDATE metadata
		SET key = ?, value_hash = ?, value_encrypted = ?, last_updated_date = ?
		WHERE metadata_id = ?
	`

	result, err := r.db.Exec(query,
		metadata.Key,
		metadata.ValueHash,
		metadata.ValueEncrypted,
		metadata.LastUpdatedDate,
		metadata.MetadataID.String(),
	)

	if err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("metadata not found")
	}

	return nil
}

// Delete removes metadata from the database
func (r *MetadataRepository) Delete(metadataID uuid.UUID) error {
	query := `DELETE FROM metadata WHERE metadata_id = ?`

	result, err := r.db.Exec(query, metadataID.String())
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("metadata not found")
	}

	return nil
}

// DeleteBySecretID removes all metadata for a specific secret
func (r *MetadataRepository) DeleteBySecretID(secretID uuid.UUID) error {
	query := `DELETE FROM metadata WHERE secret_id = ?`

	_, err := r.db.Exec(query, secretID.String())
	if err != nil {
		return fmt.Errorf("failed to delete metadata for secret: %w", err)
	}

	return nil
}

// GetChangedSince returns all metadata modified since the given date
func (r *MetadataRepository) GetChangedSince(since time.Time) ([]*models.Metadata, error) {
	query := `
		SELECT metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date
		FROM metadata
		WHERE last_updated_date > ?
		ORDER BY last_updated_date ASC
	`

	rows, err := r.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*models.Metadata
	for rows.Next() {
		metadata := &models.Metadata{}
		var metadataIDStr, secretIDStr string

		err := rows.Scan(
			&metadataIDStr,
			&secretIDStr,
			&metadata.Key,
			&metadata.ValueHash,
			&metadata.ValueEncrypted,
			&metadata.CreatedDate,
			&metadata.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}

		metadata.MetadataID, err = uuid.Parse(metadataIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata ID: %w", err)
		}

		metadata.SecretID, err = uuid.Parse(secretIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse secret ID: %w", err)
		}

		metadataList = append(metadataList, metadata)
	}

	return metadataList, rows.Err()
}
