package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
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
func (r *MetadataRepository) Create(metadata *storage.Metadata) error {
	if metadata.MetadataID == uuid.Nil {
		metadata.MetadataID = uuid.New()
	}

	now := time.Now()
	metadata.CreatedDate = now
	metadata.LastUpdatedDate = now

	query := `
		INSERT INTO metadata (metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(query,
		metadata.MetadataID,
		metadata.SecretID,
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
func (r *MetadataRepository) GetByID(metadataID uuid.UUID) (*storage.Metadata, error) {
	query := `
		SELECT metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date
		FROM metadata
		WHERE metadata_id = $1
	`

	metadata := &storage.Metadata{}

	err := r.db.QueryRow(query, metadataID).Scan(
		&metadata.MetadataID,
		&metadata.SecretID,
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

	return metadata, nil
}

// GetBySecretID retrieves all metadata for a specific secret
func (r *MetadataRepository) GetBySecretID(secretID uuid.UUID) ([]*storage.Metadata, error) {
	query := `
		SELECT metadata_id, secret_id, key, value_hash, value_encrypted, created_date, last_updated_date
		FROM metadata
		WHERE secret_id = $1
		ORDER BY created_date DESC
	`

	rows, err := r.db.Query(query, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*storage.Metadata
	for rows.Next() {
		metadata := &storage.Metadata{}

		err := rows.Scan(
			&metadata.MetadataID,
			&metadata.SecretID,
			&metadata.Key,
			&metadata.ValueHash,
			&metadata.ValueEncrypted,
			&metadata.CreatedDate,
			&metadata.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}

		metadataList = append(metadataList, metadata)
	}

	return metadataList, rows.Err()
}

// Update updates an existing metadata entry
func (r *MetadataRepository) Update(metadata *storage.Metadata) error {
	metadata.LastUpdatedDate = time.Now()

	query := `
		UPDATE metadata
		SET key = $2, value_hash = $3, value_encrypted = $4, last_updated_date = $5
		WHERE metadata_id = $1
	`

	result, err := r.db.Exec(query,
		metadata.MetadataID,
		metadata.Key,
		metadata.ValueHash,
		metadata.ValueEncrypted,
		metadata.LastUpdatedDate,
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
	query := `DELETE FROM metadata WHERE metadata_id = $1`

	result, err := r.db.Exec(query, metadataID)
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
	query := `DELETE FROM metadata WHERE secret_id = $1`

	_, err := r.db.Exec(query, secretID)
	if err != nil {
		return fmt.Errorf("failed to delete metadata for secret: %w", err)
	}

	return nil
}

// GetChangedSince returns all metadata modified since the given date for secrets belonging to a user
func (r *MetadataRepository) GetChangedSince(userID uuid.UUID, since time.Time) ([]*storage.Metadata, error) {
	query := `
		SELECT m.metadata_id, m.secret_id, m.key, m.value_hash, m.value_encrypted, m.created_date, m.last_updated_date
		FROM metadata m
		INNER JOIN secrets s ON m.secret_id = s.secret_id
		WHERE s.user_id = $1 AND m.last_updated_date > $2
		ORDER BY m.last_updated_date ASC
	`

	rows, err := r.db.Query(query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*storage.Metadata
	for rows.Next() {
		metadata := &storage.Metadata{}

		err := rows.Scan(
			&metadata.MetadataID,
			&metadata.SecretID,
			&metadata.Key,
			&metadata.ValueHash,
			&metadata.ValueEncrypted,
			&metadata.CreatedDate,
			&metadata.LastUpdatedDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}

		metadataList = append(metadataList, metadata)
	}

	return metadataList, rows.Err()
}
