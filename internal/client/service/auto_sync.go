package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// AutoSyncService управляет автоматической синхронизацией каждые 10 секунд
type AutoSyncService struct {
	clientSyncService *ClientSyncService
	syncService       *SyncService
	syncInterval      time.Duration
	isRunning         bool
	mutex             sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	lastSyncTime      time.Time
	lastSyncTimeLock  sync.RWMutex
	syncStateFile     string // Путь к файлу для сохранения состояния
}

// SyncState структура для сохранения состояния синхронизации
type SyncState struct {
	LastSyncTime time.Time `json:"last_sync_time"`
}

// NewAutoSyncService создает новый сервис автоматической синхронизации
func NewAutoSyncService(clientSyncService *ClientSyncService, syncService *SyncService) *AutoSyncService {
	service := &AutoSyncService{
		clientSyncService: clientSyncService,
		syncService:       syncService,
		syncInterval:      10 * time.Second, // 10 секунд
	}

	// Устанавливаем путь к файлу состояния (будет установлен позже)
	service.loadSyncState()

	return service
}

// SetSyncStateFile устанавливает путь к файлу состояния синхронизации
func (s *AutoSyncService) SetSyncStateFile(userDataDir string) {
	s.syncStateFile = filepath.Join(userDataDir, "sync_state.json")
	s.loadSyncState()
}

// loadSyncState загружает состояние синхронизации из файла
func (s *AutoSyncService) loadSyncState() {
	if s.syncStateFile == "" {
		s.lastSyncTime = time.Now()
		return
	}

	data, err := os.ReadFile(s.syncStateFile)
	if err != nil {
		// Файл не существует или не может быть прочитан - используем текущее время
		s.lastSyncTime = time.Now()
		log.Zap.Debug("Could not load sync state, using current time", zap.Error(err))
		return
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		s.lastSyncTime = time.Now()
		log.Zap.Warn("Could not parse sync state, using current time", zap.Error(err))
		return
	}

	s.lastSyncTime = state.LastSyncTime
	log.Zap.Debug("Loaded sync state", zap.Time("last_sync_time", s.lastSyncTime))
}

// saveSyncState сохраняет состояние синхронизации в файл
func (s *AutoSyncService) saveSyncState() error {
	if s.syncStateFile == "" {
		return nil // Нет файла для сохранения
	}

	state := SyncState{
		LastSyncTime: s.GetLastSyncTime(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sync state: %w", err)
	}

	// Создаем директорию если она не существует
	if err := os.MkdirAll(filepath.Dir(s.syncStateFile), 0755); err != nil {
		return fmt.Errorf("failed to create sync state directory: %w", err)
	}

	if err := os.WriteFile(s.syncStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to save sync state: %w", err)
	}

	log.Zap.Debug("Saved sync state", zap.Time("last_sync_time", state.LastSyncTime))
	return nil
}

// Start запускает автоматическую синхронизацию
func (s *AutoSyncService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		log.Zap.Warn("Auto sync is already running")
		return nil
	}

	// Создаем контекст для управления горутиной
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.isRunning = true

	// Запускаем горутину синхронизации
	go s.syncLoop()

	log.Zap.Info("Auto sync started", zap.Duration("interval", s.syncInterval))
	return nil
}

// Stop останавливает автоматическую синхронизацию
func (s *AutoSyncService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		log.Zap.Warn("Auto sync is not running")
		return
	}

	// Останавливаем горутину
	s.cancel()
	s.isRunning = false

	log.Zap.Info("Auto sync stopped")
}

// IsRunning возвращает статус автоматической синхронизации
func (s *AutoSyncService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.isRunning
}

// GetLastSyncTime возвращает время последней синхронизации
func (s *AutoSyncService) GetLastSyncTime() time.Time {
	s.lastSyncTimeLock.RLock()
	defer s.lastSyncTimeLock.RUnlock()
	return s.lastSyncTime
}

// setLastSyncTime устанавливает время последней синхронизации
func (s *AutoSyncService) setLastSyncTime(t time.Time) {
	s.lastSyncTimeLock.Lock()
	defer s.lastSyncTimeLock.Unlock()
	s.lastSyncTime = t

	// Сохраняем состояние в файл
	if err := s.saveSyncState(); err != nil {
		log.Zap.Error("Failed to save sync state", zap.Error(err))
	}
}

// syncLoop основной цикл синхронизации
func (s *AutoSyncService) syncLoop() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	log.Zap.Info("Auto sync loop started")

	for {
		select {
		case <-s.ctx.Done():
			log.Zap.Info("Auto sync loop stopped")
			return
		case <-ticker.C:
			s.performIncrementalSync()
		}
	}
}

// performIncrementalSync выполняет инкрементальную синхронизацию
func (s *AutoSyncService) performIncrementalSync() {
	lastSyncTime := s.GetLastSyncTime()

	log.Zap.Debug("Starting auto sync",
		zap.Time("last_sync_time", lastSyncTime))

	// Выполняем инкрементальную синхронизацию только изменений с последней синхронизации
	err := s.clientSyncService.PerformSyncSince(s.syncService, lastSyncTime)
	if err != nil {
		log.Zap.Error("Auto sync failed", zap.Error(err))
		return
	}

	// Обновляем время последней синхронизации
	s.setLastSyncTime(time.Now())

	log.Zap.Debug("Auto sync completed successfully",
		zap.Time("new_last_sync_time", s.GetLastSyncTime()))
}

// SetSyncInterval изменяет интервал синхронизации
func (s *AutoSyncService) SetSyncInterval(interval time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if interval < time.Second {
		log.Zap.Warn("Sync interval too small, setting to minimum 1 second",
			zap.Duration("requested", interval))
		interval = time.Second
	}

	s.syncInterval = interval
	log.Zap.Info("Sync interval updated", zap.Duration("new_interval", interval))

	// Если синхронизация запущена, перезапускаем её с новым интервалом
	if s.isRunning {
		log.Zap.Info("Restarting auto sync with new interval")
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(context.Background())
		go s.syncLoop()
	}
}

// ForceSync принудительно запускает синхронизацию
func (s *AutoSyncService) ForceSync() error {
	log.Zap.Info("Force sync requested")

	lastSyncTime := s.GetLastSyncTime()
	err := s.clientSyncService.PerformSyncSince(s.syncService, lastSyncTime)
	if err != nil {
		return err
	}

	s.setLastSyncTime(time.Now())
	log.Zap.Info("Force sync completed successfully")
	return nil
}
