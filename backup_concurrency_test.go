package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupDuringWriteTransaction(t *testing.T) {
	for _, mode := range []string{"DELETE", "WAL"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.db")
			db, err := sql.Open("sqlite", source)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var actual string
			if err := db.QueryRow("PRAGMA journal_mode=" + mode).Scan(&actual); err != nil || !strings.EqualFold(actual, mode) {
				t.Fatalf("journal mode = %q, error = %v", actual, err)
			}
			if _, err := db.Exec("CREATE TABLE records (value INTEGER); INSERT INTO records VALUES (1), (1)"); err != nil {
				t.Fatal(err)
			}
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec("UPDATE records SET value=2"); err != nil {
				t.Fatal(err)
			}
			before := filepath.Join(dir, "before.db")
			if err := backupSQLite(source, before); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			after := filepath.Join(dir, "after.db")
			if err := backupSQLite(source, after); err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range []struct {
				path  string
				value int
			}{{before, 1}, {after, 2}} {
				copyDB, err := sql.Open("sqlite", snapshot.path)
				if err != nil {
					t.Fatal(err)
				}
				var count, min, max int
				err = copyDB.QueryRow("SELECT count(*), min(value), max(value) FROM records").Scan(&count, &min, &max)
				if err != nil || count != 2 || min != snapshot.value || max != snapshot.value {
					copyDB.Close()
					t.Fatalf("snapshot %s: count=%d min=%d max=%d error=%v", snapshot.path, count, min, max, err)
				}
				var integrity string
				err = copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity)
				copyDB.Close()
				if err != nil || integrity != "ok" {
					t.Fatalf("integrity=%q error=%v", integrity, err)
				}
			}
		})
	}
}

func TestBackupDuringExclusiveRollbackWrite(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.db"), filepath.Join(dir, "backup.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=DELETE; CREATE TABLE records (value INTEGER); INSERT INTO records VALUES (1); BEGIN EXCLUSIVE; UPDATE records SET value=2"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("ROLLBACK")
	if err := backupSQLite(source, destination); err == nil {
		t.Fatal("backup succeeded while source held an exclusive lock")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("failed backup published destination: %v", err)
	}
	if _, err := db.Exec("COMMIT"); err != nil {
		t.Fatal(err)
	}
	if err := backupSQLite(source, destination); err != nil {
		t.Fatal(err)
	}
}
