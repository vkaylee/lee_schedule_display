package main

import (
	"database/sql"
	"encoding/json"
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

func TestFarSeriesDisplayAndPolicy(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',X'01','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).AddDate(2, 0, 0)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 2, Duration: time.Hour}
	b := booking{RoomID: 1, Title: "series", Organizer: "admin"}
	if err = storeSeries(db, token, "csrf", b, rule); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	r := httptest.NewRequest("GET", "https://example.test/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, start.Unix())) {
		t.Fatalf("display: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "https://example.test/api/series", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"checked_through":%d`, start.AddDate(1, 0, 0).Unix())) {
		t.Fatalf("checked_through: %d %s", w.Code, w.Body.String())
	}
	other := rule
	other.Start = start.Add(-400 * 24 * time.Hour)
	other.Count = 401
	if err = storeSeries(db, token, "csrf", b, other); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("existing horizon: %v", err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&count); err != nil || count != 1 {
		t.Fatalf("partial write %d %v", count, err)
	}
}

func TestSeriesProtectsExistingHorizon(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			token := strings.Repeat("q", 43)
			if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
				t.Fatal(err)
			}
			late := time.Now().UTC().Truncate(time.Second).AddDate(3, 0, 0)
			a := recurrence.Rule{Start: late, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 1, Duration: time.Hour}
			b := recurrence.Rule{Start: late.Add(-400 * 24 * time.Hour), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 401, Duration: time.Hour}
			if reverse {
				a, b = b, a
			}
			booking := booking{RoomID: 1, Title: "series", Organizer: "admin"}
			if err = storeSeries(db, token, "csrf", booking, a); err != nil {
				t.Fatal(err)
			}
			if err = storeSeries(db, token, "csrf", booking, b); !errors.Is(err, errSeriesConflict) {
				t.Fatalf("existing or new horizon must be protected: %v", err)
			}
			var n int
			if err = db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&n); err != nil || n != 1 {
				t.Fatalf("partial write %d %v", n, err)
			}
		})
	}
}

func TestExistingHorizonBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		offset   time.Duration
		conflict bool
	}{{"inside", -time.Second, true}, {"at", 0, false}, {"after", time.Second, false}} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", tc.name, reverse), func(t *testing.T) {
				db, err := sql.Open("sqlite", ":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				db.SetMaxOpenConns(1)
				if err = migrate(db); err != nil {
					t.Fatal(err)
				}
				token := strings.Repeat("k", 43)
				if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
					t.Fatal(err)
				}
				if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
					t.Fatal(err)
				}
				first := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
				horizon := first.AddDate(1, 0, 0)
				overlap := horizon.Add(tc.offset)
				original := recurrence.Rule{Start: first, Location: time.UTC, Frequency: recurrence.Daily, Interval: 364, Count: 2, Duration: 48 * time.Hour}
				incoming := recurrence.Rule{Start: overlap.Add(-400 * 24 * time.Hour), Location: time.UTC, Frequency: recurrence.Daily, Interval: 200, Count: 3, Duration: time.Minute}
				if reverse {
					original, incoming = incoming, original
				}
				booking := booking{RoomID: 1, Title: "series", Organizer: "admin"}
				if err = storeSeries(db, token, "csrf", booking, original); err != nil {
					t.Fatal(err)
				}
				err = storeSeries(db, token, "csrf", booking, incoming)
				wantConflict := tc.conflict
				if wantConflict && !errors.Is(err, errSeriesConflict) || !wantConflict && err != nil {
					t.Fatalf("conflict=%t error=%v", wantConflict, err)
				}
			})
		}
	}
}

func TestExistingHorizonConservativeBeforeWindow(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("v", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	oldStart := now.Add(-800 * 24 * time.Hour)
	old := recurrence.Rule{Start: oldStart, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 1, Duration: time.Hour}
	b := booking{RoomID: 1, Title: "series", Organizer: "admin"}
	if err = storeSeries(db, token, "csrf", b, old); err != nil {
		t.Fatal(err)
	}
	if err = storeSeries(db, token, "csrf", b, old); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("conservative rejection before both windows: %v", err)
	}
}

func TestPastSeriesAnchorAndFarSingle(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("p", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	start := now.AddDate(0, 0, -1)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 800, Duration: time.Hour}
	far := start.Add(700 * 24 * time.Hour)
	if _, err = db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'single','admin',0,?,?)", far.Unix(), far.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	b := booking{RoomID: 1, Title: "series", Organizer: "admin"}
	if err = storeSeries(db, token, "csrf", b, rule); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("far single conflict: %v", err)
	}
	if _, err = db.Exec("DELETE FROM bookings"); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Truncate(time.Second).AddDate(1, 0, 0)
	if err = storeSeries(db, token, "csrf", b, rule); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC().Truncate(time.Second).AddDate(1, 0, 0)
	var checked int64
	if err = db.QueryRow("SELECT checked_through FROM booking_series").Scan(&checked); err != nil || checked < before.Unix() || checked > after.Unix() {
		t.Fatalf("anchor %d err=%v", checked, err)
	}
}

func TestSeriesHorizonBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		offset   time.Duration
		conflict bool
	}{
		{"inside", -time.Second, true}, {"at horizon", 0, false}, {"outside", time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrate(db); err != nil {
				t.Fatal(err)
			}
			token := strings.Repeat("h", 43)
			if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'old','admin',0,'{\"rule\":{\"Count\":1,\"Until\":\"0001-01-01T00:00:00Z\"}}',1)"); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
				t.Fatal(err)
			}
			start := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
			horizon := start.AddDate(1, 0, 0)
			instant := horizon.Add(tc.offset)
			if _, err = db.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,?,?,?)", instant.Unix(), instant.Unix(), instant.Add(time.Minute).Unix()); err != nil {
				t.Fatal(err)
			}
			rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 364, Count: 2, Duration: 48 * time.Hour}
			err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "new", Organizer: "admin"}, rule)
			if tc.conflict && !errors.Is(err, errSeriesConflict) || !tc.conflict && err != nil {
				t.Fatalf("conflict=%v error=%v", tc.conflict, err)
			}
		})
	}
}

func TestSeriesMigrationVersion10(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'old','admin',0,'{}',1); DROP TABLE series_audit; DROP TABLE room_scan_status; DROP TABLE materialization_coverage; DROP TABLE conflict_scan_status; DROP TABLE conflict_alerts; DROP TABLE occurrence_audit; DROP TABLE series_occurrences; CREATE TABLE series_occurrences (series_id INTEGER NOT NULL REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), PRIMARY KEY(series_id,starts)); CREATE INDEX series_occurrences_time ON series_occurrences(starts,ends); INSERT INTO series_occurrences VALUES(1,100,200),(1,300,400); ALTER TABLE booking_series DROP COLUMN canceled; ALTER TABLE booking_series DROP COLUMN version; ALTER TABLE booking_series DROP COLUMN checked_through; DROP INDEX devices_public_id; ALTER TABLE devices DROP COLUMN public_id; PRAGMA user_version=10`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var horizon, count, version int
	if err = db.QueryRow("SELECT checked_through,(SELECT COUNT(*) FROM series_occurrences), (SELECT user_version FROM pragma_user_version) FROM booking_series WHERE id=1").Scan(&horizon, &count, &version); err != nil || horizon != 400 || count != 2 || version != 16 {
		t.Fatalf("migration horizon=%d count=%d version=%d err=%v", horizon, count, version, err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
}

func TestFarSeriesDSTEndFailsAtomically(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	rule := recurrence.Rule{Start: time.Date(2028, 3, 10, 2, 30, 0, 0, loc), Location: loc, Frequency: recurrence.Monthly, Interval: 1, MonthDay: 10, Count: 25, Duration: time.Hour}
	err = storeSeries(db, "token", "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule)
	if !errors.Is(err, recurrence.ErrLocalTime) {
		t.Fatalf("expected DST gap at final occurrence: %v", err)
	}
	var n int
	if err = db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial series: %d %v", n, err)
	}
}

func BenchmarkStoreLongSeries(b *testing.B) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(b.TempDir(), "bench.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		b.Fatal(err)
	}
	token := strings.Repeat("b", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		b.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		b.Fatal(err)
	}
	rule := recurrence.Rule{Start: time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 10000, Duration: time.Hour}
	booking := booking{RoomID: 1, Title: "series", Organizer: "admin"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = storeSeries(db, token, "csrf", booking, rule); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if _, err = db.Exec("DELETE FROM series_occurrences; DELETE FROM booking_series"); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func TestFarSeriesAndSingleCompete(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "far.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("x", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES ('room'); INSERT INTO users(username,salt,password_hash) VALUES ('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).AddDate(0, 0, 1)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 800, Duration: time.Hour}
	far := start.Add(700 * 24 * time.Hour)
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	begin := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	wg.Go(func() {
		<-begin
		err := storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "series", Organizer: "admin"}, rule)
		if err == nil {
			codes <- 201
		} else if errors.Is(err, errSeriesConflict) {
			codes <- 409
		} else {
			t.Errorf("series: %v", err)
			codes <- 500
		}
	})
	wg.Go(func() {
		<-begin
		body := fmt.Sprintf(`{"room_id":1,"title":"single","organizer":"admin","starts":%d,"ends":%d}`, far.Unix(), far.Add(time.Hour).Unix())
		r := httptest.NewRequest(http.MethodPost, "https://example.test/api/bookings", strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		codes <- w.Code
	})
	close(begin)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("results: %v", counts)
	}
	var series, occurrences, singles int
	if err := db.QueryRow("SELECT (SELECT COUNT(*) FROM booking_series), (SELECT COUNT(*) FROM series_occurrences), (SELECT COUNT(*) FROM bookings)").Scan(&series, &occurrences, &singles); err != nil {
		t.Fatal(err)
	}
	if series == 1 && (occurrences != 800 || singles != 0) || series == 0 && (occurrences != 0 || singles != 1) {
		t.Fatalf("partial write: series=%d occurrences=%d singles=%d", series, occurrences, singles)
	}
}

func TestUnboundedMaterializationShiftAndRollback(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "materialize.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	encoded, err := json.Marshal(struct {
		Rule     recurrence.Rule `json:"rule"`
		Location string          `json:"location"`
	}{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'s','admin',0,?,1,0)", string(encoded)); err != nil {
		t.Fatal(err)
	}
	if err = materializeUnbounded(db, start, start.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err = materializeUnbounded(db, start.AddDate(0, 0, 1), start.AddDate(1, 0, 1)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences").Scan(&count); err != nil || count < 365 {
		t.Fatalf("count=%d %v", count, err)
	}
	if err = materializeUnbounded(db, time.Date(2400, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(2402, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("out of range accepted")
	}
}
func TestUnboundedSeriesAPIFutureBookingConflict(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "unbounded.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := fmt.Sprintf(`{"room_id":1,"title":"series","organizer":"admin","starts":%d,"duration_seconds":3600,"frequency":1,"interval":1}`, start.Unix())
	if w := request("/api/series", body); w.Code != 201 {
		t.Fatalf("unbounded creation %d %s", w.Code, w.Body.String())
	}
	far := start.AddDate(5, 0, 0)
	single := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, far.Unix(), far.Add(time.Hour).Unix())
	if w := request("/api/bookings", single); w.Code != 409 {
		t.Fatalf("far booking %d %s", w.Code, w.Body.String())
	}
}

func TestMaterializeUnboundedDSTFailureRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "dst.db")+"?_pragma=busy_timeout(5000)")
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
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2024, 3, 9, 2, 30, 0, 0, zone)
	rule := recurrence.Rule{Start: start, Location: zone, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	encoded, err := json.Marshal(storedSeriesRule{rule, zone.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(encoded)); err != nil {
		t.Fatal(err)
	}
	if err = materializeUnbounded(db, start, start.Add(72*time.Hour)); !errors.Is(err, recurrence.ErrLocalTime) {
		t.Fatalf("DST error %v", err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial occurrences %d %v", count, err)
	}
}

func TestUnboundedDisplayRequiresCoverage(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "display.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("d", 43)
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	rule := recurrence.Rule{Start: now.Add(-time.Hour), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Minute}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	for _, endpoint := range []string{"/api/kiosk", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		r := httptest.NewRequest("GET", "https://example.test"+endpoint, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == 200 && !strings.Contains(w.Body.String(), fmt.Sprintf(`"valid_until":%d`, now.Unix())) {
			t.Fatalf("uncovered display %s %d %s", endpoint, w.Code, w.Body.String())
		}
	}
}

func TestUnboundedBookingExpansionAtFarTime(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "far.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	far := start.AddDate(5, 0, 0)
	body := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, far.Unix(), far.Add(time.Hour).Unix())
	r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
	r.Header.Set("Origin", "https://example.test")
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("far booking %d %s", w.Code, w.Body.String())
	}
}

func TestUnboundedBookingDSTFailsClosed(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "dst-api.db")+"?_pragma=busy_timeout(5000)")
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
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2024, 3, 9, 2, 30, 0, 0, zone)
	rule := recurrence.Rule{Start: start, Location: zone, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, zone.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2024, 3, 10, 6, 0, 0, 0, time.UTC).Unix()
	body := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, from, from+7200)
	r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
	r.Header.Set("Origin", "https://example.test")
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("DST booking %d %s", w.Code, w.Body.String())
	}
	var n int
	if err = db.QueryRow("SELECT COUNT(*) FROM bookings").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial %d %v", n, err)
	}
}

func TestConcurrentUnboundedBooking(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "race.db") + "?_pragma=busy_timeout(5000)"
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
	if _, err = a.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = a.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	far := start.AddDate(5, 0, 0)
	body := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, far.Unix(), far.Add(time.Hour).Unix())
	gate := make(chan struct{})
	codes := make(chan int, 2)
	for _, db := range []*sql.DB{a, b} {
		go func(db *sql.DB) {
			<-gate
			r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
			r.Header.Set("Origin", "https://example.test")
			r.Header.Set("X-CSRF-Token", "csrf")
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
			w := httptest.NewRecorder()
			handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
			codes <- w.Code
		}(db)
	}
	close(gate)
	for i := 0; i < 2; i++ {
		if code := <-codes; code != 409 {
			t.Fatalf("booking %d", code)
		}
	}
}

func TestDisplayReadSnapshotDuringMaterialization(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "snapshot.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
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
	if _, err = a.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	tx, err := a.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRow("SELECT COUNT(*) FROM series_occurrences").Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err = materializeUnbounded(b, start, start.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	coverage, err := displayCoverage(tx, 1, start.Unix())
	if err != nil || !coverage.Valid || coverage.Int64 != start.Unix() {
		t.Fatalf("mixed snapshot %v %v", coverage, err)
	}
}

func TestMaterializationCoverageSurvivesRestartAndGap(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "coverage.db")
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
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	end := start.AddDate(1, 0, 0)
	if err = materializeUnbounded(db, start, end); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := displayCoverage(db, 1, start.Unix())
	if err != nil || !c.Valid || c.Int64 != end.Unix() {
		t.Fatalf("restart coverage %v %v", c, err)
	}
	gap := end.AddDate(0, 0, 1)
	c, err = displayCoverage(db, 1, gap.Unix())
	if err != nil || !c.Valid || c.Int64 != gap.Unix() {
		t.Fatalf("gap coverage %v %v", c, err)
	}
}

func TestFiniteSeriesConflictsWithUnboundedAtFutureHorizon(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "finite.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	infinite := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{infinite, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	finite := infinite
	finite.Start = start.AddDate(5, 0, 0)
	finite.Count = 1
	if err = storeSeries(db, token, "csrf", booking{RoomID: 1, Title: "finite", Organizer: "admin"}, finite); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("finite conflict %v", err)
	}
}

func TestDisplayCoverageIsScopedToRoomAndMovedExceptions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('one'),('two'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	rule := recurrence.Rule{Start: now, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'a','admin',0,?,1),(2,'b','admin',0,?,1)", string(raw), string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO materialization_coverage VALUES(1,?,?),(2,?,?)", now.Unix()-10, now.Unix()+3600, now.Unix()+100, now.Unix()+3600); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE conflict_scan_status SET last_success=?,checked_from=?,checked_through=? WHERE id=1; INSERT INTO room_scan_status(room_id,last_success,checked_from,checked_through) VALUES(1,?,?,?)", now.Unix(), now.Unix()-10, now.Unix()+3600, now.Unix(), now.Unix()-10, now.Unix()+3600); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("d", 43)
	if _, err = db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	get := func() string {
		r := httptest.NewRequest("GET", "https://example.test/api/kiosk", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	if body := get(); !strings.Contains(body, fmt.Sprintf(`"valid_until":%d`, now.Unix()+3600)) {
		t.Fatalf("other room blocks: %s", body)
	}
	if _, err = db.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends,room_id,version) VALUES(2,?,?,?,1,2)", now.Unix()+200, now.Unix()+200, now.Unix()+300); err != nil {
		t.Fatal(err)
	}
	if body := get(); !strings.Contains(body, fmt.Sprintf(`"starts":%d`, now.Unix()+200)) {
		t.Fatalf("moved occurrence missing: %s", body)
	}
}

func TestUnboundedDSTOtherRoomDoesNotBlockBooking(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "rooms.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('one'),('two'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2024, 3, 9, 2, 30, 0, 0, zone)
	rule := recurrence.Rule{Start: start, Location: zone, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, zone.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2024, 3, 10, 6, 0, 0, 0, time.UTC).Unix()
	body := fmt.Sprintf(`{"room_id":2,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, from, from+7200)
	r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
	r.Header.Set("Origin", "https://example.test")
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("unrelated room booking %d %s", w.Code, w.Body.String())
	}
}

func TestMovedExceptionFromOtherRoomBlocksBooking(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "moved.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('one'),('two'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2024, 3, 9, 2, 30, 0, 0, zone)
	rule := recurrence.Rule{Start: start, Location: zone, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, zone.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'s','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2024, 3, 10, 6, 0, 0, 0, time.UTC).Unix()
	if _, err = db.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends,room_id,version) VALUES(1,?,?,?,2,2)", start.Unix(), from, from+3600); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"room_id":2,"title":"one","organizer":"admin","starts":%d,"ends":%d}`, from, from+7200)
	r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
	r.Header.Set("Origin", "https://example.test")
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("moved conflict %d %s", w.Code, w.Body.String())
	}
}

func TestFutureUnboundedKioskCoverageRoomScoped(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "future-kiosk.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('one'),('two'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("d", 43)
	if _, err = db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	future := now.AddDate(0, 6, 0)
	rule := recurrence.Rule{Start: future, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}
	raw, err := json.Marshal(storedSeriesRule{rule, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'future','admin',0,?,1)", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO materialization_coverage(series_id,starts,ends) VALUES(1,?,?)", future.Unix(), future.AddDate(1, 0, 0).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE conflict_scan_status SET last_success=?,checked_from=?,checked_through=? WHERE id=1; INSERT INTO room_scan_status(room_id,last_success,checked_from,checked_through) VALUES(1,?,?,?)", now.Unix(), now.Unix()-10, now.AddDate(1, 0, 0).Unix(), now.Unix(), now.Unix()-10, now.AddDate(1, 0, 0).Unix()); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "https://example.test/api/kiosk", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handlerWithAuth(db, "https://example.test", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf(`"valid_until":%d`, future.Unix())) {
		t.Fatalf("future coverage %d %s", w.Code, w.Body.String())
	}
}
