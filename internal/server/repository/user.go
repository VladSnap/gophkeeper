package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user into the database
func (r *UserRepository) Create(user *storage.User) error {
	if user.UserID == uuid.Nil {
		user.UserID = uuid.New()
	}

	query := `
		INSERT INTO users (user_id, login, password, created_at)
		VALUES (:user_id, :login, :password, :created_at)
	`

	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	_, err := r.db.NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by its ID
func (r *UserRepository) GetByID(userID uuid.UUID) (*storage.User, error) {
	query := `
		SELECT user_id, login, password, created_at
		FROM users
		WHERE user_id = $1
	`

	user := &storage.User{}
	err := r.db.Get(user, query, userID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByLogin retrieves a user by login
func (r *UserRepository) GetByLogin(login string) (*storage.User, error) {
	query := `
		SELECT user_id, login, password, created_at
		FROM users
		WHERE login = $1
	`

	user := &storage.User{}
	err := r.db.Get(user, query, login)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// Update updates an existing user
func (r *UserRepository) Update(user *storage.User) error {
	query := `
		UPDATE users
		SET login = :login, password = :password
		WHERE user_id = :user_id
	`

	result, err := r.db.NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Delete removes a user from the database
func (r *UserRepository) Delete(userID uuid.UUID) error {
	query := `DELETE FROM users WHERE user_id = :user_id`

	params := map[string]interface{}{
		"user_id": userID,
	}

	result, err := r.db.NamedExec(query, params)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
