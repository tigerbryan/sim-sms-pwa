package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestMultipartMigrationPreservesCompletedAndPartialLegacyGroups(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-parts.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"001_initial.sql", "002_single_local_admin.sql", "003_sms_source.sql", "004_web_push.sql"} {
		raw, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", i+1, now()); err != nil {
			t.Fatal(err)
		}
	}
	legacy := &Store{db: db}
	stamp := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	for seq, body := range []string{"hello ", "world"} {
		if _, err := legacy.AddSMSPart(ctx, "fixture", "sender", 7, 8, 2, seq+1, stamp, body, fmt.Sprintf("old%d", seq)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := legacy.AddSMSPart(ctx, "fixture", "sender", 8, 8, 2, 1, stamp, "pending", "partial"); err != nil {
		t.Fatal(err)
	}
	ref := 7
	if _, _, err := legacy.InsertSMS(ctx, SMSMessage{DeviceID: "fixture", Sender: "sender", Body: "hello world", Timestamp: stamp, IsMultipart: true, ReferenceNumber: &ref, PartsTotal: 2, PartsReceived: 2, RawHash: "legacy-identity", Status: "read"}); err != nil {
		t.Fatal(err)
	}
	// The legacy inbox lacks reference bit-width. Body/time alone must not
	// claim both groups when only one was previously assembled.
	for _, bits := range []int{8, 16} {
		for seq, body := range []string{"dual ", "width"} {
			if _, err := legacy.AddSMSPart(ctx, "fixture", "sender", 9, bits, 2, seq+1, stamp, body, fmt.Sprintf("dual-%d-%d", bits, seq)); err != nil {
				t.Fatal(err)
			}
		}
	}
	ref = 9
	if _, _, err := legacy.InsertSMS(ctx, SMSMessage{DeviceID: "fixture", Sender: "sender", Body: "dual width", Timestamp: stamp, IsMultipart: true, ReferenceNumber: &ref, PartsTotal: 2, PartsReceived: 2, RawHash: "legacy-dual-identity", Status: "read"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var total, assembled int
	if err := store.DB().QueryRow("SELECT count(*),sum(assembled) FROM sms_parts").Scan(&total, &assembled); err != nil || total != 7 || assembled != 2 {
		t.Fatalf("migration changed part ownership/data: total=%d assembled=%d err=%v", total, assembled, err)
	}
	items, total, err := store.ListSMS(ctx, 1, 10, "hello", "")
	if err != nil || total != 1 || items[0].Status != "read" || items[0].Body != "hello world" {
		t.Fatal("legacy inbox changed during migration")
	}
	if added, err := store.AddSMSPart(ctx, "fixture", "sender", 7, 8, 2, 1, stamp.Add(time.Hour), "new", "new-ref-reuse"); err != nil || !added {
		t.Fatalf("completed reference blocked a new message: added=%v err=%v", added, err)
	}
	if added, err := store.AddSMSPart(ctx, "fixture", "sender", 7, 8, 2, 1, stamp, "hello ", "old0"); err != nil || added {
		t.Fatal("legacy PDU replay was not deduplicated")
	}
	parts, err := store.Parts(ctx, "fixture", "sender", 8, 8, 2, stamp)
	if err != nil || len(parts) != 1 || parts[0].Body != "pending" {
		t.Fatal("legacy partial message cannot resume")
	}
	if _, err := store.AddSMSPart(ctx, "fixture", "sender", 8, 8, 2, 2, stamp.Add(time.Second), " complete", "partial-second"); err != nil {
		t.Fatal(err)
	}
	parts, err = store.Parts(ctx, "fixture", "sender", 8, 8, 2, stamp)
	if err != nil {
		t.Fatal(err)
	}
	ref = 8
	if _, saved, err := store.CompleteMultipartSMS(ctx, SMSMessage{DeviceID: "fixture", Sender: "sender", Body: "pending complete", Timestamp: stamp, IsMultipart: true, ReferenceNumber: &ref, PartsTotal: 2, RawHash: "new-completed-partial"}, parts, "old-partial-identity"); err != nil || !saved {
		t.Fatalf("legacy partial failed after migration: saved=%v err=%v", saved, err)
	}
}

func TestMultipartCompletionRollsBackBothWritesOnFailure(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "atomic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	stamp := time.Now().UTC()
	for seq := 1; seq <= 2; seq++ {
		if _, err := store.AddSMSPart(ctx, "fixture", "sender", 1, 8, 2, seq, stamp, "part", fmt.Sprintf("raw%d", seq)); err != nil {
			t.Fatal(err)
		}
	}
	parts, err := store.Parts(ctx, "fixture", "sender", 1, 8, 2, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec("CREATE TRIGGER reject_part_claim BEFORE UPDATE OF assembled ON sms_parts BEGIN SELECT RAISE(ABORT,'fixture claim failure'); END"); err != nil {
		t.Fatal(err)
	}
	ref := 1
	m := SMSMessage{DeviceID: "fixture", Sender: "sender", Body: "partpart", Timestamp: stamp, IsMultipart: true, ReferenceNumber: &ref, PartsTotal: 2, RawHash: "new-identity"}
	if _, _, err := store.CompleteMultipartSMS(ctx, m, parts, "legacy-identity"); err == nil {
		t.Fatal("expected claim failure")
	}
	_, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 0 {
		t.Fatal("inbox write escaped failed completion transaction")
	}
	var pending int
	if err := store.DB().QueryRow("SELECT count(*) FROM sms_parts WHERE assembled=0").Scan(&pending); err != nil || pending != 2 {
		t.Fatal("parts were lost on rollback")
	}
	if _, err := store.DB().Exec("DROP TRIGGER reject_part_claim"); err != nil {
		t.Fatal(err)
	}
	if _, saved, err := store.CompleteMultipartSMS(ctx, m, parts, "legacy-identity"); err != nil || !saved {
		t.Fatalf("retry after rollback failed: saved=%v err=%v", saved, err)
	}
	if _, saved, err := store.CompleteMultipartSMS(ctx, m, parts, "legacy-identity"); err != nil || saved {
		t.Fatal("completed parts were claimed twice")
	}
}
