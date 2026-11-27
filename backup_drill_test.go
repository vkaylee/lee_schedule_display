package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupRestoreDrill(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	snapshot := filepath.Join(dir, "snapshot.db")
	restore := filepath.Join(dir, "restore.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA journal_mode=WAL; INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name) VALUES('Room A')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	h := handlerWithTimezone(db, "https://example.test", netip.Addr{}, time.UTC)
	request := func(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	start := time.Now().UTC().Truncate(time.Hour).Add(24 * time.Hour)
	seriesStart := start.Add(7 * 24 * time.Hour)
	booking := fmt.Sprintf(`{"room_id":1,"title":"One","organizer":"admin","starts":%d,"ends":%d}`, start.Unix(), start.Add(time.Hour).Unix())
	if w := request(h, "POST", "/api/bookings", booking); w.Code != 201 {
		t.Fatalf("booking: %d %s", w.Code, w.Body.String())
	}
	series := fmt.Sprintf(`{"room_id":1,"title":"Weekly","organizer":"admin","private":true,"starts":%d,"duration_seconds":3600,"frequency":2,"interval":1,"weekdays":[%d],"count":3}`, seriesStart.Unix(), seriesStart.Weekday())
	if w := request(h, "POST", "/api/series", series); w.Code != 201 {
		t.Fatalf("series: %d %s", w.Code, w.Body.String())
	}
	if err := backupSQLite(source, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := backupSQLite(snapshot, restore); err != nil {
		t.Fatal(err)
	}
	copyDB, err := sql.Open("sqlite", restore)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var integrity string
	if err := copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity: %q %v", integrity, err)
	}
	rows, err := copyDB.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		rows.Close()
		t.Fatal("foreign key violation")
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for table, want := range map[string]int{"rooms": 1, "bookings": 1, "booking_series": 1, "series_occurrences": 3} {
		var got int
		if err := copyDB.QueryRow("SELECT count(*) FROM " + table).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: %d %v", table, got, err)
		}
	}
	restoredHandler := handlerWithTimezone(copyDB, "https://example.test", netip.Addr{}, time.UTC)
	w := request(restoredHandler, "GET", "/api/series", "")
	var items []struct {
		Title       string `json:"title"`
		Private     bool   `json:"private"`
		Occurrences []struct {
			Starts int64 `json:"starts"`
			Ends   int64 `json:"ends"`
		} `json:"occurrences"`
	}
	if w.Code != 200 {
		t.Fatalf("series API: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil || len(items) != 1 || items[0].Title != "Weekly" || !items[0].Private || len(items[0].Occurrences) != 3 || items[0].Occurrences[0].Starts != seriesStart.Unix() {
		t.Fatalf("series data: %+v %v", items, err)
	}
	w = request(restoredHandler, "GET", "/api/bookings", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"title":"One"`) {
		t.Fatalf("booking API: %d %s", w.Code, w.Body.String())
	}
	second := fmt.Sprintf(`{"room_id":1,"title":"Restored only","organizer":"admin","starts":%d,"ends":%d}`, start.Add(time.Hour).Unix(), start.Add(2*time.Hour).Unix())
	if w := request(restoredHandler, "POST", "/api/bookings", second); w.Code != 201 {
		t.Fatalf("restored write: %d %s", w.Code, w.Body.String())
	}
	var sourceCount int
	if err := db.QueryRow("SELECT count(*) FROM bookings").Scan(&sourceCount); err != nil || sourceCount != 1 {
		t.Fatalf("source changed: %d %v", sourceCount, err)
	}
}
