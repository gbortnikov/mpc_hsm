package tss

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"golang.org/x/crypto/sha3"
)

// KeygenSession представляет сессию распределённой генерации ключей
type KeygenSession struct {
	SessionID    string
	PartyID      *tss.PartyID
	Party        *keygen.LocalParty
	PreParams    *keygen.LocalPreParams
	OutCh        chan tss.Message
	EndCh        chan *keygen.LocalPartySaveData
	ErrCh        chan *tss.Error
	SavedData    *keygen.LocalPartySaveData
	Completed    bool
	Error        error
	mu           sync.RWMutex
	parties      []*tss.PartyID
	threshold    int
	totalParties int

	// Буфер исходящих сообщений
	outgoingBuffer []OutgoingMessage
	bufferMu       sync.Mutex
	msgNotify      chan struct{} // уведомление о новых сообщениях

	// Признак сохранения в БД
	saved bool
}

// KeygenResult результат генерации ключей для одной ноды
type KeygenResult struct {
	PublicKey    string
	Address      string
	PublicShare  string
	PartyIndex   int32
	Threshold    int32
	TotalParties int32
}

// NewKeygenSession создаёт новую сессию keygen
func NewKeygenSession(sessionID string, partyIndex int, threshold, totalParties int) (*KeygenSession, error) {
	if threshold >= totalParties {
		return nil, fmt.Errorf("threshold must be less than total parties")
	}

	return &KeygenSession{
		SessionID:    sessionID,
		threshold:    threshold,
		totalParties: totalParties,
		OutCh:        make(chan tss.Message, totalParties*10),
		EndCh:        make(chan *keygen.LocalPartySaveData, 1),
		ErrCh:        make(chan *tss.Error, 1),
		msgNotify:    make(chan struct{}, 1), // буферизованный канал для уведомлений
	}, nil
}

// GeneratePreParams генерирует предварительные параметры (можно выполнять офлайн)
func (ks *KeygenSession) GeneratePreParams() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	preParams, err := keygen.GeneratePreParams(2 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to generate pre-params: %w", err)
	}
	ks.PreParams = preParams
	return nil
}

// Initialize инициализирует сессию с информацией об участниках
func (ks *KeygenSession) Initialize(partyID string, partyIndex int, parties []PartyInfo) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// Создаём PartyID для всех участников
	ks.parties = make([]*tss.PartyID, len(parties))
	for i, p := range parties {
		key := new(big.Int).SetInt64(int64(p.PartyIndex + 1))
		ks.parties[i] = tss.NewPartyID(p.PartyID, fmt.Sprintf("Party %s", p.PartyID), key)

		if p.PartyID == partyID {
			ks.PartyID = ks.parties[i]
		}
	}

	if ks.PartyID == nil {
		return fmt.Errorf("party %s not found in parties list", partyID)
	}

	// Сортируем PartyIDs
	sortedParties := tss.SortPartyIDs(ks.parties)
	ctx := tss.NewPeerContext(sortedParties)

	// Создаём параметры и party
	params := tss.NewParameters(tss.S256(), ctx, ks.PartyID, ks.totalParties, ks.threshold)

	if ks.PreParams == nil {
		return fmt.Errorf("pre-params not generated, call GeneratePreParams first")
	}

	ks.Party = keygen.NewLocalParty(params, ks.OutCh, ks.EndCh, *ks.PreParams).(*keygen.LocalParty)

	return nil
}

// Start запускает протокол keygen
func (ks *KeygenSession) Start() error {
	ks.mu.Lock()
	if ks.Party == nil {
		ks.mu.Unlock()
		return fmt.Errorf("party not initialized")
	}
	party := ks.Party
	ks.mu.Unlock()

	// Запускаем горутину для буферизации исходящих сообщений до party.Start()
	go ks.bufferOutgoingMessages()

	// Даём время горутине буферизации запуститься
	time.Sleep(10 * time.Millisecond)

	go func() {
		if err := party.Start(); err != nil {
			ks.ErrCh <- party.WrapError(err)
		}
	}()

	// Ожидаем завершения в отдельной горутине
	go ks.waitForCompletion()

	return nil
}

// waitForCompletion ожидает завершения генерации ключей
func (ks *KeygenSession) waitForCompletion() {
	select {
	case data := <-ks.EndCh:
		ks.mu.Lock()
		ks.SavedData = data
		ks.Completed = true
		ks.mu.Unlock()

	case err := <-ks.ErrCh:
		ks.mu.Lock()
		ks.Error = fmt.Errorf("keygen error: %v", err)
		ks.Completed = true
		ks.mu.Unlock()

	case <-time.After(5 * time.Minute):
		ks.mu.Lock()
		ks.Error = fmt.Errorf("keygen timeout")
		ks.Completed = true
		ks.mu.Unlock()
	}
}

// ProcessMessage обрабатывает входящее сообщение от другого участника
func (ks *KeygenSession) ProcessMessage(fromPartyID string, round int, payload []byte, isBroadcast bool) ([]OutgoingMessage, error) {
	ks.mu.RLock()
	party := ks.Party
	ks.mu.RUnlock()

	if party == nil {
		return nil, fmt.Errorf("party not initialized")
	}

	// Поиск отправителя
	var fromParty *tss.PartyID
	for _, p := range ks.parties {
		if p.Id == fromPartyID {
			fromParty = p
			break
		}
	}
	if fromParty == nil {
		return nil, fmt.Errorf("unknown sender party: %s", fromPartyID)
	}

	slog.Debug("ProcessMessage: calling UpdateFromBytes",
		"session_id", ks.SessionID,
		"from_party", fromPartyID,
		"is_broadcast", isBroadcast,
		"payload_len", len(payload),
	)

	// Обновление party входящими данными
	ok, err := party.UpdateFromBytes(payload, fromParty, isBroadcast)
	if err != nil {
		slog.Error("ProcessMessage: UpdateFromBytes failed",
			"session_id", ks.SessionID,
			"from_party", fromPartyID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	slog.Debug("ProcessMessage: UpdateFromBytes completed",
		"session_id", ks.SessionID,
		"from_party", fromPartyID,
		"ok", ok,
	)

	// Сбор исходящих сообщений
	return ks.collectOutgoingMessages(), nil
}

// bufferOutgoingMessages читает сообщения из канала и помещает в буфер
func (ks *KeygenSession) bufferOutgoingMessages() {
	for {
		select {
		case msg := <-ks.OutCh:
			wireBytes, _, err := msg.WireBytes()
			if err != nil {
				slog.Error("bufferOutgoingMessages: failed to get wire bytes", "error", err)
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

			slog.Debug("bufferOutgoingMessages: received message",
				"session_id", ks.SessionID,
				"from", outMsg.FromParty,
				"to", outMsg.ToParties,
				"is_broadcast", outMsg.IsBroadcast,
			)

			ks.bufferMu.Lock()
			ks.outgoingBuffer = append(ks.outgoingBuffer, outMsg)
			ks.bufferMu.Unlock()

			// Уведомляем о новом сообщении (неблокирующе)
			select {
			case ks.msgNotify <- struct{}{}:
			default:
			}

		default:
			// Проверка завершения сессии
			ks.mu.RLock()
			completed := ks.Completed
			ks.mu.RUnlock()
			if completed {
				return
			}
			// Небольшая пауза для снижения нагрузки на CPU
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// GetOutgoingMessages возвращает все ожидающие исходящие сообщения и очищает буфер
func (ks *KeygenSession) GetOutgoingMessages() []OutgoingMessage {
	ks.bufferMu.Lock()
	defer ks.bufferMu.Unlock()

	messages := ks.outgoingBuffer
	ks.outgoingBuffer = nil
	return messages
}

// collectOutgoingMessages собирает сообщения из буфера (для совместимости с ProcessMessage)
func (ks *KeygenSession) collectOutgoingMessages() []OutgoingMessage {
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
	case <-ks.msgNotify:
		// Получили уведомление, даём время на буферизацию остальных сообщений
		time.Sleep(settleTime)
	case <-time.After(maxWait):
		// Таймаут — возможно сообщений нет
	}

	// Дополнительно проверяем, есть ли ещё сообщения
	for time.Now().Before(deadline) {
		ks.bufferMu.Lock()
		hasMessages := len(ks.outgoingBuffer) > 0
		ks.bufferMu.Unlock()

		if hasMessages {
			break
		}
		time.Sleep(pollInterval)
	}

	messages := ks.GetOutgoingMessages()
	slog.Debug("collectOutgoingMessages: collected messages",
		"session_id", ks.SessionID,
		"count", len(messages),
	)
	return messages
}

// IsCompleted проверяет завершение сессии
func (ks *KeygenSession) IsCompleted() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.Completed
}

// GetResult возвращает результат генерации ключей
func (ks *KeygenSession) GetResult() (*KeygenResult, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if !ks.Completed {
		return nil, fmt.Errorf("keygen not completed")
	}

	if ks.Error != nil {
		return nil, ks.Error
	}

	if ks.SavedData == nil {
		return nil, fmt.Errorf("no saved data")
	}

	// Получение публичного ключа
	pubKey := ks.SavedData.ECDSAPub
	pkX, pkY := pubKey.X(), pubKey.Y()

	pk := ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     pkX,
		Y:     pkY,
	}

	// Кодирование публичного ключа
	pubKeyBytes := append(pk.X.Bytes(), pk.Y.Bytes()...)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	// Генерация Ethereum-адреса
	address := PublicKeyToAddress(&pk)

	// Публичная доля ключа
	publicShare := hex.EncodeToString(ks.SavedData.ShareID.Bytes())

	return &KeygenResult{
		PublicKey:    pubKeyHex,
		Address:      address,
		PublicShare:  publicShare,
		PartyIndex:   int32(ks.PartyID.Index),
		Threshold:    int32(ks.threshold),
		TotalParties: int32(ks.totalParties),
	}, nil
}

// GetSavedData возвращает сохранённые данные для дальнейшего использования
func (ks *KeygenSession) GetSavedData() *keygen.LocalPartySaveData {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.SavedData
}

// IsSaved проверяет, была ли доля уже сохранена в БД
func (ks *KeygenSession) IsSaved() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.saved
}

// MarkSaved отмечает, что доля была сохранена в БД
func (ks *KeygenSession) MarkSaved() {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.saved = true
}

// PartyInfo содержит информацию об участнике
type PartyInfo struct {
	PartyID    string
	PartyIndex int
	Address    string
}

// OutgoingMessage представляет исходящее сообщение для других участников
type OutgoingMessage struct {
	FromParty   string
	ToParties   []string
	Payload     []byte
	IsBroadcast bool
}

// PublicKeyToAddress преобразует публичный ключ ECDSA в Ethereum-адрес
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) string {
	paddedPubKey := make([]byte, 64)
	copy(paddedPubKey[32-len(pubKey.X.Bytes()):32], pubKey.X.Bytes())
	copy(paddedPubKey[64-len(pubKey.Y.Bytes()):64], pubKey.Y.Bytes())

	hash := sha3.NewLegacyKeccak256()
	hash.Write(paddedPubKey)
	hashBytes := hash.Sum(nil)

	address := hashBytes[len(hashBytes)-20:]
	return "0x" + hex.EncodeToString(address)
}
