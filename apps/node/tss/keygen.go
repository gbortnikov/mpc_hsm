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

	// Буфер для исходящих сообщений
	outgoingBuffer []OutgoingMessage
	bufferMu       sync.Mutex

	// Флаг сохранения в БД
	saved bool
}

// KeygenResult содержит результат генерации ключей для одной ноды
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
	}, nil
}

// GeneratePreParams генерирует предварительные параметры (можно делать офлайн)
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

	// Запускаем горутину для буферизации исходящих сообщений ДО party.Start()
	go ks.bufferOutgoingMessages()

	// Даём время горутине буферизации запуститься
	time.Sleep(10 * time.Millisecond)

	go func() {
		if err := party.Start(); err != nil {
			ks.ErrCh <- party.WrapError(err)
		}
	}()

	// Слушаем завершение в отдельной горутине
	go ks.waitForCompletion()

	return nil
}

// waitForCompletion ожидает завершения keygen
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

	// Находим отправителя
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

	// Обновляем party с входящими данными
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

	// Собираем исходящие сообщения
	return ks.collectOutgoingMessages(), nil
}

// bufferOutgoingMessages читает сообщения из канала и складывает в буфер
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

		default:
			// Проверяем, завершена ли сессия
			ks.mu.RLock()
			completed := ks.Completed
			ks.mu.RUnlock()
			if completed {
				return
			}
			// Небольшая пауза, чтобы не грузить CPU
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

// collectOutgoingMessages собирает сообщения из канала (для совместимости с ProcessMessage)
func (ks *KeygenSession) collectOutgoingMessages() []OutgoingMessage {
	// Даём время для буферизации сообщений после UpdateFromBytes
	// TSS библиотека генерирует сообщения асинхронно
	time.Sleep(500 * time.Millisecond)
	messages := ks.GetOutgoingMessages()
	slog.Debug("collectOutgoingMessages: collected messages",
		"session_id", ks.SessionID,
		"count", len(messages),
	)
	return messages
}

// IsCompleted проверяет, завершена ли сессия
func (ks *KeygenSession) IsCompleted() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.Completed
}

// GetResult возвращает результат keygen
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

	// Получаем публичный ключ
	pubKey := ks.SavedData.ECDSAPub
	pkX, pkY := pubKey.X(), pubKey.Y()

	pk := ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     pkX,
		Y:     pkY,
	}

	// Кодируем публичный ключ
	pubKeyBytes := append(pk.X.Bytes(), pk.Y.Bytes()...)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	// Генерируем Ethereum адрес
	address := PublicKeyToAddress(&pk)

	// Публичная доля
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

// GetSavedData возвращает сохранённые данные для последующего использования
func (ks *KeygenSession) GetSavedData() *keygen.LocalPartySaveData {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.SavedData
}

// IsSaved проверяет, был ли share уже сохранён в БД
func (ks *KeygenSession) IsSaved() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.saved
}

// MarkSaved отмечает, что share был сохранён в БД
func (ks *KeygenSession) MarkSaved() {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.saved = true
}

// PartyInfo информация об участнике
type PartyInfo struct {
	PartyID    string
	PartyIndex int
	Address    string
}

// OutgoingMessage исходящее сообщение для других участников
type OutgoingMessage struct {
	FromParty   string
	ToParties   []string
	Payload     []byte
	IsBroadcast bool
}

// PublicKeyToAddress конвертирует ECDSA публичный ключ в Ethereum адрес
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
