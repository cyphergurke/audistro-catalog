CREATE TABLE IF NOT EXISTS artists (
  artist_id TEXT PRIMARY KEY,
  pubkey_hex TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  bio TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS payees (
  payee_id TEXT PRIMARY KEY,
  artist_id TEXT NOT NULL,
  fap_public_base_url TEXT NOT NULL,
  fap_payee_id TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS assets (
  asset_id TEXT PRIMARY KEY,
  artist_id TEXT NOT NULL,
  payee_id TEXT NOT NULL,
  title TEXT NOT NULL,
  duration_ms INTEGER NOT NULL,
  content_id TEXT NOT NULL,
  hls_master_url TEXT NOT NULL,
  preview_hls_url TEXT NOT NULL DEFAULT '',
  price_msat INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE,
  FOREIGN KEY(payee_id) REFERENCES payees(payee_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS provider_hints (
  hint_id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL,
  transport TEXT NOT NULL,
  base_url TEXT NOT NULL,
  priority INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(asset_id) REFERENCES assets(asset_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS moderation_state (
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  state TEXT NOT NULL,
  reason_code TEXT NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY(target_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_assets_artist_id ON assets(artist_id);
CREATE INDEX IF NOT EXISTS idx_provider_hints_asset_id ON provider_hints(asset_id);
CREATE INDEX IF NOT EXISTS idx_payees_artist_id ON payees(artist_id);
