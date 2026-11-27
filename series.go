package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"lee_schedule_display/recurrence"
)

var errSeriesConflict = errors.New("series conflict")
var errSeriesInvalid = errors.New("invalid series")
var errSeriesUnauthorized = errors.New("unauthorized")
var errSeriesBeyondHorizon = errors.New("series exceeds the 12-month validation limit")

func storeSeries(db *sql.DB, token, csrf string, b booking, rule recurrence.Rule) error {
	if b.RoomID <= 0 || b.Title == "" || strings.TrimSpace(b.Title) != b.Title || len(b.Title) > 256 || b.Organizer == "" || strings.TrimSpace(b.Organizer) != b.Organizer || len(b.Organizer) > 128 {
		return errSeriesInvalid
	}
	if rule.Start.Nanosecond() != 0 || rule.Duration%time.Second != 0 {
		return errSeriesInvalid
	}
	if err := recurrence.ValidateForStorage(rule); err != nil {
		return err
	}
	end := time.Date(2401, 1, 2, 0, 0, 0, 0, time.UTC).Add(recurrence.MaxDuration)
	occurrences, err := recurrence.Expand(rule, rule.Start, end)
	if err != nil {
		return err
	}
	if len(occurrences) == 0 {
		return errSeriesInvalid
	}
	anchor := time.Now().In(rule.Location)
	if rule.Start.After(anchor) {
		anchor = rule.Start.In(rule.Location)
	}
	horizon := anchor.AddDate(1, 0, 0)
	if occurrences[len(occurrences)-1].End.After(horizon) {
		return errSeriesBeyondHorizon
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
	for _, occurrence := range occurrences {
		var conflict bool
		err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM bookings WHERE room_id=? AND canceled=0 AND starts<? AND ends>? UNION ALL SELECT 1 FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE s.room_id=? AND o.starts<? AND o.ends>?)`, b.RoomID, occurrence.End.Unix(), occurrence.Start.Unix(), b.RoomID, occurrence.End.Unix(), occurrence.Start.Unix()).Scan(&conflict)
		if err != nil {
			return err
		}
		if conflict {
			return errSeriesConflict
		}
	}
	result, err := tx.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(?,?,?,?,?,?)", b.RoomID, b.Title, b.Organizer, b.Private, string(encoded), actor)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, occurrence := range occurrences {
		if _, err = tx.Exec("INSERT INTO series_occurrences(series_id,starts,ends) VALUES(?,?,?)", id, occurrence.Start.Unix(), occurrence.End.Unix()); err != nil {
			return err
		}
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
		rows, err := db.Query("SELECT s.id,s.room_id,s.title,s.organizer,s.private,s.rule,o.starts,o.ends FROM booking_series s JOIN series_occurrences o ON o.series_id=s.id ORDER BY s.id,o.starts")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type occurrence struct {
			Starts int64 `json:"starts"`
			Ends   int64 `json:"ends"`
		}
		type item struct {
			Occurrences    []occurrence    `json:"occurrences"`
			CheckedThrough int64           `json:"checked_through"`
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
			if rows.Scan(&x.ID, &x.RoomID, &x.Title, &x.Organizer, &x.Private, &raw, &o.Starts, &o.Ends) != nil || !json.Valid([]byte(raw)) {
				http.Error(w, "internal error", 500)
				return
			}
			if len(items) == 0 || items[len(items)-1].ID != x.ID {
				x.Rule = json.RawMessage(raw)
				items = append(items, x)
			}
			last := &items[len(items)-1]
			last.Occurrences = append(last.Occurrences, o)
			if o.Ends > last.CheckedThrough {
				last.CheckedThrough = o.Ends
			}
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})
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
		case errors.Is(err, errSeriesBeyondHorizon):
			http.Error(w, "series exceeds the 12-month validation limit; choose an earlier end", 400)
		case errors.Is(err, errSeriesInvalid), errors.Is(err, recurrence.ErrInvalid), errors.Is(err, recurrence.ErrLimit), errors.Is(err, recurrence.ErrUnbounded), errors.Is(err, recurrence.ErrLocalTime):
			http.Error(w, "invalid recurrence or ambiguous/nonexistent local time", 400)
		default:
			http.Error(w, "internal error", 500)
		}
	}))
}
