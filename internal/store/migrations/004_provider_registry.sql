CREATE TABLE IF NOT EXISTS providers (
  provider_id TEXT PRIMARY KEY,
  public_key TEXT NOT NULL,
  transport TEXT NOT NULL DEFAULT 'https',
  base_url TEXT NOT NULL,
  region TEXT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_assets (
  provider_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  transport TEXT NOT NULL,
  base_url TEXT NOT NULL,
  priority INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  nonce TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (provider_id, asset_id),
  FOREIGN KEY(provider_id) REFERENCES providers(provider_id) ON DELETE CASCADE,
  FOREIGN KEY(asset_id) REFERENCES assets(asset_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_provider_assets_asset_expires ON provider_assets(asset_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_provider_assets_expires ON provider_assets(expires_at);
