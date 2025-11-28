package graph

import (
	"context"
	"log"
	"time"

	"github.com/mpc_hsm/orchestrator/graph/model"
	"github.com/mpc_hsm/orchestrator/nodeclient"
)

// coordinateKeygen координирует обмен сообщениями между нодами во время keygen
func (r *mutationResolver) coordinateKeygen(ctx context.Context, sessionID string, nodes []*nodeclient.NodeClient) {
	log.Printf("Начата координация keygen для сессии %s", sessionID)

	// Простой polling-based подход для координации
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)
	completedNodes := make(map[string]bool)

	// Создаём мапу partyID -> node для быстрого поиска
	nodeMap := make(map[string]*nodeclient.NodeClient)
	for _, node := range nodes {
		nodeMap[node.GetPartyID()] = node
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("Координация keygen отменена для сессии %s", sessionID)
			return

		case <-timeout:
			log.Printf("Таймаут keygen для сессии %s", sessionID)
			r.mu.Lock()
			if session, ok := r.sessions[sessionID]; ok {
				session.Status = model.SessionStatusFailed
			}
			r.mu.Unlock()
			return

		case <-ticker.C:
			allCompleted := true

			for _, node := range nodes {
				partyID := node.GetPartyID()
				if completedNodes[partyID] {
					continue
				}

				// Получаем результат (включая исходящие сообщения)
				result, err := node.GetKeygenResult(ctx, sessionID)
				if err != nil {
					log.Printf("Ошибка получения результата от %s: %v", partyID, err)
					allCompleted = false
					continue
				}

				if result.Completed {
					completedNodes[partyID] = true
					if result.Success {
						log.Printf("Keygen завершён на ноде %s: публичный ключ %s", partyID, result.Result.PublicKey)
					} else {
						log.Printf("Keygen не удался на ноде %s: %s", partyID, result.ErrorMessage)
					}
				} else {
					allCompleted = false

					// Маршрутизируем исходящие сообщения другим нодам
					// (они уже возвращаются в GetKeygenResult)
					if len(result.OutgoingMessages) > 0 {
						log.Printf("[%s] Получено %d исходящих сообщений от %s", sessionID, len(result.OutgoingMessages), partyID)
					}
					for _, msg := range result.OutgoingMessages {
						if msg.IsBroadcast {
							// Broadcast - отправляем всем кроме отправителя
							for targetPartyID, targetNode := range nodeMap {
								if targetPartyID != msg.FromParty {
									log.Printf("[%s] Маршрутизация broadcast: %s -> %s (payload: %d bytes)", sessionID, msg.FromParty, targetPartyID, len(msg.Payload))
									_, err := targetNode.ProcessKeygenMessage(ctx, msg)
									if err != nil {
										log.Printf("Ошибка отправки broadcast от %s к %s: %v", msg.FromParty, targetPartyID, err)
									}
								}
							}
						} else {
							// Point-to-point - отправляем конкретным получателям
							for _, toParty := range msg.ToParties {
								if targetNode, ok := nodeMap[toParty]; ok {
									log.Printf("[%s] Маршрутизация p2p: %s -> %s (payload: %d bytes)", sessionID, msg.FromParty, toParty, len(msg.Payload))
									_, err := targetNode.ProcessKeygenMessage(ctx, msg)
									if err != nil {
										log.Printf("Ошибка отправки от %s к %s: %v", msg.FromParty, toParty, err)
									}
								}
							}
						}
					}
				}
			}

			if allCompleted && len(completedNodes) == len(nodes) {
				log.Printf("Keygen завершён для всех нод в сессии %s", sessionID)
				r.mu.Lock()
				if session, ok := r.sessions[sessionID]; ok {
					session.Status = model.SessionStatusCompleted
					now := time.Now().UTC().Format(time.RFC3339)
					session.CompletedAt = &now
				}
				r.mu.Unlock()
				return
			}
		}
	}
}
