package main

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func insertTestDisplay(t *testing.T, db *sql.DB, roomID int64, publicID string) {
	t.Helper()
	config := defaultDisplayConfig()
	for i := range config.Blocks {
		if config.Blocks[i].ID != "" {
			config.Blocks[i].RoomID = roomID
		}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO devices(name,public_id,display_config) VALUES('panel',?,?)`, publicID, string(raw)); err != nil {
		t.Fatal(err)
	}
}
