package main

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func TestWidgetRoomMigrationAndValidation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO rooms(id,name,active) VALUES(1,'old',0); INSERT INTO devices(id,room_id,name,token_hash,public_id,revoked) VALUES(7,1,'old',X'01','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1); PRAGMA user_version=16`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var raw string
	var room int64
	var revoked bool
	if err = db.QueryRow("SELECT display_config,room_id,revoked FROM devices WHERE id=7").Scan(&raw, &room, &revoked); err != nil {
		t.Fatal(err)
	}
	var c displayConfig
	if err = json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	if room != 1 || !revoked || !validDisplayConfig(c) {
		t.Fatalf("migration lost device: %+v", c)
	}
	for _, b := range c.Blocks {
		if b.Type == "room" || b.Type == "status" || b.Type == "upcoming" {
			if b.RoomID != 1 || b.ID == "" {
				t.Fatalf("missing room: %+v", b)
			}
		}
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	c.Blocks = append(c.Blocks, displayBlock{Type: "room", ID: "second", RoomID: 2, W: 1, H: 1})
	if !validDisplayConfig(c) {
		t.Fatal("multiple room widgets rejected")
	}
	c.Blocks[len(c.Blocks)-1].ID = "room"
	if validDisplayConfig(c) {
		t.Fatal("duplicate widget ID accepted")
	}
}
