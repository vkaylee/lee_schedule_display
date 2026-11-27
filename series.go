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

	"lee_schedule_display/recurrence"
)

var errSeriesConflict = errors.New("series conflict")
var errSeriesInvalid = errors.New("invalid series")
var errSeriesUnauthorized = errors.New("unauthorized")

func storeSeries(db *sql.DB, token, csrf string, b booking, rule recurrence.Rule) error {
	if b.RoomID <= 0 || b.Title == "" || strings.TrimSpace(b.Title) != b.Title || len(b.Title) > 256 || b.Organizer == "" || strings.TrimSpace(b.Organizer) != b.Organizer || len(b.Organizer) > 128 {
		return errSeriesInvalid
	}
	if rule.Start.Nanosecond() != 0 || rule.Duration%time.Second != 0 {
		return errSeriesInvalid
	}
	if rule.Location == nil {
		return errSeriesInvalid
	}
	anchor := time.Now().In(rule.Location).Truncate(time.Second)
	if rule.Start.After(anchor) {
		anchor = rule.Start.In(rule.Location)
	}
	horizon := anchor.AddDate(1, 0, 0)
	unbounded := rule.Count == 0 && rule.Until.IsZero()
	end := time.Date(2401, 1, 2, 0, 0, 0, 0, time.UTC).Add(recurrence.MaxDuration)
	from := rule.Start
	if unbounded {
		if horizon.After(time.Date(2401, 1, 1, 0, 0, 0, 0, time.UTC)) {
			return recurrence.ErrLimit
		}
		from = anchor
		end = horizon
	} else if err := recurrence.ValidateForStorage(rule); err != nil {
		return err
	}
	occurrences, err := recurrence.Expand(rule, from, end)
	if err != nil {
		return err
	}
	if len(occurrences) == 0 {
		return errSeriesInvalid
	}
	for i := 1; i < len(occurrences); i++ {
		if occurrences[i].Start.Before(occurrences[i-1].End) {
			return errSeriesConflict
		}
	}
	encoded, err := json.Marshal(struct {
		Rule     recurrence.Rule `json:"rule"`
		Location string          `json:"location"`
	}{rule, rule.Location.String()})
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("CREATE TEMP TABLE pending_occurrences(starts INTEGER NOT NULL, ends INTEGER NOT NULL)"); err != nil {
		return err
	}
	for i := 0; i < len(occurrences); {
		var query strings.Builder
		query.WriteString("INSERT INTO pending_occurrences VALUES ")
		args := make([]any, 0, 400)
		for n := 0; i < len(occurrences) && n < 200; n++ {
			if n > 0 {
				query.WriteByte(',')
			}
			query.WriteString("(?,?)")
			args = append(args, occurrences[i].Start.Unix(), occurrences[i].End.Unix())
			i++
		}
		if _, err = tx.Exec(query.String(), args...); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
		return err
	}
	var actor int64
	if err = tx.QueryRow("SELECT users.id FROM sessions JOIN users ON users.id=sessions.user_id WHERE sessions.token_hash=? AND sessions.csrf=? AND sessions.expires>? AND users.active=1", tokenHash(token), csrf, time.Now().Unix()).Scan(&actor); errors.Is(err, sql.ErrNoRows) {
		return errSeriesUnauthorized
	} else if err != nil {
		return err
	}
	var active bool
	if err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", b.RoomID).Scan(&active); errors.Is(err, sql.ErrNoRows) || err == nil && !active {
		return errSeriesInvalid
	} else if err != nil {
		return err
	}
	if err = materializeUnboundedRoomTx(tx, anchor, horizon, b.RoomID); err != nil {
		return err
	}
	var conflict bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pending_occurrences p JOIN bookings b ON b.room_id=? AND b.canceled=0 AND b.starts<p.ends AND b.ends>p.starts UNION ALL SELECT 1 FROM pending_occurrences p JOIN booking_series s ON s.canceled=0 JOIN series_occurrences o ON o.series_id=s.id AND COALESCE(o.room_id,s.room_id)=? AND o.canceled=0 AND o.starts<p.ends AND o.ends>p.starts WHERE o.version>1 OR (p.starts<? AND p.ends>? AND o.starts<? AND o.ends>?) OR (p.starts<s.checked_through AND o.starts<s.checked_through))`, b.RoomID, b.RoomID, horizon.Unix(), anchor.Unix(), horizon.Unix(), anchor.Unix()).Scan(&conflict)
	if err != nil {
		return err
	}
	if conflict {
		return errSeriesConflict
	}
	result, err := tx.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(?,?,?,?,?,?,?)", b.RoomID, b.Title, b.Organizer, b.Private, string(encoded), actor, horizon.Unix())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) SELECT ?,starts,starts,ends FROM pending_occurrences", id); err != nil {
		return err
	}
	if unbounded {
		if _, err = tx.Exec("INSERT INTO materialization_coverage(series_id,starts,ends) VALUES(?,?,?)", id, anchor.Unix(), horizon.Unix()); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DROP TABLE pending_occurrences"); err != nil {
		return err
	}
	return tx.Commit()
}

type seriesInput struct {
	RoomID     int64                `json:"room_id"`
	Title      string               `json:"title"`
	Organizer  string               `json:"organizer"`
	Private    bool                 `json:"private"`
	Starts     int64                `json:"starts"`
	Duration   int64                `json:"duration_seconds"`
	Frequency  recurrence.Frequency `json:"frequency"`
	Interval   int                  `json:"interval"`
	Weekdays   []time.Weekday       `json:"weekdays"`
	MonthDay   int                  `json:"month_day"`
	Nth        int                  `json:"nth"`
	NthWeekday time.Weekday         `json:"nth_weekday"`
	Until      string               `json:"until"`
	Count      int                  `json:"count"`
}

func registerSeriesHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck, zone *time.Location) {
	mux.HandleFunc("GET /api/series", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query("SELECT s.id,s.room_id,s.title,s.organizer,s.private,s.rule,COALESCE(s.checked_through,0),s.version,s.canceled,o.starts,o.ends,o.original_starts,o.version,o.canceled,COALESCE(o.room_id,s.room_id),COALESCE(o.title,s.title),COALESCE(o.organizer,s.organizer),COALESCE(o.private,s.private) FROM booking_series s JOIN series_occurrences o ON o.series_id=s.id ORDER BY s.id,o.starts")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type occurrence struct {
			Starts    int64  `json:"starts"`
			Ends      int64  `json:"ends"`
			Original  int64  `json:"original_starts"`
			Version   int64  `json:"version"`
			Canceled  bool   `json:"canceled"`
			RoomID    int64  `json:"room_id"`
			Title     string `json:"title"`
			Organizer string `json:"organizer"`
			Private   bool   `json:"private"`
		}
		type item struct {
			Occurrences    []occurrence    `json:"occurrences"`
			CheckedThrough int64           `json:"checked_through"`
			Version        int64           `json:"version"`
			Canceled       bool            `json:"canceled"`
			ID             int64           `json:"id"`
			RoomID         int64           `json:"room_id"`
			Title          string          `json:"title"`
			Organizer      string          `json:"organizer"`
			Private        bool            `json:"private"`
			Rule           json.RawMessage `json:"rule"`
		}
		items := []item{}
		for rows.Next() {
			var x item
			var raw string
			var o occurrence
			if rows.Scan(&x.ID, &x.RoomID, &x.Title, &x.Organizer, &x.Private, &raw, &x.CheckedThrough, &x.Version, &x.Canceled, &o.Starts, &o.Ends, &o.Original, &o.Version, &o.Canceled, &o.RoomID, &o.Title, &o.Organizer, &o.Private) != nil || !json.Valid([]byte(raw)) {
				http.Error(w, "internal error", 500)
				return
			}
			if len(items) == 0 || items[len(items)-1].ID != x.ID {
				x.Rule = json.RawMessage(raw)
				items = append(items, x)
			}
			last := &items[len(items)-1]
			last.Occurrences = append(last.Occurrences, o)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})
	mutate := func(cancel bool) http.HandlerFunc {
		return post(func(w http.ResponseWriter, r *http.Request) {
			token, _, csrf, ok := session(w, r)
			if !ok {
				return
			}
			if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
				http.Error(w, "forbidden", 403)
				return
			}
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil || id <= 0 {
				http.Error(w, "invalid request", 400)
				return
			}
			original, err := strconv.ParseInt(r.PathValue("original"), 10, 64)
			if err != nil || original < 0 {
				http.Error(w, "invalid request", 400)
				return
			}
			var b booking
			if cancel {
				b.Version, err = strconv.ParseInt(r.Header.Get("If-Match"), 10, 64)
			} else {
				d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				d.DisallowUnknownFields()
				if d.Decode(&b) != nil || d.Decode(&struct{}{}) != io.EOF || b.ID != 0 || b.Canceled {
					http.Error(w, "invalid request", 400)
					return
				}
			}
			if err != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			err = mutateOccurrence(db, token, csrf, id, original, b, cancel)
			switch {
			case err == nil:
				w.WriteHeader(204)
			case errors.Is(err, sql.ErrNoRows):
				http.Error(w, "not found", 404)
			case errors.Is(err, errSeriesConflict):
				http.Error(w, "conflict", 409)
			case errors.Is(err, errSeriesUnauthorized):
				http.Error(w, "unauthorized", 401)
			case errors.Is(err, errSeriesInvalid):
				http.Error(w, "invalid request", 400)
			default:
				http.Error(w, "internal error", 500)
			}
		})
	}
	mux.HandleFunc("POST /api/series/{id}/occurrences/{original}", mutate(false))
	mux.HandleFunc("POST /api/series/{id}/occurrences/{original}/cancel", mutate(true))
	registerSeriesScopeHandlers(mux, db, post, session, zone)
	mux.HandleFunc("POST /api/series", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		var x seriesInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&x) != nil || decoder.Decode(&struct{}{}) != io.EOF || x.Starts < 0 || x.Starts > 253402300799 || x.Duration <= 0 || x.Duration > int64(recurrence.MaxDuration/time.Second) {
			http.Error(w, "invalid request", 400)
			return
		}
		rule := recurrence.Rule{Start: time.Unix(x.Starts, 0), Location: zone, Frequency: x.Frequency, Interval: x.Interval, Weekdays: x.Weekdays, MonthDay: x.MonthDay, Nth: x.Nth, NthWeekday: x.NthWeekday, Count: x.Count, Duration: time.Duration(x.Duration) * time.Second}
		if x.Until != "" {
			var err error
			rule.Until, err = time.Parse("2006-01-02", x.Until)
			if err != nil {
				http.Error(w, "invalid request", 400)
				return
			}
		}
		err := storeSeries(db, token, csrf, booking{RoomID: x.RoomID, Title: x.Title, Organizer: x.Organizer, Private: x.Private}, rule)
		switch {
		case err == nil:
			w.WriteHeader(201)
		case errors.Is(err, errSeriesUnauthorized):
			http.Error(w, "unauthorized", 401)
		case errors.Is(err, errSeriesConflict):
			http.Error(w, "conflict", 409)
		case errors.Is(err, errSeriesInvalid), errors.Is(err, recurrence.ErrInvalid), errors.Is(err, recurrence.ErrLimit), errors.Is(err, recurrence.ErrUnbounded), errors.Is(err, recurrence.ErrLocalTime):
			http.Error(w, "invalid recurrence or ambiguous/nonexistent local time", 400)
		default:
			http.Error(w, "internal error", 500)
		}
	}))
}
