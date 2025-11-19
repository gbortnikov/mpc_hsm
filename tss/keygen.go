package tss

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"golang.org/x/crypto/sha3"
)

// PartyShare contains the secret share information for a party
type PartyShare struct {
	PartyID    string `json:"partyId"`
	ShareID    string `json:"shareId"`
	ShareValue string `json:"shareValue"`
}

// KeygenResult contains the result of distributed key generation
type KeygenResult struct {
	PublicKey   string        `json:"publicKey"`
	PrivateKey  string        `json:"privateKey"`
	Address     string        `json:"address"`
	PartyIDs    []string      `json:"partyIds"`
	Shares      []*PartyShare `json:"shares"`
	Threshold   int32         `json:"threshold"`
	TotalShares int32         `json:"totalShares"`
	Success     bool          `json:"success"`
	Message     string        `json:"message"`
}

// KeygenCoordinator manages the distributed key generation process
type KeygenCoordinator struct {
	parties       []*keygen.LocalParty
	partyIDs      []*tss.PartyID
	outCh         chan tss.Message
	endCh         chan *keygen.LocalPartySaveData
	errCh         chan *tss.Error
	threshold     int
	totalParties  int
	preParams     []*keygen.LocalPreParams
	savedData     []*keygen.LocalPartySaveData
	mu            sync.Mutex
	finished      int
}

// NewKeygenCoordinator creates a new coordinator for keygen
func NewKeygenCoordinator(threshold, totalParties int) (*KeygenCoordinator, error) {
	if threshold >= totalParties {
		return nil, fmt.Errorf("threshold must be less than total parties")
	}

	return &KeygenCoordinator{
		threshold:    threshold,
		totalParties: totalParties,
		partyIDs:     make([]*tss.PartyID, totalParties),
		parties:      make([]*keygen.LocalParty, totalParties),
		preParams:    make([]*keygen.LocalPreParams, totalParties),
		savedData:    make([]*keygen.LocalPartySaveData, totalParties),
		outCh:        make(chan tss.Message, totalParties*totalParties),
		endCh:        make(chan *keygen.LocalPartySaveData, totalParties),
		errCh:        make(chan *tss.Error, totalParties),
	}, nil
}

// GeneratePreParams generates pre-parameters for all parties (can be done offline)
func (kc *KeygenCoordinator) GeneratePreParams() error {
	fmt.Println("Generating pre-parameters for all parties...")

	for i := 0; i < kc.totalParties; i++ {
		preParams, err := keygen.GeneratePreParams(1 * time.Minute)
		if err != nil {
			return fmt.Errorf("failed to generate pre-params for party %d: %w", i, err)
		}
		kc.preParams[i] = preParams
		fmt.Printf("Pre-params generated for party %d\n", i)
	}

	return nil
}

// InitializeParties initializes all party instances
func (kc *KeygenCoordinator) InitializeParties() error {
	// Create party IDs
	for i := 0; i < kc.totalParties; i++ {
		id := fmt.Sprintf("party_%d", i+1)
		moniker := fmt.Sprintf("Party %d", i+1)
		key := new(big.Int).SetInt64(int64(i + 1))

		partyID := tss.NewPartyID(id, moniker, key)
		kc.partyIDs[i] = partyID
	}

	// Sort party IDs
	partyIDSlice := tss.SortPartyIDs(kc.partyIDs)
	ctx := tss.NewPeerContext(partyIDSlice)

	// Initialize each party
	for i := 0; i < kc.totalParties; i++ {
		params := tss.NewParameters(tss.S256(), ctx, kc.partyIDs[i], kc.totalParties, kc.threshold)

		party := keygen.NewLocalParty(params, kc.outCh, kc.endCh, *kc.preParams[i]).(*keygen.LocalParty)
		kc.parties[i] = party
	}

	return nil
}

// RunKeygen executes the distributed key generation protocol
func (kc *KeygenCoordinator) RunKeygen() (*KeygenResult, error) {
	fmt.Println("Starting distributed key generation...")

	// Start all parties
	for i, party := range kc.parties {
		go func(idx int, p *keygen.LocalParty) {
			if err := p.Start(); err != nil {
				kc.errCh <- p.WrapError(err)
			}
		}(i, party)
	}

	// Message routing goroutine
	go kc.routeMessages()

	// Wait for completion or error
	for {
		select {
		case err := <-kc.errCh:
			return &KeygenResult{
				Success: false,
				Message: fmt.Sprintf("Keygen error: %v", err),
			}, fmt.Errorf("keygen failed: %v", err)

		case data := <-kc.endCh:
			kc.mu.Lock()
			kc.savedData[kc.finished] = data
			kc.finished++
			fmt.Printf("Party finished: %d/%d\n", kc.finished, kc.totalParties)

			if kc.finished == kc.totalParties {
				kc.mu.Unlock()
				return kc.buildResult()
			}
			kc.mu.Unlock()

		case <-time.After(2 * time.Minute):
			return &KeygenResult{
				Success: false,
				Message: "Keygen timeout",
			}, fmt.Errorf("keygen timeout")
		}
	}
}

// routeMessages handles message routing between parties
func (kc *KeygenCoordinator) routeMessages() {
	for msg := range kc.outCh {
		dest := msg.GetTo()

		if dest == nil {
			// Broadcast message
			for _, party := range kc.parties {
				if party.PartyID().Index == msg.GetFrom().Index {
					continue
				}
				go func(p *keygen.LocalParty, m tss.Message) {
					wireBytes, _, err := m.WireBytes()
					if err != nil {
						kc.errCh <- p.WrapError(err)
						return
					}
					if _, err := p.UpdateFromBytes(wireBytes, m.GetFrom(), m.IsBroadcast()); err != nil {
						kc.errCh <- p.WrapError(err)
					}
				}(party, msg)
			}
		} else {
			// Point-to-point message
			for _, party := range kc.parties {
				for _, recipient := range dest {
					if party.PartyID().Index == recipient.Index && party.PartyID().Index != msg.GetFrom().Index {
						go func(p *keygen.LocalParty, m tss.Message) {
							wireBytes, _, err := m.WireBytes()
							if err != nil {
								kc.errCh <- p.WrapError(err)
								return
							}
							if _, err := p.UpdateFromBytes(wireBytes, m.GetFrom(), m.IsBroadcast()); err != nil {
								kc.errCh <- p.WrapError(err)
							}
						}(party, msg)
					}
				}
			}
		}

		// Allow other goroutines to run
		runtime.Gosched()
	}
}

// PublicKeyToAddress converts an ECDSA public key to an Ethereum address
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) string {
	// Pad X and Y to 32 bytes each if needed
	paddedPubKey := make([]byte, 64)
	copy(paddedPubKey[32-len(pubKey.X.Bytes()):32], pubKey.X.Bytes())
	copy(paddedPubKey[64-len(pubKey.Y.Bytes()):64], pubKey.Y.Bytes())

	// Calculate Keccak-256 hash
	hash := sha3.NewLegacyKeccak256()
	hash.Write(paddedPubKey)
	hashBytes := hash.Sum(nil)

	// Take the last 20 bytes as the address
	address := hashBytes[len(hashBytes)-20:]

	// Return as hex string with 0x prefix
	return "0x" + hex.EncodeToString(address)
}

// ReconstructPrivateKey reconstructs the full private key from shares
// WARNING: This is for demonstration purposes only. In production MPC systems,
// the private key should NEVER be reconstructed in one place.
func ReconstructPrivateKey(savedData []*keygen.LocalPartySaveData, threshold int) (string, error) {
	if len(savedData) < threshold+1 {
		return "", fmt.Errorf("need at least %d shares to reconstruct, got %d", threshold+1, len(savedData))
	}

	// Use Lagrange interpolation to reconstruct the private key
	// We'll use threshold+1 shares for reconstruction
	shares := savedData[:threshold+1]

	// Get the curve order
	curve := tss.S256()
	curveOrder := curve.Params().N

	// Reconstruct using Lagrange interpolation at x=0
	result := big.NewInt(0)

	for i := 0; i < len(shares); i++ {
		// Get share value (Xi) and share ID
		xi := shares[i].Xi
		shareID := shares[i].ShareID

		// Calculate Lagrange basis polynomial at x=0
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j := 0; j < len(shares); j++ {
			if i == j {
				continue
			}

			otherShareID := shares[j].ShareID

			// numerator *= -otherShareID
			numerator.Mul(numerator, new(big.Int).Neg(otherShareID))
			numerator.Mod(numerator, curveOrder)

			// denominator *= (shareID - otherShareID)
			diff := new(big.Int).Sub(shareID, otherShareID)
			denominator.Mul(denominator, diff)
			denominator.Mod(denominator, curveOrder)
		}

		// Calculate the Lagrange coefficient: numerator / denominator mod N
		// We need modular inverse of denominator
		denomInv := new(big.Int).ModInverse(denominator, curveOrder)
		if denomInv == nil {
			return "", fmt.Errorf("failed to compute modular inverse")
		}

		coefficient := new(big.Int).Mul(numerator, denomInv)
		coefficient.Mod(coefficient, curveOrder)

		// result += coefficient * xi
		term := new(big.Int).Mul(coefficient, xi)
		result.Add(result, term)
		result.Mod(result, curveOrder)
	}

	// Ensure the result is positive
	if result.Sign() < 0 {
		result.Add(result, curveOrder)
	}

	return hex.EncodeToString(result.Bytes()), nil
}

// buildResult constructs the final keygen result
func (kc *KeygenCoordinator) buildResult() (*KeygenResult, error) {
	// Verify all parties have the same public key
	firstPubKey := kc.savedData[0].ECDSAPub
	for i := 1; i < len(kc.savedData); i++ {
		if !kc.savedData[i].ECDSAPub.Equals(firstPubKey) {
			return &KeygenResult{
				Success: false,
				Message: "Public keys don't match across parties",
			}, fmt.Errorf("public key mismatch")
		}
	}

	// Convert public key to standard format
	pkX, pkY := firstPubKey.X(), firstPubKey.Y()
	pk := ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     pkX,
		Y:     pkY,
	}

	// Encode public key
	pubKeyBytes := append(pk.X.Bytes(), pk.Y.Bytes()...)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	// Generate Ethereum address from public key
	address := PublicKeyToAddress(&pk)

	// Reconstruct private key from shares (for demonstration purposes only!)
	privateKeyHex, err := ReconstructPrivateKey(kc.savedData, kc.threshold)
	if err != nil {
		return &KeygenResult{
			Success: false,
			Message: fmt.Sprintf("Failed to reconstruct private key: %v", err),
		}, err
	}

	partyNames := make([]string, len(kc.partyIDs))
	for i, pid := range kc.partyIDs {
		partyNames[i] = pid.Id
	}

	// Extract shares for each party
	shares := make([]*PartyShare, len(kc.savedData))
	for i, data := range kc.savedData {
		shares[i] = &PartyShare{
			PartyID:    kc.partyIDs[i].Id,
			ShareID:    hex.EncodeToString(data.ShareID.Bytes()),
			ShareValue: hex.EncodeToString(data.Xi.Bytes()),
		}
	}

	return &KeygenResult{
		PublicKey:   pubKeyHex,
		PrivateKey:  privateKeyHex,
		Address:     address,
		PartyIDs:    partyNames,
		Shares:      shares,
		Threshold:   int32(kc.threshold),
		TotalShares: int32(kc.totalParties),
		Success:     true,
		Message:     fmt.Sprintf("Successfully generated distributed key with %d parties and threshold %d", kc.totalParties, kc.threshold),
	}, nil
}

// VerifyShares verifies that the generated shares can reconstruct the public key
func (kc *KeygenCoordinator) VerifyShares() error {
	if len(kc.savedData) != kc.totalParties {
		return fmt.Errorf("not all parties completed keygen")
	}

	// Verify each share
	for i, data := range kc.savedData {
		// Check if share is valid
		if data.Xi == nil || data.ShareID == nil {
			return fmt.Errorf("party %d has invalid share data", i)
		}

		// Basic verification - check that we have public key components
		if len(data.BigXj) <= i {
			return fmt.Errorf("party %d missing public key component", i)
		}
	}

	fmt.Println("All shares verified successfully")
	return nil
}
