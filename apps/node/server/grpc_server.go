package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	pb "github.com/mpc_hsm/node/proto"
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

	// Хранилище сгенерированных ключей (keyID -> savedData)
	savedKeys map[string]*keygen.LocalPartySaveData
	keysMu    sync.RWMutex

	// Поддерживаемые кривые
	supportedCurves []string
}

// NewMPCNodeServer создаёт новый gRPC сервер
func NewMPCNodeServer(nodeID, partyID string) *MPCNodeServer {
	return &MPCNodeServer{
		nodeID:          nodeID,
		partyID:         partyID,
		status:          pb.NodeStatus_NODE_STATUS_ONLINE,
		version:         "1.0.0",
		keygenSessions:  make(map[string]*tss.KeygenSession),
		signingSessions: make(map[string]*tss.SigningSession),
		savedKeys:       make(map[string]*keygen.LocalPartySaveData),
		supportedCurves: []string{"secp256k1"},
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
	return &pb.GetNodeInfoResponse{
		NodeId:          s.nodeID,
		PartyId:         s.partyID,
		PublicKey:       s.publicKey,
		Address:         s.address,
		Status:          s.status,
		SupportedCurves: s.supportedCurves,
	}, nil
}

// InitKeygen инициализирует сессию генерации ключей
func (s *MPCNodeServer) InitKeygen(ctx context.Context, req *pb.InitKeygenRequest) (*pb.InitKeygenResponse, error) {
	slog.Info("InitKeygen called",
		"session_id", req.SessionId,
		"threshold", req.Threshold,
		"parties_count", len(req.Parties),
	)

	s.keygenMu.Lock()
	defer s.keygenMu.Unlock()

	// Проверяем, существует ли уже сессия
	if _, exists := s.keygenSessions[req.SessionId]; exists {
		slog.Warn("InitKeygen: session already exists", "session_id", req.SessionId)
		return &pb.InitKeygenResponse{
			Success:      false,
			ErrorMessage: "session already exists",
			SessionId:    req.SessionId,
		}, nil
	}

	// Создаём новую сессию
	session, err := tss.NewKeygenSession(req.SessionId, 0, int(req.Threshold), len(req.Parties))
	if err != nil {
		slog.Error("InitKeygen: failed to create session", "session_id", req.SessionId, "error", err)
		return &pb.InitKeygenResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			SessionId:    req.SessionId,
		}, nil
	}

	// Генерируем предварительные параметры
	if err := session.GeneratePreParams(); err != nil {
		slog.Error("InitKeygen: failed to generate pre-params", "session_id", req.SessionId, "error", err)
		return &pb.InitKeygenResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to generate pre-params: %v", err),
			SessionId:    req.SessionId,
		}, nil
	}

	// Конвертируем информацию об участниках
	parties := make([]tss.PartyInfo, len(req.Parties))
	for i, p := range req.Parties {
		parties[i] = tss.PartyInfo{
			PartyID:    p.PartyId,
			PartyIndex: int(p.PartyIndex),
			Address:    p.Address,
		}
	}

	// Инициализируем сессию
	if err := session.Initialize(s.partyID, s.findPartyIndex(req.Parties), parties); err != nil {
		slog.Error("InitKeygen: failed to initialize session", "session_id", req.SessionId, "error", err)
		return &pb.InitKeygenResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to initialize session: %v", err),
			SessionId:    req.SessionId,
		}, nil
	}

	// Запускаем keygen
	if err := session.Start(); err != nil {
		slog.Error("InitKeygen: failed to start keygen", "session_id", req.SessionId, "error", err)
		return &pb.InitKeygenResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to start keygen: %v", err),
			SessionId:    req.SessionId,
		}, nil
	}

	s.keygenSessions[req.SessionId] = session

	slog.Info("InitKeygen: session started successfully", "session_id", req.SessionId)
	return &pb.InitKeygenResponse{
		Success:   true,
		SessionId: req.SessionId,
	}, nil
}

// findPartyIndex находит индекс текущей ноды среди участников
func (s *MPCNodeServer) findPartyIndex(parties []*pb.PartyInfo) int {
	for _, p := range parties {
		if p.PartyId == s.partyID {
			return int(p.PartyIndex)
		}
	}
	return 0
}

// ProcessKeygenMessage обрабатывает входящее сообщение keygen
func (s *MPCNodeServer) ProcessKeygenMessage(ctx context.Context, req *pb.KeygenMessage) (*pb.KeygenMessageResponse, error) {
	slog.Debug("ProcessKeygenMessage called",
		"session_id", req.SessionId,
		"from_party", req.FromParty,
		"round", req.Round,
		"is_broadcast", req.IsBroadcast,
	)

	s.keygenMu.RLock()
	session, exists := s.keygenSessions[req.SessionId]
	s.keygenMu.RUnlock()

	if !exists {
		slog.Warn("ProcessKeygenMessage: session not found", "session_id", req.SessionId)
		return &pb.KeygenMessageResponse{
			Success:      false,
			ErrorMessage: "session not found",
		}, nil
	}

	// Обрабатываем входящее сообщение
	outgoing, err := session.ProcessMessage(req.FromParty, int(req.Round), req.Payload, req.IsBroadcast)
	if err != nil {
		slog.Error("ProcessKeygenMessage: failed to process message",
			"session_id", req.SessionId,
			"from_party", req.FromParty,
			"error", err,
		)
		return &pb.KeygenMessageResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Конвертируем исходящие сообщения
	pbMessages := make([]*pb.KeygenMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMessages[i] = &pb.KeygenMessage{
			SessionId:   req.SessionId,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       int32(req.Round),
			Payload:     msg.Payload,
			IsBroadcast: msg.IsBroadcast,
		}
	}

	return &pb.KeygenMessageResponse{
		Success:          true,
		OutgoingMessages: pbMessages,
	}, nil
}

// GetKeygenResult возвращает результат генерации ключей
func (s *MPCNodeServer) GetKeygenResult(ctx context.Context, req *pb.GetKeygenResultRequest) (*pb.GetKeygenResultResponse, error) {
	s.keygenMu.RLock()
	session, exists := s.keygenSessions[req.SessionId]
	s.keygenMu.RUnlock()

	if !exists {
		return &pb.GetKeygenResultResponse{
			Completed:    false,
			Success:      false,
			ErrorMessage: "session not found",
		}, nil
	}

	if !session.IsCompleted() {
		// Получаем и возвращаем исходящие сообщения
		outgoing := session.GetOutgoingMessages()
		if len(outgoing) > 0 {
			slog.Info("GetKeygenResult: returning outgoing messages",
				"session_id", req.SessionId,
				"party_id", s.partyID,
				"message_count", len(outgoing),
			)
		}
		pbMessages := make([]*pb.KeygenMessage, len(outgoing))
		for i, msg := range outgoing {
			pbMessages[i] = &pb.KeygenMessage{
				SessionId:   req.SessionId,
				FromParty:   msg.FromParty,
				ToParties:   msg.ToParties,
				Round:       0,
				Payload:     msg.Payload,
				IsBroadcast: msg.IsBroadcast,
			}
		}
		return &pb.GetKeygenResultResponse{
			Completed:        false,
			Success:          true,
			OutgoingMessages: pbMessages,
		}, nil
	}

	result, err := session.GetResult()
	if err != nil {
		return &pb.GetKeygenResultResponse{
			Completed:    true,
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Сохраняем ключ
	s.keysMu.Lock()
	s.savedKeys[req.SessionId] = session.GetSavedData()
	s.keysMu.Unlock()

	return &pb.GetKeygenResultResponse{
		Completed: true,
		Success:   true,
		Result: &pb.KeygenResult{
			PublicKey:    result.PublicKey,
			Address:      result.Address,
			PublicShare:  result.PublicShare,
			PartyIndex:   result.PartyIndex,
			Threshold:    result.Threshold,
			TotalParties: result.TotalParties,
		},
	}, nil
}

// InitSigning инициализирует сессию подписания
func (s *MPCNodeServer) InitSigning(ctx context.Context, req *pb.InitSigningRequest) (*pb.InitSigningResponse, error) {
	s.signingMu.Lock()
	defer s.signingMu.Unlock()

	// Проверяем существование сессии
	if _, exists := s.signingSessions[req.SessionId]; exists {
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: "session already exists",
			SessionId:    req.SessionId,
		}, nil
	}

	// Получаем сохранённый ключ
	s.keysMu.RLock()
	keyData, exists := s.savedKeys[req.KeyId]
	s.keysMu.RUnlock()

	if !exists {
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: "key not found",
			SessionId:    req.SessionId,
		}, nil
	}

	// Создаём сессию подписания
	session, err := tss.NewSigningSession(req.SessionId, keyData, req.MessageToSign)
	if err != nil {
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			SessionId:    req.SessionId,
		}, nil
	}

	// Конвертируем участников
	parties := make([]tss.PartyInfo, len(req.Parties))
	for i, p := range req.Parties {
		parties[i] = tss.PartyInfo{
			PartyID:    p.PartyId,
			PartyIndex: int(p.PartyIndex),
			Address:    p.Address,
		}
	}

	// Инициализируем
	if err := session.Initialize(s.partyID, parties); err != nil {
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			SessionId:    req.SessionId,
		}, nil
	}

	// Запускаем
	if err := session.Start(); err != nil {
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			SessionId:    req.SessionId,
		}, nil
	}

	s.signingSessions[req.SessionId] = session

	return &pb.InitSigningResponse{
		Success:   true,
		SessionId: req.SessionId,
	}, nil
}

// ProcessSigningMessage обрабатывает сообщение подписания
func (s *MPCNodeServer) ProcessSigningMessage(ctx context.Context, req *pb.SigningMessage) (*pb.SigningMessageResponse, error) {
	s.signingMu.RLock()
	session, exists := s.signingSessions[req.SessionId]
	s.signingMu.RUnlock()

	if !exists {
		return &pb.SigningMessageResponse{
			Success:      false,
			ErrorMessage: "session not found",
		}, nil
	}

	outgoing, err := session.ProcessMessage(req.FromParty, int(req.Round), req.Payload, req.IsBroadcast)
	if err != nil {
		return &pb.SigningMessageResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	pbMessages := make([]*pb.SigningMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMessages[i] = &pb.SigningMessage{
			SessionId:   req.SessionId,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       int32(req.Round),
			Payload:     msg.Payload,
			IsBroadcast: msg.IsBroadcast,
		}
	}

	return &pb.SigningMessageResponse{
		Success:          true,
		OutgoingMessages: pbMessages,
	}, nil
}

// GetSigningResult возвращает результат подписания
func (s *MPCNodeServer) GetSigningResult(ctx context.Context, req *pb.GetSigningResultRequest) (*pb.GetSigningResultResponse, error) {
	s.signingMu.RLock()
	session, exists := s.signingSessions[req.SessionId]
	s.signingMu.RUnlock()

	if !exists {
		return &pb.GetSigningResultResponse{
			Completed:    false,
			Success:      false,
			ErrorMessage: "session not found",
		}, nil
	}

	if !session.IsCompleted() {
		return &pb.GetSigningResultResponse{
			Completed: false,
			Success:   true,
		}, nil
	}

	result, err := session.GetResult()
	if err != nil {
		return &pb.GetSigningResultResponse{
			Completed:    true,
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &pb.GetSigningResultResponse{
		Completed: true,
		Success:   true,
		Result: &pb.SigningResult{
			Signature: result.Signature,
			R:         result.R,
			S:         result.S,
			V:         result.V,
		},
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
