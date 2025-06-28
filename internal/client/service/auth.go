package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/crypto"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// AuthService handles client authentication
type AuthService struct {
	serverURL             string
	tokenFile             string
	encryptKey            []byte
	httpClient            *http.Client
	masterPasswordManager *crypto.MasterPasswordManager
	username              string // Current username
}

// AuthRequest represents login/register request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents server auth response
type AuthResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewAuthService creates a new authentication service
func NewAuthService(serverURL string) *AuthService {
	return &AuthService{
		serverURL:  serverURL,
		encryptKey: crypto.GenerateKey(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetUser configures the auth service for a specific user
func (a *AuthService) SetUser(username, userDataDir string) {
	a.username = username
	a.tokenFile = filepath.Join(userDataDir, "token.enc")
	a.masterPasswordManager = crypto.NewMasterPasswordManager(userDataDir)
}

// hashPassword creates a SHA-256 hash of the password on client side
func (a *AuthService) hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// Register registers a new user
func (a *AuthService) Register(username, password string) error {
	hashedPassword := a.hashPassword(password)

	req := AuthRequest{
		Username: username,
		Password: hashedPassword,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := a.httpClient.Post(
		a.serverURL+"/api/v1/auth/register",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send register request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !authResp.Success {
		return fmt.Errorf("registration failed: %s", authResp.Message)
	}

	log.Zap.Info("User registered successfully", zap.String("username", username))
	return nil
}

// Login authenticates user and stores encrypted token
func (a *AuthService) Login(username, password string) error {
	hashedPassword := a.hashPassword(password)

	req := AuthRequest{
		Username: username,
		Password: hashedPassword,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := a.httpClient.Post(
		a.serverURL+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !authResp.Success {
		return fmt.Errorf("login failed: %s", authResp.Message)
	}

	// Encrypt and save token
	if err := a.saveToken(authResp.Token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	log.Zap.Info("User logged in successfully",
		zap.String("username", username),
		zap.String("user_id", authResp.UserID))

	return nil
}

// saveToken encrypts and saves the JWT token to file using master password
func (a *AuthService) saveToken(token string) error {
	if !a.masterPasswordManager.IsUnlocked() {
		return fmt.Errorf("master password is locked")
	}

	// Шифруем токен с помощью мастер-пароля
	encryptedToken, salt, err := a.masterPasswordManager.EncryptData([]byte(token))
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Создаем структуру для хранения
	tokenData := struct {
		EncryptedToken string `json:"encrypted_token"`
		Salt           string `json:"salt"`
	}{
		EncryptedToken: encryptedToken,
		Salt:           salt,
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(a.tokenFile), 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	if err := os.WriteFile(a.tokenFile, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// LoadToken loads and decrypts the JWT token from file using master password
func (a *AuthService) LoadToken() (string, error) {
	if !a.masterPasswordManager.IsUnlocked() {
		return "", fmt.Errorf("master password is locked")
	}

	if _, err := os.Stat(a.tokenFile); os.IsNotExist(err) {
		return "", fmt.Errorf("token file not found")
	}

	// Читаем файл
	jsonData, err := os.ReadFile(a.tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read token file: %w", err)
	}

	// Десериализуем JSON
	var tokenData struct {
		EncryptedToken string `json:"encrypted_token"`
		Salt           string `json:"salt"`
	}

	if err := json.Unmarshal(jsonData, &tokenData); err != nil {
		return "", fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	// Расшифровываем токен
	tokenBytes, err := a.masterPasswordManager.DecryptData(tokenData.EncryptedToken, tokenData.Salt)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return string(tokenBytes), nil
}

// IsLoggedIn checks if user has a valid token and master password is unlocked
func (a *AuthService) IsLoggedIn() bool {
	if !a.masterPasswordManager.IsUnlocked() {
		return false
	}

	token, err := a.LoadToken()
	if err != nil {
		return false
	}

	// TODO: Verify token with server or check expiration locally
	return token != ""
}

// Logout removes the stored token
func (a *AuthService) Logout() error {
	if _, err := os.Stat(a.tokenFile); os.IsNotExist(err) {
		return nil // Already logged out
	}

	if err := os.Remove(a.tokenFile); err != nil {
		return fmt.Errorf("failed to remove token file: %w", err)
	}

	log.Zap.Info("User logged out successfully")
	return nil
}

// GetAuthHeader returns the Authorization header value
func (a *AuthService) GetAuthHeader() (string, error) {
	token, err := a.LoadToken()
	if err != nil {
		return "", err
	}

	return "Bearer " + token, nil
}

// SetMasterPassword устанавливает мастер-пароль (первый раз)
func (a *AuthService) SetMasterPassword(password string) error {
	return a.masterPasswordManager.SetPassword(password)
}

// UnlockMasterPassword разблокирует приложение с помощью мастер-пароля
func (a *AuthService) UnlockMasterPassword(password string) error {
	return a.masterPasswordManager.UnlockWithPassword(password)
}

// IsMasterPasswordSet проверяет, установлен ли мастер-пароль
func (a *AuthService) IsMasterPasswordSet() bool {
	return a.masterPasswordManager.IsPasswordSet()
}

// IsMasterPasswordUnlocked проверяет, разблокирован ли мастер-пароль
func (a *AuthService) IsMasterPasswordUnlocked() bool {
	return a.masterPasswordManager.IsUnlocked()
}

// LockMasterPassword блокирует мастер-пароль
func (a *AuthService) LockMasterPassword() {
	a.masterPasswordManager.Lock()
}

// ChangeMasterPassword изменяет мастер-пароль
func (a *AuthService) ChangeMasterPassword(oldPassword, newPassword string) error {
	return a.masterPasswordManager.ChangePassword(oldPassword, newPassword)
}

// GetMasterPasswordManager возвращает менеджер мастер-пароля
func (a *AuthService) GetMasterPasswordManager() *crypto.MasterPasswordManager {
	return a.masterPasswordManager
}

// RegisterAndLogin performs automatic registration and login for new user
func (a *AuthService) RegisterAndLogin(username, password string) error {
	// First, try to register
	if err := a.Register(username, password); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Then automatically login
	if err := a.Login(username, password); err != nil {
		return fmt.Errorf("auto-login after registration failed: %w", err)
	}

	log.Zap.Info("User registered and logged in successfully", zap.String("username", username))
	return nil
}

// AutoLogin attempts to login using stored credentials for existing user
func (a *AuthService) AutoLogin() error {
	if a.username == "" {
		return fmt.Errorf("username not set")
	}

	// Check if master password is unlocked (required to decrypt stored token)
	if !a.masterPasswordManager.IsUnlocked() {
		return fmt.Errorf("master password is locked - cannot access stored credentials")
	}

	// Try to load existing token
	token, err := a.LoadToken()
	if err != nil {
		// Token doesn't exist or can't be loaded
		return fmt.Errorf("stored token not found or invalid: %w", err)
	}

	if token == "" {
		return fmt.Errorf("empty token found")
	}

	// TODO: In a real application, you would validate the token with the server here
	// For now, we assume the token is valid if we can decrypt it
	log.Zap.Info("Auto-login successful using stored token", zap.String("username", a.username))
	return nil
}

// GetCurrentUsername returns the current username
func (a *AuthService) GetCurrentUsername() string {
	return a.username
}
