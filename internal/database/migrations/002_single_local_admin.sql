-- Replace the retired setup-created users table with one fixed local account.
-- Existing SMS, settings, notifications, device history, and master key files
-- are untouched. Existing sessions are intentionally invalidated.
CREATE TABLE IF NOT EXISTS local_admin (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  username TEXT NOT NULL COLLATE NOCASE CHECK (username = 'admin'),
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_login_at TEXT
);

-- Preserve the prior administrator's hash when a legacy account exists. If a
-- malformed legacy database has more than one row, prefer its admin-named row.
INSERT OR IGNORE INTO local_admin(id, username, password_hash, created_at, updated_at, last_login_at)
SELECT 1, 'admin', password_hash, created_at, updated_at, NULL
FROM users
ORDER BY CASE WHEN username COLLATE NOCASE = 'admin' THEN 0 ELSE 1 END, id
LIMIT 1;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  admin_id INTEGER NOT NULL REFERENCES local_admin(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
