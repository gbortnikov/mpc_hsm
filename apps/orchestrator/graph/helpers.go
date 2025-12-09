package graph

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/orchestrator/logger"
)

const (
	// Количество байт для генерации session ID (128 бит энтропии)
	sessionIDEntropyBytes = 16
)

// generateSecureSessionID генерирует криптографически случайный ID сессии
func generateSecureSessionID(prefix string) string {
	bytes := make([]byte, sessionIDEntropyBytes)
	if _, err := rand.Read(bytes); err != nil {
		// Критическая ошибка: если crypto/rand не работает, система небезопасна
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes))
}

// broadcastSigningAndCollect рассылает signing сообщение получателям и собирает ответы
func (r *mutationResolver) broadcastSigningAndCollect(ctx context.Context, msg *pb.SigningMessage) ([]*pb.SigningMessage, error) {
	var responses []*pb.SigningMessage
	var failedRecipients []string

	// Получаем список получателей
	var recipients []string
	if msg.IsBroadcast {
		// Broadcast - отправляем всем кроме отправителя
		r.mu.RLock()
		for _, node := range r.nodes {
			if node.PartyID != msg.FromParty {
				recipients = append(recipients, node.PartyID)
			}
		}
		r.mu.RUnlock()
	} else {
		// Unicast - отправляем указанным получателям
		recipients = msg.ToParties
	}

	// Отправляем сообщение каждому получателю
	for _, recipientPartyID := range recipients {
		client, exists := r.NodeManager.GetNode(recipientPartyID)
		if !exists {
			logger.Warn("Получатель не найден", map[string]interface{}{
				"from": msg.FromParty,
				"to":   recipientPartyID,
			})
			failedRecipients = append(failedRecipients, recipientPartyID)
			continue
		}

		resp, err := client.ProcessSigningMessage(ctx, msg)
		if err != nil {
			logger.Error("Ошибка отправки signing сообщения", map[string]interface{}{
				"from":  msg.FromParty,
				"to":    recipientPartyID,
				"error": err.Error(),
			})
			failedRecipients = append(failedRecipients, recipientPartyID)
			continue
		}

		if !resp.Success {
			logger.Error("Нода отклонила signing сообщение", map[string]interface{}{
				"from":          msg.FromParty,
				"to":            recipientPartyID,
				"error_message": resp.ErrorMessage,
			})
			failedRecipients = append(failedRecipients, recipientPartyID)
			continue
		}

		// Собираем ответные сообщения
		responses = append(responses, resp.OutgoingMessages...)
	}

	// Если слишком много неудачных отправок, возвращаем ошибку
	if len(failedRecipients) > 0 {
		logger.Warn("Некоторые получатели недоступны", map[string]interface{}{
			"failed_count": len(failedRecipients),
			"failed_nodes": failedRecipients,
			"total_count":  len(recipients),
		})
	}

	return responses, nil
}
