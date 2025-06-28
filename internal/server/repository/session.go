package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SessionRepository handles database operations for sessions
type SessionRepository struct {
	db *sqlx.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create inserts a new session into the database
func (r *SessionRepository) Create(session *storage.Session) error {
	if session.SessionID == uuid.Nil {
		session.SessionID = uuid.New()
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO sessions (session_id, user_id, is_active, last_sync_date, created_at, expires_at)
		VALUES (:session_id, :user_id, :is_active, :last_sync_date, :created_at, :expires_at)
	`
	_, err := r.db.NamedExec(query, session)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetByID retrieves a session by its ID
func (r *SessionRepository) GetByID(sessionID uuid.UUID) (*storage.Session, error) {
	query := `
		SELECT session_id, user_id, is_active, last_sync_date, created_at, expires_at
		FROM sessions
		WHERE session_id = $1
	`

	session := &storage.Session{}
	err := r.db.Get(session, query, sessionID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetByUserID retrieves all sessions for a specific user
func (r *SessionRepository) GetByUserID(userID uuid.UUID) ([]*storage.Session, error) {
	query := `
		SELECT session_id, user_id, is_active, started_date, last_sync_date, end_date
		FROM sessions
		WHERE user_id = $1
		ORDER BY started_date DESC
	`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session := &storage.Session{}

		err := rows.Scan(
			&session.SessionID,
			&session.UserID,
			&session.IsActive,
			&session.StartedDate,
			&session.LastSyncDate,
			&session.EndDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetActiveSessions retrieves all active sessions for a user
func (r *SessionRepository) GetActiveSessions(userID uuid.UUID) ([]*storage.Session, error) {
	query := `
		SELECT session_id, user_id, is_active, started_date, last_sync_date, end_date
		FROM sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY started_date DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session := &storage.Session{}

		err := rows.Scan(
			&session.SessionID,
			&session.UserID,
			&session.IsActive,
			&session.StartedDate,
			&session.LastSyncDate,
			&session.EndDate,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// Update updates an existing session
func (r *SessionRepository) Update(session *storage.Session) error {
	query := `
		UPDATE sessions
		SET is_active = $2, last_sync_date = $3, end_date = $4
		WHERE session_id = $1
	`

	result, err := r.db.Exec(query,
		session.SessionID,
		session.IsActive,
		session.LastSyncDate,
		session.EndDate,
	)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// Delete removes a session from the database
func (r *SessionRepository) Delete(sessionID uuid.UUID) error {
	query := `DELETE FROM sessions WHERE session_id = $1`

	result, err := r.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}
