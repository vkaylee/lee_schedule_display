package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// backupSQLite creates a consistent snapshot without replacing an existing file.
func backupSQLite(source, destination string) error {
	if source == "" || destination == "" {
		return fmt.Errorf("source and destination are required")
	}
	sourcePath, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if strings.HasSuffix(destination, suffix) {
			return fmt.Errorf("destination must not be a SQLite sidecar")
		}
	}
	destinationPath, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}
	sourceURL := url.URL{Scheme: "file", Path: sourcePath, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", sourceURL.String())
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	dir, err := os.MkdirTemp(filepath.Dir(destinationPath), ".sqlite-backup-*")
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}
	defer os.RemoveAll(dir)
	snapshot := filepath.Join(dir, "snapshot.db")
	if _, err := db.Exec("VACUUM main INTO ?", snapshot); err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	if err := os.Chmod(snapshot, 0600); err != nil {
		return fmt.Errorf("protect snapshot: %w", err)
	}
	if err := os.Link(snapshot, destinationPath); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	return nil
}
