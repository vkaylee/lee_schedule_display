package main

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type publicEntry struct {
	Starts int64 `json:"starts"`
	Ends   int64 `json:"ends"`
}
type publicRoom struct {
	Room        string        `json:"room"`
	Entries     []publicEntry `json:"entries"`
	ValidUntil  *int64        `json:"valid_until"`
	Unavailable bool          `json:"unavailable"`
}

func readDisplayConfig(raw string, roomID int64) (displayConfig, error) {
	c := defaultDisplayConfig()
	if raw == "" {
		for i := range c.Blocks {
			b := &c.Blocks[i]
			if b.Type == "room" || b.Type == "status" || b.Type == "upcoming" {
				b.ID, b.RoomID = b.Type, roomID
			}
		}
	} else if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, err
	}
	if !validDisplayConfig(c) {
		return c, errors.New("invalid display config")
	}
	return c, nil
}

func loadPublicRoom(tx *sql.Tx, roomID, now int64) publicRoom {
	unavailable := publicRoom{Entries: []publicEntry{}, Unavailable: true}
	var result publicRoom
	var active bool
	if err := tx.QueryRow("SELECT name,active FROM rooms WHERE id=?", roomID).Scan(&result.Room, &active); err != nil || !active {
		return unavailable
	}
	rows, err := tx.Query("SELECT starts,ends FROM (SELECT starts,ends FROM bookings WHERE room_id=? AND canceled=0 AND ends>? UNION ALL SELECT o.starts,o.ends FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE s.canceled=0 AND COALESCE(o.room_id,s.room_id)=? AND o.canceled=0 AND o.ends>?) ORDER BY starts,ends LIMIT 101", roomID, now, roomID, now)
	if err != nil {
		return unavailable
	}
	result.Entries = []publicEntry{}
	for rows.Next() {
		var e publicEntry
		if rows.Scan(&e.Starts, &e.Ends) != nil {
			rows.Close()
			return unavailable
		}
		result.Entries = append(result.Entries, e)
	}
	if rows.Err() != nil {
		rows.Close()
		return unavailable
	}
	if rows.Close() != nil {
		return unavailable
	}
	if len(result.Entries) > 100 {
		cutoff := result.Entries[100].Starts
		result.ValidUntil = &cutoff
		result.Entries = result.Entries[:100]
	}
	coverage, err := trustedDisplayCoverage(tx, roomID, now)
	if err != nil {
		return unavailable
	}
	if coverage.Valid && (result.ValidUntil == nil || coverage.Int64 < *result.ValidUntil) {
		result.ValidUntil = &coverage.Int64
	}
	return result
}
