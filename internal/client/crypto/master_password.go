package crypto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MasterPasswordManager управляет мастер-паролем
type MasterPasswordManager struct {
	dataDir      string
	password     string
	isUnlocked   bool
	passwordFile string
}

// PasswordVerificationData содержит данные для проверки пароля
type PasswordVerificationData struct {
	EncryptedData string `json:"encrypted_data"`
	Salt          string `json:"salt"`
}

// NewMasterPasswordManager создает новый менеджер мастер-пароля
func NewMasterPasswordManager(dataDir string) *MasterPasswordManager {
	return &MasterPasswordManager{
		dataDir:      dataDir,
		isUnlocked:   false,
		passwordFile: filepath.Join(dataDir, "master_password.json"),
	}
}

// SetPassword устанавливает мастер-пароль (первый раз)
func (m *MasterPasswordManager) SetPassword(password string) error {
	// Проверяем, не установлен ли уже пароль
	if m.IsPasswordSet() {
		return fmt.Errorf("master password is already set")
	}

	// Создаем данные для проверки пароля
	encryptedData, salt, err := CreatePasswordVerificationData(password)
	if err != nil {
		return fmt.Errorf("failed to create password verification data: %w", err)
	}

	// Сохраняем данные проверки
	verificationData := PasswordVerificationData{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	if err := m.savePasswordVerification(verificationData); err != nil {
		return fmt.Errorf("failed to save password verification: %w", err)
	}

	// Устанавливаем пароль и разблокируем
	m.password = password
	m.isUnlocked = true

	return nil
}

// UnlockWithPassword разблокирует менеджер с помощью пароля
func (m *MasterPasswordManager) UnlockWithPassword(password string) error {
	if !m.IsPasswordSet() {
		return fmt.Errorf("master password is not set")
	}

	// Загружаем данные проверки
	verificationData, err := m.loadPasswordVerification()
	if err != nil {
		return fmt.Errorf("failed to load password verification: %w", err)
	}

	// Проверяем пароль
	if !ValidatePassword(password, verificationData.EncryptedData, verificationData.Salt) {
		return fmt.Errorf("invalid master password")
	}

	// Устанавливаем пароль и разблокируем
	m.password = password
	m.isUnlocked = true

	return nil
}

// IsPasswordSet проверяет, установлен ли мастер-пароль
func (m *MasterPasswordManager) IsPasswordSet() bool {
	_, err := os.Stat(m.passwordFile)
	return err == nil
}

// IsUnlocked проверяет, разблокирован ли менеджер
func (m *MasterPasswordManager) IsUnlocked() bool {
	return m.isUnlocked
}

// GetPassword возвращает мастер-пароль (только если разблокирован)
func (m *MasterPasswordManager) GetPassword() (string, error) {
	if !m.isUnlocked {
		return "", fmt.Errorf("master password manager is locked")
	}
	return m.password, nil
}

// EncryptData шифрует данные с помощью мастер-пароля
func (m *MasterPasswordManager) EncryptData(data []byte) (encryptedData string, salt string, err error) {
	if !m.isUnlocked {
		return "", "", fmt.Errorf("master password manager is locked")
	}

	return EncryptWithPassword(data, m.password)
}

// DecryptData расшифровывает данные с помощью мастер-пароля
func (m *MasterPasswordManager) DecryptData(encryptedData, salt string) ([]byte, error) {
	if !m.isUnlocked {
		return nil, fmt.Errorf("master password manager is locked")
	}

	return DecryptWithPassword(encryptedData, m.password, salt)
}

// Lock блокирует менеджер
func (m *MasterPasswordManager) Lock() {
	m.password = ""
	m.isUnlocked = false
}

// ChangePassword изменяет мастер-пароль
func (m *MasterPasswordManager) ChangePassword(oldPassword, newPassword string) error {
	if !m.IsPasswordSet() {
		return fmt.Errorf("master password is not set")
	}

	// Проверяем старый пароль
	verificationData, err := m.loadPasswordVerification()
	if err != nil {
		return fmt.Errorf("failed to load password verification: %w", err)
	}

	if !ValidatePassword(oldPassword, verificationData.EncryptedData, verificationData.Salt) {
		return fmt.Errorf("invalid old password")
	}

	// Создаем новые данные проверки
	encryptedData, salt, err := CreatePasswordVerificationData(newPassword)
	if err != nil {
		return fmt.Errorf("failed to create new password verification data: %w", err)
	}

	// Сохраняем новые данные
	newVerificationData := PasswordVerificationData{
		EncryptedData: encryptedData,
		Salt:          salt,
	}

	if err := m.savePasswordVerification(newVerificationData); err != nil {
		return fmt.Errorf("failed to save new password verification: %w", err)
	}

	// Обновляем пароль
	m.password = newPassword
	m.isUnlocked = true

	return nil
}

// savePasswordVerification сохраняет данные проверки пароля
func (m *MasterPasswordManager) savePasswordVerification(data PasswordVerificationData) error {
	// Создаем директорию если не существует
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Сериализуем данные
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal verification data: %w", err)
	}

	// Сохраняем в файл
	if err := os.WriteFile(m.passwordFile, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write password file: %w", err)
	}

	return nil
}

// loadPasswordVerification загружает данные проверки пароля
func (m *MasterPasswordManager) loadPasswordVerification() (PasswordVerificationData, error) {
	var data PasswordVerificationData

	// Читаем файл
	jsonData, err := os.ReadFile(m.passwordFile)
	if err != nil {
		return data, fmt.Errorf("failed to read password file: %w", err)
	}

	// Десериализуем данные
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return data, fmt.Errorf("failed to unmarshal verification data: %w", err)
	}

	return data, nil
}
