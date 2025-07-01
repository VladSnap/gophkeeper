package service

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/VladSnap/gophkeeper/internal/client/config"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// UserManager handles user selection and creation
type UserManager struct {
	config *config.Config
}

// NewUserManager creates a new user manager
func NewUserManager(config *config.Config) *UserManager {
	return &UserManager{
		config: config,
	}
}

// SelectOrCreateUser prompts user to select existing user or create new one
func (um *UserManager) SelectOrCreateUser(scanner *bufio.Scanner) (string, bool, error) {
	// Get list of existing users
	users, err := um.config.ListUsers()
	if err != nil {
		return "", false, fmt.Errorf("failed to list users: %w", err)
	}

	fmt.Println("\n=== Gophkeeper User Selection ===")

	if len(users) == 0 {
		fmt.Println("No local users found on this machine.")
		fmt.Println("You can:")
		fmt.Println("1. Create a new user (will register on server)")
		fmt.Println("2. Login with existing server account")
		fmt.Print("Choose option (1 or 2): ")

		if !scanner.Scan() {
			return "", false, fmt.Errorf("failed to read choice")
		}

		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "1":
			username, err := um.promptForNewUser(scanner, false)
			if err != nil {
				return "", false, err
			}
			return username, true, nil // true means new user (register)
		case "2":
			username, err := um.promptForExistingServerUser(scanner)
			if err != nil {
				return "", false, err
			}
			return username, false, nil // false means existing user (login only)
		default:
			return "", false, fmt.Errorf("invalid choice: %s", choice)
		}
	}

	// Display existing users
	fmt.Println("Local users found:")
	for i, user := range users {
		fmt.Printf("%d. %s\n", i+1, user)
	}
	fmt.Printf("%d. Create new user\n", len(users)+1)
	fmt.Printf("%d. Login with existing server account\n", len(users)+2)
	fmt.Print("Choose option: ")

	if !scanner.Scan() {
		return "", false, fmt.Errorf("failed to read user choice")
	}

	choice := strings.TrimSpace(scanner.Text())
	choiceNum, err := strconv.Atoi(choice)
	if err != nil {
		return "", false, fmt.Errorf("invalid choice: %s", choice)
	}

	// User selected existing local user
	if choiceNum >= 1 && choiceNum <= len(users) {
		selectedUser := users[choiceNum-1]
		log.Zap.Info("User selected existing local user", zap.String("username", selectedUser))
		return selectedUser, false, nil // false means existing user
	}

	// User selected to create new user
	if choiceNum == len(users)+1 {
		username, err := um.promptForNewUser(scanner, true)
		if err != nil {
			return "", false, err
		}
		return username, true, nil // true means new user
	}

	// User selected to login with existing server account
	if choiceNum == len(users)+2 {
		username, err := um.promptForExistingServerUser(scanner)
		if err != nil {
			return "", false, err
		}
		return username, false, nil // false means existing user (login only)
	}

	return "", false, fmt.Errorf("invalid choice: %d", choiceNum)
}

// promptForNewUser prompts for new user creation
func (um *UserManager) promptForNewUser(scanner *bufio.Scanner, checkLocal bool) (string, error) {
	fmt.Print("Enter username for new user: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read username")
	}

	username := strings.TrimSpace(scanner.Text())
	if username == "" {
		return "", fmt.Errorf("username cannot be empty")
	}

	// Check if user already exists locally only if checkLocal is true
	if checkLocal && um.config.UserExists(username) {
		return "", fmt.Errorf("user '%s' already exists locally", username)
	}

	return username, nil
}

// promptForExistingServerUser prompts for existing server user login
func (um *UserManager) promptForExistingServerUser(scanner *bufio.Scanner) (string, error) {
	fmt.Print("Enter username (existing server account): ")
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read username")
	}

	username := strings.TrimSpace(scanner.Text())
	if username == "" {
		return "", fmt.Errorf("username cannot be empty")
	}

	return username, nil
}

// SetupUserEnvironment sets up the environment for a specific user
func (um *UserManager) SetupUserEnvironment(username string, isNewerUser bool) error {
	// Configure config for this user
	if err := um.config.SetUser(username, isNewerUser); err != nil {
		return fmt.Errorf("failed to setup user environment: %w", err)
	}

	// Save user info
	if err := um.config.SaveUserInfo(username); err != nil {
		return fmt.Errorf("failed to save user info: %w", err)
	}

	log.Zap.Info("User environment setup completed",
		zap.String("username", username),
		zap.String("user_data_dir", um.config.GetUserDataDir()))

	return nil
}
