package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/mpc_hsm/node/db"
	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/tss"
	"google.golang.org/protobuf/proto"
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

// InitKeygen инициализирует сессию генерации ключей
// SECURITY: Проверяет whitelist и регистрирует публичные ключи участников
func (s *MPCNodeServer) InitKeygen(ctx context.Context, req *pb.InitKeygenRequest) (*pb.InitKeygenResponse, error) {
	slog.Info("InitKeygen called",
		"session_id", req.SessionId,
		"threshold", req.Threshold,
		"parties_count", len(req.Parties),
	)

	// SECURITY: Регистрируем публичные ключи всех участников в whitelist
	if s.whitelist != nil {
		for _, p := range req.Parties {
			if p.SigningPublicKey != "" {
				if err := s.whitelist.AddFromBase64(p.PartyId, p.SigningPublicKey); err != nil {
					slog.Warn("InitKeygen: failed to add party to whitelist",
						"party_id", p.PartyId,
						"error", err,
					)
				} else {
					slog.Debug("InitKeygen: added party to whitelist", "party_id", p.PartyId)
				}
			}
		}
		slog.Info("InitKeygen: whitelist updated", "parties_count", s.whitelist.Count())
	}

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
// SECURITY: Проверяет подпись отправителя перед обработкой
func (s *MPCNodeServer) ProcessKeygenMessage(ctx context.Context, req *pb.KeygenMessage) (*pb.KeygenMessageResponse, error) {
	slog.Debug("ProcessKeygenMessage called",
		"session_id", req.SessionId,
		"from_party", req.FromParty,
		"round", req.Round,
		"is_broadcast", req.IsBroadcast,
	)

	// SECURITY: Проверяем подпись сообщения
	if s.authenticator != nil {
		msgData := &security.KeygenMessageData{
			SessionID:   req.SessionId,
			FromParty:   req.FromParty,
			Round:       req.Round,
			Payload:     req.Payload,
			IsBroadcast: req.IsBroadcast,
		}

		if err := s.authenticator.VerifyKeygenMessage(msgData, req.Signature, req.Timestamp, req.Nonce); err != nil {
			slog.Warn("ProcessKeygenMessage: signature verification failed",
				"session_id", req.SessionId,
				"from_party", req.FromParty,
				"error", err,
			)
			return &pb.KeygenMessageResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("signature verification failed: %v", err),
			}, nil
		}
		slog.Debug("ProcessKeygenMessage: signature verified", "from_party", req.FromParty)
	}

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

	// Конвертируем и подписываем исходящие сообщения
	pbMessages := make([]*pb.KeygenMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMsg := &pb.KeygenMessage{
			SessionId:   req.SessionId,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       int32(req.Round),
			Payload:     msg.Payload,
			IsBroadcast: msg.IsBroadcast,
		}

		// SECURITY: Подписываем исходящие сообщения
		if s.authenticator != nil {
			msgData := &security.KeygenMessageData{
				SessionID:   pbMsg.SessionId,
				FromParty:   pbMsg.FromParty,
				Round:       pbMsg.Round,
				Payload:     pbMsg.Payload,
				IsBroadcast: pbMsg.IsBroadcast,
			}
			sig, ts, nonce, err := s.authenticator.SignKeygenMessage(msgData)
			if err == nil {
				pbMsg.Signature = sig
				pbMsg.Timestamp = ts
				pbMsg.Nonce = nonce
			}
		}

		pbMessages[i] = pbMsg
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
			pbMsg := &pb.KeygenMessage{
				SessionId:   req.SessionId,
				FromParty:   msg.FromParty,
				ToParties:   msg.ToParties,
				Round:       0,
				Payload:     msg.Payload,
				IsBroadcast: msg.IsBroadcast,
			}

			// SECURITY: Подписываем исходящие сообщения
			if s.authenticator != nil {
				msgData := &security.KeygenMessageData{
					SessionID:   pbMsg.SessionId,
					FromParty:   pbMsg.FromParty,
					Round:       pbMsg.Round,
					Payload:     pbMsg.Payload,
					IsBroadcast: pbMsg.IsBroadcast,
				}
				sig, ts, nonce, err := s.authenticator.SignKeygenMessage(msgData)
				if err == nil {
					pbMsg.Signature = sig
					pbMsg.Timestamp = ts
					pbMsg.Nonce = nonce
				}
			}

			pbMessages[i] = pbMsg
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

	// Проверяем, не сохранён ли share уже
	if !session.IsSaved() {
		savedData := session.GetSavedData()

		// Сохраняем share в базу данных
		_, err = s.shareStore.SaveShare(ctx, db.SaveShareParams{
			SessionID:    req.SessionId,
			PartyID:      s.partyID,
			PublicKey:    result.PublicKey,
			Address:      result.Address,
			Threshold:    result.Threshold,
			TotalParties: result.TotalParties,
			PartyIndex:   result.PartyIndex,
			Curve:        "secp256k1",
			SaveData:     savedData,
		})
		if err != nil {
			slog.Error("GetKeygenResult: failed to save share to database",
				"session_id", req.SessionId,
				"error", err,
			)
			return &pb.GetKeygenResultResponse{
				Completed:    true,
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to save share: %v", err),
			}, nil
		}
		session.MarkSaved()
		slog.Info("GetKeygenResult: share saved to database",
			"session_id", req.SessionId,
			"address", result.Address,
		)
	}

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

	// Загружаем ключ из базы данных по адресу
	// KeyId содержит Ethereum-адрес кошелька
	keyData, err := s.shareStore.LoadShareDataByAddress(ctx, req.KeyId)
	if err != nil {
		slog.Error("InitSigning: failed to load share from database",
			"address", req.KeyId,
			"error", err,
		)
		return &pb.InitSigningResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("key not found for address %s: %v", req.KeyId, err),
			SessionId:    req.SessionId,
		}, nil
	}
	slog.Info("InitSigning: loaded share from database", "address", req.KeyId)

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
// SECURITY: Проверяет подпись отправителя перед обработкой
func (s *MPCNodeServer) ProcessSigningMessage(ctx context.Context, req *pb.SigningMessage) (*pb.SigningMessageResponse, error) {
	slog.Debug("ProcessSigningMessage called",
		"session_id", req.SessionId,
		"from_party", req.FromParty,
		"round", req.Round,
		"is_broadcast", req.IsBroadcast,
	)

	// SECURITY: Проверяем подпись сообщения
	if s.authenticator != nil {
		msgData := &security.SigningMessageData{
			SessionID:   req.SessionId,
			FromParty:   req.FromParty,
			Round:       req.Round,
			Payload:     req.Payload,
			IsBroadcast: req.IsBroadcast,
		}

		if err := s.authenticator.VerifySigningMessage(msgData, req.Signature, req.Timestamp, req.Nonce); err != nil {
			slog.Warn("ProcessSigningMessage: signature verification failed",
				"session_id", req.SessionId,
				"from_party", req.FromParty,
				"error", err,
			)
			return &pb.SigningMessageResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("signature verification failed: %v", err),
			}, nil
		}
		slog.Debug("ProcessSigningMessage: signature verified", "from_party", req.FromParty)
	}

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
		slog.Error("ProcessSigningMessage: failed to process message",
			"session_id", req.SessionId,
			"from_party", req.FromParty,
			"error", err,
		)
		return &pb.SigningMessageResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Конвертируем и подписываем исходящие сообщения
	pbMessages := make([]*pb.SigningMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMsg := &pb.SigningMessage{
			SessionId:   req.SessionId,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       int32(req.Round),
			Payload:     msg.Payload,
			IsBroadcast: msg.IsBroadcast,
		}

		// SECURITY: Подписываем исходящие сообщения
		if s.authenticator != nil {
			msgData := &security.SigningMessageData{
				SessionID:   pbMsg.SessionId,
				FromParty:   pbMsg.FromParty,
				Round:       pbMsg.Round,
				Payload:     pbMsg.Payload,
				IsBroadcast: pbMsg.IsBroadcast,
			}
			sig, ts, nonce, err := s.authenticator.SignSigningMessage(msgData)
			if err == nil {
				pbMsg.Signature = sig
				pbMsg.Timestamp = ts
				pbMsg.Nonce = nonce
			}
		}

		pbMessages[i] = pbMsg
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
		// Получаем и возвращаем исходящие сообщения
		outgoing := session.GetOutgoingMessages()
		if len(outgoing) > 0 {
			slog.Info("GetSigningResult: returning outgoing messages",
				"session_id", req.SessionId,
				"party_id", s.partyID,
				"message_count", len(outgoing),
			)
		}
		pbMessages := make([]*pb.SigningMessage, len(outgoing))
		for i, msg := range outgoing {
			pbMsg := &pb.SigningMessage{
				SessionId:   req.SessionId,
				FromParty:   msg.FromParty,
				ToParties:   msg.ToParties,
				Round:       0,
				Payload:     msg.Payload,
				IsBroadcast: msg.IsBroadcast,
			}

			// SECURITY: Подписываем исходящие сообщения
			if s.authenticator != nil {
				msgData := &security.SigningMessageData{
					SessionID:   pbMsg.SessionId,
					FromParty:   pbMsg.FromParty,
					Round:       pbMsg.Round,
					Payload:     pbMsg.Payload,
					IsBroadcast: pbMsg.IsBroadcast,
				}
				sig, ts, nonce, err := s.authenticator.SignSigningMessage(msgData)
				if err == nil {
					pbMsg.Signature = sig
					pbMsg.Timestamp = ts
					pbMsg.Nonce = nonce
				}
			}

			pbMessages[i] = pbMsg
		}
		return &pb.GetSigningResultResponse{
			Completed:        false,
			Success:          true,
			OutgoingMessages: pbMessages,
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

// ============================================
// Защищённые P2P методы
// ============================================

// Ping обрабатывает ping запрос
func (s *MPCNodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	slog.Debug("Ping received", "from_party", req.FromPartyId)
	return &pb.PingResponse{
		PartyId:   s.partyID,
		Timestamp: req.Timestamp,
		Healthy:   true,
	}, nil
}

// ExchangePeerInfo обменивается информацией о пирах
func (s *MPCNodeServer) ExchangePeerInfo(ctx context.Context, req *pb.PeerInfoRequest) (*pb.PeerInfoResponse, error) {
	slog.Debug("ExchangePeerInfo", "from_party", req.RequesterPartyId)

	var signingPublicKey string
	if s.identity != nil {
		signingPublicKey = s.identity.PublicKeyBase64()
	}

	return &pb.PeerInfoResponse{
		PartyId:          s.partyID,
		SigningPublicKey: signingPublicKey,
		Address:          s.address,
		KnownPeers:       nil, // TODO: заполнить известными пирами
	}, nil
}

// ProcessSignedKeygenMessage обрабатывает подписанное keygen сообщение
func (s *MPCNodeServer) ProcessSignedKeygenMessage(ctx context.Context, envelope *pb.SignedEnvelope) (*pb.SignedKeygenResponse, error) {
	slog.Debug("ProcessSignedKeygenMessage",
		"from_party", envelope.SignerPartyId,
		"payload_len", len(envelope.Payload),
	)

	// Верифицируем подпись если verifier настроен
	if s.verifier != nil {
		secEnvelope := &security.SignedEnvelope{
			Payload:       envelope.Payload,
			SignerPartyID: envelope.SignerPartyId,
			Signature:     envelope.Signature,
			Timestamp:     envelope.Timestamp,
			Nonce:         envelope.Nonce,
		}

		_, err := s.verifier.Verify(secEnvelope)
		if err != nil {
			slog.Warn("Message verification failed",
				"from_party", envelope.SignerPartyId,
				"error", err,
			)
			return &pb.SignedKeygenResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("verification failed: %v", err),
			}, nil
		}
	}

	// Десериализуем KeygenMessage из payload
	var keygenMsg pb.KeygenMessage
	if err := proto.Unmarshal(envelope.Payload, &keygenMsg); err != nil {
		slog.Warn("Failed to unmarshal keygen message", "error", err)
		return &pb.SignedKeygenResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to unmarshal: %v", err),
		}, nil
	}

	// Обрабатываем сообщение
	resp, err := s.ProcessKeygenMessage(ctx, &keygenMsg)
	if err != nil {
		return &pb.SignedKeygenResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Подписываем исходящие сообщения если signer настроен
	var signedOutgoing []*pb.SignedEnvelope
	if s.signer != nil && resp.OutgoingMessages != nil {
		for _, outMsg := range resp.OutgoingMessages {
			payload, err := proto.Marshal(outMsg)
			if err != nil {
				continue
			}

			signed, err := s.signer.Sign(payload)
			if err != nil {
				continue
			}

			signedOutgoing = append(signedOutgoing, &pb.SignedEnvelope{
				Payload:       signed.Payload,
				SignerPartyId: signed.SignerPartyID,
				Signature:     signed.Signature,
				Timestamp:     signed.Timestamp,
				Nonce:         signed.Nonce,
			})
		}
	}

	return &pb.SignedKeygenResponse{
		Success:          resp.Success,
		ErrorMessage:     resp.ErrorMessage,
		OutgoingMessages: signedOutgoing,
	}, nil
}

// ProcessSignedSigningMessage обрабатывает подписанное signing сообщение
func (s *MPCNodeServer) ProcessSignedSigningMessage(ctx context.Context, envelope *pb.SignedEnvelope) (*pb.SignedSigningResponse, error) {
	slog.Debug("ProcessSignedSigningMessage",
		"from_party", envelope.SignerPartyId,
		"payload_len", len(envelope.Payload),
	)

	// Верифицируем подпись
	if s.verifier != nil {
		secEnvelope := &security.SignedEnvelope{
			Payload:       envelope.Payload,
			SignerPartyID: envelope.SignerPartyId,
			Signature:     envelope.Signature,
			Timestamp:     envelope.Timestamp,
			Nonce:         envelope.Nonce,
		}

		_, err := s.verifier.Verify(secEnvelope)
		if err != nil {
			return &pb.SignedSigningResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("verification failed: %v", err),
			}, nil
		}
	}

	// Десериализуем SigningMessage
	var signingMsg pb.SigningMessage
	if err := proto.Unmarshal(envelope.Payload, &signingMsg); err != nil {
		return &pb.SignedSigningResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to unmarshal: %v", err),
		}, nil
	}

	// Обрабатываем
	resp, err := s.ProcessSigningMessage(ctx, &signingMsg)
	if err != nil {
		return &pb.SignedSigningResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Подписываем исходящие
	var signedOutgoing []*pb.SignedEnvelope
	if s.signer != nil && resp.OutgoingMessages != nil {
		for _, outMsg := range resp.OutgoingMessages {
			payload, err := proto.Marshal(outMsg)
			if err != nil {
				continue
			}

			signed, err := s.signer.Sign(payload)
			if err != nil {
				continue
			}

			signedOutgoing = append(signedOutgoing, &pb.SignedEnvelope{
				Payload:       signed.Payload,
				SignerPartyId: signed.SignerPartyID,
				Signature:     signed.Signature,
				Timestamp:     signed.Timestamp,
				Nonce:         signed.Nonce,
			})
		}
	}

	return &pb.SignedSigningResponse{
		Success:          resp.Success,
		ErrorMessage:     resp.ErrorMessage,
		OutgoingMessages: signedOutgoing,
	}, nil
}
