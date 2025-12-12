package server

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mpc_hsm/node/db"
	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/tss"
)

// MPCNodeServer реализует gRPC сервис MPC ноды
type MPCNodeServer struct {
	pb.UnimplementedMPCNodeServiceServer

	nodeID  string
	partyID string
	status  pb.NodeStatus
	version string

	// Хранилище сессий генерации ключей
	keygenSessions map[string]*tss.KeygenSession
	keygenMu       sync.RWMutex

	// Хранилище сессий подписания
	signingSessions map[string]*tss.SigningSession
	signingMu       sync.RWMutex

	// Поддерживаемые кривые
	supportedCurves []string

	// Компоненты безопасности
	identity      *security.NodeIdentity
	signer        *security.MessageSigner
	verifier      *security.MessageVerifier
	whitelist     *security.PartyWhitelist
	authenticator *security.MessageAuthenticator
	replayGuard   *security.ReplayGuard

	// База данных (обязательно)
	shareStore *db.ShareStore
}

// NewMPCNodeServer создаёт новый gRPC сервер с обязательным ShareStore
func NewMPCNodeServer(nodeID, partyID string, shareStore *db.ShareStore) *MPCNodeServer {
	return &MPCNodeServer{
		nodeID:          nodeID,
		partyID:         partyID,
		status:          pb.NodeStatus_NODE_STATUS_ONLINE,
		version:         "1.0.0",
		keygenSessions:  make(map[string]*tss.KeygenSession),
		signingSessions: make(map[string]*tss.SigningSession),
		supportedCurves: []string{"secp256k1"},
		shareStore:      shareStore,
	}
}

// NewMPCNodeServerWithSecurity создаёт gRPC сервер с компонентами безопасности
func NewMPCNodeServerWithSecurity(
	nodeID, partyID string,
	identity *security.NodeIdentity,
	signer *security.MessageSigner,
	verifier *security.MessageVerifier,
	whitelist *security.PartyWhitelist,
	replayGuard *security.ReplayGuard,
	shareStore *db.ShareStore,
) *MPCNodeServer {
	// Создание аутентификатора для проверки сообщений
	authenticator := security.NewMessageAuthenticator(identity, whitelist, replayGuard)

	return &MPCNodeServer{
		nodeID:          nodeID,
		partyID:         partyID,
		status:          pb.NodeStatus_NODE_STATUS_ONLINE,
		version:         "1.0.0",
		keygenSessions:  make(map[string]*tss.KeygenSession),
		signingSessions: make(map[string]*tss.SigningSession),
		supportedCurves: []string{"secp256k1"},
		identity:        identity,
		signer:          signer,
		verifier:        verifier,
		whitelist:       whitelist,
		authenticator:   authenticator,
		replayGuard:     replayGuard,
		shareStore:      shareStore,
	}
}

// HealthCheck проверяет состояние ноды
func (s *MPCNodeServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	slog.Debug("HealthCheck called", "node_id", s.nodeID)
	return &pb.HealthCheckResponse{
		Healthy: true,
		Status:  "running",
		Version: s.version,
	}, nil
}

// GetNodeInfo возвращает информацию о ноде
func (s *MPCNodeServer) GetNodeInfo(ctx context.Context, req *pb.GetNodeInfoRequest) (*pb.GetNodeInfoResponse, error) {
	slog.Debug("GetNodeInfo called", "node_id", s.nodeID, "party_id", s.partyID)

	var signingPublicKey string
	if s.identity != nil {
		signingPublicKey = s.identity.PublicKeyBase64()
	}
	return &pb.GetNodeInfoResponse{
		NodeId:           s.nodeID,
		PartyId:          s.partyID,
		Status:           s.status,
		SupportedCurves:  s.supportedCurves,
		SigningPublicKey: signingPublicKey,
	}, nil
}

