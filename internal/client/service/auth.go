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
	serverURL  string
	tokenFile  string
	encryptKey []byte
	httpClient *http.Client
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
func NewAuthService(serverURL, dataDir string) *AuthService {
	return &AuthService{
		serverURL:  serverURL,
		tokenFile:  filepath.Join(dataDir, "token.enc"),
		encryptKey: crypto.GenerateKey(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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

// saveToken encrypts and saves the JWT token to file
func (a *AuthService) saveToken(token string) error {
	encryptedToken, err := crypto.Encrypt([]byte(token), a.encryptKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(a.tokenFile), 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	if err := os.WriteFile(a.tokenFile, []byte(encryptedToken), 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// LoadToken loads and decrypts the JWT token from file
func (a *AuthService) LoadToken() (string, error) {
	if _, err := os.Stat(a.tokenFile); os.IsNotExist(err) {
		return "", fmt.Errorf("token file not found")
	}

	encryptedData, err := os.ReadFile(a.tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read token file: %w", err)
	}

	tokenBytes, err := crypto.Decrypt(string(encryptedData), a.encryptKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return string(tokenBytes), nil
}

// IsLoggedIn checks if user has a valid token
func (a *AuthService) IsLoggedIn() bool {
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
