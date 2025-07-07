package resManager

import (
	"sync"

	"github.com/VladSnap/gophkeeper/pkg/log"
)

// ResourceManager - управляет жизненным циклом объектов, чтобы при завершении программы вызвать очистку.
type ResourceManager struct {
	cleanupFuncs []func() error
	mu           sync.Mutex
}

// NewResourceManager - Создает новую структуру ResourceManager с указателем.
func NewResourceManager() *ResourceManager {
	return &ResourceManager{}
}

// Register - регистрирует функцию очистки.
func (rm *ResourceManager) Register(cleanupFunc func() error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.cleanupFuncs = append(rm.cleanupFuncs, cleanupFunc)
}

// Cleanup - вызывает все зарегистрированные функции очистки.
func (rm *ResourceManager) Cleanup() error {
	log.Zap.Info("ResourceManager.Cleanup start")
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for _, cleanup := range rm.cleanupFuncs {
		err := cleanup()
		if err != nil {
			return err
		}
	}
	log.Zap.Info("ResourceManager.Cleanup finish")
	return nil
}
