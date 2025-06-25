package storage

import (
	"errors"
	"fmt"

	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// DatabaseClient - Структура БД клиента. Содержит в себе ссылку на объект sqlx и путь к БД.
type DatabaseClient struct {
	*sqlx.DB
	DbPath string
}

// NewDatabaseClient - Создает новую структуру DatabaseClient с указателем.
func NewDatabaseClient(dbPath string) (*DatabaseClient, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys support
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	dc := &DatabaseClient{db, dbPath}

	// Initialize database with migrations
	if err := dc.InitDatabase(); err != nil {
		dc.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	log.Zap.Info("Database initialized successfully",
		zap.String("path", dbPath))

	return dc, nil
}

// Close - Закрывает соединение с БД.
func (dc *DatabaseClient) Close() error {
	err := dc.DB.Close()
	if err != nil {
		return fmt.Errorf("failed database connection close: %w", err)
	}
	log.Zap.Info("database connection closed")

	return nil
}

// InitDatabase - Инициализирует и применяет миграции БД.
func (dc *DatabaseClient) InitDatabase() error {
	driver, err := sqlite3.WithInstance(dc.DB.DB, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite driver: %w", err)
	}

	// Create iofs source from embedded filesystem
	sourceDriver, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	err = m.Up()
	noApply := errors.Is(err, migrate.ErrNoChange)

	if err != nil && !noApply {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	if !noApply {
		log.Zap.Info("Migrations applied successfully")
	}

	return nil
}
