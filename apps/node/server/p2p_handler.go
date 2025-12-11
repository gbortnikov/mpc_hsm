package server

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/security"
	"google.golang.org/protobuf/proto"
)

// Ping обрабатывает запрос проверки связи
func (s *MPCNodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	slog.Debug("Ping received", "from_party", req.FromPartyId)
	return &pb.PingResponse{
		PartyId:   s.partyID,
		Timestamp: req.Timestamp,
		Healthy:   true,
	}, nil
}

// ExchangePeerInfo обменивается информацией об узлах
func (s *MPCNodeServer) ExchangePeerInfo(ctx context.Context, req *pb.PeerInfoRequest) (*pb.PeerInfoResponse, error) {
	slog.Debug("ExchangePeerInfo", "from_party", req.RequesterPartyId)

	var signingPublicKey string
	if s.identity != nil {
		signingPublicKey = s.identity.PublicKeyBase64()
	}

	return &pb.PeerInfoResponse{
		PartyId:          s.partyID,
		SigningPublicKey: signingPublicKey,
		KnownPeers:       nil,
	}, nil
}

// ProcessSignedKeygenMessage обрабатывает подписанное сообщение генерации ключей
func (s *MPCNodeServer) ProcessSignedKeygenMessage(ctx context.Context, envelope *pb.SignedEnvelope) (*pb.SignedKeygenResponse, error) {
	slog.Debug("ProcessSignedKeygenMessage",
		"from_party", envelope.SignerPartyId,
		"payload_len", len(envelope.Payload),
	)

	// Верификация подписи если verifier настроен
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

	// Десериализация KeygenMessage из payload
	var keygenMsg pb.KeygenMessage
	if err := proto.Unmarshal(envelope.Payload, &keygenMsg); err != nil {
		slog.Warn("Failed to unmarshal keygen message", "error", err)
		return &pb.SignedKeygenResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to unmarshal: %v", err),
		}, nil
	}

	// Обработка сообщения
	resp, err := s.ProcessKeygenMessage(ctx, &keygenMsg)
	if err != nil {
		return &pb.SignedKeygenResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Подписание исходящих сообщений если signer настроен
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

// ProcessSignedSigningMessage обрабатывает подписанное сообщение подписания
func (s *MPCNodeServer) ProcessSignedSigningMessage(ctx context.Context, envelope *pb.SignedEnvelope) (*pb.SignedSigningResponse, error) {
	slog.Debug("ProcessSignedSigningMessage",
		"from_party", envelope.SignerPartyId,
		"payload_len", len(envelope.Payload),
	)

	// Верификация подписи
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

	// Десериализация SigningMessage
	var signingMsg pb.SigningMessage
	if err := proto.Unmarshal(envelope.Payload, &signingMsg); err != nil {
		return &pb.SignedSigningResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to unmarshal: %v", err),
		}, nil
	}

	// Обработка
	resp, err := s.ProcessSigningMessage(ctx, &signingMsg)
	if err != nil {
		return &pb.SignedSigningResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Подписание исходящих
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
