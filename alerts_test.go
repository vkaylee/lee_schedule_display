package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"lee_schedule_display/recurrence"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConflictScanDurable(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "alerts.db") + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'one','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1),(1,'two','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1); INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,200,200,300),(2,200,200,300)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = scanConflicts(db, time.Unix(100, 0)); err != nil {
			t.Fatal(err)
		}
	}
	var total, active int
	if err = db.QueryRow("SELECT COUNT(*),SUM(active) FROM conflict_alerts").Scan(&total, &active); err != nil || total != 1 || active != 1 {
		t.Fatalf("alerts %d %d %v", total, active, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err = scanConflicts(db, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE series_occurrences SET canceled=1 WHERE series_id=2"); err != nil {
		t.Fatal(err)
	}
	if err = scanConflicts(db, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT COUNT(*),SUM(active) FROM conflict_alerts").Scan(&total, &active); err != nil || total != 1 || active != 0 {
		t.Fatalf("resolved %d %d %v", total, active, err)
	}
}

func TestConflictScanConcurrentAndAPI(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "alerts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	at := now.Add(2 * time.Hour).Unix()
	token := strings.Repeat("z", 43)
	if _, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'one','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1),(1,'two','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO sessions VALUES (?,1,'csrf',?);`, tokenHash(token), now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,?,?,?),(2,?,?,?)`, at, at, at+3600, at, at, at+3600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- scanConflicts(db, now) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, csrf string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, nil)
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", csrf)
		if auth {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/alerts", "", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/alerts/1/acknowledge", "bad", true); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/alerts/1/acknowledge", "csrf", true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = scanConflicts(db, now); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/alerts", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), `"acknowledged":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	var n int
	if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE canceled=0").Scan(&n); err != nil || n != 2 {
		t.Fatalf("occurrences canceled %d %v", n, err)
	}
}

func TestScanStatusAndCheckedThrough(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'a','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1),(1,'b','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1); INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,200,200,300),(2,200,200,300)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0).UTC()
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	var checked, last int64
	var failed bool
	if err = db.QueryRow("SELECT checked_through FROM booking_series WHERE id=1").Scan(&checked); err != nil || checked != 1 {
		t.Fatalf("checked=%d err=%v", checked, err)
	}
	if err = db.QueryRow("SELECT last_success,failed FROM conflict_scan_status WHERE id=1").Scan(&last, &failed); err != nil || last != now.Unix() || failed {
		t.Fatalf("status=%d %t %v", last, failed, err)
	}
}

func TestScanBoundaryAndFailure(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "boundary.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	h := now.AddDate(1, 0, 0).Unix()
	if _, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'a','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1),(1,'b','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,?,?,?),(2,?,?,?)", h, h, h+60, h, h, h+60); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM conflict_alerts").Scan(&count); err != nil || count != 0 {
		t.Fatalf("boundary %d %v", count, err)
	}
	if err = runConflictScan(db, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM conflict_alerts WHERE active=1").Scan(&count); err != nil || count != 1 {
		t.Fatalf("entered %d %v", count, err)
	}
	if _, err = db.Exec("DELETE FROM conflict_alerts"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DROP TABLE conflict_alerts"); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, now.Add(2*time.Second)); err == nil {
		t.Fatal("missing scan error")
	}
	var failed bool
	var last int64
	if err = db.QueryRow("SELECT failed,last_success FROM conflict_scan_status").Scan(&failed, &last); err != nil || !failed || last != now.Add(time.Second).Unix() {
		t.Fatalf("failure status %t %d %v", failed, last, err)
	}
}

func TestWorkerStartsAndRestarts(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "worker.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, instant := range []time.Time{time.Unix(1000, 0), time.Unix(2000, 0)} {
		ctx, cancel := context.WithCancel(context.Background())
		ticks := make(chan time.Time)
		done := make(chan struct{})
		go func() { defer close(done); conflictWorker(ctx, db, func() time.Time { return instant }, ticks) }()
		deadline := time.After(3 * time.Second)
		for {
			var last sql.NullInt64
			if err = db.QueryRow("SELECT last_success FROM conflict_scan_status").Scan(&last); err != nil {
				t.Fatal(err)
			}
			if last.Valid && last.Int64 == instant.Unix() {
				break
			}
			select {
			case <-deadline:
				t.Fatal("worker did not scan")
			default:
				time.Sleep(time.Millisecond)
			}
		}
		cancel()
		<-done
	}
}

func TestScanDoesNotHideCoverageGap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'a','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	var from, through int64
	if err = db.QueryRow("SELECT checked_from,checked_through FROM conflict_scan_status WHERE id=1").Scan(&from, &through); err != nil || from != now.Unix() || through != now.AddDate(1, 0, 0).Unix() {
		t.Fatalf("coverage %d %d %v", from, through, err)
	}
}

func TestOldFailureCannotOverrideNewSuccess(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room')"); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, time.Unix(900, 0)); err != nil {
		t.Fatal(err)
	}
	var old int64
	if err = db.QueryRow("UPDATE room_scan_status SET attempt=attempt+1 WHERE room_id=1 RETURNING attempt").Scan(&old); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE room_scan_status SET failed=1 WHERE room_id=1 AND attempt=?", old); err != nil {
		t.Fatal(err)
	}
	var failed bool
	if err = db.QueryRow("SELECT failed FROM room_scan_status WHERE room_id=1").Scan(&failed); err != nil || failed {
		t.Fatalf("stale failure: %t %v", failed, err)
	}
}

func TestScanStatusAPIShowsLatestWindowAfterGap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('Room A'),('Room B'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	first := time.Now().UTC().AddDate(-3, 0, 0).Truncate(time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	if err = runConflictScan(db, first); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE room_scan_status SET failed=1 WHERE room_id=2"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "https://example.test/api/alerts/status", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"checked_from":%d`, now.Unix())) || !strings.Contains(w.Body.String(), fmt.Sprintf(`"checked_through":%d`, now.AddDate(1, 0, 0).Unix())) || !strings.Contains(w.Body.String(), `"room":"Room B"`) || !strings.Contains(w.Body.String(), `"room_id":2`) || !strings.Contains(w.Body.String(), `"failed":true`) {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestScanKeepsFutureSeriesConflictProtection(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "future.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).AddDate(8, 0, 0)
	a := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 1, Duration: time.Hour}
	b := booking{RoomID: 1, Title: "a", Organizer: "admin"}
	if err = storeSeries(db, token, "csrf", b, a); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	other := a
	other.Start = start.AddDate(-2, 0, 0)
	other.Count = 732
	if err = storeSeries(db, token, "csrf", b, other); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("future conflict not rejected: %v", err)
	}
}
func TestConcurrentOlderFailureDoesNotOverrideSuccess(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "attempts.db") + "?_pragma=busy_timeout(5000)"
	a, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err = migrate(a); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec("INSERT INTO rooms(name) VALUES('room')"); err != nil {
		t.Fatal(err)
	}
	failed := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runConflictScanWith(a, time.Unix(1000, 0), func(db *sql.DB, now time.Time, attempt int64) error {
			close(failed)
			<-resume
			return errors.New("simulated scan error")
		})
	}()
	<-failed
	if err = runConflictScan(b, time.Unix(2000, 0)); err != nil {
		t.Fatal(err)
	}
	close(resume)
	if err = <-done; err == nil {
		t.Fatal("expected older failure")
	}
	var last int64
	var flag bool
	if err = b.QueryRow("SELECT last_success,failed FROM room_scan_status WHERE room_id=1").Scan(&last, &flag); err != nil || last != 2000 || flag {
		t.Fatalf("room last=%d failed=%t error=%v", last, flag, err)
	}
	var aggregateLast int64
	if err = b.QueryRow("SELECT last_success,failed FROM conflict_scan_status WHERE id=1").Scan(&aggregateLast, &flag); err != nil || aggregateLast != 2000 || flag {
		t.Fatalf("aggregate last=%d failed=%t error=%v", aggregateLast, flag, err)
	}
}

func TestConflictScanRoomIsolationOnDSTFailure(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "room-worker.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('NY'),('UTC'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	ny, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 3, 9, 2, 30, 0, 0, ny)
	bad := recurrence.Rule{Start: start, Location: ny, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	goodStart := time.Now().UTC().Add(time.Hour)
	good := recurrence.Rule{Start: goodStart, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	a, _ := json.Marshal(storedSeriesRule{bad, ny.String()})
	b, _ := json.Marshal(storedSeriesRule{good, "UTC"})
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'bad','admin',0,?,1),(2,'good','admin',0,?,1)", string(a), string(b)); err != nil {
		t.Fatal(err)
	}
	if err = runConflictScan(db, time.Now()); err == nil {
		t.Fatal("expected room-local DST error")
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE s.room_id=2").Scan(&count); err != nil || count == 0 {
		t.Fatalf("good room not scanned: %d %v", count, err)
	}
}

func TestNewRoomCreationCoverageBeforeWorkerScan(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name) VALUES('new room'); INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); INSERT INTO sessions VALUES(?,1,'csrf',?)", tokenHash(token), tokenHash(token), now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	rule := recurrence.Rule{Start: now.Add(time.Hour), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "https://example.test/api/kiosk", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"valid_until":`) || strings.Contains(w.Body.String(), `"valid_until":`+fmt.Sprint(now.Unix())) {
		t.Fatalf("creation coverage %d %s", w.Code, w.Body.String())
	}
	var last sql.NullInt64
	var failed bool
	err = db.QueryRow("SELECT last_success,failed FROM room_scan_status WHERE room_id=1").Scan(&last, &failed)
	if !errors.Is(err, sql.ErrNoRows) || last.Valid || failed {
		t.Fatalf("room should be unscanned, last=%v failed=%t err=%v", last, failed, err)
	}
}

func TestRoomScanFailureDoesNotInvalidateOtherRoomCoverage(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('A'),('B'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO sessions VALUES(?,1,'csrf',?)", tokenHash(token), now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	for room := int64(1); room <= 2; room++ {
		rule := recurrence.Rule{Start: now.Add(time.Hour), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
		if err = storeSeries(db, token, "csrf", booking{RoomID: room, Title: "s", Organizer: "admin"}, rule); err != nil {
			t.Fatal(err)
		}
	}
	if err = runConflictScan(db, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE room_scan_status SET failed=1 WHERE room_id=1"); err != nil {
		t.Fatal(err)
	}
	coverage, err := trustedDisplayCoverage(db, 1, now.Unix())
	if err != nil || !coverage.Valid || coverage.Int64 != now.Unix() {
		t.Fatalf("room A failed scan cutoff %v %v", coverage, err)
	}
	coverage, err = trustedDisplayCoverage(db, 2, now.Unix())
	if err != nil || !coverage.Valid || coverage.Int64 <= now.Unix() {
		t.Fatalf("room B healthy scan coverage %v %v", coverage, err)
	}
}
