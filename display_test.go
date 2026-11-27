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

func TestDisplayPublicPrivacyAndConfiguration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	token := strings.Repeat("a", 43)
	if _, err := db.Exec(`INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name,show_title,show_organizer) VALUES('Room',1,1); INSERT INTO devices(room_id,name,token_hash) VALUES(1,'panel',X'01');`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(token_hash,user_id,csrf,expires) VALUES(?,1,'csrf',?)`, tokenHash(token), now+3600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'private title','private organizer',0,?,?)`, now+300, now+600); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body string, admin bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		if admin {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
			r.Header.Set("X-CSRF-Token", "csrf")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/api/display/1", "", false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private title") || strings.Contains(w.Body.String(), "private organizer") || !strings.Contains(w.Body.String(), `"room":"Room"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
	config := defaultDisplayConfig()
	config.Organization = "Example"
	raw, _ := json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), false); w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/display/1", "", false); w.Code != 200 || !strings.Contains(w.Body.String(), "Example") {
		t.Fatal(w.Code, w.Body.String())
	}
	config.Blocks[1].X = 0
	raw, _ = json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec("UPDATE devices SET revoked=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/api/display/1", "", false); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
