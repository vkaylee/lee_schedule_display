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
	if w = request("POST", "/api/devices/1/revoke", "", true, ""); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = request("GET", "/api/kiosk", "", false, result.Token); w.Code != 401 {
		t.Fatal(w.Code)
	}
}
