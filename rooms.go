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

type room struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Active        bool   `json:"active"`
	ShowTitle     bool   `json:"show_title"`
	ShowOrganizer bool   `json:"show_organizer"`
}

func registerRoomHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	mux.HandleFunc("GET /api/rooms", func(w http.ResponseWriter, r *http.Request) {
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query("SELECT id, name, active, show_title, show_organizer FROM rooms ORDER BY id")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		rooms := []room{}
		for rows.Next() {
			var item room
			if rows.Scan(&item.ID, &item.Name, &item.Active, &item.ShowTitle, &item.ShowOrganizer) != nil {
				http.Error(w, "internal error", 500)
				return
			}
			rooms = append(rooms, item)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rooms)
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
			var input struct {
				Name          string `json:"name"`
				Active        *bool  `json:"active"`
				ShowTitle     bool   `json:"show_title"`
				ShowOrganizer bool   `json:"show_organizer"`
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.Name == "" || strings.TrimSpace(input.Name) != input.Name || len(input.Name) > 128 || (!create && input.Active == nil) {
				http.Error(w, "invalid request", 400)
				return
			}
			var id int64
			var err error
			if !create {
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
			if _, err = tx.Exec("UPDATE users SET active = active WHERE 0"); err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			var actor int64
			err = tx.QueryRow("SELECT users.id FROM sessions JOIN users ON users.id = sessions.user_id WHERE sessions.token_hash = ? AND sessions.csrf = ? AND sessions.expires > ? AND users.active = 1", tokenHash(token), csrf, time.Now().Unix()).Scan(&actor)
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "unauthorized", 401)
				return
			}
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			var duplicate bool
			if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM rooms WHERE name = ? AND id != ?)", input.Name, id).Scan(&duplicate); err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			if duplicate {
				http.Error(w, "conflict", 409)
				return
			}
			if create {
				active := input.Active == nil || *input.Active
				_, err = tx.Exec("INSERT INTO rooms(name, active, show_title, show_organizer) VALUES (?, ?, ?, ?)", input.Name, active, input.ShowTitle, input.ShowOrganizer)
			} else {
				var result sql.Result
				result, err = tx.Exec("UPDATE rooms SET name = ?, active = ?, show_title = ?, show_organizer = ? WHERE id = ?", input.Name, *input.Active, input.ShowTitle, input.ShowOrganizer, id)
				if err == nil {
					var n int64
					n, err = result.RowsAffected()
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
				w.WriteHeader(201)
			} else {
				w.WriteHeader(204)
			}
		})
	}
	mux.HandleFunc("POST /api/rooms", mutate(true))
	mux.HandleFunc("POST /api/rooms/{id}", mutate(false))
}
