CREATE TABLE IF NOT EXISTS ingest_jobs (
  job_id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL,
  artist_id TEXT NOT NULL,
  payee_id TEXT NOT NULL,
  title TEXT NOT NULL,
  price_msat INTEGER NOT NULL,
  source_path TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE,
  FOREIGN KEY(payee_id) REFERENCES payees(payee_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ingest_jobs_status_created ON ingest_jobs(status, created_at);
