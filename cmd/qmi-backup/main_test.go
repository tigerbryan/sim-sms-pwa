package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotRejectsAliasesWithoutChangingSource(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	source := filepath.Join(dir, "source.db")
	const original = "source must survive"
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(dir, "hardlink.db")
	if err := os.Link(source, hardlink); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink.db")
	if err := os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{source, "source.db", hardlink, symlink} {
		if err := createSnapshot(source, target); err == nil {
			t.Fatalf("accepted alias %q", target)
		}
		contents, err := os.ReadFile(source)
		if err != nil || string(contents) != original {
			t.Fatalf("source changed for alias %q: %v", target, err)
		}
	}
}

func TestFailedSnapshotPreservesPreviousBackup(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "broken.db"), filepath.Join(dir, "backup.db")
	if err := os.WriteFile(source, []byte("not a SQLite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("previous backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := createSnapshot(source, target); err == nil {
		t.Fatal("invalid SQLite source accepted")
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "previous backup" {
		t.Fatalf("failed attempt destroyed previous backup: %v", err)
	}
	staged, err := filepath.Glob(filepath.Join(dir, ".qmi-backup-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("left staged private data after failure: %v %v", staged, err)
	}
}

func TestSnapshotIncludesWALAndReplacesBackupPrivately(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "live.db"), filepath.Join(dir, "backup.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=WAL; CREATE TABLE example(value TEXT); INSERT INTO example VALUES('test fixture')"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("previous backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := createSnapshot(source, target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup does not have private permissions: %v %v", info, err)
	}
	snapshot, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	var value, integrity string
	if err := snapshot.QueryRow("SELECT value FROM example").Scan(&value); err != nil || value != "test fixture" {
		t.Fatalf("WAL data missing from snapshot: %q %v", value, err)
	}
	if err := snapshot.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("snapshot integrity failed: %q %v", integrity, err)
	}
}

func TestMissingSourceDoesNotCreateAnEmptyDatabase(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "missing.db")
	if err := createSnapshot(source, filepath.Join(dir, "snapshot.db")); err == nil {
		t.Fatal("accepted nonexistent source")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("missing source was created: %v", err)
	}
}
