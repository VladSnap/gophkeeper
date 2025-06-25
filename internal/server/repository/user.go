package repository

import (
	"database/sql"
	"fmt"

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
		INSERT INTO users (user_id, login, password)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Exec(query,
		user.UserID,
		user.Login,
		user.Password,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by its ID
func (r *UserRepository) GetByID(userID uuid.UUID) (*storage.User, error) {
	query := `
		SELECT user_id, login, password
		FROM users
		WHERE user_id = $1
	`

	user := &storage.User{}

	err := r.db.QueryRow(query, userID).Scan(
		&user.UserID,
		&user.Login,
		&user.Password,
	)

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
		SELECT user_id, login, password
		FROM users
		WHERE login = $1
	`

	user := &storage.User{}

	err := r.db.QueryRow(query, login).Scan(
		&user.UserID,
		&user.Login,
		&user.Password,
	)

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
		SET login = $2, password = $3
		WHERE user_id = $1
	`

	result, err := r.db.Exec(query,
		user.UserID,
		user.Login,
		user.Password,
	)

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
	query := `DELETE FROM users WHERE user_id = $1`

	result, err := r.db.Exec(query, userID)
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
