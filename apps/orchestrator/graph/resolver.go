package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/mpc_hsm/orchestrator/graph/model"
	"github.com/mpc_hsm/orchestrator/nodeclient"
	pb "github.com/mpc_hsm/node/proto"
)

// Resolver - корневой резолвер для GraphQL.
type Resolver struct {
	mu sync.RWMutex

	// Хранилище в памяти
	nodes    map[string]*model.Node
	sessions map[string]*model.Session

	// Счётчики для генерации ID
	sessionCounter int
	nodeCounter    int

	// Менеджер подключений к MPC нодам
	NodeManager *nodeclient.NodeManager
}

// NewResolver создаёт новый экземпляр резолвера.
func NewResolver() *Resolver {
	return &Resolver{
		nodes:       make(map[string]*model.Node),
		sessions:    make(map[string]*model.Session),
		NodeManager: nodeclient.NewNodeManager(),
	}
}

// broadcastAndCollect рассылает keygen сообщение и собирает ответные сообщения
func (r *mutationResolver) broadcastAndCollect(ctx context.Context, msg *pb.KeygenMessage) ([]*pb.KeygenMessage, error) {
	var responseMessages []*pb.KeygenMessage

	// Определяем получателей
	recipients := r.NodeManager.GetAllNodes()

	for _, client := range recipients {
		// Пропускаем отправителя
		if client.GetPartyID() == msg.FromParty {
			continue
		}

		// Проверяем, является ли нода получателем для P2P сообщений
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

		// Отправляем сообщение и получаем ответ
		resp, err := client.ProcessKeygenMessage(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to send message to %s: %w", client.GetPartyID(), err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("node %s rejected message: %s", client.GetPartyID(), resp.ErrorMessage)
		}

		// Добавляем ответные сообщения в список
		responseMessages = append(responseMessages, resp.OutgoingMessages...)
	}

	return responseMessages, nil
}
