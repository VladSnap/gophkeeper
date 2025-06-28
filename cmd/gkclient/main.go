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
		zap.String("data_dir", cfg.DataDir),
		zap.String("server_url", cfg.ServerURL))

	// Create user manager
	userManager := service.NewUserManager(cfg)
	scanner := bufio.NewScanner(os.Stdin)

	// Select or create user
	username, isNewUser, err := userManager.SelectOrCreateUser(scanner)
	if err != nil {
		log.Zap.Error("Failed to select user", zap.Error(err))
		os.Exit(1)
	}

	// Setup user environment
	if err := userManager.SetupUserEnvironment(username); err != nil {
		log.Zap.Error("Failed to setup user environment", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("User selected",
		zap.String("username", username),
		zap.Bool("is_new_user", isNewUser),
		zap.String("database_path", cfg.DatabasePath),
		zap.String("user_data_dir", cfg.GetUserDataDir()))

	// Initialize database for this user
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
	authService := service.NewAuthService(cfg.ServerURL)
	authService.SetUser(username, cfg.GetUserDataDir())
	clientService := service.NewClientService(secretRepo, metadataRepo, authService.GetMasterPasswordManager())
	syncService := service.NewSyncService(cfg.ServerURL, authService)

	log.Zap.Info("Services initialized successfully")

	// Handle user authentication
	if err := handleUserAuthentication(authService, username, isNewUser, scanner); err != nil {
		log.Zap.Error("Authentication failed", zap.Error(err))
		os.Exit(1)
	}

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
			fmt.Printf("\n=== Gophkeeper Client - %s (Authenticated) ===\n", authService.GetCurrentUsername())
			fmt.Println("1. Sync with server (last 24 hours)")
			fmt.Println("2. Full sync (all data)")
			fmt.Println("3. Create test secret")
			fmt.Println("4. List secrets")
			fmt.Println("5. Logout")
			fmt.Println("6. Lock master password")
			fmt.Println("7. Change master password")
			fmt.Println("8. Exit")
		} else if !authService.IsMasterPasswordUnlocked() && authService.IsMasterPasswordSet() {
			// Master password is locked but user might have stored authentication
			fmt.Printf("\n=== Gophkeeper Client - %s (Master Password Locked) ===\n", authService.GetCurrentUsername())
			fmt.Println("1. Unlock master password")
			fmt.Println("2. Exit")
		} else {
			fmt.Printf("\n=== Gophkeeper Client - %s (Not Authenticated) ===\n", authService.GetCurrentUsername())
			fmt.Println("1. Login")
			fmt.Println("2. Exit")
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
		} else if !authService.IsMasterPasswordUnlocked() && authService.IsMasterPasswordSet() {
			if err := handleLockedMasterPasswordChoice(choice, authService, scanner); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			if err := handleUnauthenticatedChoice(choice, authService, scanner); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}

		// Exit conditions
		if choice == "2" && (!authService.IsLoggedIn() || (!authService.IsMasterPasswordUnlocked() && authService.IsMasterPasswordSet())) || choice == "8" && authService.IsLoggedIn() {
			break
		}
	}

	return nil
}

func handleUnauthenticatedChoice(choice string, authService *service.AuthService, scanner *bufio.Scanner) error {
	switch choice {
	case "1": // Login
		return handleLogin(authService, scanner)
	case "2": // Exit
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
	// This should not be used in the new multi-user system
	// Registration is handled automatically during user creation
	fmt.Println("Registration is handled automatically during user creation.")
	fmt.Println("Please restart the application to create a new user.")
	return nil
}

func handleLogin(authService *service.AuthService, scanner *bufio.Scanner) error {
	// Проверяем и разблокируем мастер-пароль если нужно
	if !authService.IsMasterPasswordUnlocked() {
		fmt.Print("Enter master password: ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read master password")
		}
		masterPassword := strings.TrimSpace(scanner.Text())

		if err := authService.UnlockMasterPassword(masterPassword); err != nil {
			return fmt.Errorf("failed to unlock master password: %w", err)
		}
		fmt.Println("Master password unlocked!")
	}

	fmt.Printf("Logging in user: %s\n", authService.GetCurrentUsername())
	fmt.Print("Password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read password")
	}
	password := strings.TrimSpace(scanner.Text())

	if err := authService.Login(authService.GetCurrentUsername(), password); err != nil {
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
	fmt.Println("Your server authentication is preserved and no re-login is required.")

	// НЕ удаляем токен - авторизация остается, только блокируем локальный доступ
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

// handleUserAuthentication handles authentication for new and existing users
func handleUserAuthentication(authService *service.AuthService, username string, isNewUser bool, scanner *bufio.Scanner) error {
	if isNewUser {
		// For new users, first setup master password, then register + login
		fmt.Printf("Creating new user: %s\n", username)

		// Setup master password FIRST for new local user
		fmt.Print("Enter master password for encrypting your local data: ")
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

		// Now get server password and register + login
		fmt.Print("Enter password for server account: ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read password")
		}
		password := strings.TrimSpace(scanner.Text())

		if len(password) < 6 {
			return fmt.Errorf("password must be at least 6 characters long")
		}

		// Try to register and login automatically
		if err := authService.RegisterAndLogin(username, password); err != nil {
			// If user already exists on server, try to login instead
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "user exists") {
				fmt.Printf("User %s already exists on server. Attempting login...\n", username)
				if err := authService.Login(username, password); err != nil {
					return fmt.Errorf("failed to login existing user: %w", err)
				}
				fmt.Println("Login successful!")
			} else {
				return fmt.Errorf("failed to register and login new user: %w", err)
			}
		} else {
			fmt.Println("User registered and logged in successfully!")
		}
	} else {
		// For existing users, first setup master password if needed
		fmt.Printf("Welcome back, %s!\n", username)

		// Setup master password if it's not set
		if !authService.IsMasterPasswordSet() {
			fmt.Print("Master password not found. Enter master password for encrypting your local data: ")
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
		}

		// Unlock master password if it's locked
		if !authService.IsMasterPasswordUnlocked() {
			fmt.Print("Enter master password: ")
			if !scanner.Scan() {
				return fmt.Errorf("failed to read master password")
			}
			masterPassword := strings.TrimSpace(scanner.Text())

			if err := authService.UnlockMasterPassword(masterPassword); err != nil {
				return fmt.Errorf("failed to unlock master password: %w", err)
			}
			fmt.Println("Master password unlocked!")
		}

		// Try auto login with stored token
		if err := authService.AutoLogin(); err != nil {
			// Auto login failed, need manual server authentication
			fmt.Println("Stored authentication not found or invalid. Please login to server.")
			fmt.Print("Enter server password: ")
			if !scanner.Scan() {
				return fmt.Errorf("failed to read server password")
			}
			password := strings.TrimSpace(scanner.Text())

			if err := authService.Login(username, password); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			fmt.Println("Login successful!")
		} else {
			fmt.Println("Successfully authenticated using stored credentials!")
		}
	}

	return nil
}

// handleUnlockMasterPassword разблокирует мастер-пароль без повторной авторизации
func handleUnlockMasterPassword(authService *service.AuthService, scanner *bufio.Scanner) error {
	if authService.IsMasterPasswordUnlocked() {
		fmt.Println("Master password is already unlocked.")
		return nil
	}

	fmt.Print("Enter master password: ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read master password")
	}
	masterPassword := strings.TrimSpace(scanner.Text())

	if err := authService.UnlockMasterPassword(masterPassword); err != nil {
		return fmt.Errorf("failed to unlock master password: %w", err)
	}

	fmt.Println("Master password unlocked successfully!")

	// Проверяем, есть ли сохраненный токен для автоматической авторизации
	if authService.IsLoggedIn() {
		fmt.Println("Authentication restored using stored credentials.")
	}

	return nil
}

func handleLockedMasterPasswordChoice(choice string, authService *service.AuthService, scanner *bufio.Scanner) error {
	switch choice {
	case "1": // Unlock master password
		return handleUnlockMasterPassword(authService, scanner)
	case "2": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}
