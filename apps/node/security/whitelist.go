package security

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// PartyWhitelist управляет списком разрешённых участников
type PartyWhitelist struct {
	mu      sync.RWMutex
	parties map[string]ed25519.PublicKey // partyID -> publicKey
}

// WhitelistEntry запись в whitelist для сериализации
type WhitelistEntry struct {
	PartyID   string `json:"party_id"`
	PublicKey string `json:"public_key"` // base64 encoded
}

// WhitelistConfig конфигурация whitelist для загрузки из файла
type WhitelistConfig struct {
	Parties []WhitelistEntry `json:"parties"`
}

// NewPartyWhitelist создаёт новый пустой whitelist
func NewPartyWhitelist() *PartyWhitelist {
	return &PartyWhitelist{
		parties: make(map[string]ed25519.PublicKey),
	}
}

// Add добавляет участника в whitelist
func (w *PartyWhitelist) Add(partyID string, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: got %d, expected %d", len(publicKey), ed25519.PublicKeySize)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.parties[partyID] = publicKey
	return nil
}

// Remove удаляет участника из whitelist
func (w *PartyWhitelist) Remove(partyID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.parties[partyID]; exists {
		delete(w.parties, partyID)
		return true
	}
	return false
}

// IsAllowed проверяет, есть ли участник в whitelist
func (w *PartyWhitelist) IsAllowed(partyID string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	_, exists := w.parties[partyID]
	return exists
}

// GetPublicKey возвращает публичный ключ участника
func (w *PartyWhitelist) GetPublicKey(partyID string) (ed25519.PublicKey, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	key, exists := w.parties[partyID]
	return key, exists
}

// Verify проверяет, что partyID соответствует публичному ключу в whitelist
// SECURITY: Используется constant-time сравнение для защиты от timing attacks
func (w *PartyWhitelist) Verify(partyID string, publicKey ed25519.PublicKey) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	storedKey, exists := w.parties[partyID]
	if !exists {
		return false
	}

	// SECURITY: Constant-time сравнение ключей для защиты от timing attacks
	return subtle.ConstantTimeCompare(storedKey, publicKey) == 1
}

// GetAll возвращает копию всех записей whitelist
func (w *PartyWhitelist) GetAll() map[string]ed25519.PublicKey {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make(map[string]ed25519.PublicKey, len(w.parties))
	for k, v := range w.parties {
		keyCopy := make(ed25519.PublicKey, len(v))
		copy(keyCopy, v)
		result[k] = keyCopy
	}
	return result
}

// Count возвращает количество участников в whitelist
func (w *PartyWhitelist) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.parties)
}

// LoadFromFile загружает whitelist из JSON файла
func (w *PartyWhitelist) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read whitelist file: %w", err)
	}

	var config WhitelistConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse whitelist JSON: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, entry := range config.Parties {
		publicKey, err := LoadPublicKeyFromBase64(entry.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to decode public key for %s: %w", entry.PartyID, err)
		}
		w.parties[entry.PartyID] = publicKey
	}

	return nil
}

// SaveToFile сохраняет whitelist в JSON файл
func (w *PartyWhitelist) SaveToFile(path string) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	config := WhitelistConfig{
		Parties: make([]WhitelistEntry, 0, len(w.parties)),
	}

	for partyID, publicKey := range w.parties {
		identity := &NodeIdentity{PublicKey: publicKey}
		config.Parties = append(config.Parties, WhitelistEntry{
			PartyID:   partyID,
			PublicKey: identity.PublicKeyBase64(),
		})
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal whitelist: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write whitelist file: %w", err)
	}

	return nil
}

// AddFromBase64 добавляет участника с публичным ключом в base64
func (w *PartyWhitelist) AddFromBase64(partyID, publicKeyBase64 string) error {
	publicKey, err := LoadPublicKeyFromBase64(publicKeyBase64)
	if err != nil {
		return err
	}
	return w.Add(partyID, publicKey)
}
