package security

import (
	"crypto/ed25519"
	"fmt"
	"time"
)

// VerificationError тип ошибки верификации
type VerificationError struct {
	Code    string
	Message string
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
	ErrUnknownSigner     = &VerificationError{"UNKNOWN_SIGNER", "signer not in whitelist"}
	ErrInvalidSignature  = &VerificationError{"INVALID_SIGNATURE", "signature verification failed"}
	ErrMessageExpired    = &VerificationError{"MESSAGE_EXPIRED", "message timestamp too old"}
	ErrMessageFromFuture = &VerificationError{"MESSAGE_FROM_FUTURE", "message timestamp in future"}
	ErrReplayDetected    = &VerificationError{"REPLAY_DETECTED", "nonce already used"}
)

// MessageVerifier проверяет входящие подписанные сообщения
type MessageVerifier struct {
	whitelist   *PartyWhitelist
	replayGuard *ReplayGuard
	maxAge      time.Duration
	maxFuture   time.Duration // максимальное отклонение в будущее (для расхождения часов)
}

// VerifierConfig конфигурация верификатора
type VerifierConfig struct {
	MaxAge    time.Duration // Максимальный возраст сообщения
	MaxFuture time.Duration // Максимальное отклонение в будущее
}

// DefaultVerifierConfig возвращает конфигурацию по умолчанию
func DefaultVerifierConfig() *VerifierConfig {
	return &VerifierConfig{
		MaxAge:    5 * time.Minute,
		MaxFuture: 30 * time.Second,
	}
}

// NewMessageVerifier создаёт новый верификатор сообщений
func NewMessageVerifier(whitelist *PartyWhitelist, replayGuard *ReplayGuard, config *VerifierConfig) *MessageVerifier {
	if config == nil {
		config = DefaultVerifierConfig()
	}

	return &MessageVerifier{
		whitelist:   whitelist,
		replayGuard: replayGuard,
		maxAge:      config.MaxAge,
		maxFuture:   config.MaxFuture,
	}
}

// Verify проверяет подписанное сообщение
// Возвращает payload если верификация успешна
func (mv *MessageVerifier) Verify(envelope *SignedEnvelope) ([]byte, error) {
	// 1. Проверка, что отправитель в белом списке
	publicKey, ok := mv.whitelist.GetPublicKey(envelope.SignerPartyID)
	if !ok {
		return nil, ErrUnknownSigner
	}

	// 2. Проверяем timestamp
	now := time.Now().UnixNano()
	messageTime := envelope.Timestamp

	// Проверка: сообщение из будущего?
	if messageTime > now+mv.maxFuture.Nanoseconds() {
		return nil, ErrMessageFromFuture
	}

	// Проверка: сообщение слишком старое?
	if now-messageTime > mv.maxAge.Nanoseconds() {
		return nil, ErrMessageExpired
	}

	// 3. Проверка nonce (защита от повторных атак)
	if err := mv.replayGuard.Check(envelope.Nonce, envelope.Timestamp); err != nil {
		return nil, ErrReplayDetected
	}

	// 4. Проверка подписи
	dataToVerify := buildSignatureData(envelope.Payload, envelope.Timestamp, envelope.Nonce)

	if !ed25519.Verify(publicKey, dataToVerify, envelope.Signature) {
		return nil, ErrInvalidSignature
	}

	return envelope.Payload, nil
}

// VerifyWithoutReplay проверяет сообщение без проверки повторных атак
// Полезно для идемпотентных операций
func (mv *MessageVerifier) VerifyWithoutReplay(envelope *SignedEnvelope) ([]byte, error) {
	// 1. Проверка белого списка
	publicKey, ok := mv.whitelist.GetPublicKey(envelope.SignerPartyID)
	if !ok {
		return nil, ErrUnknownSigner
	}

	// 2. Проверка timestamp
	now := time.Now().UnixNano()
	messageTime := envelope.Timestamp

	if messageTime > now+mv.maxFuture.Nanoseconds() {
		return nil, ErrMessageFromFuture
	}

	if now-messageTime > mv.maxAge.Nanoseconds() {
		return nil, ErrMessageExpired
	}

	// 3. Проверка подписи
	dataToVerify := buildSignatureData(envelope.Payload, envelope.Timestamp, envelope.Nonce)

	if !ed25519.Verify(publicKey, dataToVerify, envelope.Signature) {
		return nil, ErrInvalidSignature
	}

	return envelope.Payload, nil
}

// VerifySignatureOnly проверяет только подпись (для особых случаев)
func (mv *MessageVerifier) VerifySignatureOnly(envelope *SignedEnvelope, publicKey ed25519.PublicKey) bool {
	dataToVerify := buildSignatureData(envelope.Payload, envelope.Timestamp, envelope.Nonce)
	return ed25519.Verify(publicKey, dataToVerify, envelope.Signature)
}
