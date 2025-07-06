package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/VladSnap/gophkeeper/internal/client/app"
	"github.com/VladSnap/gophkeeper/internal/client/service"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

type CLI struct {
	scanner *bufio.Scanner
	app     *app.Application
}

func NewCLI(app *app.Application) *CLI {
	return &CLI{
		scanner: bufio.NewScanner(os.Stdin),
		app:     app,
	}
}

func (cli *CLI) Init() error {
	// Select or create user
	username, isNewUser, err := cli.app.UserManager.SelectOrCreateUser(cli.scanner)
	if err != nil {
		return fmt.Errorf("failed to select or create user: %w", err)
	}

	if err := cli.app.SetAppUser(username, isNewUser); err != nil {
		return fmt.Errorf("failed to set app user: %w", err)
	}

	return nil
}

// RunCLI starts the command-line interface for the Gophkeeper client
func (cli *CLI) Run() error {
	authService := cli.app.AuthService

	// Handle user authentication
	if err := cli.handleUserAuthentication(); err != nil {
		log.Zap.Error("Authentication failed", zap.Error(err))
		os.Exit(1)
	}

	for {
		if authService.IsLoggedIn() {
			fmt.Printf("\n=== Gophkeeper Client - %s (Authenticated) ===\n",
				authService.GetCurrentUsername())
			fmt.Println("1. Sync with server (last 24 hours)")
			fmt.Println("2. Full sync (all data)")
			fmt.Println("3. Create test secret")
			fmt.Println("4. List secrets")
			fmt.Println("5. Auto sync - Start/Stop")
			fmt.Println("6. Auto sync - Status")
			fmt.Println("7. Force sync now")
			fmt.Println("8. Logout")
			fmt.Println("9. Lock master password")
			fmt.Println("10. Change master password")
			fmt.Println("11. Exit")
		} else if !authService.IsMasterPasswordUnlocked() && authService.IsMasterPasswordSet() {
			// Master password is locked but user might have stored authentication
			fmt.Printf("\n=== Gophkeeper Client - %s (Master Password Locked) ===\n",
				authService.GetCurrentUsername())
			fmt.Println("1. Unlock master password")
			fmt.Println("2. Exit")
		} else {
			fmt.Printf("\n=== Gophkeeper Client - %s (Not Authenticated) ===\n",
				authService.GetCurrentUsername())
			fmt.Println("1. Login")
			fmt.Println("2. Exit")
		}

		fmt.Print("Choose option: ")
		if !cli.scanner.Scan() {
			break
		}

		choice := strings.TrimSpace(cli.scanner.Text())

		if authService.IsLoggedIn() {
			if err := cli.handleAuthenticatedChoice(choice); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else if !authService.IsMasterPasswordUnlocked() && authService.IsMasterPasswordSet() {
			if err := cli.handleLockedMasterPasswordChoice(choice); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			if err := cli.handleUnauthenticatedChoice(choice); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}

		// Exit conditions
		if choice == "2" && (!authService.IsLoggedIn() || (!authService.IsMasterPasswordUnlocked() &&
			authService.IsMasterPasswordSet())) || choice == "8" && authService.IsLoggedIn() {
			break
		}
	}

	return nil
}

func (cli *CLI) handleUnauthenticatedChoice(choice string) error {
	switch choice {
	case "1": // Login
		return cli.handleLogin()
	case "2": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}

func (cli *CLI) handleAuthenticatedChoice(choice string) error {
	switch choice {
	case "1": // Sync
		return cli.handleSync()
	case "2": // Full sync
		return cli.handleFullSync()
	case "3": // Create secret
		return cli.handleCreateSecret()
	case "4": // List secrets
		return cli.handleListSecrets()
	case "5": // Auto sync - Start/Stop
		return cli.handleToggleAutoSync()
	case "6": // Auto sync - Status
		return cli.handleAutoSyncStatus()
	case "7": // Force sync now
		return cli.handleForceSync()
	case "8": // Logout
		return cli.app.AuthService.Logout()
	case "9": // Lock master password
		return cli.handleLockMasterPassword()
	case "10": // Change master password
		return cli.handleChangeMasterPassword()
	case "11": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}

func (cli *CLI) handleLogin() error {
	authService := cli.app.AuthService
	scanner := cli.scanner

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

func (cli *CLI) handleSync() error {
	fmt.Println("Syncing with server...")

	// Выполняем инкрементальную синхронизацию
	if err := cli.app.ServiceFactory.ClientSyncService().PerformSync(cli.app.SyncService,
		service.SyncTypeIncremental); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("Sync completed successfully!")
	return nil
}

func (cli *CLI) handleCreateSecret() error {
	fmt.Print("Enter secret data: ")
	if !cli.scanner.Scan() {
		return fmt.Errorf("failed to read secret data")
	}
	secretData := strings.TrimSpace(cli.scanner.Text())

	metadata := map[string]string{
		"type":        "test",
		"description": "Test secret created from CLI",
		"created_by":  "cli",
	}

	secret, err := cli.app.ServiceFactory.SecretsService().CreateSecret(secretData)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	// Add metadata to the secret
	for key, value := range metadata {
		if err := cli.app.ServiceFactory.MetadataService().AddMetadata(secret.SecretID, key, value); err != nil {
			log.Zap.Error("Failed to add metadata", zap.Error(err), zap.String("key", key))
		}
	}

	fmt.Printf("Secret created: %s\n", secret.SecretID.String())
	return nil
}

func (cli *CLI) handleListSecrets() error {
	fmt.Println("Listing all secrets...")

	secrets, err := cli.app.ServiceFactory.SecretsService().GetAllSecrets()
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
		decryptedData, err := cli.app.ServiceFactory.CryptoService().DecryptSecretData(secret.Encrypted)
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
		metadata, err := cli.app.ServiceFactory.MetadataService().GetMetadataBySecretID(secret.SecretID)
		if err != nil {
			fmt.Printf("   Metadata: Error retrieving (%v)\n", err)
		} else if len(metadata) > 0 {
			fmt.Printf("   Metadata:\n")
			for _, meta := range metadata {
				// Расшифровываем значение метаданных
				decryptedValue := cli.app.ServiceFactory.CryptoService().DecryptMetadataValue(meta.ValueEncrypted)
				fmt.Printf("     %s: %s\n", meta.Key, decryptedValue)
			}
		} else {
			fmt.Printf("   Metadata: None\n")
		}
		fmt.Println()
	}

	return nil
}

func (cli *CLI) handleFullSync() error {
	fmt.Println("Performing full synchronization...")

	// Выполняем полную синхронизацию
	if err := cli.app.ServiceFactory.ClientSyncService().PerformSync(cli.app.SyncService, service.SyncTypeFull); err != nil {
		return fmt.Errorf("full sync failed: %w", err)
	}

	fmt.Println("Full synchronization completed successfully!")
	return nil
}

// handleLockMasterPassword блокирует мастер-пароль
func (cli *CLI) handleLockMasterPassword() error {
	cli.app.AuthService.LockMasterPassword()
	fmt.Println("Master password locked. You will need to unlock it to access your data again.")
	fmt.Println("Your server authentication is preserved and no re-login is required.")

	// НЕ удаляем токен - авторизация остается, только блокируем локальный доступ
	return nil
}

// handleChangeMasterPassword изменяет мастер-пароль
func (cli *CLI) handleChangeMasterPassword() error {
	scanner := cli.scanner

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

	if err := cli.app.AuthService.ChangeMasterPassword(oldPassword, newPassword); err != nil {
		return fmt.Errorf("failed to change master password: %w", err)
	}

	fmt.Println("Master password changed successfully!")
	return nil
}

// handleUserAuthentication handles authentication for new and existing users
func (cli *CLI) handleUserAuthentication() error {
	username := cli.app.Cfg.Username
	authService := cli.app.AuthService

	if cli.app.Cfg.IsNewUser {
		// For new users, first setup master password, then register + login
		fmt.Printf("Creating new user: %s\n", username)

		// Setup master password FIRST for new local user
		fmt.Print("Enter master password for encrypting your local data: ")
		if !cli.scanner.Scan() {
			return fmt.Errorf("failed to read master password")
		}
		masterPassword := strings.TrimSpace(cli.scanner.Text())

		if len(masterPassword) < 6 {
			return fmt.Errorf("master password must be at least 6 characters long")
		}

		if err := authService.SetMasterPassword(masterPassword); err != nil {
			return fmt.Errorf("failed to set master password: %w", err)
		}
		fmt.Println("Master password set successfully!")

		// Now get server password and register + login
		fmt.Print("Enter password for server account: ")
		if !cli.scanner.Scan() {
			return fmt.Errorf("failed to read password")
		}
		password := strings.TrimSpace(cli.scanner.Text())

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
			if !cli.scanner.Scan() {
				return fmt.Errorf("failed to read master password")
			}
			masterPassword := strings.TrimSpace(cli.scanner.Text())

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
			if !cli.scanner.Scan() {
				return fmt.Errorf("failed to read master password")
			}
			masterPassword := strings.TrimSpace(cli.scanner.Text())

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
			if !cli.scanner.Scan() {
				return fmt.Errorf("failed to read server password")
			}
			password := strings.TrimSpace(cli.scanner.Text())

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
func (cli *CLI) handleUnlockMasterPassword() error {
	scanner := cli.scanner
	authService := cli.app.AuthService

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

func (cli *CLI) handleLockedMasterPasswordChoice(choice string) error {
	switch choice {
	case "1": // Unlock master password
		return cli.handleUnlockMasterPassword()
	case "2": // Exit
		fmt.Println("Goodbye!")
		return nil
	default:
		fmt.Println("Invalid option")
		return nil
	}
}

// handleToggleAutoSync включает или отключает автоматическую синхронизацию
func (cli *CLI) handleToggleAutoSync() error {
	if cli.app.IsAutoSyncRunning() {
		cli.app.StopAutoSync()
		fmt.Println("Auto sync stopped successfully!")
	} else {
		if err := cli.app.StartAutoSync(); err != nil {
			return fmt.Errorf("failed to start auto sync: %w", err)
		}
		fmt.Println("Auto sync started successfully! Syncing every 10 seconds.")
	}
	return nil
}

// handleAutoSyncStatus показывает статус автоматической синхронизации
func (cli *CLI) handleAutoSyncStatus() error {
	isRunning := cli.app.IsAutoSyncRunning()
	lastSyncTime := cli.app.GetLastSyncTime()

	fmt.Printf("Auto sync status: %s\n", map[bool]string{true: "RUNNING", false: "STOPPED"}[isRunning])
	if !lastSyncTime.IsZero() {
		fmt.Printf("Last sync time: %s\n", lastSyncTime.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Last sync time: Never")
	}

	if isRunning {
		fmt.Println("Sync interval: 10 seconds")
	}

	return nil
}

// handleForceSync принудительно запускает синхронизацию
func (cli *CLI) handleForceSync() error {
	fmt.Println("Starting force sync...")

	if err := cli.app.ForceSync(); err != nil {
		return fmt.Errorf("force sync failed: %w", err)
	}

	fmt.Println("Force sync completed successfully!")
	return nil
}
