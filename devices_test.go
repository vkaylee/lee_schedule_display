package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestDeviceLanguageMigrationFromVersion6(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO rooms(name) VALUES('room'); INSERT INTO devices(room_id,name,token_hash) VALUES(1,'panel',X'01'); ALTER TABLE devices DROP COLUMN language; ALTER TABLE devices DROP COLUMN display_config; ALTER TABLE users DROP COLUMN language; PRAGMA user_version=6"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var language string
	var version int
	if err := db.QueryRow("SELECT language FROM devices WHERE id=1").Scan(&language); err != nil || language != "vi" {
		t.Fatalf("language=%q error=%v", language, err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 9 {
		t.Fatalf("version=%d error=%v", version, err)
	}
}

func TestKioskScheduleLimitHasSafeHorizon(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO rooms(name) VALUES('room')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err := db.Exec("INSERT INTO devices(room_id,name,token_hash) VALUES(1,'panel',?)", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Unix() + 3600
	for i := range 101 {
		if _, err := db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'meeting','organizer',1,?,?)", start+int64(i)*120, start+int64(i)*120+60); err != nil {
			t.Fatal(err)
		}
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	r := httptest.NewRequest("GET", "https://example.test/api/kiosk", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var payload struct {
		Entries []struct {
			Starts int64 `json:"starts"`
		} `json:"entries"`
		ValidUntil *int64 `json:"valid_until"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || len(payload.Entries) != 100 || payload.ValidUntil == nil || *payload.ValidUntil != start+12000 {
		t.Fatalf("status=%d count=%d horizon=%v", w.Code, len(payload.Entries), payload.ValidUntil)
	}
	if strings.Contains(w.Body.String(), "meeting") || strings.Contains(w.Body.String(), "organizer") {
		t.Fatal("private details exposed")
	}
}

func TestDeviceRoomPrivacyAndRevocation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name,show_title,show_organizer) VALUES('one',1,1),('two',1,1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if _, err = db.Exec(`INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'secret','secret person',1,?,?),(2,'other room','other person',0,?,?)`, now, now+3600, now, now+3600); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES(?,1,'csrf',?)", tokenHash(token), now+3600); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body string, admin bool, device string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		if admin {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
			r.Header.Set("X-CSRF-Token", "csrf")
		}
		if device != "" {
			r.Header.Set("Authorization", "Bearer "+device)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("POST", "/api/devices", `{"name":"panel","room_id":1}`, false, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request("POST", "/api/devices", `{"name":"panel","room_id":1}`, true, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil || len(result.Token) != 43 {
		t.Fatal(err)
	}
	w = request("GET", "/api/kiosk", "", false, result.Token)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "other room") {
		t.Fatal(w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"language":"vi"`) {
		t.Fatal(w.Body.String())
	}
	if w = request("PUT", "/api/devices/1/language", `{"language":"en"}`, false, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/devices/1/language", `{"language":"fr"}`, true, ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	noCSRF := httptest.NewRequest("PUT", "https://example.test/api/devices/1/language", strings.NewReader(`{"language":"en"}`))
	noCSRF.Header.Set("Origin", "https://example.test")
	noCSRF.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	forbidden := httptest.NewRecorder()
	h.ServeHTTP(forbidden, noCSRF)
	if forbidden.Code != 403 {
		t.Fatal(forbidden.Code)
	}
	if w = request("PUT", "/api/devices/1/language", `{"language":"en"}`, true, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	badOrigin := httptest.NewRequest("PUT", "https://example.test/api/devices/1/language", strings.NewReader(`{"language":"vi"}`))
	badOrigin.Header.Set("Origin", "https://other.test")
	badOrigin.Header.Set("X-CSRF-Token", "csrf")
	badOrigin.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	forbidden = httptest.NewRecorder()
	h.ServeHTTP(forbidden, badOrigin)
	if forbidden.Code != 403 {
		t.Fatal(forbidden.Code)
	}
	if w = request("GET", "/api/kiosk", "", false, result.Token); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/devices", "", true, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/kiosk", "", false, result.Token)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("kiosk response may be cached")
	}
	if _, err := db.Exec("UPDATE rooms SET active=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/api/kiosk", "", false, result.Token); w.Code != 401 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec("UPDATE rooms SET active=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("POST", "/api/devices/1/revoke", "", true, ""); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = request("GET", "/api/kiosk", "", false, result.Token); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/devices/1/language", `{"language":"vi"}`, true, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
