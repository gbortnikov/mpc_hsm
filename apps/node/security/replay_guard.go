package security

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ReplayGuard защищает от replay-атак
type ReplayGuard struct {
	mu              sync.RWMutex
	usedNonces      map[string]int64 // nonce (hex) -> timestamp
	maxAge          time.Duration    // максимальный возраст nonce
	cleanupInterval time.Duration    // интервал очистки
	stopCh          chan struct{}
}

// ReplayGuardConfig конфигурация replay guard
type ReplayGuardConfig struct {
	MaxAge          time.Duration
	CleanupInterval time.Duration
}

// DefaultReplayGuardConfig возвращает конфигурацию по умолчанию
func DefaultReplayGuardConfig() *ReplayGuardConfig {
	return &ReplayGuardConfig{
		MaxAge:          5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// NewReplayGuard создаёт новый replay guard
func NewReplayGuard(config *ReplayGuardConfig) *ReplayGuard {
	if config == nil {
		config = DefaultReplayGuardConfig()
	}

	rg := &ReplayGuard{
		usedNonces:      make(map[string]int64),
		maxAge:          config.MaxAge,
		cleanupInterval: config.CleanupInterval,
		stopCh:          make(chan struct{}),
	}

	// Запуск фоновой очистки
	go rg.cleanupLoop()

	return rg
}

// Check проверяет nonce и timestamp
// Возвращает ошибку если nonce уже использовался или timestamp невалидный
func (rg *ReplayGuard) Check(nonce []byte, timestamp int64) error {
	nonceHex := hex.EncodeToString(nonce)

	rg.mu.Lock()
	defer rg.mu.Unlock()

	// Проверка, не использовался ли nonce
	if _, exists := rg.usedNonces[nonceHex]; exists {
		return fmt.Errorf("nonce already used")
	}

	// Запись nonce
	rg.usedNonces[nonceHex] = timestamp

	return nil
}

// CheckWithoutRecord проверяет nonce без записи (для предварительной проверки)
func (rg *ReplayGuard) CheckWithoutRecord(nonce []byte) bool {
	nonceHex := hex.EncodeToString(nonce)

	rg.mu.RLock()
	defer rg.mu.RUnlock()

	_, exists := rg.usedNonces[nonceHex]
	return !exists
}

// Record записывает nonce (для двухфазной проверки)
func (rg *ReplayGuard) Record(nonce []byte, timestamp int64) {
	nonceHex := hex.EncodeToString(nonce)

	rg.mu.Lock()
	defer rg.mu.Unlock()

	rg.usedNonces[nonceHex] = timestamp
}

// cleanupLoop периодически удаляет устаревшие nonce
func (rg *ReplayGuard) cleanupLoop() {
	ticker := time.NewTicker(rg.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rg.cleanup()
		case <-rg.stopCh:
			return
		}
	}
}

// cleanup удаляет устаревшие nonce
func (rg *ReplayGuard) cleanup() {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	now := time.Now().UnixNano()
	maxAgeNano := rg.maxAge.Nanoseconds()

	for nonce, timestamp := range rg.usedNonces {
		if now-timestamp > maxAgeNano {
			delete(rg.usedNonces, nonce)
		}
	}
}

// Stop останавливает фоновую очистку
func (rg *ReplayGuard) Stop() {
	close(rg.stopCh)
}

// Size возвращает количество записанных nonce
func (rg *ReplayGuard) Size() int {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	return len(rg.usedNonces)
}

// Clear очищает все записанные nonce
func (rg *ReplayGuard) Clear() {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.usedNonces = make(map[string]int64)
}
