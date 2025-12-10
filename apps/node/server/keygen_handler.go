package server

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/tss"
)

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
	pbMessages := s.signOutgoingKeygenMessages(req.SessionId, outgoing, req.Round)

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
		outgoing := session.GetOutgoingMessages()
		if len(outgoing) > 0 {
			slog.Info("GetKeygenResult: returning outgoing messages",
				"session_id", req.SessionId,
				"party_id", s.partyID,
				"message_count", len(outgoing),
			)
		}
		pbMessages := s.signOutgoingKeygenMessages(req.SessionId, outgoing, 0)
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

	// Сохраняем share в базу данных если еще не сохранен
	if err := s.saveKeygenResult(ctx, req.SessionId, session, result); err != nil {
		return &pb.GetKeygenResultResponse{
			Completed:    true,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save share: %v", err),
		}, nil
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

// signOutgoingKeygenMessages подписывает исходящие keygen сообщения
func (s *MPCNodeServer) signOutgoingKeygenMessages(sessionID string, outgoing []tss.OutgoingMessage, round int32) []*pb.KeygenMessage {
	pbMessages := make([]*pb.KeygenMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMsg := &pb.KeygenMessage{
			SessionId:   sessionID,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       round,
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
	return pbMessages
}
