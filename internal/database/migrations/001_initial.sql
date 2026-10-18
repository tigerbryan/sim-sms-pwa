CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  control_path TEXT NOT NULL,
  driver TEXT,
  usb_vid TEXT,
  usb_pid TEXT,
  usb_manufacturer TEXT,
  usb_product TEXT,
  network_interface TEXT,
  serial_ports TEXT,
  sysfs_path TEXT,
  status TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sms_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL,
  storage_index INTEGER,
  sender TEXT,
  recipient TEXT,
  timestamp TEXT,
  received_at TEXT NOT NULL,
  encoding TEXT NOT NULL,
  body TEXT NOT NULL,
  is_multipart INTEGER NOT NULL DEFAULT 0,
  reference_number INTEGER,
  parts_total INTEGER NOT NULL DEFAULT 1,
  parts_received INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'unread',
  raw_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(device_id, raw_hash)
);
CREATE INDEX IF NOT EXISTS idx_sms_messages_received_at ON sms_messages(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_sms_messages_status ON sms_messages(status);

CREATE TABLE IF NOT EXISTS sms_parts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL,
  sender TEXT NOT NULL,
  concat_reference INTEGER NOT NULL,
  concat_reference_bits INTEGER NOT NULL,
  total_parts INTEGER NOT NULL,
  sequence_number INTEGER NOT NULL,
  received_at TEXT NOT NULL,
  body TEXT NOT NULL,
  raw_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(device_id, sender, concat_reference, concat_reference_bits, total_parts, sequence_number)
);

CREATE TABLE IF NOT EXISTS notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  dedup_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  notification_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
  notifier TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT,
  delivered_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,
  payload TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  action TEXT NOT NULL,
  actor TEXT,
  remote_addr TEXT,
  details TEXT,
  created_at TEXT NOT NULL
);
