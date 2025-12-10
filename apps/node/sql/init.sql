-- MPC HSM Node Database Schema
-- This script initializes the database for MPC key share storage

-- Create shares table
CREATE TABLE IF NOT EXISTS shares (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    party_id VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,
    address VARCHAR(255) NOT NULL,
    share_data JSONB NOT NULL,
    threshold INTEGER NOT NULL,
    total_parties INTEGER NOT NULL,
    party_index INTEGER NOT NULL,
    curve VARCHAR(50) NOT NULL DEFAULT 'secp256k1',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(session_id, party_id)
);

-- Create index on address for faster lookups
CREATE INDEX IF NOT EXISTS idx_shares_address ON shares(address);

-- Create index on session_id
CREATE INDEX IF NOT EXISTS idx_shares_session_id ON shares(session_id);

-- Create index on party_id
CREATE INDEX IF NOT EXISTS idx_shares_party_id ON shares(party_id);

-- Create index on created_at for time-based queries
CREATE INDEX IF NOT EXISTS idx_shares_created_at ON shares(created_at);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at on row update
DROP TRIGGER IF EXISTS update_shares_updated_at ON shares;
CREATE TRIGGER update_shares_updated_at
    BEFORE UPDATE ON shares
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert sample comment
COMMENT ON TABLE shares IS 'Stores MPC key shares for each party in a threshold signature scheme';
COMMENT ON COLUMN shares.session_id IS 'Unique identifier for the keygen session';
COMMENT ON COLUMN shares.party_id IS 'Identifier for the party that owns this share';
COMMENT ON COLUMN shares.public_key IS 'Hex-encoded public key for the generated wallet';
COMMENT ON COLUMN shares.address IS 'Ethereum address derived from the public key';
COMMENT ON COLUMN shares.share_data IS 'Encrypted key share data in JSON format';
COMMENT ON COLUMN shares.threshold IS 'Minimum number of parties required for signing';
COMMENT ON COLUMN shares.total_parties IS 'Total number of parties in the keygen';
COMMENT ON COLUMN shares.party_index IS 'Index of this party in the TSS protocol';
COMMENT ON COLUMN shares.curve IS 'Elliptic curve used (default: secp256k1)';
