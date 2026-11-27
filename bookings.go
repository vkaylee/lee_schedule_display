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

type booking struct {
	ID        int64  `json:"id"`
	RoomID    int64  `json:"room_id"`
	Title     string `json:"title"`
	Organizer string `json:"organizer"`
	Private   bool   `json:"private"`
	Starts    int64  `json:"starts"`
	Ends      int64  `json:"ends"`
	Canceled  bool   `json:"canceled"`
	Version   int64  `json:"version"`
}

func registerBookingHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	mux.HandleFunc("GET /api/bookings", func(w http.ResponseWriter, r *http.Request) {
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query("SELECT id,room_id,title,organizer,private,starts,ends,canceled,version FROM bookings ORDER BY starts,id")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		items := []booking{}
		for rows.Next() {
			var b booking
			if rows.Scan(&b.ID, &b.RoomID, &b.Title, &b.Organizer, &b.Private, &b.Starts, &b.Ends, &b.Canceled, &b.Version) != nil {
				http.Error(w, "internal error", 500)
				return
			}
			items = append(items, b)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
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
			var id int64
			var err error
			if action != "create" {
				id, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
				if err != nil || id <= 0 {
					http.Error(w, "invalid request", 400)
					return
				}
			}
			var b booking
			if action == "cancel" {
				b.Version, err = strconv.ParseInt(r.Header.Get("If-Match"), 10, 64)
				if err != nil || b.Version < 1 {
					http.Error(w, "invalid request", 400)
					return
				}
			}
			if action != "cancel" {
				d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				d.DisallowUnknownFields()
				if d.Decode(&b) != nil || d.Decode(&struct{}{}) != io.EOF || b.ID != 0 || b.Canceled || (action == "create" && b.Version != 0) || (action == "update" && b.Version < 1) || b.RoomID <= 0 || strings.TrimSpace(b.Title) != b.Title || b.Title == "" || len(b.Title) > 256 || strings.TrimSpace(b.Organizer) != b.Organizer || b.Organizer == "" || len(b.Organizer) > 128 || b.Starts < 0 || b.Ends <= b.Starts || b.Ends > 253402300799 {
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
			if action != "create" {
				var canceled bool
				var version int64
				err = tx.QueryRow("SELECT canceled,version FROM bookings WHERE id=?", id).Scan(&canceled, &version)
				if errors.Is(err, sql.ErrNoRows) {
					http.Error(w, "not found", 404)
					return
				}
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				if canceled || version != b.Version {
					http.Error(w, "conflict", 409)
					return
				}
			}
			if action != "cancel" {
				var active bool
				err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", b.RoomID).Scan(&active)
				if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
					http.Error(w, "invalid room", 400)
					return
				}
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				var conflict bool
				err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM bookings WHERE room_id=? AND canceled=0 AND id!=? AND starts<? AND ends>?)", b.RoomID, id, b.Ends, b.Starts).Scan(&conflict)
				if err != nil {
					http.Error(w, "internal error", 500)
					return
				}
				if conflict {
					http.Error(w, "conflict", 409)
					return
				}
			}
			if action == "create" {
				var res sql.Result
				res, err = tx.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(?,?,?,?,?,?)", b.RoomID, b.Title, b.Organizer, b.Private, b.Starts, b.Ends)
				if err == nil {
					id, err = res.LastInsertId()
				}
			} else if action == "update" {
				_, err = tx.Exec("UPDATE bookings SET room_id=?,title=?,organizer=?,private=?,starts=?,ends=?,version=version+1 WHERE id=?", b.RoomID, b.Title, b.Organizer, b.Private, b.Starts, b.Ends, id)
			} else {
				_, err = tx.Exec("UPDATE bookings SET canceled=1,version=version+1 WHERE id=?", id)
			}
			if err == nil {
				_, err = tx.Exec("INSERT INTO booking_audit(actor_id,booking_id,action,created_at) VALUES(?,?,?,?)", actor, id, action, time.Now().Unix())
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
	mux.HandleFunc("POST /api/bookings", mutate("create"))
	mux.HandleFunc("POST /api/bookings/{id}", mutate("update"))
	mux.HandleFunc("POST /api/bookings/{id}/cancel", mutate("cancel"))
}
