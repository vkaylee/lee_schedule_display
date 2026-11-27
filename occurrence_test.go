package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lee_schedule_display/recurrence"
)

func TestOccurrenceMutation(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Now().UTC().Truncate(time.Second).AddDate(2, 0, 0)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 3, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "meeting", Organizer: "admin"}, rule); err != nil {
		t.Fatal(err)
	}
	move := booking{RoomID: 1, Title: "moved", Organizer: "admin", Starts: start.Add(3 * time.Hour).Unix(), Ends: start.Add(4 * time.Hour).Unix(), Version: 1}
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), move, false); err != nil {
		t.Fatal(err)
	}
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), move, false); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("stale version: %v", err)
	}
	var original, actual, version int64
	if err = db.QueryRow("SELECT original_starts,starts,version FROM series_occurrences WHERE series_id=1 AND original_starts=?", start.Unix()).Scan(&original, &actual, &version); err != nil || original != start.Unix() || actual != move.Starts || version != 2 {
		t.Fatalf("identity: %d %d %d %v", original, actual, version, err)
	}
	move.Version = 2
	move.Starts = start.Add(24 * time.Hour).Unix()
	move.Ends = move.Starts + 3600
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), move, false); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), booking{Version: 2}, true); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- mutateOccurrence(db, token, "csrf", 1, start.Add(24*time.Hour).Unix(), booking{Version: 1}, true)
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, errSeriesConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("concurrent success=%d conflicts=%d", success, conflicts)
	}
	var audit int
	if err = db.QueryRow("SELECT COUNT(*) FROM occurrence_audit").Scan(&audit); err != nil || audit != 3 {
		t.Fatalf("audit: %d %v", audit, err)
	}
}

func TestOccurrenceAPIAndRestart(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "restart.db") + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name,show_title,show_organizer) VALUES('one',1,1),('two',1,1); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	device := strings.Repeat("d", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(2,'screen',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')", tokenHash(device)); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 2, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body, csrf string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", csrf)
		if auth {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	pathOccurrence := fmt.Sprintf("/api/series/1/occurrences/%d", start.Unix())
	moved := start.Add(3 * time.Hour).Unix()
	body := fmt.Sprintf(`{"room_id":2,"title":"secret override","organizer":"secret person","private":true,"starts":%d,"ends":%d,"version":1}`, moved, moved+3600)
	for _, tc := range []struct {
		auth bool
		csrf string
		want int
	}{{false, "csrf", 401}, {true, "bad", 403}, {true, "csrf", 204}, {true, "csrf", 409}} {
		if w := request("POST", pathOccurrence, body, tc.csrf, tc.auth); w.Code != tc.want {
			t.Fatalf("mutation status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
		}
	}
	single := fmt.Sprintf(`{"room_id":2,"title":"x","organizer":"x","starts":%d,"ends":%d}`, moved, moved+3600)
	if w := request("POST", "/api/bookings", single, "csrf", true); w.Code != 409 {
		t.Fatalf("booking conflict %d %s", w.Code, w.Body.String())
	}
	conflicting := recurrence.Rule{Start: time.Unix(moved, 0), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 1, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", booking{RoomID: 2, Title: "x", Organizer: "x"}, conflicting); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("series conflict %v", err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	h = handlerWithAuth(db, "https://example.test", netip.Addr{})
	if w := request("GET", "/api/series", "", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"original_starts":%d`, start.Unix())) || !strings.Contains(w.Body.String(), `"title":"secret override"`) {
		t.Fatalf("restart list %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("GET", "https://example.test/api/kiosk", nil)
	r.Header.Set("Authorization", "Bearer "+device)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, moved)) || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("kiosk privacy %d %s", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "", false); w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, moved)) || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("display privacy %d %s", w.Code, w.Body.String())
	}
	if _, err = db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'single','admin',0,?,?)", start.Unix(), start.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	conflictBody := fmt.Sprintf(`{"room_id":1,"title":"x","organizer":"x","starts":%d,"ends":%d,"version":2}`, start.Unix(), start.Add(time.Hour).Unix())
	if w = request("POST", pathOccurrence, conflictBody, "csrf", true); w.Code != 409 {
		t.Fatalf("reverse conflict %d %s", w.Code, w.Body.String())
	}
	cancel := httptest.NewRequest("POST", "https://example.test"+pathOccurrence+"/cancel", nil)
	cancel.Header.Set("Origin", "https://example.test")
	cancel.Header.Set("X-CSRF-Token", "csrf")
	cancel.Header.Set("If-Match", "2")
	cancel.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, cancel)
	if w.Code != 204 {
		t.Fatalf("cancel %d %s", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "", false); w.Code != 200 || strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, moved)) {
		t.Fatalf("canceled display %d %s", w.Code, w.Body.String())
	}
}

func TestOccurrenceConcurrentMoves(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "race.db") + "?_pragma=busy_timeout(5000)"
	first, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err = migrate(first); err != nil {
		t.Fatal(err)
	}
	if _, err = first.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = first.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 2, Duration: time.Hour}
	if err = storeSeries(first, token, "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule); err != nil {
		t.Fatal(err)
	}
	moved := start.Add(3 * time.Hour).Unix()
	b := booking{RoomID: 1, Title: "move", Organizer: "admin", Starts: moved, Ends: moved + 3600, Version: 1}
	gate := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-gate; results <- mutateOccurrence(first, token, "csrf", 1, start.Unix(), b, false) }()
	go func() {
		<-gate
		results <- mutateOccurrence(second, token, "csrf", 1, start.Add(24*time.Hour).Unix(), b, false)
	}()
	close(gate)
	success, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, errSeriesConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	var audit int
	if err = first.QueryRow("SELECT COUNT(*) FROM occurrence_audit").Scan(&audit); err != nil || audit != 1 || success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d audit=%d error=%v", success, conflicts, audit, err)
	}
}

func TestOccurrenceRacesBookingAndSeries(t *testing.T) {
	for _, kind := range []string{"booking", "series"} {
		t.Run(kind, func(t *testing.T) {
			path := "file:" + filepath.Join(t.TempDir(), "race.db") + "?_pragma=busy_timeout(5000)"
			first, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Close()
			second, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer second.Close()
			if err = migrate(first); err != nil {
				t.Fatal(err)
			}
			if _, err = first.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
				t.Fatal(err)
			}
			token := strings.Repeat("z", 43)
			if _, err = first.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
				t.Fatal(err)
			}
			start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
			rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 1, Duration: time.Hour}
			if err = storeSeries(first, token, "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule); err != nil {
				t.Fatal(err)
			}
			moved := start.Add(3 * time.Hour).Unix()
			b := booking{RoomID: 1, Title: "move", Organizer: "admin", Starts: moved, Ends: moved + 3600, Version: 1}
			gate := make(chan struct{})
			results := make(chan bool, 2)
			go func() {
				<-gate
				err := mutateOccurrence(first, token, "csrf", 1, start.Unix(), b, false)
				if err != nil && !errors.Is(err, errSeriesConflict) {
					t.Errorf("move: %v", err)
				}
				results <- err == nil
			}()
			go func() {
				<-gate
				if kind == "series" {
					r := rule
					r.Start = time.Unix(moved, 0)
					err := storeSeries(second, token, "csrf", booking{RoomID: 1, Title: "other", Organizer: "admin"}, r)
					if err != nil && !errors.Is(err, errSeriesConflict) {
						t.Errorf("series: %v", err)
					}
					results <- err == nil
					return
				}
				h := handlerWithAuth(second, "https://example.test", netip.Addr{})
				body := fmt.Sprintf(`{"room_id":1,"title":"other","organizer":"admin","starts":%d,"ends":%d}`, moved, moved+3600)
				r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
				r.Header.Set("Origin", "https://example.test")
				r.Header.Set("X-CSRF-Token", "csrf")
				r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != 201 && w.Code != 409 {
					t.Errorf("booking: %d %s", w.Code, w.Body.String())
				}
				results <- w.Code == 201
			}()
			close(gate)
			a, both := <-results, <-results
			if a == both {
				t.Fatalf("expected one success, got %v %v", a, both)
			}
			var audit int
			if err = first.QueryRow("SELECT COUNT(*) FROM occurrence_audit").Scan(&audit); err != nil || audit > 1 {
				t.Fatalf("audit %d %v", audit, err)
			}
		})
	}
}

func TestOccurrenceMovesAcrossHorizon(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "horizon.db") + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
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
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 2, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "s", Organizer: "admin"}, rule); err != nil {
		t.Fatal(err)
	}
	device := strings.Repeat("d", 43)
	if _, err = db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')", tokenHash(device)); err != nil {
		t.Fatal(err)
	}
	check := func(want, absent int64) {
		t.Helper()
		h := handlerWithAuth(db, "https://example.test", netip.Addr{})
		for _, endpoint := range []string{"/api/kiosk", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
			r := httptest.NewRequest("GET", "https://example.test"+endpoint, nil)
			r.Header.Set("Authorization", "Bearer "+device)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, want)) || strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, absent)) {
				t.Fatalf("%s status=%d body=%s", endpoint, w.Code, w.Body.String())
			}
		}
	}
	far := start.AddDate(2, 0, 0)
	b := booking{RoomID: 1, Title: "move", Organizer: "admin", Starts: far.Unix(), Ends: far.Add(time.Hour).Unix(), Version: 1}
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), b, false); err != nil {
		t.Fatal(err)
	}
	check(far.Unix(), start.Unix())
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	check(far.Unix(), start.Unix())
	r := rule
	r.Start = far
	r.Count = 1
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "other", Organizer: "admin"}, r); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("far conflict %v", err)
	}
	b.Version = 2
	b.Starts = start.Add(4 * time.Hour).Unix()
	b.Ends = b.Starts + 3600
	if err = mutateOccurrence(db, token, "csrf", 1, start.Unix(), b, false); err != nil {
		t.Fatal(err)
	}
	check(b.Starts, far.Unix())
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	check(b.Starts, far.Unix())
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "other", Organizer: "admin"}, r); err != nil {
		t.Fatalf("vacated far slot %v", err)
	}
}

func TestMigration11PreservesOccurrencesAndRollsBack(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("failure_", fail), func(t *testing.T) {
			db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "migration.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`DROP TABLE room_scan_status; DROP TABLE materialization_coverage; DROP TABLE conflict_scan_status; DROP TABLE conflict_alerts; DROP TABLE occurrence_audit; DROP TABLE series_occurrences; CREATE TABLE series_occurrences (series_id INTEGER NOT NULL REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends>starts), PRIMARY KEY(series_id,starts)); CREATE INDEX series_occurrences_time ON series_occurrences(starts,ends); ALTER TABLE booking_series DROP COLUMN checked_through; INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name) VALUES('room'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'series','admin',1,'{}',1); INSERT INTO series_occurrences VALUES(1,100,200),(1,300,400); PRAGMA user_version=11`)
			if err != nil {
				t.Fatal(err)
			}
			if fail {
				if _, err = db.Exec("CREATE TABLE occurrence_audit (blocker INTEGER)"); err != nil {
					t.Fatal(err)
				}
			}
			err = migrate(db)
			if fail && err == nil || !fail && err != nil {
				t.Fatalf("migrate %v", err)
			}
			var version, count int
			if err = db.QueryRow("SELECT (SELECT user_version FROM pragma_user_version), COUNT(*) FROM series_occurrences WHERE series_id=1 AND ((starts=100 AND ends=200) OR (starts=300 AND ends=400))").Scan(&version, &count); err != nil || count != 2 {
				t.Fatalf("data version=%d count=%d err=%v", version, count, err)
			}
			if fail {
				if version != 11 {
					t.Fatalf("rollback version %d", version)
				}
				if _, err = db.Exec("SELECT original_starts FROM series_occurrences"); err == nil {
					t.Fatal("rollback left new schema")
				}
				if _, err = db.Exec("DROP TABLE occurrence_audit"); err != nil {
					t.Fatal(err)
				}
				if err = migrate(db); err != nil {
					t.Fatal(err)
				}
			} else if version != 15 {
				t.Fatalf("version %d", version)
			}
			var originals int
			if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE original_starts=starts AND version=1 AND canceled=0 AND room_id IS NULL").Scan(&originals); err != nil || originals != 2 {
				t.Fatalf("identities %d %v", originals, err)
			}
		})
	}
}

func TestOfficialMigrationPreservesPublicID(t *testing.T) {
	for _, v := range []int{10, 11} {
		t.Run(fmt.Sprint(v), func(t *testing.T) {
			db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "official.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`DROP TABLE room_scan_status; DROP TABLE materialization_coverage; DROP TABLE conflict_scan_status; DROP TABLE conflict_alerts; DROP TABLE occurrence_audit; DROP TABLE series_occurrences; CREATE TABLE series_occurrences(series_id INTEGER NOT NULL REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends>starts), PRIMARY KEY(series_id,starts)); CREATE INDEX series_occurrences_time ON series_occurrences(starts,ends); ALTER TABLE booking_series DROP COLUMN checked_through; INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',X'01','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'meeting','admin',0,'{}',1); INSERT INTO series_occurrences VALUES(1,100,200); PRAGMA user_version=11`)
			if err != nil {
				t.Fatal(err)
			}
			if v == 10 {
				if _, err = db.Exec("DROP INDEX devices_public_id; ALTER TABLE devices DROP COLUMN public_id; PRAGMA user_version=10"); err != nil {
					t.Fatal(err)
				}
			}
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			var id string
			var version, horizon, original int
			if err = db.QueryRow("SELECT devices.public_id,(SELECT user_version FROM pragma_user_version),booking_series.checked_through,series_occurrences.original_starts FROM devices,booking_series,series_occurrences").Scan(&id, &version, &horizon, &original); err != nil || len(id) != 32 || version != 15 || horizon != 200 || original != 100 || (v == 11 && id != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
				t.Fatalf("id=%s version=%d horizon=%d original=%d error=%v", id, version, horizon, original, err)
			}
		})
	}
}
func TestRejectExperimentalVersion11(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DROP INDEX devices_public_id; ALTER TABLE devices DROP COLUMN public_id; PRAGMA user_version=11"); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err == nil || !strings.Contains(err.Error(), "unsupported experimental") {
		t.Fatalf("expected rejection: %v", err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 11 {
		t.Fatalf("version=%d %v", version, err)
	}
}

func TestRejectVersion13MissingSchema(t *testing.T) {
	for _, statement := range []string{"DROP TABLE occurrence_audit", "ALTER TABLE booking_series DROP COLUMN checked_through", "DROP TABLE series_occurrences"} {
		t.Run(statement, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if err = migrate(db); err == nil {
				t.Fatal("missing v13 schema accepted")
			}
		})
	}
}

func TestRejectVersion13DamagedSeriesAndIndex(t *testing.T) {
	for _, statement := range []string{
		"DROP TABLE booking_series; CREATE TABLE booking_series(checked_through INTEGER)",
		"DROP INDEX devices_public_id",
		"DROP INDEX devices_public_id; CREATE INDEX devices_public_id ON devices(public_id)",
		"DROP TABLE series_occurrences; CREATE TABLE series_occurrences(series_id INTEGER,original_starts INTEGER,starts INTEGER,ends INTEGER,version INTEGER,canceled INTEGER,room_id INTEGER,title TEXT,organizer TEXT,private INTEGER,PRIMARY KEY(series_id,original_starts))",
	} {
		t.Run(statement, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if err = migrate(db); err == nil {
				t.Fatal("damaged schema accepted")
			}
		})
	}
}

func TestRejectOccurrenceMissingDefaults(t *testing.T) {
	for _, column := range []string{"version", "canceled"} {
		t.Run(column, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			var schema string
			if err = db.QueryRow("SELECT sql FROM sqlite_schema WHERE type='table' AND name='series_occurrences'").Scan(&schema); err != nil {
				t.Fatal(err)
			}
			schema = strings.Replace(schema, "CREATE TABLE \"series_occurrences\"", "CREATE TABLE replacement_occurrences", 1)
			schema = strings.Replace(schema, column+" INTEGER NOT NULL DEFAULT "+map[string]string{"version": "1", "canceled": "0"}[column], column+" INTEGER NOT NULL", 1)
			if _, err = db.Exec(schema); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("DROP TABLE series_occurrences; ALTER TABLE replacement_occurrences RENAME TO series_occurrences"); err != nil {
				t.Fatal(err)
			}
			if err = migrate(db); err == nil {
				t.Fatal("missing default accepted")
			}
		})
	}
}
