package main

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"lee_schedule_display/recurrence"
)

func TestMovedUnboundedOccurrenceLimitsPublicRoomCoverage(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(storedSeriesRule{recurrence.Rule{Start: now, Location: time.UTC, Frequency: recurrence.Daily, Interval: 1, Duration: time.Hour}, "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rooms(name) VALUES('source'),('destination'),('unrelated');
		INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02');
		INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'series','admin',0,?,1)`, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends,room_id) VALUES(1,?,?,?,2)", now.Unix()+100, now.Unix()+100, now.Unix()+200)
	if err != nil {
		t.Fatal(err)
	}
	check := func(room int64, want *int64) {
		t.Helper()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		got := loadPublicRoom(tx, room, now.Unix())
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if got.Unavailable || (got.ValidUntil == nil) != (want == nil) || want != nil && *got.ValidUntil != *want {
			t.Fatalf("room %d: got %+v, want valid_until %v", room, got, want)
		}
	}
	cutoff := now.Unix()
	check(2, &cutoff)
	check(3, nil)
	if _, err = db.Exec("INSERT INTO materialization_coverage VALUES(1,?,?)", now.Unix()-1, now.Unix()+3600); err != nil {
		t.Fatal(err)
	}
	cutoff = now.Unix() + 3600
	check(2, &cutoff)
	if _, err = db.Exec("INSERT INTO room_scan_status(room_id,failed) VALUES(2,1)"); err != nil {
		t.Fatal(err)
	}
	cutoff = now.Unix()
	check(2, &cutoff)
	if _, err = db.Exec("UPDATE series_occurrences SET canceled=1 WHERE series_id=1"); err != nil {
		t.Fatal(err)
	}
	check(2, nil)
}
