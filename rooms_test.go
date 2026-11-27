package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRooms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users (username,salt,password_hash) VALUES ('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err := db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body string, authenticated, csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", "csrf")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/rooms", "", false, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/rooms", `{"name":"Room"}`, true, false); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/rooms", `{"name":" "}`, true, true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/rooms", `{"name":"Room"}`, true, true); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/rooms", `{"name":"Room"}`, true, true); w.Code != 409 {
		t.Fatal(w.Code)
	}
	w := request("GET", "/api/rooms", "", true, false)
	var rooms []room
	if err := json.Unmarshal(w.Body.Bytes(), &rooms); err != nil || len(rooms) != 1 || !rooms[0].Active || rooms[0].ShowTitle || rooms[0].ShowOrganizer {
		t.Fatalf("rooms=%+v err=%v", rooms, err)
	}
	if w := request("POST", "/api/rooms/1", `{"name":"New","active":false,"show_title":true,"show_organizer":true}`, true, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	db2, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var name string
	var active, title, organizer bool
	if err := db2.QueryRow("SELECT name,active,show_title,show_organizer FROM rooms WHERE id=1").Scan(&name, &active, &title, &organizer); err != nil || name != "New" || active || !title || !organizer {
		t.Fatalf("%s %v %v %v %v", name, active, title, organizer, err)
	}
}
