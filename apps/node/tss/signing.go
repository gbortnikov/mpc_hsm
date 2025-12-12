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

	// Буфер исходящих сообщений
	outgoingBuffer []OutgoingMessage
	bufferMu       sync.Mutex
	msgNotify      chan struct{} // уведомление о новых сообщениях
}

// SigningResult представляет результат подписания
type SigningResult struct {
	Signature string // Полная подпись в hex
	R         string
	S         string
	V         int32 // Идентификатор восстановления
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
		msgNotify:     make(chan struct{}, 1), // буферизованный канал для уведомлений
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

	// Threshold берётся из KeyData - должен соответствовать threshold генерации ключей
	// В TSS-библиотеке threshold = t, где для подписания необходимо t+1 участников
	// KeyData.Ks содержит индексы всех участников генерации ключей
	threshold := totalParties - 1

	slog.Debug("SigningSession.Initialize: using threshold",
		"threshold", threshold,
		"total_parties", totalParties,
		"key_ks_count", len(ss.KeyData.Ks),
	)

	// Создание параметров
	params := tss.NewParameters(tss.S256(), ctx, ss.PartyID, totalParties, threshold)

	// Преобразование сообщения в big.Int
	msgBigInt := new(big.Int).SetBytes(ss.MessageToSign)

	// Создание party для подписания
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

	// Запуск горутины для буферизации исходящих сообщений до party.Start()
	go ss.bufferOutgoingMessages()

	// Ожидание запуска горутины буферизации
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

	// Ожидание завершения
	go ss.waitForCompletion()

	slog.Debug("SigningSession.Start: signing protocol started", "session_id", ss.SessionID)
	return nil
}

// waitForCompletion ожидает завершения процесса подписания
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

	// Поиск отправителя
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

	// Обновление party
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

// bufferOutgoingMessages читает сообщения из канала и помещает в буфер
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

			// Уведомляем о новом сообщении (неблокирующе)
			select {
			case ss.msgNotify <- struct{}{}:
			default:
			}

		default:
			// Проверка завершения сессии
			ss.mu.RLock()
			completed := ss.Completed
			ss.mu.RUnlock()
			if completed {
				return
			}
			// Небольшая пауза для снижения нагрузки на CPU
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
	// Ожидание буферизации сообщений после UpdateFromBytes
	// TSS-библиотека генерирует сообщения асинхронно
	// Используем умное ожидание: ждём уведомления или короткий таймаут
	const (
		maxWait      = 50 * time.Millisecond // максимальное время ожидания
		settleTime   = 10 * time.Millisecond // время на "устаканивание" после получения сообщения
		pollInterval = 5 * time.Millisecond  // интервал проверки буфера
	)

	deadline := time.Now().Add(maxWait)

	// Ждём первое сообщение или таймаут
	select {
	case <-ss.msgNotify:
		// Получили уведомление, даём время на буферизацию остальных сообщений
		time.Sleep(settleTime)
	case <-time.After(maxWait):
		// Таймаут — возможно сообщений нет
	}

	// Дополнительно проверяем, есть ли ещё сообщения
	for time.Now().Before(deadline) {
		ss.bufferMu.Lock()
		hasMessages := len(ss.outgoingBuffer) > 0
		ss.bufferMu.Unlock()

		if hasMessages {
			break
		}
		time.Sleep(pollInterval)
	}

	messages := ss.GetOutgoingMessages()
	slog.Debug("SigningSession.collectOutgoingMessages: collected messages",
		"session_id", ss.SessionID,
		"count", len(messages),
	)
	return messages
}

// IsCompleted проверяет завершение сессии
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

	// Извлечение компонентов подписи
	r := ss.SignatureData.R
	s := ss.SignatureData.S
	v := ss.SignatureData.SignatureRecovery

	// Формирование полной подписи (R || S || V)
	signature := append(r, s...)
	signature = append(signature, v...)

	return &SigningResult{
		Signature: hex.EncodeToString(signature),
		R:         hex.EncodeToString(r),
		S:         hex.EncodeToString(s),
		V:         int32(v[0]),
	}, nil
}
