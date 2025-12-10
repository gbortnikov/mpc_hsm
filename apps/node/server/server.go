package server

import (
	"context"
	"io"
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

	nodeID    string
	partyID   string
	publicKey string
	address   string
	status    pb.NodeStatus
	version   string

	// Хранилище сессий keygen
	keygenSessions map[string]*tss.KeygenSession
	keygenMu       sync.RWMutex

	// Хранилище сессий подписания
	signingSessions map[string]*tss.SigningSession
	signingMu       sync.RWMutex

	// Поддерживаемые кривые
	supportedCurves []string

	// Security компоненты
	identity      *security.NodeIdentity
	signer        *security.MessageSigner
	verifier      *security.MessageVerifier
	whitelist     *security.PartyWhitelist
	authenticator *security.MessageAuthenticator
	replayGuard   *security.ReplayGuard

	// Database (обязательно)
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
	// Создаём аутентификатор для проверки сообщений
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
		PublicKey:        s.publicKey,
		Address:          s.address,
		Status:           s.status,
		SupportedCurves:  s.supportedCurves,
		SigningPublicKey: signingPublicKey,
	}, nil
}

// MPCMessageStream реализует двунаправленный поток для обмена сообщениями
func (s *MPCNodeServer) MPCMessageStream(stream pb.MPCNodeService_MPCMessageStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Обрабатываем входящее сообщение
		var responses []*pb.MPCStreamMessage

		switch payload := msg.MessageType.(type) {
		case *pb.MPCStreamMessage_Keygen:
			s.keygenMu.RLock()
			session, exists := s.keygenSessions[msg.SessionId]
			s.keygenMu.RUnlock()

			if exists {
				outgoing, err := session.ProcessMessage(msg.FromParty, int(payload.Keygen.Round), payload.Keygen.Data, payload.Keygen.IsBroadcast)
				if err == nil {
					for _, out := range outgoing {
						responses = append(responses, &pb.MPCStreamMessage{
							SessionId: msg.SessionId,
							FromParty: out.FromParty,
							ToParties: out.ToParties,
							MessageType: &pb.MPCStreamMessage_Keygen{
								Keygen: &pb.KeygenStreamPayload{
									Round:       payload.Keygen.Round,
									Data:        out.Payload,
									IsBroadcast: out.IsBroadcast,
								},
							},
						})
					}
				}
			}

		case *pb.MPCStreamMessage_Signing:
			s.signingMu.RLock()
			session, exists := s.signingSessions[msg.SessionId]
			s.signingMu.RUnlock()

			if exists {
				outgoing, err := session.ProcessMessage(msg.FromParty, int(payload.Signing.Round), payload.Signing.Data, payload.Signing.IsBroadcast)
				if err == nil {
					for _, out := range outgoing {
						responses = append(responses, &pb.MPCStreamMessage{
							SessionId: msg.SessionId,
							FromParty: out.FromParty,
							ToParties: out.ToParties,
							MessageType: &pb.MPCStreamMessage_Signing{
								Signing: &pb.SigningStreamPayload{
									Round:       payload.Signing.Round,
									Data:        out.Payload,
									IsBroadcast: out.IsBroadcast,
								},
							},
						})
					}
				}
			}

		case *pb.MPCStreamMessage_Control:
			// Обработка контрольных сообщений
			switch payload.Control.Type {
			case pb.ControlType_CONTROL_TYPE_HEARTBEAT:
				responses = append(responses, &pb.MPCStreamMessage{
					SessionId: msg.SessionId,
					FromParty: s.partyID,
					MessageType: &pb.MPCStreamMessage_Control{
						Control: &pb.ControlMessage{
							Type:    pb.ControlType_CONTROL_TYPE_HEARTBEAT,
							Payload: "pong",
						},
					},
				})
			}
		}

		// Отправляем ответы
		for _, resp := range responses {
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}
