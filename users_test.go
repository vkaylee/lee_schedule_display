package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAccountLanguageMigration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP INDEX devices_public_id; ALTER TABLE devices DROP COLUMN public_id; ALTER TABLE users DROP COLUMN language; ALTER TABLE devices DROP COLUMN display_config; DROP TABLE series_occurrences; DROP TABLE booking_series; PRAGMA user_version = 7;
INSERT INTO users(username,salt,password_hash) VALUES('existing',X'01',X'02');
INSERT INTO rooms(name) VALUES('room');
INSERT INTO devices(room_id,name,token_hash,language) VALUES(1,'device',X'03','en')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var accountLanguage, deviceLanguage string
	if err := db.QueryRow(`SELECT users.language, devices.language FROM users, devices`).Scan(&accountLanguage, &deviceLanguage); err != nil {
		t.Fatal(err)
	}
	if accountLanguage != "vi" || deviceLanguage != "en" {
		t.Fatalf("languages: %q %q", accountLanguage, deviceLanguage)
	}
	if _, err := db.Exec(`UPDATE users SET language = 'fr'`); err == nil {
		t.Fatal("invalid language accepted")
	}
}

func TestUserManagement(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, "admin", "long-password-123"); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Header.Set("Origin", "https://example.test")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/users", "", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	login := request("POST", "/api/login", `{"username":"admin","password":"long-password-123"}`, nil, "")
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookie := login.Result().Cookies()[0]
	crossOrigin := httptest.NewRequest("PUT", "https://example.test/api/session/language", strings.NewReader(`{"language":"en"}`))
	crossOrigin.Header.Set("Origin", "https://evil.test")
	crossOrigin.Header.Set("X-CSRF-Token", session.CSRF)
	crossOrigin.AddCookie(cookie)
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, crossOrigin)
	if blocked.Code != 403 {
		t.Fatal(blocked.Code)
	}
	if w := request("PUT", "/api/session/language", `{"language":"en"}`, nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("PUT", "/api/session/language", `{"language":"en"}`, cookie, "wrong"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("PUT", "/api/session/language", `{"language":"fr"}`, cookie, session.CSRF); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request("PUT", "/api/session/language", `{"language":"en"}`, cookie, session.CSRF); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/api/session", "", cookie, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/users", `{"username":"other","password":"long-password-456"}`, cookie, "wrong"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/users/1/disable", "", cookie, session.CSRF); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/users", `{"username":"other","password":"long-password-456"}`, cookie, session.CSRF); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/users", `{"username":"other","password":"long-password-456"}`, cookie, session.CSRF); w.Code != 409 {
		t.Fatal(w.Code)
	}
	other := request("POST", "/api/login", `{"username":"other","password":"long-password-456"}`, nil, "")
	if other.Code != 200 {
		t.Fatal(other.Code)
	}
	otherCookie := other.Result().Cookies()[0]
	if w := request("POST", "/api/users/2/password", `{"password":"new-password-789"}`, cookie, session.CSRF); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/api/session", "", otherCookie, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/users/2/disable", "", cookie, session.CSRF); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/users/1/disable", "", cookie, session.CSRF); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/login", `{"username":"other","password":"new-password-789"}`, nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM account_audit").Scan(&count); err != nil || count != 3 {
		t.Fatalf("audit count %d, %v", count, err)
	}
}

func TestConcurrentDisablePreservesLastAccount(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users (username, salt, password_hash) VALUES ('first', X'01', X'02'), ('second', X'01', X'02')"); err != nil {
		t.Fatal(err)
	}
	for i, token := range []string{strings.Repeat("a", 43), strings.Repeat("b", 43)} {
		if _, err := db.Exec("INSERT INTO sessions VALUES (?, ?, ?, ?)", tokenHash(token), i+1, "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, item := range []struct{ token, path string }{{strings.Repeat("a", 43), "/api/users/2/disable"}, {strings.Repeat("b", 43), "/api/users/1/disable"}} {
		wg.Go(func() {
			<-start
			r := httptest.NewRequest("POST", "https://example.test"+item.path, nil)
			r.Header.Set("Origin", "https://example.test")
			r.Header.Set("X-CSRF-Token", "csrf")
			// Session tokens have a fixed encoded length.
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: item.token})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			codes <- w.Code
		})
	}
	close(start)
	wg.Wait()
	close(codes)
	var success int
	for code := range codes {
		if code == http.StatusNoContent {
			success++
		} else if code != http.StatusUnauthorized && code != http.StatusConflict {
			t.Fatalf("unexpected status %d", code)
		}
	}
	var active int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE active = 1").Scan(&active); err != nil || active != 1 || success != 1 {
		t.Fatalf("active=%d success=%d err=%v", active, success, err)
	}
}

func TestMigrationFromVersionOne(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, salt BLOB NOT NULL, password_hash BLOB NOT NULL);
CREATE TABLE sessions (token_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), csrf TEXT NOT NULL, expires INTEGER NOT NULL);
INSERT INTO users VALUES (1, 'admin', X'01', X'02'); PRAGMA user_version = 1`)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var active bool
	if err := db.QueryRow("SELECT active FROM users WHERE id = 1").Scan(&active); err != nil || !active {
		t.Fatalf("active=%v err=%v", active, err)
	}
}
