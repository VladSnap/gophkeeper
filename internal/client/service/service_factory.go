package service

import (
	"github.com/VladSnap/gophkeeper/internal/client/crypto"
	"github.com/VladSnap/gophkeeper/internal/client/repository"
)

// ServiceFactory создает и настраивает все сервисы
type ServiceFactory struct {
	cryptoService     *CryptoService
	secretsService    *SecretsService
	metadataService   *MetadataService
	clientSyncService *ClientSyncService
	clientService     *ClientService
}

// NewServiceFactory создает новую фабрику сервисов
func NewServiceFactory(
	secretRepo repository.SecretRepositoryInterface,
	metadataRepo repository.MetadataRepositoryInterface,
	masterPasswordManager *crypto.MasterPasswordManager,
) *ServiceFactory {
	// Создаем базовые сервисы
	cryptoService := NewCryptoService(masterPasswordManager)
	secretsService := NewSecretsService(secretRepo, cryptoService)
	metadataService := NewMetadataService(metadataRepo, secretRepo, cryptoService)
	clientSyncService := NewClientSyncService(secretsService, metadataService, cryptoService)
	clientService := NewClientService(secretRepo, metadataRepo, masterPasswordManager)

	return &ServiceFactory{
		cryptoService:     cryptoService,
		secretsService:    secretsService,
		metadataService:   metadataService,
		clientSyncService: clientSyncService,
		clientService:     clientService,
	}
}

// CryptoService возвращает сервис для работы с шифрованием
func (sf *ServiceFactory) CryptoService() *CryptoService {
	return sf.cryptoService
}

// SecretsService возвращает сервис для работы с секретами
func (sf *ServiceFactory) SecretsService() *SecretsService {
	return sf.secretsService
}

// MetadataService возвращает сервис для работы с метаданными
func (sf *ServiceFactory) MetadataService() *MetadataService {
	return sf.metadataService
}

// ClientSyncService возвращает сервис для синхронизации
func (sf *ServiceFactory) ClientSyncService() *ClientSyncService {
	return sf.clientSyncService
}

// ClientService возвращает основной клиентский сервис
func (sf *ServiceFactory) ClientService() *ClientService {
	return sf.clientService
}
