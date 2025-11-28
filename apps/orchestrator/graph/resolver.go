package graph

import (
	"sync"
	"time"

	"github.com/mpc_hsm/orchestrator/graph/model"
	"github.com/mpc_hsm/orchestrator/nodeclient"
)

// Version - версия оркестратора
const Version = "1.0.0"

// Resolver - корневой резолвер для GraphQL.
// Этот файл не будет автоматически перегенерирован.
// Используется для внедрения зависимостей в приложение.
type Resolver struct {
	mu sync.RWMutex

	// Время запуска сервера
	startTime time.Time

	// Хранилище в памяти (для продакшена заменить на БД)
	nodes    map[string]*model.Node
	sessions map[string]*model.Session
	messages map[string][]*model.MPCMessage // sessionId -> сообщения

	// Счётчики для генерации ID
	messageCounter int
	sessionCounter int
	nodeCounter    int

	// Каналы для подписок
	nodeUpdates    chan *model.Node                  // обновления статуса нод
	sessionUpdates map[string]chan *model.Session    // sessionId -> канал обновлений
	messageStreams map[string]chan *model.MPCMessage // partyId:sessionId -> канал сообщений

	// Менеджер подключений к MPC нодам
	NodeManager *nodeclient.NodeManager
}

// NewResolver создаёт новый экземпляр резолвера.
func NewResolver() *Resolver {
	return &Resolver{
		startTime:      time.Now(),
		nodes:          make(map[string]*model.Node),
		sessions:       make(map[string]*model.Session),
		messages:       make(map[string][]*model.MPCMessage),
		nodeUpdates:    make(chan *model.Node, 100),
		sessionUpdates: make(map[string]chan *model.Session),
		messageStreams: make(map[string]chan *model.MPCMessage),
		NodeManager:    nodeclient.NewNodeManager(),
	}
}
