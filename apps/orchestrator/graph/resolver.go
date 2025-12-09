package graph

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/orchestrator/config"
	"github.com/mpc_hsm/orchestrator/graph/model"
	"github.com/mpc_hsm/orchestrator/logger"
	"github.com/mpc_hsm/orchestrator/nodeclient"
)

// Resolver - корневой резолвер для GraphQL.
type Resolver struct {
	mu sync.RWMutex

	// Хранилище в памяти
	nodes map[string]*model.Node

	// Счётчики для генерации ID
	sessionCounter int
	nodeCounter    int

	// Менеджер подключений к MPC нодам
	NodeManager *nodeclient.NodeManager

	// Конфигурация
	config *config.Config
}

// NewResolver создаёт новый экземпляр резолвера.
func NewResolver(cfg *config.Config) *Resolver {
	return &Resolver{
		nodes:       make(map[string]*model.Node),
		NodeManager: nodeclient.NewNodeManager(&cfg.TLS),
		config:      cfg,
	}
}

// GetConfig возвращает конфигурацию оркестратора
func (r *Resolver) GetConfig() *config.Config {
	return r.config
}

// broadcastAndCollect рассылает keygen сообщение и собирает ответные сообщения
func (r *mutationResolver) broadcastAndCollect(ctx context.Context, msg *pb.KeygenMessage) ([]*pb.KeygenMessage, error) {
	var responseMessages []*pb.KeygenMessage

	recipients := r.NodeManager.GetAllNodes()

	for _, client := range recipients {
		if client.GetPartyID() == msg.FromParty {
			continue
		}

		// Проверяем, является ли нода получателем
		if !msg.IsBroadcast && len(msg.ToParties) > 0 {
			isRecipient := false
			for _, to := range msg.ToParties {
				if to == client.GetPartyID() {
					isRecipient = true
					break
				}
			}
			if !isRecipient {
				continue
			}
		}

		resp, err := client.ProcessKeygenMessage(ctx, msg)
		if err != nil {
			logger.Error("Ошибка отправки keygen сообщения", map[string]interface{}{
				"from":  msg.FromParty,
				"to":    client.GetPartyID(),
				"error": err.Error(),
			})
			return nil, fmt.Errorf("не удалось отправить сообщение %s -> %s: %w", msg.FromParty, client.GetPartyID(), err)
		}

		if !resp.Success {
			logger.Error("Нода отклонила keygen сообщение", map[string]interface{}{
				"from":          msg.FromParty,
				"to":            client.GetPartyID(),
				"error_message": resp.ErrorMessage,
			})
			return nil, fmt.Errorf("нода %s отклонила сообщение: %s", client.GetPartyID(), resp.ErrorMessage)
		}

		responseMessages = append(responseMessages, resp.OutgoingMessages...)
	}

	return responseMessages, nil
}
