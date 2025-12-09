package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// MessageAuthenticator создаёт и проверяет подписи для MPC сообщений
type MessageAuthenticator struct {
	identity    *NodeIdentity
	whitelist   *PartyWhitelist
	replayGuard *ReplayGuard
	maxAge      time.Duration
	maxFuture   time.Duration
}

// NewMessageAuthenticator создаёт новый аутентификатор
func NewMessageAuthenticator(
	identity *NodeIdentity,
	whitelist *PartyWhitelist,
	replayGuard *ReplayGuard,
) *MessageAuthenticator {
	return &MessageAuthenticator{
		identity:    identity,
		whitelist:   whitelist,
		replayGuard: replayGuard,
		maxAge:      5 * time.Minute,
		maxFuture:   30 * time.Second,
	}
}

// KeygenMessageData данные для подписи keygen сообщения
type KeygenMessageData struct {
	SessionID   string
	FromParty   string
	Round       int32
	Payload     []byte
	IsBroadcast bool
}

// SignKeygenMessage подписывает данные keygen сообщения
// Возвращает signature, timestamp, nonce
func (ma *MessageAuthenticator) SignKeygenMessage(data *KeygenMessageData) ([]byte, int64, []byte, error) {
	if ma.identity == nil {
		return nil, 0, nil, fmt.Errorf("identity not configured")
	}

	// Генерируем nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	timestamp := time.Now().UnixNano()

	// Формируем данные для подписи
	dataToSign := buildKeygenSignatureData(data, timestamp, nonce)

	// Подписываем
	signature := ed25519.Sign(ma.identity.PrivateKey, dataToSign)

	return signature, timestamp, nonce, nil
}

// VerifyKeygenMessage проверяет подпись keygen сообщения
func (ma *MessageAuthenticator) VerifyKeygenMessage(
	data *KeygenMessageData,
	signature []byte,
	timestamp int64,
	nonce []byte,
) error {
	// 1. Проверяем, что отправитель в whitelist
	publicKey, ok := ma.whitelist.GetPublicKey(data.FromParty)
	if !ok {
		return fmt.Errorf("unknown sender: %s not in whitelist", data.FromParty)
	}

	// 2. Проверяем timestamp
	now := time.Now().UnixNano()

	// Сообщение из будущего?
	if timestamp > now+ma.maxFuture.Nanoseconds() {
		return fmt.Errorf("message timestamp in future")
	}

	// Сообщение слишком старое?
	if now-timestamp > ma.maxAge.Nanoseconds() {
		return fmt.Errorf("message expired (age: %v)", time.Duration(now-timestamp))
	}

	// 3. Проверяем nonce (защита от replay)
	if ma.replayGuard != nil {
		if err := ma.replayGuard.Check(nonce, timestamp); err != nil {
			return fmt.Errorf("replay detected: %w", err)
		}
	}

	// 4. Проверяем подпись
	dataToVerify := buildKeygenSignatureData(data, timestamp, nonce)

	if !ed25519.Verify(publicKey, dataToVerify, signature) {
		return fmt.Errorf("invalid signature from %s", data.FromParty)
	}

	return nil
}

// SigningMessageData данные для подписи signing сообщения
type SigningMessageData struct {
	SessionID   string
	FromParty   string
	Round       int32
	Payload     []byte
	IsBroadcast bool
}

// SignSigningMessage подписывает данные signing сообщения
func (ma *MessageAuthenticator) SignSigningMessage(data *SigningMessageData) ([]byte, int64, []byte, error) {
	if ma.identity == nil {
		return nil, 0, nil, fmt.Errorf("identity not configured")
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	timestamp := time.Now().UnixNano()
	dataToSign := buildSigningSignatureData(data, timestamp, nonce)
	signature := ed25519.Sign(ma.identity.PrivateKey, dataToSign)

	return signature, timestamp, nonce, nil
}

// VerifySigningMessage проверяет подпись signing сообщения
func (ma *MessageAuthenticator) VerifySigningMessage(
	data *SigningMessageData,
	signature []byte,
	timestamp int64,
	nonce []byte,
) error {
	publicKey, ok := ma.whitelist.GetPublicKey(data.FromParty)
	if !ok {
		return fmt.Errorf("unknown sender: %s not in whitelist", data.FromParty)
	}

	now := time.Now().UnixNano()
	if timestamp > now+ma.maxFuture.Nanoseconds() {
		return fmt.Errorf("message timestamp in future")
	}
	if now-timestamp > ma.maxAge.Nanoseconds() {
		return fmt.Errorf("message expired")
	}

	if ma.replayGuard != nil {
		if err := ma.replayGuard.Check(nonce, timestamp); err != nil {
			return fmt.Errorf("replay detected: %w", err)
		}
	}

	dataToVerify := buildSigningSignatureData(data, timestamp, nonce)
	if !ed25519.Verify(publicKey, dataToVerify, signature) {
		return fmt.Errorf("invalid signature from %s", data.FromParty)
	}

	return nil
}

// GetPartyID возвращает PartyID аутентификатора
func (ma *MessageAuthenticator) GetPartyID() string {
	if ma.identity == nil {
		return ""
	}
	return ma.identity.PartyID
}

// buildKeygenSignatureData формирует данные для подписи keygen сообщения
func buildKeygenSignatureData(data *KeygenMessageData, timestamp int64, nonce []byte) []byte {
	// Формат: "KEYGEN" || len(sessionID) || sessionID || len(fromParty) || fromParty ||
	//         round || len(payload) || payload || isBroadcast || timestamp || nonce
	prefix := []byte("KEYGEN")
	sessionIDBytes := []byte(data.SessionID)
	fromPartyBytes := []byte(data.FromParty)

	size := len(prefix) + 4 + len(sessionIDBytes) + 4 + len(fromPartyBytes) +
		4 + 4 + len(data.Payload) + 1 + 8 + len(nonce)

	buf := make([]byte, size)
	offset := 0

	// Prefix
	copy(buf[offset:], prefix)
	offset += len(prefix)

	// Session ID
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(sessionIDBytes)))
	offset += 4
	copy(buf[offset:], sessionIDBytes)
	offset += len(sessionIDBytes)

	// From Party
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(fromPartyBytes)))
	offset += 4
	copy(buf[offset:], fromPartyBytes)
	offset += len(fromPartyBytes)

	// Round
	binary.BigEndian.PutUint32(buf[offset:], uint32(data.Round))
	offset += 4

	// Payload
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.Payload)))
	offset += 4
	copy(buf[offset:], data.Payload)
	offset += len(data.Payload)

	// IsBroadcast
	if data.IsBroadcast {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++

	// Timestamp
	binary.BigEndian.PutUint64(buf[offset:], uint64(timestamp))
	offset += 8

	// Nonce
	copy(buf[offset:], nonce)

	return buf
}

// buildSigningSignatureData формирует данные для подписи signing сообщения
func buildSigningSignatureData(data *SigningMessageData, timestamp int64, nonce []byte) []byte {
	prefix := []byte("SIGNING")
	sessionIDBytes := []byte(data.SessionID)
	fromPartyBytes := []byte(data.FromParty)

	size := len(prefix) + 4 + len(sessionIDBytes) + 4 + len(fromPartyBytes) +
		4 + 4 + len(data.Payload) + 1 + 8 + len(nonce)

	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], prefix)
	offset += len(prefix)

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(sessionIDBytes)))
	offset += 4
	copy(buf[offset:], sessionIDBytes)
	offset += len(sessionIDBytes)

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(fromPartyBytes)))
	offset += 4
	copy(buf[offset:], fromPartyBytes)
	offset += len(fromPartyBytes)

	binary.BigEndian.PutUint32(buf[offset:], uint32(data.Round))
	offset += 4

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.Payload)))
	offset += 4
	copy(buf[offset:], data.Payload)
	offset += len(data.Payload)

	if data.IsBroadcast {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++

	binary.BigEndian.PutUint64(buf[offset:], uint64(timestamp))
	offset += 8

	copy(buf[offset:], nonce)

	return buf
}
