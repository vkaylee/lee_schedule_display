package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBookingConflictConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookings.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'),('admin2',X'03',X'04'); INSERT INTO rooms(name) VALUES('room')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	otherToken := strings.Repeat("b", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?),(?,2,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix(), tokenHash(otherToken), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	requestAs := func(path, body, credential string) int {
		r := httptest.NewRequest("POST", "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.Header.Set("If-Match", "1")
		if credential != "" {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: credential})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	request := func(path, body string, auth bool) int {
		credential := ""
		if auth {
			credential = token
		}
		return requestAs(path, body, credential)
	}
	body := func(start, end int64) string {
		return fmt.Sprintf(`{"room_id":1,"title":"meeting","organizer":"admin","private":true,"starts":%d,"ends":%d}`, start, end)
	}
	if code := request("/api/bookings", body(100, 200), false); code != 401 {
		t.Fatal(code)
	}
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, credential := range []string{token, otherToken} {
		wg.Go(func() { <-start; codes <- requestAs("/api/bookings", body(10000000000, 10000000100), credential) })
	}
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
	if code := request("/api/bookings", body(10000000100, 10000000200), true); code != 201 {
		t.Fatal(code)
	}
	if code := request("/api/bookings/1", strings.TrimSuffix(body(10000000150, 10000000250), "}")+`,"version":1}`, true); code != 409 {
		t.Fatal(code)
	}
	if code := request("/api/bookings/1", strings.TrimSuffix(body(10000000000, 10000000100), "}")+`,"version":1}`, true); code != 204 {
		t.Fatal(code)
	}
	if code := request("/api/bookings/1", strings.TrimSuffix(body(10000000000, 10000000100), "}")+`,"version":1}`, true); code != 409 {
		t.Fatal(code)
	}
	if code := request("/api/bookings/1/cancel", "", true); code != 409 {
		t.Fatal(code)
	}
	if code := request("/api/bookings/2/cancel", "", true); code != 204 {
		t.Fatal(code)
	}
	if code := request("/api/bookings", body(10000000100, 10000000200), true); code != 201 {
		t.Fatal(code)
	}
	if _, err = db.Exec("UPDATE rooms SET active=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM bookings").Scan(&count); err != nil || count != 3 {
		t.Fatalf("%d %v", count, err)
	}
	if code := request("/api/bookings", body(3, 4), true); code != 400 {
		t.Fatal(code)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.QueryRow("SELECT COUNT(*) FROM bookings WHERE canceled=0 AND ((starts=10000000000 AND ends=10000000100) OR (starts=10000000100 AND ends=10000000200))").Scan(&count); err != nil || count != 2 {
		t.Fatalf("after reopen: %d %v", count, err)
	}
}
