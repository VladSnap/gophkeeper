package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
)

// getSystemInfo получает информацию о системе для создания уникального ключа
func getSystemInfo() []byte {
	info := fmt.Sprintf("%s-%s-%s", runtime.GOOS, runtime.GOARCH, os.Getenv("COMPUTERNAME"))
	hash := sha256.Sum256([]byte(info))
	return hash[:]
}

// GenerateKey создает ключ шифрования на основе данных ПК
func GenerateKey() []byte {
	systemInfo := getSystemInfo()
	// Добавляем статическую соль для дополнительной безопасности
	salt := "gophkeeper-client-salt-2025"
	combined := append(systemInfo, []byte(salt)...)

	hash := sha256.Sum256(combined)
	return hash[:]
}

// Encrypt шифрует данные с использованием AES-GCM
func Encrypt(data []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt расшифровывает данные с использованием AES-GCM
func Decrypt(encryptedHex string, key []byte) ([]byte, error) {
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
