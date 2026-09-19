PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS credential_meta (
  server_name TEXT PRIMARY KEY,
  masked_preview TEXT NOT NULL DEFAULT '',
  fingerprint TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY,
  created_at TEXT NOT NULL,
  action TEXT NOT NULL,
  server_name TEXT NOT NULL DEFAULT '',
  success INTEGER NOT NULL,
  detail TEXT NOT NULL DEFAULT ''
);
