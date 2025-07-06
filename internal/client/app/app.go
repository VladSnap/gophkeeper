package app

import (
	"fmt"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/config"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/internal/client/service"
	"github.com/VladSnap/gophkeeper/internal/client/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/VladSnap/gophkeeper/pkg/resManager"
	"go.uber.org/zap"
)

type Application struct {
	ResMng          *resManager.ResourceManager
	Cfg             *config.Config
	UserManager     *service.UserManager
	Db              *storage.DatabaseClient
	AuthService     *service.AuthService
	ServiceFactory  *service.ServiceFactory
	SyncService     *service.SyncService
	AutoSyncService *service.AutoSyncService
}

func New() *Application {
	return &Application{
		ResMng: resManager.NewResourceManager(),
	}
}

func (app *Application) Init() error {
	err := app.initConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	err = app.initFirstServices()
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	return nil
}

func (app *Application) Stop() error {
	// Останавливаем автоматическую синхронизацию
	app.StopAutoSync()

	err := app.ResMng.Cleanup()
	if err != nil {
		return fmt.Errorf("failed to cleanup resources: %w", err)
	}
	log.Zap.Info("Application stopped successfully")
	return nil
}

func (app *Application) SetAppUser(username string, isNewUser bool) error {
	if app.UserManager == nil {
		return fmt.Errorf("user manager is not initialized")
	}

	// Setup user environment
	if err := app.UserManager.SetupUserEnvironment(username, isNewUser); err != nil {
		return fmt.Errorf("failed to setup user environment: %w", err)
	}

	log.Zap.Info("User selected",
		zap.String("username", username),
		zap.Bool("is_new_user", isNewUser),
		zap.String("database_path", app.Cfg.DatabasePath),
		zap.String("user_data_dir", app.Cfg.GetUserDataDir()))

	if err := app.InitDataBase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize user-specific services
	if err := app.initUserServices(); err != nil {
		return fmt.Errorf("failed to initialize user services: %w", err)
	}

	return nil
}

func (app *Application) initConfig() error {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	app.Cfg = cfg

	log.Zap.Info("Configuration loaded",
		zap.String("data_dir", cfg.DataDir),
		zap.String("server_url", cfg.ServerURL))

	return nil
}

func (app *Application) initFirstServices() error {
	// Create user manager
	app.UserManager = service.NewUserManager(app.Cfg)
	log.Zap.Info("First services initialized successfully")
	return nil
}

func (app *Application) InitDataBase() error {
	// Initialize database for this user
	db, err := storage.NewDatabaseClient(app.Cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	app.ResMng.Register(db.Close)
	app.Db = db
	log.Zap.Info("Database initialized successfully")

	return nil
}

func (app *Application) initUserServices() error {
	// Initialize repositories
	secretRepo := repository.NewSecretRepository(app.Db.DB)
	metadataRepo := repository.NewMetadataRepository(app.Db.DB)

	// Initialize services
	authService := service.NewAuthService(app.Cfg.ServerURL)
	authService.SetUser(app.Cfg.Username, app.Cfg.GetUserDataDir())

	app.AuthService = authService
	app.ServiceFactory = service.NewServiceFactory(secretRepo, metadataRepo, authService.GetMasterPasswordManager())
	app.SyncService = service.NewSyncService(app.Cfg.ServerURL, authService)
	app.AutoSyncService = service.NewAutoSyncService(app.ServiceFactory.ClientSyncService(), app.SyncService)

	// Устанавливаем путь к файлу состояния синхронизации
	app.AutoSyncService.SetSyncStateFile(app.Cfg.GetUserDataDir())

	log.Zap.Info("Services initialized successfully")
	return nil
}

// StartAutoSync запускает автоматическую синхронизацию
func (app *Application) StartAutoSync() error {
	if app.AutoSyncService == nil {
		return fmt.Errorf("auto sync service is not initialized")
	}
	return app.AutoSyncService.Start()
}

// StopAutoSync останавливает автоматическую синхронизацию
func (app *Application) StopAutoSync() {
	if app.AutoSyncService != nil {
		app.AutoSyncService.Stop()
	}
}

// IsAutoSyncRunning возвращает статус автоматической синхронизации
func (app *Application) IsAutoSyncRunning() bool {
	if app.AutoSyncService == nil {
		return false
	}
	return app.AutoSyncService.IsRunning()
}

// ForceSync принудительно запускает синхронизацию
func (app *Application) ForceSync() error {
	if app.AutoSyncService == nil {
		return fmt.Errorf("auto sync service is not initialized")
	}
	return app.AutoSyncService.ForceSync()
}

// GetLastSyncTime возвращает время последней синхронизации
func (app *Application) GetLastSyncTime() time.Time {
	if app.AutoSyncService == nil {
		return time.Time{}
	}
	return app.AutoSyncService.GetLastSyncTime()
}
