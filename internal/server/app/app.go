package app

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/handlers"
	"github.com/VladSnap/gophkeeper/internal/server/repository"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/VladSnap/gophkeeper/pkg/resManager"
	"go.uber.org/zap"
)

type Application struct {
	ResMng       *resManager.ResourceManager
	Cfg          *AppConfig
	DB           *storage.DatabaseServer
	UserRepo     repository.UserRepositoryInterface
	SessionRepo  repository.SessionRepositoryInterface
	SecretRepo   repository.SecretRepositoryInterface
	MetadataRepo repository.MetadataRepositoryInterface
	AuthHandler  *handlers.AuthHandler
	SyncHandler  *handlers.SyncHandler
	Server       *Server
}

func New() *Application {
	return &Application{
		ResMng: resManager.NewResourceManager(),
	}
}

func (app *Application) Init() error {
	// Parse configuration
	cfg, err := ParseFlags()
	if err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}
	app.Cfg = cfg

	log.Zap.Info("Configuration loaded",
		zap.String("database_uri", cfg.DatabaseURI),
		zap.String("server_address", cfg.ServerAddress))

	// Initialize database
	if err := app.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize repositories
	if err := app.initRepositories(); err != nil {
		return fmt.Errorf("failed to initialize repositories: %w", err)
	}

	// Initialize handlers
	if err := app.initHandlers(); err != nil {
		return fmt.Errorf("failed to initialize handlers: %w", err)
	}

	// Initialize server
	if err := app.initServer(); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	return nil
}

func (app *Application) Stop() error {
	log.Zap.Info("Stopping application...")

	// Stop server first
	if app.Server != nil {
		if err := app.Server.Stop(); err != nil {
			log.Zap.Error("Failed to stop server", zap.Error(err))
		}
	}

	// Cleanup resources
	if err := app.ResMng.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup resources: %w", err)
	}

	log.Zap.Info("Application stopped successfully")
	return nil
}

func (app *Application) Run() error {
	log.Zap.Info("Starting application...")

	// Start server
	if err := app.Server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func (app *Application) initDatabase() error {
	db, err := storage.NewDatabaseServer(app.Cfg.DatabaseURI)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}

	// Register database cleanup
	app.ResMng.Register(db.Close)
	app.DB = db

	// Initialize database schema
	if err := db.InitDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	log.Zap.Info("Database initialized successfully")
	return nil
}

func (app *Application) initRepositories() error {
	app.UserRepo = repository.NewUserRepository(app.DB.DB)
	app.SessionRepo = repository.NewSessionRepository(app.DB.DB)
	app.SecretRepo = repository.NewSecretRepository(app.DB.DB)
	app.MetadataRepo = repository.NewMetadataRepository(app.DB.DB)

	log.Zap.Info("Repositories initialized successfully")
	return nil
}

func (app *Application) initHandlers() error {
	// TODO: Move JWT secret to config
	jwtSecret := "secret-key"

	app.AuthHandler = handlers.NewAuthHandler(app.UserRepo, app.SessionRepo, jwtSecret)
	app.SyncHandler = handlers.NewSyncHandler(app.SecretRepo, app.MetadataRepo)

	log.Zap.Info("Handlers initialized successfully")
	return nil
}

func (app *Application) initServer() error {
	router := app.SetupRoutes()

	app.Server = NewServer(app.Cfg.ServerAddress, router, 30*time.Second)

	log.Zap.Info("Server initialized successfully")
	return nil
}
