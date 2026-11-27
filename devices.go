package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func registerDeviceHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query("SELECT id,name,room_id,revoked,last_seen FROM devices ORDER BY id")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type device struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			RoomID   int64  `json:"room_id"`
			Revoked  bool   `json:"revoked"`
			LastSeen *int64 `json:"last_seen"`
		}
		devices := []device{}
		for rows.Next() {
			var d device
			if rows.Scan(&d.ID, &d.Name, &d.RoomID, &d.Revoked, &d.LastSeen) != nil {
				http.Error(w, "internal error", 500)
				return
			}
			devices = append(devices, d)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(devices)
	})
	mutate := func(create bool) http.HandlerFunc {
		return post(func(w http.ResponseWriter, r *http.Request) {
			token, _, csrf, ok := session(w, r)
			if !ok {
				return
			}
			if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
				http.Error(w, "forbidden", 403)
				return
			}
			var name string
			var roomID, id int64
			var deviceToken string
			var err error
			if create {
				var input struct {
					Name   string `json:"name"`
					RoomID int64  `json:"room_id"`
				}
				d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				d.DisallowUnknownFields()
				if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF || input.Name == "" || strings.TrimSpace(input.Name) != input.Name || len(input.Name) > 128 || input.RoomID <= 0 {
					http.Error(w, "invalid request", 400)
					return
				}
				name = input.Name
				roomID = input.RoomID
				deviceToken, err = randomToken()
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
			} else {
				id, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
				if err != nil || id <= 0 {
					http.Error(w, "invalid request", 400)
					return
				}
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
			if create {
				var active bool
				err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", roomID).Scan(&active)
				if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
					http.Error(w, "invalid room", 400)
					return
				}
				if err == nil {
					_, err = tx.Exec("INSERT INTO devices(name,room_id,token_hash) VALUES(?,?,?)", name, roomID, tokenHash(deviceToken))
				}
			} else {
				var result sql.Result
				result, err = tx.Exec("UPDATE devices SET revoked=1 WHERE id=? AND revoked=0", id)
				if err == nil {
					n, e := result.RowsAffected()
					err = e
					if err == nil && n == 0 {
						http.Error(w, "not found", 404)
						return
					}
				}
			}
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			if create {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				json.NewEncoder(w).Encode(map[string]string{"token": deviceToken})
			} else {
				w.WriteHeader(204)
			}
		})
	}
	mux.HandleFunc("POST /api/devices", mutate(true))
	mux.HandleFunc("POST /api/devices/{id}/revoke", mutate(false))
	mux.HandleFunc("GET /api/kiosk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") || len(authorization) != 50 {
			http.Error(w, "unauthorized", 401)
			return
		}
		token := strings.TrimPrefix(authorization, "Bearer ")
		now := time.Now().Unix()
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE devices SET last_seen=? WHERE token_hash=? AND revoked=0", now, tokenHash(token)); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		var roomID int64
		var roomName string
		var showTitle, showOrganizer bool
		err = tx.QueryRow("SELECT rooms.id,rooms.name,rooms.show_title,rooms.show_organizer FROM devices JOIN rooms ON rooms.id=devices.room_id WHERE devices.token_hash=? AND devices.revoked=0 AND rooms.active=1", tokenHash(token)).Scan(&roomID, &roomName, &showTitle, &showOrganizer)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unauthorized", 401)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		rows, err := tx.Query("SELECT starts,ends,private,title,organizer FROM bookings WHERE room_id=? AND canceled=0 AND ends>? ORDER BY starts,id LIMIT 101", roomID, now)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		type entry struct {
			Starts    int64  `json:"starts"`
			Ends      int64  `json:"ends"`
			Title     string `json:"title,omitempty"`
			Organizer string `json:"organizer,omitempty"`
		}
		entries := []entry{}
		for rows.Next() {
			var e entry
			var private bool
			var title, organizer string
			if rows.Scan(&e.Starts, &e.Ends, &private, &title, &organizer) != nil {
				rows.Close()
				http.Error(w, "internal error", 500)
				return
			}
			if !private && showTitle {
				e.Title = title
			}
			if !private && showOrganizer {
				e.Organizer = organizer
			}
			entries = append(entries, e)
		}
		if rows.Err() != nil {
			rows.Close()
			http.Error(w, "internal error", 500)
			return
		}
		rows.Close()
		var validUntil *int64
		if len(entries) > 100 {
			cutoff := entries[100].Starts
			validUntil = &cutoff
			entries = entries[:100]
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Room       string  `json:"room"`
			Now        int64   `json:"now"`
			Entries    []entry `json:"entries"`
			ValidUntil *int64  `json:"valid_until"`
		}{roomName, now, entries, validUntil})
	})
}
