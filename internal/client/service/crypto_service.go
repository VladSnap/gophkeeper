package service

import (
	"encoding/json"
	"fmt"

	"github.com/VladSnap/gophkeeper/internal/client/crypto"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// CryptoService handles encryption and decryption operations
type CryptoService struct {
	masterPasswordManager *crypto.MasterPasswordManager
}

// NewCryptoService creates a new crypto service
func NewCryptoService(masterPasswordManager *crypto.MasterPasswordManager) *CryptoService {
	return &CryptoService{
		masterPasswordManager: masterPasswordManager,
	}
}

// IsUnlocked checks if the master password is unlocked
func (cs *CryptoService) IsUnlocked() bool {
	return cs.masterPasswordManager.IsUnlocked()
}

// EncryptSecretData encrypts secret data with salt
func (cs *CryptoService) EncryptSecretData(plaintext string) (string, error) {
	if !cs.masterPasswordManager.IsUnlocked() {
		return "", fmt.Errorf("master password is locked")
	}

	// Encrypt the secret data
	encryptedData, salt, err := cs.masterPasswordManager.EncryptData([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt secret data: %w", err)
	}

	// Store encrypted data + salt in JSON format
	secretData := struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	encryptedJSON, err := json.Marshal(secretData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal encrypted secret data: %w", err)
	}

	return string(encryptedJSON), nil
}

// DecryptSecretData decrypts secret data
func (cs *CryptoService) DecryptSecretData(encryptedData string) (string, error) {
	if !cs.masterPasswordManager.IsUnlocked() {
		return "", fmt.Errorf("master password is locked")
	}

	// Deserialize JSON
	var secretData struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}

	if err := json.Unmarshal([]byte(encryptedData), &secretData); err != nil {
		return "", fmt.Errorf("failed to unmarshal secret data: %w", err)
	}

	// Decrypt
	decryptedData, err := cs.masterPasswordManager.DecryptData(secretData.EncryptedData, secretData.Salt)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret data: %w", err)
	}

	return string(decryptedData), nil
}

// EncryptMetadataValue encrypts metadata value
func (cs *CryptoService) EncryptMetadataValue(value string) string {
	if !cs.masterPasswordManager.IsUnlocked() {
		log.Zap.Warn("Master password is locked, cannot encrypt metadata value")
		return value // Fallback to plaintext (not recommended in production)
	}

	encryptedData, salt, err := cs.masterPasswordManager.EncryptData([]byte(value))
	if err != nil {
		log.Zap.Error("Failed to encrypt metadata value", zap.Error(err))
		return value // Fallback to plaintext
	}

	// Store as JSON with salt
	metaData := struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	jsonData, err := json.Marshal(metaData)
	if err != nil {
		log.Zap.Error("Failed to marshal encrypted metadata", zap.Error(err))
		return value // Fallback to plaintext
	}

	return string(jsonData)
}

// DecryptMetadataValue decrypts metadata value
func (cs *CryptoService) DecryptMetadataValue(encryptedValue string) string {
	if !cs.masterPasswordManager.IsUnlocked() {
		return encryptedValue // Return as-is if locked
	}

	// Try to deserialize as JSON
	var metaData struct {
		EncryptedData string `json:"encrypted_data"`
		Salt          string `json:"salt"`
	}

	if err := json.Unmarshal([]byte(encryptedValue), &metaData); err != nil {
		// If not JSON, it's probably plaintext
		return encryptedValue
	}

	// Decrypt
	decryptedData, err := cs.masterPasswordManager.DecryptData(metaData.EncryptedData, metaData.Salt)
	if err != nil {
		log.Zap.Error("Failed to decrypt metadata value", zap.Error(err))
		return encryptedValue // Return encrypted value as fallback
	}

	return string(decryptedData)
}
