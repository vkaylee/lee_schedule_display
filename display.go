package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type displayBlock struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}
type displayConfig struct {
	Organization string         `json:"organization"`
	Logo         string         `json:"logo"`
	Blocks       []displayBlock `json:"blocks"`
}

func defaultDisplayConfig() displayConfig {
	return displayConfig{Blocks: []displayBlock{{"clock", 0, 0, 3, 1}, {"branding", 3, 0, 9, 1}, {"room", 0, 1, 12, 1}, {"status", 0, 2, 7, 4}, {"upcoming", 7, 2, 5, 4}, {"footer", 0, 6, 12, 1}}}
}

func validDisplayConfig(c displayConfig) bool {
	if len(c.Organization) > 128 || len(c.Logo) > 256 || c.Blocks == nil || len(c.Blocks) > 6 {
		return false
	}
	if c.Logo != "" {
		u, err := url.Parse(c.Logo)
		if err != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(c.Logo, "/") || strings.HasPrefix(c.Logo, "//") || strings.ContainsAny(c.Logo, "\\%") {
			return false
		}
	}
	seen := map[string]bool{}
	for i, a := range c.Blocks {
		switch a.Type {
		case "clock", "branding", "room", "status", "upcoming", "footer":
		default:
			return false
		}
		if seen[a.Type] || a.X < 0 || a.Y < 0 || a.W < 1 || a.H < 1 || a.X > 11 || a.Y > 23 || a.W > 12-a.X || a.H > 24-a.Y {
			return false
		}
		seen[a.Type] = true
		for _, b := range c.Blocks[:i] {
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				return false
			}
		}
	}
	return true
}

func registerDisplayHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	idOf := func(r *http.Request) (int64, error) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if id <= 0 {
			return 0, errors.New("invalid id")
		}
		return id, err
	}
	mux.HandleFunc("GET /api/devices/{id}/display", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		id, err := idOf(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var raw string
		err = db.QueryRow("SELECT display_config FROM devices WHERE id=? AND revoked=0", id).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		c := defaultDisplayConfig()
		if raw != "" && (json.Unmarshal([]byte(raw), &c) != nil || !validDisplayConfig(c)) {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)
	})
	mux.HandleFunc("POST /api/devices/{id}/display", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		id, err := idOf(r)
		var c displayConfig
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if err != nil || d.Decode(&c) != nil || d.Decode(&struct{}{}) != io.EOF || !validDisplayConfig(c) {
			http.Error(w, "invalid request", 400)
			return
		}
		raw, err := json.Marshal(c)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		var actor int64
		err = tx.QueryRow("SELECT users.id FROM sessions JOIN users ON users.id=sessions.user_id WHERE sessions.token_hash=? AND sessions.csrf=? AND sessions.expires>? AND users.active=1", tokenHash(token), csrf, time.Now().Unix()).Scan(&actor)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unauthorized", 401)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		result, err := tx.Exec("UPDATE devices SET display_config=? WHERE id=? AND revoked=0", string(raw), id)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		if tx.Commit() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/display/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		id, err := idOf(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var raw, room, language string
		var roomID int64
		err = db.QueryRow("SELECT devices.display_config,rooms.id,rooms.name,devices.language FROM devices JOIN rooms ON rooms.id=devices.room_id WHERE devices.id=? AND devices.revoked=0 AND rooms.active=1", id).Scan(&raw, &roomID, &room, &language)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		c := defaultDisplayConfig()
		if raw != "" && (json.Unmarshal([]byte(raw), &c) != nil || !validDisplayConfig(c)) {
			http.Error(w, "internal error", 500)
			return
		}
		now := time.Now().Unix()
		rows, err := db.Query("SELECT starts,ends FROM bookings WHERE room_id=? AND canceled=0 AND ends>? ORDER BY starts,id LIMIT 101", roomID, now)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type entry struct {
			Starts int64 `json:"starts"`
			Ends   int64 `json:"ends"`
		}
		entries := []entry{}
		for rows.Next() {
			var e entry
			if rows.Scan(&e.Starts, &e.Ends) != nil {
				http.Error(w, "internal error", 500)
				return
			}
			entries = append(entries, e)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		var validUntil *int64
		if len(entries) > 100 {
			cutoff := entries[100].Starts
			validUntil = &cutoff
			entries = entries[:100]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Config     displayConfig `json:"config"`
			Room       string        `json:"room"`
			Language   string        `json:"language"`
			Now        int64         `json:"now"`
			Entries    []entry       `json:"entries"`
			ValidUntil *int64        `json:"valid_until"`
		}{c, room, language, now, entries, validUntil})
	})
}
