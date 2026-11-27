package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
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

type sessionCheck func(http.ResponseWriter, *http.Request) (string, string, string, bool)

func registerUserHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query("SELECT id, username, active FROM users ORDER BY id")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		users := make([]struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Active   bool   `json:"active"`
		}, 0)
		for rows.Next() {
			var user struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Active   bool   `json:"active"`
			}
			if err := rows.Scan(&user.ID, &user.Username, &user.Active); err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			users = append(users, user)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
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
			var target int64
			if action != "create" {
				var err error
				target, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
				if err != nil || target <= 0 {
					http.Error(w, "invalid request", 400)
					return
				}
			}
			var input struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			var salt, hash []byte
			if action != "disable" {
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				decoder.DisallowUnknownFields()
				if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || len(input.Password) < 12 || len(input.Password) > 1024 || (action == "create" && (input.Username == "" || len(input.Username) > 128 || strings.TrimSpace(input.Username) != input.Username)) || (action == "password" && input.Username != "") {
					http.Error(w, "invalid request", 400)
					return
				}
				salt = make([]byte, 16)
				if _, err := rand.Read(salt); err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				var err error
				hash, err = pbkdf2.Key(sha256.New, input.Password, salt, passwordIterations, 32)
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
			}
			tx, err := db.Begin()
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			defer tx.Rollback()
			// Acquire the SQLite write lock before revalidating authorization and invariants.
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
			if action == "create" {
				var exists int
				if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", input.Username).Scan(&exists); err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				if exists != 0 {
					http.Error(w, "conflict", 409)
					return
				}
				var result sql.Result
				result, err = tx.Exec("INSERT INTO users (username, salt, password_hash) VALUES (?, ?, ?)", input.Username, salt, hash)
				if err == nil {
					target, err = result.LastInsertId()
				}
			} else {
				var active bool
				err = tx.QueryRow("SELECT active FROM users WHERE id = ?", target).Scan(&active)
				if errors.Is(err, sql.ErrNoRows) {
					http.Error(w, "not found", 404)
					return
				}
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				if !active {
					http.Error(w, "conflict", 409)
					return
				}
				if action == "disable" {
					var count int
					if err = tx.QueryRow("SELECT COUNT(*) FROM users WHERE active = 1").Scan(&count); err != nil {
						http.Error(w, "internal error", 500)
						return
					}
					if count <= 1 {
						http.Error(w, "conflict", 409)
						return
					}
					_, err = tx.Exec("UPDATE users SET active = 0 WHERE id = ?", target)
				} else {
					_, err = tx.Exec("UPDATE users SET salt = ?, password_hash = ? WHERE id = ?", salt, hash, target)
				}
				if err == nil {
					_, err = tx.Exec("DELETE FROM sessions WHERE user_id = ?", target)
				}
			}
			if err == nil {
				_, err = tx.Exec("INSERT INTO account_audit (actor_id, target_id, action, created_at) VALUES (?, ?, ?, ?)", actor, target, action, time.Now().Unix())
			}
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			if action == "create" {
				w.WriteHeader(http.StatusCreated)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		})
	}
	mux.HandleFunc("POST /api/users", mutate("create"))
	mux.HandleFunc("POST /api/users/{id}/disable", mutate("disable"))
	mux.HandleFunc("POST /api/users/{id}/password", mutate("password"))
}
