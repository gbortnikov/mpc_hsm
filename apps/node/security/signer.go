package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// SignedEnvelope содержит подписанное сообщение
type SignedEnvelope struct {
	Payload       []byte // Оригинальные данные сообщения
	SignerPartyID string // ID участника, подписавшего сообщение
	Signature     []byte // Подпись Ed25519
	Timestamp     int64  // Unix timestamp в наносекундах
	Nonce         []byte // 16 байт случайных данных для защиты от replay
}

// MessageSigner подписывает исходящие сообщения
type MessageSigner struct {
	identity *NodeIdentity
}

// NewMessageSigner создаёт новый подписчик сообщений
func NewMessageSigner(identity *NodeIdentity) *MessageSigner {
	return &MessageSigner{
		identity: identity,
	}
}

// Sign подписывает payload и возвращает SignedEnvelope
func (ms *MessageSigner) Sign(payload []byte) (*SignedEnvelope, error) {
	// Генерация nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	timestamp := time.Now().UnixNano()

	// Формирование данных для подписи: payload || timestamp || nonce
	dataToSign := buildSignatureData(payload, timestamp, nonce)

	// Подписание
	signature := ed25519.Sign(ms.identity.PrivateKey, dataToSign)

	return &SignedEnvelope{
		Payload:       payload,
		SignerPartyID: ms.identity.PartyID,
		Signature:     signature,
		Timestamp:     timestamp,
		Nonce:         nonce,
	}, nil
}

// GetPartyID возвращает PartyID подписчика
func (ms *MessageSigner) GetPartyID() string {
	return ms.identity.PartyID
}

// GetPublicKey возвращает публичный ключ подписчика
func (ms *MessageSigner) GetPublicKey() ed25519.PublicKey {
	return ms.identity.PublicKey
}

// buildSignatureData формирует данные для подписи
func buildSignatureData(payload []byte, timestamp int64, nonce []byte) []byte {
	// Формат: len(payload) || payload || timestamp || nonce
	// len используется для предотвращения атак с удлинением сообщения
	data := make([]byte, 8+len(payload)+8+len(nonce))

	// Длина payload (8 байт)
	binary.BigEndian.PutUint64(data[0:8], uint64(len(payload)))

	// Payload
	copy(data[8:8+len(payload)], payload)

	// Timestamp (8 байт)
	binary.BigEndian.PutUint64(data[8+len(payload):16+len(payload)], uint64(timestamp))

	// Nonce
	copy(data[16+len(payload):], nonce)

	return data
}
