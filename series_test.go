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

func TestSeriesStorageAtomicConflicts(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "series.db")+"?_pragma=busy_timeout(5000)")
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
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	rule := recurrence.Rule{Start: start, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 3, Duration: time.Hour}
	b := booking{RoomID: 1, Title: "meeting", Organizer: "admin", Private: true}
	tooLong := rule
	tooLong.Count = 800
	if err = storeSeries(db, token, "csrf", b, tooLong); !errors.Is(err, errSeriesBeyondHorizon) {
		t.Fatalf("horizon: %v", err)
	}
	selfOverlap := rule
	selfOverlap.Duration = 25 * time.Hour
	if err = storeSeries(db, token, "csrf", b, selfOverlap); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("self conflict: %v", err)
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	dst := recurrence.Rule{Start: time.Date(2024, 3, 9, 7, 30, 0, 0, time.UTC), Location: loc, Frequency: recurrence.Daily, Interval: 1, Count: 4, Duration: time.Hour}
	if err = storeSeries(db, token, "csrf", b, dst); !errors.Is(err, recurrence.ErrLocalTime) {
		t.Fatalf("DST gap: %v", err)
	}
	if _, err = db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'x','x',0,?,?)", start.Add(24*time.Hour).Unix(), start.Add(25*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if err = storeSeries(db, token, "csrf", b, rule); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("conflict: %v", err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial series %d %v", count, err)
	}
	if _, err = db.Exec("DELETE FROM bookings"); err != nil {
		t.Fatal(err)
	}
	if err = storeSeries(db, token, "wrong", b, rule); !errors.Is(err, errSeriesUnauthorized) {
		t.Fatalf("invalid session: %v", err)
	}
	if err = storeSeries(db, token, "csrf", b, rule); err != nil {
		t.Fatal(err)
	}
	if err = storeSeries(db, token, "csrf", b, rule); !errors.Is(err, errSeriesConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM series_occurrences").Scan(&count); err != nil || count != 3 {
		t.Fatalf("occurrences %d %v", count, err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	seriesHandler := handlerWithTimezone(db, "https://example.test", netip.Addr{}, time.UTC)
	get := httptest.NewRequest("GET", "https://example.test/api/series", nil)
	get.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	list := httptest.NewRecorder()
	seriesHandler.ServeHTTP(list, get)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"title":"meeting"`) || !strings.Contains(list.Body.String(), fmt.Sprintf(`"checked_through":%d`, start.Add(49*time.Hour).Unix())) || !strings.Contains(list.Body.String(), fmt.Sprintf(`"starts":%d`, start.Unix())) {
		t.Fatalf("series list %d %s", list.Code, list.Body.String())
	}
	post := func(body string, authenticated bool, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://example.test/api/series", strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", csrf)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		}
		w := httptest.NewRecorder()
		seriesHandler.ServeHTTP(w, r)
		return w
	}
	for _, body := range []string{
		`{"room_id":1,"title":"weekly","organizer":"admin","starts":1893495600,"duration_seconds":3600,"frequency":2,"interval":1,"weekdays":[2,4],"count":3}`,
		`{"room_id":1,"title":"monthly","organizer":"admin","starts":1896076800,"duration_seconds":3600,"frequency":3,"interval":1,"month_day":31,"count":3}`,
		`{"room_id":1,"title":"nth","organizer":"admin","starts":1894017600,"duration_seconds":3600,"frequency":3,"interval":1,"nth":1,"nth_weekday":1,"until":"2030-03-31"}`,
	} {
		if w := post(body, true, "csrf"); w.Code != 201 {
			t.Fatalf("rule %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	payload := fmt.Sprintf(`{"room_id":1,"title":"other","organizer":"admin","private":true,"starts":%d,"duration_seconds":3600,"frequency":1,"interval":1,"count":2}`, start.Add(10*24*time.Hour).Unix())
	if w := post(payload, false, ""); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	if w := post(payload, true, ""); w.Code != 403 {
		t.Fatalf("csrf: %d", w.Code)
	}
	if w := post(payload, true, "csrf"); w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	long := fmt.Sprintf(`{"room_id":1,"title":"other","organizer":"admin","starts":%d,"duration_seconds":3600,"frequency":1,"interval":1,"count":800}`, start.Add(10*24*time.Hour).Unix())
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	dstHandler := handlerWithTimezone(db, "https://example.test", netip.Addr{}, zone)
	dstBody := `{"room_id":1,"title":"dst","organizer":"admin","starts":1709969400,"duration_seconds":3600,"frequency":1,"interval":1,"count":4}`
	dstRequest := httptest.NewRequest("POST", "https://example.test/api/series", strings.NewReader(dstBody))
	dstRequest.Header.Set("Origin", "https://example.test")
	dstRequest.Header.Set("X-CSRF-Token", "csrf")
	dstRequest.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	dstResponse := httptest.NewRecorder()
	dstHandler.ServeHTTP(dstResponse, dstRequest)
	if dstResponse.Code != 400 || !strings.Contains(dstResponse.Body.String(), "ambiguous") {
		t.Fatalf("DST: %d %s", dstResponse.Code, dstResponse.Body.String())
	}
	if w := post(long, true, "csrf"); w.Code != 400 || !strings.Contains(w.Body.String(), "12-month") {
		t.Fatalf("limit: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/bookings/1", "/api/bookings/1/cancel"} {
		r := httptest.NewRequest("POST", "https://example.test"+path, strings.NewReader(`{"room_id":1,"title":"changed","organizer":"admin","starts":100,"ends":200,"version":1}`))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.Header.Set("If-Match", "1")
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatalf("occurrence mutation %s: %d", path, w.Code)
		}
	}
	if _, err = db.Exec("INSERT INTO bookings(id,room_id,title,organizer,private,starts,ends) VALUES(1,1,'single','admin',0,100,200)"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/bookings/1", "/api/bookings/1/cancel"} {
		r := httptest.NewRequest("POST", "https://example.test"+path, strings.NewReader(`{"room_id":1,"title":"changed","organizer":"admin","starts":200,"ends":300,"version":1}`))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.Header.Set("If-Match", "1")
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("booking mutation %s: %d", path, w.Code)
		}
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=1 AND ((starts=? AND ends=?) OR (starts=? AND ends=?) OR (starts=? AND ends=?))", start.Unix(), start.Add(time.Hour).Unix(), start.Add(24*time.Hour).Unix(), start.Add(25*time.Hour).Unix(), start.Add(48*time.Hour).Unix(), start.Add(49*time.Hour).Unix()).Scan(&n); err != nil || n != 3 {
			t.Fatalf("series content changed: %d %v", n, err)
		}
		var title string
		var starts, ends int64
		var canceled bool
		if err := db.QueryRow("SELECT title,starts,ends,canceled FROM bookings WHERE id=1").Scan(&title, &starts, &ends, &canceled); err != nil || title != "changed" || starts != 200 || ends != 300 || canceled != (path == "/api/bookings/1/cancel") {
			t.Fatalf("booking changed incorrectly: %s %d %d %t %v", title, starts, ends, canceled, err)
		}
		if _, err := db.Exec("UPDATE bookings SET version=1 WHERE id=1"); err != nil {
			t.Fatal(err)
		}
	}
	body := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","private":false,"starts":%d,"ends":%d}`, start.Unix(), start.Add(time.Hour).Unix())
	r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
	r.Header.Set("Origin", "https://example.test")
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("single vs series: %d", w.Code)
	}
}

func TestSeriesConcurrent(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "race.db")+"?_pragma=busy_timeout(5000)")
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
	rule := recurrence.Rule{Start: time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC), Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Count: 3, Duration: time.Hour}
	b := booking{RoomID: 1, Title: "t", Organizer: "o"}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() { <-start; results <- storeSeries(db, token, "csrf", b, rule) })
	}
	close(start)
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
		t.Fatalf("success %d conflicts %d", success, conflicts)
	}
	if _, err := db.Exec("DELETE FROM series_occurrences; DELETE FROM booking_series"); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	start = make(chan struct{})
	codes := make(chan int, 2)
	wg.Go(func() {
		<-start
		err := storeSeries(db, token, "csrf", b, rule)
		if err == nil {
			codes <- 201
		} else if errors.Is(err, errSeriesConflict) {
			codes <- 409
		} else {
			codes <- 500
		}
	})
	wg.Go(func() {
		<-start
		body := fmt.Sprintf(`{"room_id":1,"title":"one","organizer":"admin","private":false,"starts":%d,"ends":%d}`, rule.Start.Unix(), rule.Start.Add(time.Hour).Unix())
		r := httptest.NewRequest("POST", "https://example.test/api/bookings", strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		codes <- w.Code
	})
	close(start)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
