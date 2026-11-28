package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func insertPair(conn *sql.Conn, value int) (error, error) {
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err, nil
	}
	_, err := conn.ExecContext(ctx, "INSERT INTO records(value) VALUES(?), (?); COMMIT", value, value)
	if err != nil {
		_, rollbackErr := conn.ExecContext(ctx, "ROLLBACK")
		return err, rollbackErr
	}
	return nil, nil
}

func TestInsertPairRollsBackBusyCommit(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=DELETE; CREATE TABLE records (value INTEGER); INSERT INTO records VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	reader, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	writer, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := reader.ExecContext(ctx, "BEGIN"); err != nil {
		t.Fatal(err)
	}
	var value int
	if err := reader.QueryRowContext(ctx, "SELECT value FROM records").Scan(&value); err != nil || value != 1 {
		t.Fatalf("read lock: value=%d error=%v", value, err)
	}
	defer reader.ExecContext(ctx, "ROLLBACK")
	if _, err := writer.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		t.Fatal(err)
	}
	execErr, rollbackErr := insertPair(writer, 2)
	if execErr == nil || (!strings.Contains(execErr.Error(), "database is locked") && !strings.Contains(execErr.Error(), "SQLITE_BUSY")) || rollbackErr != nil {
		t.Fatalf("expected busy commit and successful rollback: error=%v rollback=%v", execErr, rollbackErr)
	}
	if _, err := reader.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if execErr, rollbackErr = insertPair(writer, 3); execErr != nil || rollbackErr != nil {
		t.Fatalf("writer not reusable: error=%v rollback=%v", execErr, rollbackErr)
	}
	var count, sum int
	if err := db.QueryRow("SELECT count(*), sum(value) FROM records").Scan(&count, &sum); err != nil || count != 3 || sum != 7 {
		t.Fatalf("unexpected committed records: count=%d sum=%d error=%v", count, sum, err)
	}
}

func TestBackupDuringCommitContention(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=DELETE; CREATE TABLE records (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	wg.Add(2)
	go func() {
		defer wg.Done()
		conn, connErr := db.Conn(context.Background())
		if connErr != nil {
			errs <- connErr
			return
		}
		defer conn.Close()
		for i := 1; i <= 25; i++ {
			execErr, rollbackErr := insertPair(conn, i)
			if rollbackErr != nil {
				errs <- fmt.Errorf("writer error %v; rollback failed: %w", execErr, rollbackErr)
				return
			}
			if execErr != nil && !strings.Contains(execErr.Error(), "SQLITE_BUSY") && !strings.Contains(execErr.Error(), "database is locked") {
				errs <- fmt.Errorf("unexpected writer error: %w", execErr)
				return
			}
		}
	}()
	var snapshots []string
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			dst := filepath.Join(dir, fmt.Sprintf("snap-%d.db", i))
			if bErr := backupSQLite(source, dst); bErr == nil {
				snapshots = append(snapshots, dst)
			} else if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
				errs <- fmt.Errorf("failed backup left partial file %s: %v", dst, statErr)
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for _, snap := range snapshots {
		copyDB, err := sql.Open("sqlite", snap)
		if err != nil {
			t.Fatal(err)
		}
		var integrity string
		var count int
		err = copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity)
		if err == nil {
			err = copyDB.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
		}
		copyDB.Close()
		if err != nil || integrity != "ok" || count%2 != 0 {
			t.Fatalf("snapshot %s broken: integrity=%q count=%d err=%v", snap, integrity, count, err)
		}
	}
}
