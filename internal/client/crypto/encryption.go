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

	"golang.org/x/crypto/pbkdf2"
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

// DeriveKeyFromPassword создает ключ шифрования из мастер-пароля
func DeriveKeyFromPassword(password string, salt []byte) []byte {
	// Используем PBKDF2 с SHA256 для создания ключа из пароля
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

// GenerateSalt создает новую соль для PBKDF2
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// EncryptWithPassword шифрует данные с использованием пароля
func EncryptWithPassword(data []byte, password string) (encryptedData string, salt string, err error) {
	// Генерируем соль
	saltBytes, err := GenerateSalt()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Создаем ключ из пароля
	key := DeriveKeyFromPassword(password, saltBytes)

	// Шифруем данные
	encrypted, err := Encrypt(data, key)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt data: %w", err)
	}

	return encrypted, hex.EncodeToString(saltBytes), nil
}

// DecryptWithPassword расшифровывает данные с использованием пароля
func DecryptWithPassword(encryptedData string, password string, saltHex string) ([]byte, error) {
	// Декодируем соль
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Создаем ключ из пароля
	key := DeriveKeyFromPassword(password, salt)

	// Расшифровываем данные
	data, err := Decrypt(encryptedData, key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return data, nil
}

// HashValue создает SHA256 хэш от значения
func HashValue(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

// ValidatePassword проверяет правильность мастер-пароля
func ValidatePassword(password, encryptedData, saltHex string) bool {
	// Пытаемся расшифровать тестовые данные
	_, err := DecryptWithPassword(encryptedData, password, saltHex)
	return err == nil
}

// CreatePasswordVerificationData создает зашифрованные тестовые данные для проверки пароля
func CreatePasswordVerificationData(password string) (encryptedData string, salt string, err error) {
	testData := []byte("gophkeeper-password-test")
	return EncryptWithPassword(testData, password)
}
