// Package database owns SQLite access and migration application.
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db *sql.DB
}

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
}

type Session struct {
	ID        string
	AdminID   int64
	Username  string
	CSRFToken string
	ExpiresAt time.Time
}

type SMSMessage struct {
	ID              int64     `json:"id"`
	DeviceID        string    `json:"device_id"`
	StorageIndex    *int64    `json:"storage_index,omitempty"`
	Sender          string    `json:"sender"`
	Recipient       string    `json:"recipient,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	ReceivedAt      time.Time `json:"received_at"`
	Encoding        string    `json:"encoding"`
	Body            string    `json:"body"`
	IsMultipart     bool      `json:"is_multipart"`
	ReferenceNumber *int      `json:"reference_number,omitempty"`
	PartsTotal      int       `json:"parts_total"`
	PartsReceived   int       `json:"parts_received"`
	Status          string    `json:"status"`
	Source          string    `json:"source"`
	RawHash         string    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
}

type Notification struct {
	ID            int64     `json:"id"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	Body          string    `json:"-"`
	DedupKey      string    `json:"-"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file::memory:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path
	if !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid migration name %q", entry.Name())
		}
		var exists int
		err = s.db.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", version).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("migration status %s: %w", entry.Name(), err)
		}
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(content)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)", version, now())
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (s *Store) Healthy(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM local_admin WHERE id=1").Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// CreateDefaultAdmin inserts the fixed local account only if no account exists.
// The schema enforces both its single row and its fixed username.
func (s *Store) CreateDefaultAdmin(ctx context.Context, passwordHash string) (bool, error) {
	if passwordHash == "" {
		return false, errors.New("password hash is required")
	}
	result, err := s.db.ExecContext(ctx, "INSERT INTO local_admin(id, username, password_hash, created_at, updated_at) VALUES(1, 'admin', ?, ?, ?) ON CONFLICT(id) DO NOTHING", passwordHash, now(), now())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) Admin(ctx context.Context) (Admin, error) {
	var admin Admin
	err := s.db.QueryRowContext(ctx, "SELECT id, username, password_hash FROM local_admin WHERE id=1").Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	return admin, err
}

// UpdateAdminPassword atomically replaces the hash and revokes every session.
func (s *Store) UpdateAdminPassword(ctx context.Context, passwordHash string) error {
	if passwordHash == "" {
		return errors.New("password hash is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE local_admin SET password_hash=?, updated_at=? WHERE id=1", passwordHash, now())
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n != 1 {
		_ = tx.Rollback()
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions"); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) RecordAdminLogin(ctx context.Context) error {
	result, err := s.db.ExecContext(ctx, "UPDATE local_admin SET last_login_at=? WHERE id=1", now())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, id string, admin Admin, csrf string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions(id, admin_id, csrf_token, expires_at, created_at) VALUES(?, ?, ?, ?, ?)", id, admin.ID, csrf, expiresAt.UTC().Format(time.RFC3339Nano), now())
	return err
}

func (s *Store) Session(ctx context.Context, id string) (Session, error) {
	var result Session
	var expiry string
	err := s.db.QueryRowContext(ctx, "SELECT s.id, s.admin_id, a.username, s.csrf_token, s.expires_at FROM sessions s JOIN local_admin a ON a.id=s.admin_id WHERE s.id=?", id).
		Scan(&result.ID, &result.AdminID, &result.Username, &result.CSRFToken, &expiry)
	if err != nil {
		return result, err
	}
	result.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiry)
	if err == nil && !result.ExpiresAt.After(time.Now().UTC()) {
		_ = s.DeleteSession(ctx, id)
		return result, sql.ErrNoRows
	}
	return result, err
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id=?", id)
	return err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings(key,value,updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`,
		key, value, now())
	return err
}

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v, err
}

func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key,value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		values[k] = v
	}
	return values, rows.Err()
}

func (s *Store) InsertSMS(ctx context.Context, m SMSMessage) (int64, bool, error) {
	return insertSMS(ctx, s.db, m)
}

type smsExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertSMS(ctx context.Context, db smsExecer, m SMSMessage) (int64, bool, error) {
	if m.ReceivedAt.IsZero() {
		m.ReceivedAt = time.Now().UTC()
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = m.ReceivedAt
	}
	if m.PartsTotal < 1 {
		m.PartsTotal = 1
	}
	if m.PartsReceived < 1 {
		m.PartsReceived = 1
	}
	if m.Status == "" {
		m.Status = "unread"
	}
	if m.Encoding == "" {
		m.Encoding = "unknown"
	}
	if m.Source == "" {
		m.Source = "unknown"
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO sms_messages(device_id,storage_index,sender,recipient,timestamp,received_at,encoding,body,is_multipart,reference_number,parts_total,parts_received,status,raw_hash,source,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.DeviceID, m.StorageIndex, m.Sender, m.Recipient, rfc(m.Timestamp), rfc(m.ReceivedAt), m.Encoding, m.Body, boolInt(m.IsMultipart), m.ReferenceNumber, m.PartsTotal, m.PartsReceived, m.Status, m.RawHash, m.Source, now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, false, nil
		}
		return 0, false, err
	}
	id, err := result.LastInsertId()
	return id, true, err
}

func (s *Store) ListSMS(ctx context.Context, page, perPage int, query, status string) ([]SMSMessage, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	where := []string{"1=1"}
	args := []any{}
	if query = strings.TrimSpace(query); query != "" {
		where = append(where, "(sender LIKE ? OR body LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	if status == "read" || status == "unread" {
		where = append(where, "status=?")
		args = append(args, status)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM sms_messages WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,device_id,storage_index,sender,recipient,timestamp,received_at,encoding,body,is_multipart,reference_number,parts_total,parts_received,status,raw_hash,source,created_at
		FROM sms_messages WHERE `+clause+` ORDER BY received_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var result []SMSMessage
	for rows.Next() {
		m, err := scanSMS(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, m)
	}
	return result, total, rows.Err()
}

func (s *Store) SMS(ctx context.Context, id int64) (SMSMessage, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id,device_id,storage_index,sender,recipient,timestamp,received_at,encoding,body,is_multipart,reference_number,parts_total,parts_received,status,raw_hash,source,created_at
		FROM sms_messages WHERE id=?`, id)
	return scanSMS(row)
}

func (s *Store) MarkSMSRead(ctx context.Context, id int64, read bool) error {
	status := "unread"
	if read {
		status = "read"
	}
	result, err := s.db.ExecContext(ctx, "UPDATE sms_messages SET status=? WHERE id=?", status, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UnreadSMSCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM sms_messages WHERE status='unread'").Scan(&n)
	return n, err
}

func (s *Store) LatestSMSAt(ctx context.Context) (*time.Time, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT received_at FROM sms_messages ORDER BY received_at DESC LIMIT 1").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, fmt.Errorf("parse latest SMS timestamp: %w", err)
	}
	return &value, nil
}

func (s *Store) AddSMSPart(ctx context.Context, deviceID, sender string, ref, refBits, total, sequence int, receivedAt time.Time, body, rawHash string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO sms_parts(device_id,sender,concat_reference,concat_reference_bits,total_parts,sequence_number,received_at,body,raw_hash,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, deviceID, sender, ref, refBits, total, sequence, rfc(receivedAt), body, rawHash, now())
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n == 1, nil
}

type SMSPart struct {
	ID        int64
	Sequence  int
	Timestamp time.Time
	Body      string
	RawHash   string
}

const MultipartAssemblyWindow = 5 * time.Minute

// Parts uses carrier timestamps relative to the incoming part, not the wall
// clock: messages kept in modem storage must still assemble after a long outage.
// Completed PDU identities remain recorded to make repeated scans harmless.
// A narrow window avoids joining complementary remnants of reused references.
// Slow or ambiguous groups are retained for recovery, never guessed or deleted.
func (s *Store) Parts(ctx context.Context, deviceID, sender string, ref, refBits, total int, anchor time.Time) ([]SMSPart, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,sequence_number,received_at,body,raw_hash FROM sms_parts
		WHERE device_id=? AND sender=? AND concat_reference=? AND concat_reference_bits=? AND total_parts=? AND assembled=0 AND received_at >= ? AND received_at <= ?
		ORDER BY sequence_number,id`, deviceID, sender, ref, refBits, total, rfc(anchor.Add(-MultipartAssemblyWindow)), rfc(anchor.Add(MultipartAssemblyWindow)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SMSPart
	for rows.Next() {
		var part SMSPart
		var stamp string
		if err := rows.Scan(&part.ID, &part.Sequence, &stamp, &part.Body, &part.RawHash); err != nil {
			return nil, err
		}
		part.Timestamp, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, err
		}
		out = append(out, part)
	}
	return out, rows.Err()
}

// CompleteMultipartSMS commits the inbox row and part ownership together. A
// restart cannot resurrect completed parts or lose them between the two writes.
// Legacy hashes are recognized only for the same carrier-time range, preserving
// existing inbox messages without hiding a later identical message.
func (s *Store) CompleteMultipartSMS(ctx context.Context, m SMSMessage, parts []SMSPart, legacyHash string) (int64, bool, error) {
	if len(parts) == 0 {
		return 0, false, errors.New("multipart SMS has no parts")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	first, last := parts[0].Timestamp, parts[0].Timestamp
	for _, part := range parts {
		var assembled int
		if err := tx.QueryRowContext(ctx, "SELECT assembled FROM sms_parts WHERE id=? AND device_id=? AND raw_hash=?", part.ID, m.DeviceID, part.RawHash).Scan(&assembled); err != nil {
			return 0, false, err
		}
		if assembled != 0 {
			return 0, false, nil
		}
		if part.Timestamp.Before(first) {
			first = part.Timestamp
		}
		if part.Timestamp.After(last) {
			last = part.Timestamp
		}
	}
	var id int64
	created := false
	err = tx.QueryRowContext(ctx, `SELECT id FROM sms_messages WHERE device_id=? AND (raw_hash=? OR (raw_hash=? AND timestamp>=? AND timestamp<=?)) LIMIT 1`, m.DeviceID, m.RawHash, legacyHash, rfc(first), rfc(last)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id, created, err = insertSMS(ctx, tx, m)
	}
	if err != nil {
		return 0, false, err
	}
	for _, part := range parts {
		if _, err := tx.ExecContext(ctx, "UPDATE sms_parts SET assembled=1 WHERE id=?", part.ID); err != nil {
			return 0, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, created, nil
}

func (s *Store) CreateNotification(ctx context.Context, n Notification) (int64, bool, error) {
	t := time.Now().UTC()
	if n.NextAttemptAt.IsZero() {
		n.NextAttemptAt = t
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO notifications(kind,title,body,dedup_key,status,attempts,next_attempt_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, n.Kind, n.Title, n.Body, n.DedupKey, "pending", 0, rfc(n.NextAttemptAt), rfc(t), rfc(t))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, false, nil
		}
		return 0, false, err
	}
	id, err := result.LastInsertId()
	return id, true, err
}

func (s *Store) DueNotifications(ctx context.Context, limit int) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,kind,title,body,dedup_key,status,attempts,next_attempt_at,created_at,updated_at
		FROM notifications WHERE status IN ('pending','failed') AND next_attempt_at <= ? AND attempts < 3
		ORDER BY id LIMIT ?`, now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		var next, created, updated string
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.DedupKey, &n.Status, &n.Attempts, &next, &created, &updated); err != nil {
			return nil, err
		}
		n.NextAttemptAt, _ = time.Parse(time.RFC3339Nano, next)
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		n.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListNotifications returns delivery metadata for the UI. Notification bodies
// and deduplication keys stay private because they can contain SMS content.
func (s *Store) ListNotifications(ctx context.Context, limit int) ([]Notification, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,kind,title,status,attempts,next_attempt_at,created_at,updated_at
		FROM notifications ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		var next, created, updated string
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Status, &n.Attempts, &next, &created, &updated); err != nil {
			return nil, err
		}
		n.NextAttemptAt, _ = time.Parse(time.RFC3339Nano, next)
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		n.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		items = append(items, n)
	}
	return items, rows.Err()
}

func (s *Store) UpdateNotification(ctx context.Context, id int64, status string, attempts int, next time.Time, notifier, deliveryStatus, deliveryErr string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE notifications SET status=?,attempts=?,next_attempt_at=?,updated_at=? WHERE id=?", status, attempts, rfc(next), now(), id); err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO notification_deliveries(notification_id,notifier,status,error,delivered_at,created_at) VALUES(?,?,?,?,?,?)",
			id, notifier, deliveryStatus, nullable(deliveryErr), nullableTime(status == "success"), now())
	}
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) AddAudit(ctx context.Context, action, actor, remoteAddr, details string) {
	_, _ = s.db.ExecContext(ctx, "INSERT INTO audit_logs(action,actor,remote_addr,details,created_at) VALUES(?,?,?,?,?)", action, actor, remoteAddr, details, now())
}

func (s *Store) AddEvent(ctx context.Context, typ, payload string) {
	_, _ = s.db.ExecContext(ctx, "INSERT INTO events(type,payload,created_at) VALUES(?,?,?)", typ, payload, now())
}

type scanner interface{ Scan(...any) error }

func scanSMS(s scanner) (SMSMessage, error) {
	var m SMSMessage
	var storage sql.NullInt64
	var sender, recipient sql.NullString
	var timestamp, received, created string
	var multipart int
	var reference sql.NullInt64
	err := s.Scan(&m.ID, &m.DeviceID, &storage, &sender, &recipient, &timestamp, &received, &m.Encoding, &m.Body, &multipart, &reference, &m.PartsTotal, &m.PartsReceived, &m.Status, &m.RawHash, &m.Source, &created)
	if err != nil {
		return m, err
	}
	if storage.Valid {
		m.StorageIndex = &storage.Int64
	}
	m.Sender, m.Recipient, m.IsMultipart = sender.String, recipient.String, multipart != 0
	if reference.Valid {
		r := int(reference.Int64)
		m.ReferenceNumber = &r
	}
	m.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
	m.ReceivedAt, _ = time.Parse(time.RFC3339Nano, received)
	m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return m, nil
}

func now() string            { return time.Now().UTC().Format(time.RFC3339Nano) }
func rfc(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullableTime(ok bool) any {
	if !ok {
		return nil
	}
	return now()
}
