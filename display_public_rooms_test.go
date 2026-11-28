package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestPublicDisplayPerRoomPrivacy(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	c := defaultDisplayConfig()
	c.Blocks[2].RoomID = 1
	c.Blocks[3].RoomID = 2
	c.Blocks[4].RoomID = 1
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	_, err = db.Exec(`INSERT INTO rooms(id,name,active) VALUES(1,'available',1),(2,'inactive',0); INSERT INTO devices(name,public_id,display_config) VALUES('panel','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',?)`, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'secret','secret organizer',0,?,?)`, now+60, now+120)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	w := httptest.NewRecorder()
	handlerWithAuth(db, "", netip.Addr{}).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("public response exposed private fields")
	}
	var response struct {
		Rooms map[string]publicRoom `json:"rooms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rooms) != 2 || response.Rooms["1"].Room != "available" || len(response.Rooms["1"].Entries) != 1 || !response.Rooms["2"].Unavailable {
		t.Fatalf("unexpected rooms: %+v", response.Rooms)
	}
}
