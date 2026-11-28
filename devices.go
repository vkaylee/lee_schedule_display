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
		rows, err := db.Query("SELECT id,name,revoked,language,public_id FROM devices ORDER BY id")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type device struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Revoked  bool   `json:"revoked"`
			Language string `json:"language"`
			PublicID string `json:"public_id"`
		}
		devices := []device{}
		for rows.Next() {
			var d device
			if rows.Scan(&d.ID, &d.Name, &d.Revoked, &d.Language, &d.PublicID) != nil {
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
	mutate := func(action string) http.HandlerFunc {
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
			var id int64
			var err error
			if action == "create" {
				var input struct {
					Name string `json:"name"`
				}
				d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				d.DisallowUnknownFields()
				if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF || input.Name == "" || strings.TrimSpace(input.Name) != input.Name || len(input.Name) > 128 {
					http.Error(w, "invalid request", 400)
					return
				}
				name = input.Name
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
			if action == "create" {
				_, err = tx.Exec("INSERT INTO devices(name,public_id) VALUES(?,lower(hex(randomblob(16))))", name)
			} else {
				var result sql.Result
				if action == "reactivate" {
					result, err = tx.Exec("UPDATE devices SET revoked=0 WHERE id=? AND revoked=1", id)
				} else {
					result, err = tx.Exec("UPDATE devices SET revoked=1 WHERE id=? AND revoked=0", id)
				}
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
			if action == "create" {
				w.WriteHeader(201)
			} else {
				w.WriteHeader(204)
			}
		})
	}
	mux.HandleFunc("POST /api/devices", mutate("create"))
	mux.HandleFunc("POST /api/devices/{id}/revoke", mutate("revoke"))
	mux.HandleFunc("POST /api/devices/{id}/reactivate", mutate("reactivate"))
	mux.HandleFunc("PUT /api/devices/{id}/language", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		var input struct {
			Language string `json:"language"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if err != nil || id <= 0 || d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF || (input.Language != "vi" && input.Language != "en") {
			http.Error(w, "invalid request", 400)
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
		result, err := tx.Exec("UPDATE devices SET language=? WHERE id=? AND revoked=0", input.Language, id)
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
			http.Error(w, "not found", 404)
			return
		}
		if tx.Commit() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.WriteHeader(204)
	}))
}
