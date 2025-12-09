-- Migration: Create shares table for storing MPC key shares
-- This table stores the LocalPartySaveData from TSS keygen

CREATE TABLE IF NOT EXISTS shares (
    id              BIGSERIAL PRIMARY KEY,

    -- Identifiers
    session_id      TEXT NOT NULL,
    party_id        TEXT NOT NULL,

    -- Key identification
    public_key      TEXT NOT NULL,           -- Hex encoded public key
    address         TEXT NOT NULL,           -- Ethereum address derived from public key

    -- Share data (encrypted JSON blob of LocalPartySaveData)
    share_data      BYTEA NOT NULL,          -- Encrypted share data

    -- Metadata
    threshold       INTEGER NOT NULL,
    total_parties   INTEGER NOT NULL,
    party_index     INTEGER NOT NULL,
    curve           TEXT NOT NULL DEFAULT 'secp256k1',

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT shares_party_session_unique UNIQUE (session_id, party_id),
    CONSTRAINT shares_threshold_check CHECK (threshold > 0 AND threshold < total_parties),
    CONSTRAINT shares_party_index_check CHECK (party_index >= 0 AND party_index < total_parties)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_shares_party_id ON shares(party_id);
CREATE INDEX IF NOT EXISTS idx_shares_public_key ON shares(public_key);
CREATE INDEX IF NOT EXISTS idx_shares_address ON shares(address);
CREATE INDEX IF NOT EXISTS idx_shares_created_at ON shares(created_at);

-- Trigger for updating updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_shares_updated_at
    BEFORE UPDATE ON shares
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
