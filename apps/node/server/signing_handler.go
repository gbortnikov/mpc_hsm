package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mpc_hsm/node/db"
	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"github.com/mpc_hsm/node/tss"
)

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
	pbMessages := s.signOutgoingSigningMessages(req.SessionId, outgoing, req.Round)

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
		outgoing := session.GetOutgoingMessages()
		if len(outgoing) > 0 {
			slog.Info("GetSigningResult: returning outgoing messages",
				"session_id", req.SessionId,
				"party_id", s.partyID,
				"message_count", len(outgoing),
			)
		}
		pbMessages := s.signOutgoingSigningMessages(req.SessionId, outgoing, 0)
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

// signOutgoingSigningMessages подписывает исходящие signing сообщения
func (s *MPCNodeServer) signOutgoingSigningMessages(sessionID string, outgoing []tss.OutgoingMessage, round int32) []*pb.SigningMessage {
	pbMessages := make([]*pb.SigningMessage, len(outgoing))
	for i, msg := range outgoing {
		pbMsg := &pb.SigningMessage{
			SessionId:   sessionID,
			FromParty:   msg.FromParty,
			ToParties:   msg.ToParties,
			Round:       round,
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
	return pbMessages
}

// saveKeygenResult сохраняет результат keygen в базу данных
func (s *MPCNodeServer) saveKeygenResult(ctx context.Context, sessionID string, session *tss.KeygenSession, result *tss.KeygenResult) error {
	// Проверяем, не сохранён ли share уже
	if session.IsSaved() {
		return nil
	}

	savedData := session.GetSavedData()

	// Сохраняем share в базу данных
	_, err := s.shareStore.SaveShare(ctx, db.SaveShareParams{
		SessionID:    sessionID,
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
		slog.Error("saveKeygenResult: failed to save share to database",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	session.MarkSaved()
	slog.Info("saveKeygenResult: share saved to database",
		"session_id", sessionID,
		"address", result.Address,
	)
	return nil
}
