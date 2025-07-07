package storage

import (
	"errors"
	"fmt"

	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// DatabaseServer - Структура БД сервера. Содержит в себе ссылку на объект sql и строку подключения к БД.
type DatabaseServer struct {
	*sqlx.DB
	Dsn string
}

// NewDatabaseServer - Создает новую структуру DatabaseServer с указателем.
func NewDatabaseServer(dsn string) (*DatabaseServer, error) {
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed open database: %w", err)
	}

	ds := &DatabaseServer{db, dsn}
	return ds, nil
}

// Close - Закрывает соединение с БД.
func (ds *DatabaseServer) Close() error {
	err := ds.DB.Close()
	if err != nil {
		return fmt.Errorf("failed database connection close: %w", err)
	}
	log.Zap.Info("database connection closed")

	return nil
}

// InitDatabase - Инициализирует и применяет миграции БД.
func (ds *DatabaseServer) InitDatabase() error {
	driver, err := postgres.WithInstance(ds.DB.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
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
