package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestLegacyDisplayAndKioskCompatibility(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	if _, err = db.Exec(`INSERT INTO rooms(id,name) VALUES(1,'A'),(2,'B')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO devices(id,room_id,name,token_hash,public_id) VALUES(1,1,'panel',?,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`, tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	handler := handlerWithAuth(db, "", netip.Addr{})
	request := func(path string, kiosk bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if kiosk {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	display := request("/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false)
	var payload struct {
		Room       string                `json:"room"`
		Entries    []publicEntry         `json:"entries"`
		ValidUntil *int64                `json:"valid_until"`
		Now        int64                 `json:"now"`
		Rooms      map[string]publicRoom `json:"rooms"`
	}
	if display.Code != 200 || json.Unmarshal(display.Body.Bytes(), &payload) != nil || payload.Room != "A" || payload.Entries == nil || payload.Rooms["1"].Room != "A" {
		t.Fatalf("legacy display: %d %s", display.Code, display.Body.String())
	}
	if kiosk := request("/api/kiosk", true); kiosk.Code != 200 {
		t.Fatalf("legacy kiosk: %d %s", kiosk.Code, kiosk.Body.String())
	}
	c := defaultDisplayConfig()
	for i := range c.Blocks {
		if c.Blocks[i].ID != "" {
			c.Blocks[i].RoomID = 2
		}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE devices SET display_config=? WHERE id=1`, string(raw)); err != nil {
		t.Fatal(err)
	}
	display = request("/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false)
	if display.Code != 200 || json.Unmarshal(display.Body.Bytes(), &payload) != nil || payload.Room != "" || payload.ValidUntil == nil || *payload.ValidUntil != payload.Now || payload.Rooms["2"].Room != "B" {
		t.Fatalf("changed room: %d %s", display.Code, display.Body.String())
	}
	if kiosk := request("/api/kiosk", true); kiosk.Code != 409 || strings.Contains(kiosk.Body.String(), "A") {
		t.Fatalf("stale kiosk: %d %s", kiosk.Code, kiosk.Body.String())
	}
	if _, err = db.Exec(`UPDATE devices SET room_id=NULL WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if kiosk := request("/api/kiosk", true); kiosk.Code != 409 {
		t.Fatalf("roomless kiosk: %d", kiosk.Code)
	}
}
