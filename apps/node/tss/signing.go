package tss

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

// SigningSession представляет сессию распределённого подписания
type SigningSession struct {
	SessionID     string
	PartyID       *tss.PartyID
	Party         *signing.LocalParty
	KeyData       *keygen.LocalPartySaveData
	MessageToSign []byte
	OutCh         chan tss.Message
	EndCh         chan *common.SignatureData
	ErrCh         chan *tss.Error
	SignatureData *common.SignatureData
	Completed     bool
	Error         error
	mu            sync.RWMutex
	parties       []*tss.PartyID

	// Буфер для исходящих сообщений
	outgoingBuffer []OutgoingMessage
	bufferMu       sync.Mutex
}

// SigningResult результат подписания
type SigningResult struct {
	Signature string // полная подпись в hex
	R         string
	S         string
	V         int32 // recovery id
}

// NewSigningSession создаёт новую сессию подписания
func NewSigningSession(sessionID string, keyData *keygen.LocalPartySaveData, messageToSign []byte) (*SigningSession, error) {
	slog.Debug("NewSigningSession: creating session",
		"session_id", sessionID,
		"message_len", len(messageToSign),
	)

	if keyData == nil {
		return nil, fmt.Errorf("key data is required")
	}
	if len(messageToSign) == 0 {
		return nil, fmt.Errorf("message to sign is required")
	}

	slog.Debug("NewSigningSession: session created", "session_id", sessionID)
	return &SigningSession{
		SessionID:     sessionID,
		KeyData:       keyData,
		MessageToSign: messageToSign,
		OutCh:         make(chan tss.Message, 100),
		EndCh:         make(chan *common.SignatureData, 1),
		ErrCh:         make(chan *tss.Error, 1),
	}, nil
}

// Initialize инициализирует сессию подписания
func (ss *SigningSession) Initialize(partyID string, parties []PartyInfo) error {
	slog.Debug("SigningSession.Initialize: starting",
		"session_id", ss.SessionID,
		"party_id", partyID,
		"parties_count", len(parties),
	)

	ss.mu.Lock()
	defer ss.mu.Unlock()

	totalParties := len(parties)

	// Создаём PartyID для всех участников
	ss.parties = make([]*tss.PartyID, totalParties)
	for i, p := range parties {
		key := new(big.Int).SetInt64(int64(p.PartyIndex + 1))
		ss.parties[i] = tss.NewPartyID(p.PartyID, fmt.Sprintf("Party %s", p.PartyID), key)

		if p.PartyID == partyID {
			ss.PartyID = ss.parties[i]
		}
		slog.Debug("SigningSession.Initialize: party added",
			"party_id", p.PartyID,
			"party_index", p.PartyIndex,
		)
	}

	if ss.PartyID == nil {
		slog.Error("SigningSession.Initialize: party not found", "party_id", partyID)
		return fmt.Errorf("party %s not found in parties list", partyID)
	}

	// Сортируем PartyIDs
	sortedParties := tss.SortPartyIDs(ss.parties)
	ctx := tss.NewPeerContext(sortedParties)

	// Threshold берём из KeyData - должен соответствовать keygen threshold
	// В TSS библиотеке threshold = t, где для подписания нужно t+1 участников
	// KeyData.Ks содержит индексы всех участников keygen
	threshold := totalParties - 1

	slog.Debug("SigningSession.Initialize: using threshold",
		"threshold", threshold,
		"total_parties", totalParties,
		"key_ks_count", len(ss.KeyData.Ks),
	)

	// Создаём параметры
	params := tss.NewParameters(tss.S256(), ctx, ss.PartyID, totalParties, threshold)

	// Преобразуем сообщение в big.Int
	msgBigInt := new(big.Int).SetBytes(ss.MessageToSign)

	// Создаём party для подписания
	ss.Party = signing.NewLocalParty(msgBigInt, params, *ss.KeyData, ss.OutCh, ss.EndCh).(*signing.LocalParty)

	slog.Debug("SigningSession.Initialize: completed",
		"session_id", ss.SessionID,
		"threshold", threshold,
		"total_parties", totalParties,
	)
	return nil
}

// Start запускает протокол подписания
func (ss *SigningSession) Start() error {
	slog.Debug("SigningSession.Start: starting", "session_id", ss.SessionID)

	ss.mu.Lock()
	if ss.Party == nil {
		ss.mu.Unlock()
		slog.Error("SigningSession.Start: party not initialized", "session_id", ss.SessionID)
		return fmt.Errorf("party not initialized")
	}
	party := ss.Party
	ss.mu.Unlock()

	// Запускаем горутину для буферизации исходящих сообщений ДО party.Start()
	go ss.bufferOutgoingMessages()

	// Даём время горутине буферизации запуститься
	time.Sleep(10 * time.Millisecond)

	go func() {
		slog.Debug("SigningSession.Start: starting TSS party", "session_id", ss.SessionID)
		if err := party.Start(); err != nil {
			slog.Error("SigningSession.Start: party.Start failed",
				"session_id", ss.SessionID,
				"error", err,
			)
			ss.ErrCh <- party.WrapError(err)
		}
	}()

	// Слушаем завершение
	go ss.waitForCompletion()

	slog.Debug("SigningSession.Start: signing protocol started", "session_id", ss.SessionID)
	return nil
}

// waitForCompletion ожидает завершения подписания
func (ss *SigningSession) waitForCompletion() {
	slog.Debug("SigningSession.waitForCompletion: waiting", "session_id", ss.SessionID)

	select {
	case data := <-ss.EndCh:
		ss.mu.Lock()
		ss.SignatureData = data
		ss.Completed = true
		ss.mu.Unlock()
		slog.Debug("SigningSession.waitForCompletion: signing completed successfully",
			"session_id", ss.SessionID,
		)

	case err := <-ss.ErrCh:
		ss.mu.Lock()
		ss.Error = fmt.Errorf("signing error: %v", err)
		ss.Completed = true
		ss.mu.Unlock()
		slog.Error("SigningSession.waitForCompletion: signing failed",
			"session_id", ss.SessionID,
			"error", err,
		)

	case <-time.After(2 * time.Minute):
		ss.mu.Lock()
		ss.Error = fmt.Errorf("signing timeout")
		ss.Completed = true
		ss.mu.Unlock()
		slog.Error("SigningSession.waitForCompletion: signing timeout",
			"session_id", ss.SessionID,
		)
	}
}

// ProcessMessage обрабатывает входящее сообщение
func (ss *SigningSession) ProcessMessage(fromPartyID string, round int, payload []byte, isBroadcast bool) ([]OutgoingMessage, error) {
	slog.Debug("SigningSession.ProcessMessage: processing",
		"session_id", ss.SessionID,
		"from_party", fromPartyID,
		"round", round,
		"is_broadcast", isBroadcast,
		"payload_len", len(payload),
	)

	ss.mu.RLock()
	party := ss.Party
	ss.mu.RUnlock()

	if party == nil {
		slog.Error("SigningSession.ProcessMessage: party not initialized", "session_id", ss.SessionID)
		return nil, fmt.Errorf("party not initialized")
	}

	// Находим отправителя
	var fromParty *tss.PartyID
	for _, p := range ss.parties {
		if p.Id == fromPartyID {
			fromParty = p
			break
		}
	}
	if fromParty == nil {
		slog.Error("SigningSession.ProcessMessage: unknown sender",
			"session_id", ss.SessionID,
			"from_party", fromPartyID,
		)
		return nil, fmt.Errorf("unknown sender party: %s", fromPartyID)
	}

	// Обновляем party
	ok, err := party.UpdateFromBytes(payload, fromParty, isBroadcast)
	if err != nil {
		slog.Error("SigningSession.ProcessMessage: UpdateFromBytes failed",
			"session_id", ss.SessionID,
			"from_party", fromPartyID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	slog.Debug("SigningSession.ProcessMessage: UpdateFromBytes completed",
		"session_id", ss.SessionID,
		"from_party", fromPartyID,
		"ok", ok,
	)

	return ss.collectOutgoingMessages(), nil
}

// bufferOutgoingMessages читает сообщения из канала и складывает в буфер
func (ss *SigningSession) bufferOutgoingMessages() {
	for {
		select {
		case msg := <-ss.OutCh:
			wireBytes, _, err := msg.WireBytes()
			if err != nil {
				slog.Error("SigningSession.bufferOutgoingMessages: failed to get wire bytes", "error", err)
				continue
			}

			outMsg := OutgoingMessage{
				FromParty:   msg.GetFrom().Id,
				Payload:     wireBytes,
				IsBroadcast: msg.IsBroadcast(),
			}

			if dest := msg.GetTo(); dest != nil {
				for _, d := range dest {
					outMsg.ToParties = append(outMsg.ToParties, d.Id)
				}
			}

			slog.Debug("SigningSession.bufferOutgoingMessages: received message",
				"session_id", ss.SessionID,
				"from", outMsg.FromParty,
				"to", outMsg.ToParties,
				"is_broadcast", outMsg.IsBroadcast,
			)

			ss.bufferMu.Lock()
			ss.outgoingBuffer = append(ss.outgoingBuffer, outMsg)
			ss.bufferMu.Unlock()

		default:
			// Проверяем, завершена ли сессия
			ss.mu.RLock()
			completed := ss.Completed
			ss.mu.RUnlock()
			if completed {
				return
			}
			// Небольшая пауза, чтобы не грузить CPU
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// GetOutgoingMessages возвращает все ожидающие исходящие сообщения и очищает буфер
func (ss *SigningSession) GetOutgoingMessages() []OutgoingMessage {
	ss.bufferMu.Lock()
	defer ss.bufferMu.Unlock()

	messages := ss.outgoingBuffer
	ss.outgoingBuffer = nil
	return messages
}

// collectOutgoingMessages собирает сообщения из буфера (для совместимости с ProcessMessage)
func (ss *SigningSession) collectOutgoingMessages() []OutgoingMessage {
	// Даём время для буферизации сообщений после UpdateFromBytes
	// TSS библиотека генерирует сообщения асинхронно
	time.Sleep(500 * time.Millisecond)
	messages := ss.GetOutgoingMessages()
	slog.Debug("SigningSession.collectOutgoingMessages: collected messages",
		"session_id", ss.SessionID,
		"count", len(messages),
	)
	return messages
}

// IsCompleted проверяет завершение
func (ss *SigningSession) IsCompleted() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.Completed
}

// GetResult возвращает результат подписания
func (ss *SigningSession) GetResult() (*SigningResult, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if !ss.Completed {
		return nil, fmt.Errorf("signing not completed")
	}

	if ss.Error != nil {
		return nil, ss.Error
	}

	if ss.SignatureData == nil {
		return nil, fmt.Errorf("no signature data")
	}

	// Извлекаем компоненты подписи
	r := ss.SignatureData.R
	s := ss.SignatureData.S
	v := ss.SignatureData.SignatureRecovery

	// Формируем полную подпись (R || S || V)
	signature := append(r, s...)
	signature = append(signature, v...)

	return &SigningResult{
		Signature: hex.EncodeToString(signature),
		R:         hex.EncodeToString(r),
		S:         hex.EncodeToString(s),
		V:         int32(v[0]),
	}, nil
}
