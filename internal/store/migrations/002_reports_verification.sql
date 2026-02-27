CREATE TABLE IF NOT EXISTS reports (
  report_id TEXT PRIMARY KEY,
  reporter_subject TEXT NOT NULL DEFAULT '',
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  report_type TEXT NOT NULL,
  evidence TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reports_target_created ON reports(target_type, target_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reports_target_type_created ON reports(target_type, target_id, report_type, created_at);

CREATE TABLE IF NOT EXISTS verification_state (
  pubkey_hex TEXT PRIMARY KEY,
  badge TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL
);
