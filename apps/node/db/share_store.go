package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ShareStore предоставляет методы для хранения и получения MPC долей ключей
type ShareStore struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewShareStore создаёт новый ShareStore
func NewShareStore(pool *pgxpool.Pool) *ShareStore {
	return &ShareStore{
		pool:    pool,
		queries: New(pool),
	}
}

// SaveShareParams параметры для сохранения доли
type SaveShareParams struct {
	SessionID    string
	PartyID      string
	PublicKey    string
	Address      string
	Threshold    int32
	TotalParties int32
	PartyIndex   int32
	Curve        string
	SaveData     *keygen.LocalPartySaveData
}

// SaveShare сохраняет результат генерации ключа в базу данных
func (s *ShareStore) SaveShare(ctx context.Context, params SaveShareParams) (*Share, error) {
	slog.Debug("SaveShare: starting",
		"session_id", params.SessionID,
		"party_id", params.PartyID,
		"address", params.Address,
		"threshold", params.Threshold,
		"total_parties", params.TotalParties,
	)

	// Сериализация LocalPartySaveData в JSON
	shareData, err := json.Marshal(params.SaveData)
	if err != nil {
		slog.Error("SaveShare: failed to marshal share data", "error", err)
		return nil, fmt.Errorf("failed to marshal share data: %w", err)
	}
	slog.Debug("SaveShare: share data serialized", "size_bytes", len(shareData))

	curve := params.Curve
	if curve == "" {
		curve = "secp256k1"
	}

	share, err := s.queries.CreateShare(ctx, CreateShareParams{
		SessionID:    params.SessionID,
		PartyID:      params.PartyID,
		PublicKey:    params.PublicKey,
		Address:      params.Address,
		ShareData:    shareData,
		Threshold:    params.Threshold,
		TotalParties: params.TotalParties,
		PartyIndex:   params.PartyIndex,
		Curve:        curve,
	})
	if err != nil {
		slog.Error("SaveShare: database insert failed",
			"session_id", params.SessionID,
			"party_id", params.PartyID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	slog.Debug("SaveShare: completed successfully",
		"share_id", share.ID,
		"session_id", params.SessionID,
	)
	return &share, nil
}

// LoadShare загружает долю и десериализует LocalPartySaveData
func (s *ShareStore) LoadShare(ctx context.Context, sessionID, partyID string) (*keygen.LocalPartySaveData, error) {
	slog.Debug("LoadShare: starting",
		"session_id", sessionID,
		"party_id", partyID,
	)

	share, err := s.queries.GetShareBySessionAndParty(ctx, GetShareBySessionAndPartyParams{
		SessionID: sessionID,
		PartyID:   partyID,
	})
	if err != nil {
		slog.Error("LoadShare: database query failed",
			"session_id", sessionID,
			"party_id", partyID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	slog.Debug("LoadShare: share found",
		"share_id", share.ID,
		"address", share.Address,
		"data_size", len(share.ShareData),
	)

	var saveData keygen.LocalPartySaveData
	if err := json.Unmarshal(share.ShareData, &saveData); err != nil {
		slog.Error("LoadShare: failed to unmarshal share data", "error", err)
		return nil, fmt.Errorf("failed to unmarshal share data: %w", err)
	}

	slog.Debug("LoadShare: completed successfully",
		"session_id", sessionID,
		"party_id", partyID,
	)
	return &saveData, nil
}

// LoadShareByID загружает долю по её ID
func (s *ShareStore) LoadShareByID(ctx context.Context, id int64) (*Share, *keygen.LocalPartySaveData, error) {
	share, err := s.queries.GetShareByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get share: %w", err)
	}

	var saveData keygen.LocalPartySaveData
	if err := json.Unmarshal(share.ShareData, &saveData); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal share data: %w", err)
	}

	return &share, &saveData, nil
}

// LoadShareByAddress загружает доли по Ethereum адресу
func (s *ShareStore) LoadShareByAddress(ctx context.Context, address string) ([]Share, error) {
	shares, err := s.queries.GetSharesByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares by address: %w", err)
	}
	return shares, nil
}

// LoadShareDataByAddress загружает SaveData первой доли по адресу
func (s *ShareStore) LoadShareDataByAddress(ctx context.Context, address string) (*keygen.LocalPartySaveData, error) {
	shares, err := s.queries.GetSharesByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares by address: %w", err)
	}
	if len(shares) == 0 {
		return nil, fmt.Errorf("no share found for address: %s", address)
	}

	var saveData keygen.LocalPartySaveData
	if err := json.Unmarshal(shares[0].ShareData, &saveData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal share data: %w", err)
	}

	return &saveData, nil
}

// GetSharesByParty возвращает все доли для участника
func (s *ShareStore) GetSharesByParty(ctx context.Context, partyID string) ([]Share, error) {
	return s.queries.GetSharesByPartyID(ctx, partyID)
}

// ShareExists проверяет существование доли
func (s *ShareStore) ShareExists(ctx context.Context, sessionID, partyID string) (bool, error) {
	exists, err := s.queries.ShareExists(ctx, ShareExistsParams{
		SessionID: sessionID,
		PartyID:   partyID,
	})
	if err != nil {
		return false, err
	}
	return exists, nil
}

// DeleteShare удаляет долю по ID
func (s *ShareStore) DeleteShare(ctx context.Context, id int64) error {
	return s.queries.DeleteShare(ctx, id)
}

// ListShares возвращает постраничный список долей
func (s *ShareStore) ListShares(ctx context.Context, limit, offset int32) ([]Share, error) {
	return s.queries.ListShares(ctx, ListSharesParams{
		Limit:  limit,
		Offset: offset,
	})
}

// Close закрывает пул соединений с базой данных
func (s *ShareStore) Close() {
	s.pool.Close()
}
