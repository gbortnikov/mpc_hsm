package tss

import (
	"encoding/hex"
	"fmt"
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
	if keyData == nil {
		return nil, fmt.Errorf("key data is required")
	}
	if len(messageToSign) == 0 {
		return nil, fmt.Errorf("message to sign is required")
	}

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
	}

	if ss.PartyID == nil {
		return fmt.Errorf("party %s not found in parties list", partyID)
	}

	// Сортируем PartyIDs
	sortedParties := tss.SortPartyIDs(ss.parties)
	ctx := tss.NewPeerContext(sortedParties)

	// Threshold для подписания = количество участников - 1
	threshold := totalParties - 1

	// Создаём параметры
	params := tss.NewParameters(tss.S256(), ctx, ss.PartyID, totalParties, threshold)

	// Преобразуем сообщение в big.Int
	msgBigInt := new(big.Int).SetBytes(ss.MessageToSign)

	// Создаём party для подписания
	ss.Party = signing.NewLocalParty(msgBigInt, params, *ss.KeyData, ss.OutCh, ss.EndCh).(*signing.LocalParty)

	return nil
}

// Start запускает протокол подписания
func (ss *SigningSession) Start() error {
	ss.mu.Lock()
	if ss.Party == nil {
		ss.mu.Unlock()
		return fmt.Errorf("party not initialized")
	}
	party := ss.Party
	ss.mu.Unlock()

	go func() {
		if err := party.Start(); err != nil {
			ss.ErrCh <- party.WrapError(err)
		}
	}()

	// Слушаем завершение
	go ss.waitForCompletion()

	return nil
}

// waitForCompletion ожидает завершения подписания
func (ss *SigningSession) waitForCompletion() {
	select {
	case data := <-ss.EndCh:
		ss.mu.Lock()
		ss.SignatureData = data
		ss.Completed = true
		ss.mu.Unlock()

	case err := <-ss.ErrCh:
		ss.mu.Lock()
		ss.Error = fmt.Errorf("signing error: %v", err)
		ss.Completed = true
		ss.mu.Unlock()

	case <-time.After(2 * time.Minute):
		ss.mu.Lock()
		ss.Error = fmt.Errorf("signing timeout")
		ss.Completed = true
		ss.mu.Unlock()
	}
}

// ProcessMessage обрабатывает входящее сообщение
func (ss *SigningSession) ProcessMessage(fromPartyID string, round int, payload []byte, isBroadcast bool) ([]OutgoingMessage, error) {
	ss.mu.RLock()
	party := ss.Party
	ss.mu.RUnlock()

	if party == nil {
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
		return nil, fmt.Errorf("unknown sender party: %s", fromPartyID)
	}

	// Обновляем party
	_, err := party.UpdateFromBytes(payload, fromParty, isBroadcast)
	if err != nil {
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	return ss.collectOutgoingMessages(), nil
}

// GetOutgoingMessages возвращает исходящие сообщения
func (ss *SigningSession) GetOutgoingMessages() []OutgoingMessage {
	return ss.collectOutgoingMessages()
}

// collectOutgoingMessages собирает сообщения из канала
func (ss *SigningSession) collectOutgoingMessages() []OutgoingMessage {
	var messages []OutgoingMessage

	for {
		select {
		case msg := <-ss.OutCh:
			wireBytes, _, err := msg.WireBytes()
			if err != nil {
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

			messages = append(messages, outMsg)
		default:
			return messages
		}
	}
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
