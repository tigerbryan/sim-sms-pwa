// qmi-backup creates an SQLite-consistent snapshot using VACUUM INTO.
package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: qmi-backup /path/to/qmi-web.db /path/to/snapshot.db")
		os.Exit(2)
	}
	if err := createSnapshot(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "qmi-backup: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("qmi-backup: snapshot created")
}

func createSnapshot(source, target string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular database file")
	}
	if source == target {
		return fmt.Errorf("source and target must differ")
	}
	if targetInfo, err := os.Stat(target); err == nil {
		// Compare file identity, not only spelling: hard links and symlinked
		// paths can point back to the live database.
		if os.SameFile(sourceInfo, targetInfo) {
			return fmt.Errorf("source and target must differ")
		}
		if !targetInfo.Mode().IsRegular() {
			return fmt.Errorf("target must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	// Keep the previous snapshot until its replacement is complete. The
	// private temporary file also prevents a permissive umask exposing SMS.
	staged, err := os.CreateTemp(filepath.Dir(target), ".qmi-backup-*.db")
	if err != nil {
		return fmt.Errorf("stage snapshot: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)
	if err := staged.Close(); err != nil {
		return err
	}
	dsn := (&url.URL{Scheme: "file", Path: source, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.Exec("VACUUM INTO ?", stagedPath); err != nil {
		return fmt.Errorf("consistent snapshot: %w", err)
	}
	file, err := os.Open(stagedPath)
	if err != nil {
		return fmt.Errorf("open staged snapshot: %w", err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return fmt.Errorf("sync staged snapshot: %w", syncErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(stagedPath, target); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}
