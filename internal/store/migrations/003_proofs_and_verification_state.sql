CREATE TABLE IF NOT EXISTS artist_proofs (
  proof_id TEXT PRIMARY KEY,
  artist_pubkey_hex TEXT NOT NULL,
  proof_type TEXT NOT NULL,
  proof_value TEXT NOT NULL,
  status TEXT NOT NULL,
  checked_at INTEGER NOT NULL DEFAULT 0,
  details TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE(artist_pubkey_hex, proof_type, proof_value)
);

CREATE INDEX IF NOT EXISTS idx_artist_proofs_status ON artist_proofs(status);
CREATE INDEX IF NOT EXISTS idx_artist_proofs_pubkey ON artist_proofs(artist_pubkey_hex);

ALTER TABLE verification_state ADD COLUMN computed_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE verification_state ADD COLUMN inputs_hash TEXT NOT NULL DEFAULT '';
