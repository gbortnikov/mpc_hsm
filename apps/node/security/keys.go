package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// NodeIdentity содержит криптографическую идентичность ноды
type NodeIdentity struct {
	PartyID    string
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GenerateIdentity создаёт новую криптографическую идентичность
func GenerateIdentity(partyID string) (*NodeIdentity, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 keypair: %w", err)
	}

	return &NodeIdentity{
		PartyID:    partyID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// LoadIdentity загружает идентичность из файлов
func LoadIdentity(partyID, privateKeyPath string) (*NodeIdentity, error) {
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, expected %d", len(block.Bytes), ed25519.PrivateKeySize)
	}

	privateKey := ed25519.PrivateKey(block.Bytes)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &NodeIdentity{
		PartyID:    partyID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// SavePrivateKey сохраняет приватный ключ в PEM файл
func (ni *NodeIdentity) SavePrivateKey(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: ni.PrivateKey,
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer file.Close()

	if err := pem.Encode(file, block); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	return nil
}

// SavePublicKey сохраняет публичный ключ в PEM файл
func (ni *NodeIdentity) SavePublicKey(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	block := &pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: ni.PublicKey,
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer file.Close()

	if err := pem.Encode(file, block); err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}

	return nil
}

// PublicKeyBase64 возвращает публичный ключ в base64 формате
func (ni *NodeIdentity) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(ni.PublicKey)
}

// LoadPublicKeyFromBase64 декодирует публичный ключ из base64
func LoadPublicKeyFromBase64(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, expected %d", len(decoded), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(decoded), nil
}

// LoadPublicKeyFromPEM загружает публичный ключ из PEM файла
func LoadPublicKeyFromPEM(path string) (ed25519.PublicKey, error) {
	publicKeyPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "ED25519 PUBLIC KEY" {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, expected %d", len(block.Bytes), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(block.Bytes), nil
}

// LoadOrGenerateIdentity загружает существующую или генерирует новую идентичность
func LoadOrGenerateIdentity(partyID, keyDir string) (*NodeIdentity, error) {
	privateKeyPath := filepath.Join(keyDir, "signing.key")
	publicKeyPath := filepath.Join(keyDir, "signing.pub")

	// Пробуем загрузить существующую
	if _, err := os.Stat(privateKeyPath); err == nil {
		return LoadIdentity(partyID, privateKeyPath)
	}

	// Генерируем новую
	identity, err := GenerateIdentity(partyID)
	if err != nil {
		return nil, err
	}

	// Сохраняем
	if err := identity.SavePrivateKey(privateKeyPath); err != nil {
		return nil, err
	}
	if err := identity.SavePublicKey(publicKeyPath); err != nil {
		return nil, err
	}

	return identity, nil
}
