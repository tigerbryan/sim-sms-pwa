package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationsSQLiteAndSMSUniqueness(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var migrationCount int
	if err := store.DB().QueryRow("SELECT COUNT(1) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 5 {
		t.Fatalf("migration count = %d", migrationCount)
	}
	now := time.Now().UTC()
	message := SMSMessage{
		DeviceID: "mock", Sender: "+15550000001", Body: "test", Timestamp: now,
		ReceivedAt: now, Encoding: "GSM7", RawHash: "stable-hash",
	}
	id, saved, err := store.InsertSMS(context.Background(), message)
	if err != nil || !saved || id < 1 {
		t.Fatalf("initial insert = id:%d saved:%v err:%v", id, saved, err)
	}
	if _, saved, err := store.InsertSMS(context.Background(), message); err != nil || saved {
		t.Fatalf("duplicate insert = saved:%v err:%v", saved, err)
	}
	last, err := store.LatestSMSAt(context.Background())
	if err != nil || last == nil || !last.Equal(now) {
		t.Fatalf("latest timestamp = %v %v", last, err)
	}
}

func TestSingleAdminMigrationPreservesLegacyAdminAndSMS(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := migrationFiles.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)", now()); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO users(username, password_hash, created_at, updated_at) VALUES('admin', 'legacy-argon2id-hash', ?, ?)", now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO sms_messages(device_id, received_at, encoding, body, is_multipart, parts_total, parts_received, status, raw_hash, created_at) VALUES('mock', ?, 'GSM7', 'preserved', 0, 1, 1, 'unread', 'legacy-sms', ?)", now(), now()); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admin, err := store.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Username != "admin" || admin.PasswordHash != "legacy-argon2id-hash" {
		t.Fatalf("legacy administrator was not preserved: %#v", admin)
	}
	if total, err := store.UnreadSMSCount(ctx); err != nil || total != 1 {
		t.Fatalf("legacy SMS was not preserved: total=%d err=%v", total, err)
	}
	var n int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM users").Scan(&n); err == nil {
		t.Fatal("legacy users table still exists")
	}
}
