package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/VladSnap/gophkeeper/internal/client/config"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
	"github.com/VladSnap/gophkeeper/internal/client/service"
	"github.com/VladSnap/gophkeeper/internal/client/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
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
	authService := service.NewAuthService(cfg.ServerURL, cfg.DataDir)
	clientService := service.NewClientService(secretRepo, metadataRepo, authService.GetMasterPasswordManager())
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

	// Сначала проверяем/настраиваем мастер-пароль
	if err := handleMasterPasswordSetup(authService, scanner); err != nil {
		return fmt.Errorf("master password setup failed: %w", err)
	}

	for {
		if authService.IsLoggedIn() {
			fmt.Println("\n=== Gophkeeper Client (Authenticated) ===")
			fmt.Println("1. Sync with server (last 24 hours)")
			fmt.Println("2. Full sync (all data)")
			fmt.Println("3. Create test secret")
			fmt.Println("4. List secrets")
			fmt.Println("5. Logout")
			fmt.Println("6. Lock master password")
			fmt.Println("7. Change master password")
			fmt.Println("8. Exit")
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

		// Обновленные условия выхода
		if choice == "3" && !authService.IsLoggedIn() || choice == "8" && authService.IsLoggedIn() {
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
		return handleSync(syncService, clientService)
	case "2": // Full sync
		return handleFullSync(syncService, clientService)
	case "3": // Create secret
		return handleCreateSecret(clientService, scanner)
	case "4": // List secrets
		return handleListSecrets(clientService)
	case "5": // Logout
		return authService.Logout()
	case "6": // Lock master password
		return handleLockMasterPassword(authService)
	case "7": // Change master password
		return handleChangeMasterPassword(authService, scanner)
	case "8": // Exit
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
	// Проверяем и разблокируем мастер-пароль если нужно
	if !authService.IsMasterPasswordUnlocked() {
		if err := handleMasterPasswordSetup(authService, scanner); err != nil {
			return fmt.Errorf("master password setup failed: %w", err)
		}
	}

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

func handleSync(syncService *service.SyncService, clientService *service.ClientService) error {
	fmt.Println("Syncing with server...")

	// Выполняем инкрементальную синхронизацию
	if err := clientService.PerformSync(syncService, service.SyncTypeIncremental); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("Sync completed successfully!")
	return nil
}

func handleCreateSecret(clientService *service.ClientService, scanner *bufio.Scanner) error {
	fmt.Print("Enter secret data: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read secret data")
	}
	secretData := strings.TrimSpace(scanner.Text())

	metadata := map[string]string{
		"type":        "test",
		"description": "Test secret created from CLI",
		"created_by":  "cli",
	}

	secret, err := clientService.CreateSecret(secretData, metadata)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	fmt.Printf("Secret created: %s\n", secret.SecretID.String())
	return nil
}

func handleListSecrets(clientService *service.ClientService) error {
	fmt.Println("Listing all secrets...")

	secrets, err := clientService.GetAllSecrets()
	if err != nil {
		return fmt.Errorf("failed to get secrets: %w", err)
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	fmt.Printf("Found %d secret(s):\n\n", len(secrets))
	for i, secret := range secrets {
		fmt.Printf("%d. Secret ID: %s\n", i+1, secret.SecretID.String())
		fmt.Printf("   Created: %s\n", secret.CreatedDate.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Last Updated: %s\n", secret.LastUpdatedDate.Format("2006-01-02 15:04:05"))

		// Получаем расшифрованные данные секрета
		decryptedData, err := clientService.DecryptSecretData(secret.Encrypted)
		if err != nil {
			fmt.Printf("   Data: [Error decrypting: %v]\n", err)
		} else {
			// Показываем только первые 50 символов для безопасности
			if len(decryptedData) > 50 {
				fmt.Printf("   Data: %s...\n", decryptedData[:50])
			} else {
				fmt.Printf("   Data: %s\n", decryptedData)
			}
		}

		// Get metadata for this secret
		metadata, err := clientService.GetMetadataBySecretID(secret.SecretID)
		if err != nil {
			fmt.Printf("   Metadata: Error retrieving (%v)\n", err)
		} else if len(metadata) > 0 {
			fmt.Printf("   Metadata:\n")
			for _, meta := range metadata {
				// Расшифровываем значение метаданных
				decryptedValue, err := clientService.DecryptMetadataValue(meta.ValueEncrypted)
				if err != nil {
					fmt.Printf("     %s: [Error decrypting: %v]\n", meta.Key, err)
				} else {
					fmt.Printf("     %s: %s\n", meta.Key, decryptedValue)
				}
			}
		} else {
			fmt.Printf("   Metadata: None\n")
		}
		fmt.Println()
	}

	return nil
}

func handleFullSync(syncService *service.SyncService, clientService *service.ClientService) error {
	fmt.Println("Performing full synchronization...")

	// Выполняем полную синхронизацию
	if err := clientService.PerformSync(syncService, service.SyncTypeFull); err != nil {
		return fmt.Errorf("full sync failed: %w", err)
	}

	fmt.Println("Full synchronization completed successfully!")
	return nil
}

// handleMasterPasswordSetup обрабатывает настройку/разблокирование мастер-пароля при запуске
func handleMasterPasswordSetup(authService *service.AuthService, scanner *bufio.Scanner) error {
	if !authService.IsMasterPasswordSet() {
		fmt.Println("Master password is not set. Setting up master password...")
		fmt.Print("Enter new master password: ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read master password")
		}
		masterPassword := strings.TrimSpace(scanner.Text())

		if len(masterPassword) < 6 {
			return fmt.Errorf("master password must be at least 6 characters long")
		}

		if err := authService.SetMasterPassword(masterPassword); err != nil {
			return fmt.Errorf("failed to set master password: %w", err)
		}

		fmt.Println("Master password set successfully!")
	} else if !authService.IsMasterPasswordUnlocked() {
		fmt.Println("Master password is required to access your data.")
		fmt.Print("Enter master password: ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read master password")
		}
		masterPassword := strings.TrimSpace(scanner.Text())

		if err := authService.UnlockMasterPassword(masterPassword); err != nil {
			return fmt.Errorf("failed to unlock master password: %w", err)
		}

		fmt.Println("Master password unlocked successfully!")
	}

	return nil
}

// handleLockMasterPassword блокирует мастер-пароль
func handleLockMasterPassword(authService *service.AuthService) error {
	authService.LockMasterPassword()
	fmt.Println("Master password locked. You will need to unlock it to access your data again.")

	// После блокировки мастер-пароля пользователь автоматически разлогинивается
	if err := authService.Logout(); err != nil {
		log.Zap.Warn("Failed to logout after locking master password", zap.Error(err))
	}

	return nil
}

// handleChangeMasterPassword изменяет мастер-пароль
func handleChangeMasterPassword(authService *service.AuthService, scanner *bufio.Scanner) error {
	fmt.Print("Enter current master password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read current master password")
	}
	oldPassword := strings.TrimSpace(scanner.Text())

	fmt.Print("Enter new master password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read new master password")
	}
	newPassword := strings.TrimSpace(scanner.Text())

	if len(newPassword) < 6 {
		return fmt.Errorf("new master password must be at least 6 characters long")
	}

	fmt.Print("Confirm new master password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read password confirmation")
	}
	confirmPassword := strings.TrimSpace(scanner.Text())

	if newPassword != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	if err := authService.ChangeMasterPassword(oldPassword, newPassword); err != nil {
		return fmt.Errorf("failed to change master password: %w", err)
	}

	fmt.Println("Master password changed successfully!")
	return nil
}
