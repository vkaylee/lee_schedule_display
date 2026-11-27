package main

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInvalidCommandDoesNotTouchDatabase(t *testing.T) {
	if os.Getenv("TEST_INVALID_COMMAND") == "1" {
		os.Args = []string{"server", "backups", "source", "destination"}
		main()
		return
	}
	path := filepath.Join(t.TempDir(), "missing.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidCommandDoesNotTouchDatabase$")
	cmd.Env = append(os.Environ(), "TEST_INVALID_COMMAND=1", "DATABASE_PATH="+path)
	if err := cmd.Run(); err == nil {
		t.Fatal("invalid command accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database touched: %v", err)
	}
	original := []byte("not a database; must remain unchanged")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(os.Args[0], "-test.run=^TestInvalidCommandDoesNotTouchDatabase$")
	cmd.Env = append(os.Environ(), "TEST_INVALID_COMMAND=1", "DATABASE_PATH="+path)
	if err := cmd.Run(); err == nil {
		t.Fatal("invalid command accepted")
	}
	contents, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(contents, original) {
		t.Fatalf("existing database changed: %v", err)
	}
}

func TestBackupRestoreBookingAPI(t *testing.T) {
	dir := t.TempDir()
	source, snapshot := filepath.Join(dir, "source.db"), filepath.Join(dir, "snapshot.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err := db.Exec("INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name) VALUES('room'); INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'meeting','admin',0,100,200)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if err := backupSQLite(source, snapshot); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	h := handlerWithAuth(restored, "https://example.test", netip.Addr{})
	r := httptest.NewRequest("GET", "https://example.test/api/bookings", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"meeting"`) {
		t.Fatalf("restored API: %d %s", w.Code, w.Body.String())
	}
	mutate := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(`{"room_id":1,"title":"second","organizer":"admin","private":false,"starts":200,"ends":300}`))
	mutate.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	mutate.Header.Set("Origin", "https://example.test")
	mutate.Header.Set("X-CSRF-Token", "csrf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, mutate)
	if w.Code != http.StatusCreated {
		t.Fatalf("restored booking create: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := restored.QueryRow("SELECT count(*) FROM bookings").Scan(&count); err != nil || count != 2 {
		t.Fatalf("restored bookings: %d %v", count, err)
	}
	var originalCount int
	if err := db.QueryRow("SELECT count(*) FROM bookings").Scan(&originalCount); err != nil || originalCount != 1 {
		t.Fatalf("source bookings: %d %v", originalCount, err)
	}
}

func TestBackupSQLite(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	backup := filepath.Join(dir, "backup.db")
	restored := filepath.Join(dir, "restored.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=WAL; CREATE TABLE records (value TEXT); INSERT INTO records VALUES ('saved')"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO records VALUES ('uncommitted')"); err != nil {
		t.Fatal(err)
	}
	if err := backupSQLite(source, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "existing"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "existing"), filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := backupSQLite(source, filepath.Join(dir, "alias")); err == nil {
		t.Fatal("existing symlink destination accepted")
	}
	if contents, err := os.ReadFile(filepath.Join(dir, "existing")); err != nil || string(contents) != "keep" {
		t.Fatalf("symlink target changed: %q %v", contents, err)
	}
	info, err := os.Stat(backup)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("backup permissions: %v, %v", info, err)
	}
	if err := backupSQLite(backup, restored); err != nil {
		t.Fatal(err)
	}
	copyDB, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var count int
	var value string
	if err := copyDB.QueryRow("SELECT count(*), min(value) FROM records").Scan(&count, &value); err != nil || count != 1 || value != "saved" {
		t.Fatalf("restored count=%d value=%q error=%v", count, value, err)
	}
	var integrity string
	if err := copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q error=%v", integrity, err)
	}
	if err := backupSQLite(source, backup); err == nil {
		t.Fatal("existing destination overwritten")
	}
	backupDB, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	var after string
	if err := backupDB.QueryRow("SELECT min(value) FROM records").Scan(&after); err != nil || after != "saved" {
		t.Fatalf("backup changed after failed overwrite: %q %v", after, err)
	}
	if err := backupSQLite(source, source); err == nil {
		t.Fatal("source overwritten")
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := backupSQLite(source, source+suffix); err == nil {
			t.Fatalf("SQLite sidecar destination %q accepted", suffix)
		}
	}
	if err := backupSQLite(filepath.Join(dir, "missing.db"), filepath.Join(dir, "missing-backup.db")); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing.db")); !os.IsNotExist(err) {
		t.Fatalf("missing source created: %v", err)
	}
}
