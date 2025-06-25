package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/config"
	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/internal/client/service"
	"github.com/VladSnap/gophkeeper/internal/client/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Printf("Failed to close logger: %v\n", err)
		}
	}()

	log.Zap.Info("Starting gophkeeper client application")

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Zap.Error("Failed to load configuration", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Configuration loaded",
		zap.String("database_path", cfg.DatabasePath),
		zap.String("data_dir", cfg.DataDir),
		zap.String("server_url", cfg.ServerURL))

	// Initialize database
	db, err := storage.NewDatabaseClient(cfg.DatabasePath)
	if err != nil {
		log.Zap.Error("Failed to initialize database", zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Zap.Error("Failed to close database", zap.Error(err))
		}
	}()

	log.Zap.Info("Database initialized successfully")

	// Initialize repositories
	secretRepo := repository.NewSecretRepository(db.DB)
	metadataRepo := repository.NewMetadataRepository(db.DB)

	// Initialize services
	clientService := service.NewClientService(secretRepo, metadataRepo)
	authService := service.NewAuthService(cfg.ServerURL, cfg.DataDir)
	syncService := service.NewSyncService(cfg.ServerURL, authService)

	log.Zap.Info("Services initialized successfully")

	// Run CLI
	if err := runCLI(authService, syncService, clientService); err != nil {
		log.Zap.Error("CLI error", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Application completed successfully")
}

func runCLI(authService *service.AuthService, syncService *service.SyncService, clientService *service.ClientService) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if authService.IsLoggedIn() {
			fmt.Println("\n=== Gophkeeper Client (Authenticated) ===")
			fmt.Println("1. Sync with server")
			fmt.Println("2. Create test secret")
			fmt.Println("3. List secrets")
			fmt.Println("4. Logout")
			fmt.Println("5. Exit")
		} else {
			fmt.Println("\n=== Gophkeeper Client (Not Authenticated) ===")
			fmt.Println("1. Register")
			fmt.Println("2. Login")
			fmt.Println("3. Exit")
		}

		fmt.Print("Choose option: ")
		if !scanner.Scan() {
			break
		}

		choice := strings.TrimSpace(scanner.Text())

		if authService.IsLoggedIn() {
			if err := handleAuthenticatedChoice(choice, authService, syncService, clientService, scanner); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			if err := handleUnauthenticatedChoice(choice, authService, scanner); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}

		if choice == "3" && !authService.IsLoggedIn() || choice == "5" && authService.IsLoggedIn() {
			break
		}
	}

	return nil
}

func handleUnauthenticatedChoice(choice string, authService *service.AuthService, scanner *bufio.Scanner) error {
	switch choice {
	case "1": // Register
		return handleRegister(authService, scanner)
	case "2": // Login
		return handleLogin(authService, scanner)
	case "3": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}

func handleAuthenticatedChoice(choice string, authService *service.AuthService, syncService *service.SyncService, clientService *service.ClientService, scanner *bufio.Scanner) error {
	switch choice {
	case "1": // Sync
		return handleSync(syncService)
	case "2": // Create secret
		return handleCreateSecret(clientService, scanner)
	case "3": // Logout
		return authService.Logout()
	case "4": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}

func handleRegister(authService *service.AuthService, scanner *bufio.Scanner) error {
	fmt.Print("Username: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read username")
	}
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read password")
	}
	password := strings.TrimSpace(scanner.Text())

	if err := authService.Register(username, password); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	fmt.Println("Registration successful!")
	return nil
}

func handleLogin(authService *service.AuthService, scanner *bufio.Scanner) error {
	fmt.Print("Username: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read username")
	}
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read password")
	}
	password := strings.TrimSpace(scanner.Text())

	if err := authService.Login(username, password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("Login successful!")
	return nil
}

func handleSync(syncService *service.SyncService) error {
	fmt.Println("Syncing with server...")

	// Pull changes from last 24 hours for testing
	since := time.Now().Add(-24 * time.Hour)
	pullResp, err := syncService.Pull(since)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Printf("Pulled %d secrets and %d metadata entries\n", len(pullResp.Secrets), len(pullResp.Metadata))

	// For now, just test push with empty data
	pushResp, err := syncService.Push([]*models.Secret{}, []*models.Metadata{})
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	fmt.Printf("Push result: %s\n", pushResp.Message)
	return nil
}

func handleCreateSecret(clientService *service.ClientService, scanner *bufio.Scanner) error {
	fmt.Print("Enter secret data: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read secret data")
	}
	secretData := strings.TrimSpace(scanner.Text())

	userID := uuid.New() // In real app, this would be from auth context
	metadata := map[string]string{
		"type":        "test",
		"description": "Test secret created from CLI",
		"created_by":  "cli",
	}

	secret, err := clientService.CreateSecret(userID, secretData, metadata)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	fmt.Printf("Secret created: %s\n", secret.SecretID.String())
	return nil
}
