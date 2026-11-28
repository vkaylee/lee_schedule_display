package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

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
	config := defaultDisplayConfig()
	for i := range config.Blocks {
		if config.Blocks[i].ID != "" {
			config.Blocks[i].RoomID = 1
		}
	}
	raw, _ := json.Marshal(config)
	if _, err := db.Exec("INSERT INTO devices(name,public_id,display_config) VALUES('panel','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',?)", string(raw)); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Unix() + 3600
	for i := range 101 {
		if _, err := db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'meeting','organizer',1,?,?)", start+int64(i)*120, start+int64(i)*120+60); err != nil {
			t.Fatal(err)
		}
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	r := httptest.NewRequest("GET", "https://example.test/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var result struct {
		Rooms map[string]publicRoom `json:"rooms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	payload := result.Rooms["1"]
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
	if _, err = db.Exec(`INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'series secret','series person',1,'{}',1),(2,'other series','other series person',0,'{}',1); INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,?,?,?),(2,?,?,?)`, now+100, now+100, now+200, now+100, now+100, now+200); err != nil {
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
	if w := request("POST", "/api/devices", `{"name":"panel"}`, false, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request("POST", "/api/devices", `{"name":"panel"}`, true, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	config := defaultDisplayConfig()
	for i := range config.Blocks {
		if config.Blocks[i].ID != "" {
			config.Blocks[i].RoomID = 1
		}
	}
	raw, _ := json.Marshal(config)
	if _, err = db.Exec("UPDATE devices SET display_config=? WHERE id=1", string(raw)); err != nil {
		t.Fatal(err)
	}
	var publicID string
	if err = db.QueryRow("SELECT public_id FROM devices WHERE id=1").Scan(&publicID); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/api/display/"+publicID, "", false, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "other room") || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, now+100)) {
		t.Fatal(w.Code, w.Body.String())
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
	if w = request("GET", "/api/display/"+publicID, "", false, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/devices", "", true, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/display/"+publicID, "", false, "")
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("kiosk response may be cached")
	}
	if _, err := db.Exec("UPDATE rooms SET active=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/api/display/"+publicID, "", false, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"unavailable":true`) || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec("UPDATE rooms SET active=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("POST", "/api/devices/1/revoke", "", true, ""); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = request("GET", "/api/display/"+publicID, "", false, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/devices/1/language", `{"language":"vi"}`, true, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if err := db.QueryRow("SELECT public_id FROM devices WHERE id=1").Scan(&publicID); err != nil || len(publicID) != 32 {
		t.Fatal("invalid public id", err)
	}
	if w = request("GET", "/api/display/"+publicID, "", false, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = request("POST", "/api/devices/999/reactivate", "", true, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = request("POST", "/api/devices/1/reactivate", "", false, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	noCSRF = httptest.NewRequest("POST", "https://example.test/api/devices/1/reactivate", nil)
	noCSRF.Header.Set("Origin", "https://example.test")
	noCSRF.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	forbidden = httptest.NewRecorder()
	h.ServeHTTP(forbidden, noCSRF)
	if forbidden.Code != 403 {
		t.Fatal(forbidden.Code)
	}
	w = request("POST", "/api/devices/1/reactivate", "", true, "")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/display/"+publicID, "", false, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request("GET", "/api/display/1", "", false, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = request("POST", "/api/devices/1/reactivate", "", true, ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
