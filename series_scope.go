package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"lee_schedule_display/recurrence"
)

var errSeriesException = errors.New("series change drops occurrence exception")

type seriesUpdateInput struct {
	Scope          string               `json:"scope"`
	OriginalStarts *int64               `json:"original_starts"`
	Version        int64                `json:"version"`
	RoomID         int64                `json:"room_id"`
	Title          string               `json:"title"`
	Organizer      string               `json:"organizer"`
	Private        bool                 `json:"private"`
	Starts         int64                `json:"starts"`
	Duration       int64                `json:"duration_seconds"`
	Frequency      recurrence.Frequency `json:"frequency"`
	Interval       int                  `json:"interval"`
	Weekdays       []time.Weekday       `json:"weekdays"`
	MonthDay       int                  `json:"month_day"`
	Nth            int                  `json:"nth"`
	NthWeekday     time.Weekday         `json:"nth_weekday"`
	Until          string               `json:"until"`
	Count          int                  `json:"count"`
}

type seriesCancelInput struct {
	Scope          string `json:"scope"`
	OriginalStarts *int64 `json:"original_starts"`
}

type existingOccurrence struct {
	originalStarts int64
	starts         int64
	ends           int64
	version        int64
	canceled       bool
	roomID         sql.NullInt64
	isException    bool
}

func truncateRuleBefore(rule recurrence.Rule, splitOrig int64) (recurrence.Rule, error) {
	splitTime := time.Unix(splitOrig, 0).In(rule.Location)
	if rule.Count > 0 {
		occs, err := recurrence.Expand(rule, rule.Start, time.Unix(splitOrig, 0))
		if err != nil {
			return recurrence.Rule{}, err
		}
		kept := 0
		for _, o := range occs {
			if o.Start.Unix() < splitOrig {
				kept++
			}
		}
		if kept <= 0 {
			return recurrence.Rule{}, errSeriesInvalid
		}
		rule.Count = kept
		rule.Until = time.Time{}
		return rule, nil
	}
	prevDay := time.Date(splitTime.Year(), splitTime.Month(), splitTime.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	if prevDay.Before(time.Date(rule.Start.In(rule.Location).Year(), rule.Start.In(rule.Location).Month(), rule.Start.In(rule.Location).Day(), 0, 0, 0, 0, time.UTC)) {
		return recurrence.Rule{}, errSeriesInvalid
	}
	rule.Count = 0
	rule.Until = prevDay
	return rule, nil
}

func registerSeriesScopeHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck, zone *time.Location) {
	mux.HandleFunc("POST /api/series/{id}/cancel", post(func(w http.ResponseWriter, r *http.Request) {
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
		version, err := strconv.ParseInt(r.Header.Get("If-Match"), 10, 64)
		if err != nil || version < 1 {
			http.Error(w, "invalid request", 400)
			return
		}
		var in seriesCancelInput
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "invalid request", 400)
			return
		}
		if in.Scope != "all" && (in.Scope != "from" || in.OriginalStarts == nil || *in.OriginalStarts < 0) {
			http.Error(w, "invalid request", 400)
			return
		}
		err = cancelSeriesScope(db, token, csrf, id, version, in)
		switch {
		case err == nil:
			w.WriteHeader(204)
		case errors.Is(err, sql.ErrNoRows):
			http.Error(w, "not found", 404)
		case errors.Is(err, errSeriesUnauthorized):
			http.Error(w, "unauthorized", 401)
		case errors.Is(err, errSeriesConflict):
			http.Error(w, "conflict", 409)
		case errors.Is(err, errSeriesInvalid):
			http.Error(w, "invalid request", 400)
		default:
			http.Error(w, "internal error", 500)
		}
	}))

	mux.HandleFunc("POST /api/series/{id}", post(func(w http.ResponseWriter, r *http.Request) {
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
		var in seriesUpdateInput
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF || in.Version < 1 || in.Starts < 0 || in.Starts > 253402300799 || in.Duration <= 0 || in.Duration > int64(recurrence.MaxDuration/time.Second) {
			http.Error(w, "invalid request", 400)
			return
		}
		if in.Scope != "all" && (in.Scope != "from" || in.OriginalStarts == nil || *in.OriginalStarts < 0) {
			http.Error(w, "invalid request", 400)
			return
		}
		rule := recurrence.Rule{
			Start:      time.Unix(in.Starts, 0),
			Location:   zone,
			Frequency:  in.Frequency,
			Interval:   in.Interval,
			Weekdays:   in.Weekdays,
			MonthDay:   in.MonthDay,
			Nth:        in.Nth,
			NthWeekday: in.NthWeekday,
			Count:      in.Count,
			Duration:   time.Duration(in.Duration) * time.Second,
		}
		if in.Until != "" {
			rule.Until, err = time.Parse("2006-01-02", in.Until)
			if err != nil {
				http.Error(w, "invalid request", 400)
				return
			}
		}
		err = updateSeriesScope(db, token, csrf, id, in, rule)
		switch {
		case err == nil:
			w.WriteHeader(204)
		case errors.Is(err, sql.ErrNoRows):
			http.Error(w, "not found", 404)
		case errors.Is(err, errSeriesUnauthorized):
			http.Error(w, "unauthorized", 401)
		case errors.Is(err, errSeriesException):
			http.Error(w, "exception_conflict: existing occurrence exceptions would lose their original start", 409)
		case errors.Is(err, errSeriesConflict):
			http.Error(w, "conflict", 409)
		case errors.Is(err, errSeriesInvalid), errors.Is(err, recurrence.ErrInvalid), errors.Is(err, recurrence.ErrLimit), errors.Is(err, recurrence.ErrUnbounded), errors.Is(err, recurrence.ErrLocalTime):
			http.Error(w, "invalid recurrence or ambiguous/nonexistent local time", 400)
		default:
			http.Error(w, "internal error", 500)
		}
	}))
}

func cancelSeriesScope(db *sql.DB, token, csrf string, id, version int64, in seriesCancelInput) error {
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
	var rawRule string
	var sVersion int64
	var sCanceled bool
	err = tx.QueryRow("SELECT rule,version,canceled FROM booking_series WHERE id=?", id).Scan(&rawRule, &sVersion, &sCanceled)
	if err != nil {
		return err
	}
	if sCanceled || sVersion != version {
		return errSeriesConflict
	}
	var minOrig sql.NullInt64
	if err = tx.QueryRow("SELECT MIN(original_starts) FROM series_occurrences WHERE series_id=?", id).Scan(&minOrig); err != nil {
		return err
	}
	if !minOrig.Valid {
		return sql.ErrNoRows
	}
	cancelAll := in.Scope == "all"
	var splitOrig int64
	if in.Scope == "from" {
		splitOrig = *in.OriginalStarts
		var exists bool
		if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM series_occurrences WHERE series_id=? AND original_starts=?)", id, splitOrig).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
		if splitOrig <= minOrig.Int64 {
			cancelAll = true
		}
	}
	now := time.Now().Unix()
	if cancelAll {
		if _, err = tx.Exec("INSERT INTO occurrence_audit(actor_id,series_id,original_starts,action,created_at) SELECT ?,series_id,original_starts,'cancel',? FROM series_occurrences WHERE series_id=? AND canceled=0", actor, now, id); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE series_occurrences SET canceled=1,version=version+1 WHERE series_id=? AND canceled=0", id); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE booking_series SET canceled=1,version=version+1 WHERE id=?", id); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM materialization_coverage WHERE series_id=?", id); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE conflict_alerts SET active=0 WHERE active=1 AND (series_a=? OR series_b=?)", id, id); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO series_audit(actor_id,series_id,action,created_at) VALUES(?,?,'cancel_all',?)", actor, id, now); err != nil {
			return err
		}
		return tx.Commit()
	}

	var stored storedSeriesRule
	if err = json.Unmarshal([]byte(rawRule), &stored); err != nil {
		return err
	}
	stored.Rule.Location, err = time.LoadLocation(stored.Location)
	if err != nil {
		return err
	}
	truncated, err := truncateRuleBefore(stored.Rule, splitOrig)
	if err != nil {
		return err
	}
	encodedOld, err := json.Marshal(struct {
		Rule     recurrence.Rule `json:"rule"`
		Location string          `json:"location"`
	}{truncated, stored.Location})
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO occurrence_audit(actor_id,series_id,original_starts,action,created_at) SELECT ?,series_id,original_starts,'cancel',? FROM series_occurrences WHERE series_id=? AND original_starts>=? AND canceled=0", actor, now, id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM series_occurrences WHERE series_id=? AND original_starts>=?", id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE booking_series SET rule=?,version=version+1 WHERE id=?", string(encodedOld), id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM materialization_coverage WHERE series_id=?", id); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE conflict_alerts SET active=0 WHERE active=1 AND ((series_a=? AND original_a>=?) OR (series_b=? AND original_b>=?))", id, splitOrig, id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO series_audit(actor_id,series_id,action,created_at) VALUES(?,?,'cancel_from',?)", actor, id, now); err != nil {
		return err
	}
	return tx.Commit()
}

func updateSeriesScope(db *sql.DB, token, csrf string, id int64, in seriesUpdateInput, rule recurrence.Rule) error {
	if in.RoomID <= 0 || in.Title == "" || strings.TrimSpace(in.Title) != in.Title || len(in.Title) > 256 || in.Organizer == "" || strings.TrimSpace(in.Organizer) != in.Organizer || len(in.Organizer) > 128 {
		return errSeriesInvalid
	}
	if rule.Start.Nanosecond() != 0 || rule.Duration%time.Second != 0 || rule.Location == nil {
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
	if err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", in.RoomID).Scan(&active); errors.Is(err, sql.ErrNoRows) || err == nil && !active {
		return errSeriesInvalid
	} else if err != nil {
		return err
	}
	var rawOldRule string
	var sVersion int64
	var sCanceled bool
	err = tx.QueryRow("SELECT rule,version,canceled FROM booking_series WHERE id=?", id).Scan(&rawOldRule, &sVersion, &sCanceled)
	if err != nil {
		return err
	}
	if sCanceled || sVersion != in.Version {
		return errSeriesConflict
	}

	rows, err := tx.Query(`SELECT original_starts,starts,ends,version,canceled,room_id,
		(version>1 OR canceled=1 OR room_id IS NOT NULL OR title IS NOT NULL OR organizer IS NOT NULL OR private IS NOT NULL OR starts!=original_starts OR EXISTS(SELECT 1 FROM occurrence_audit a WHERE a.series_id=series_occurrences.series_id AND a.original_starts=series_occurrences.original_starts))
		FROM series_occurrences WHERE series_id=? ORDER BY original_starts`, id)
	if err != nil {
		return err
	}
	var existing []existingOccurrence
	for rows.Next() {
		var o existingOccurrence
		if err = rows.Scan(&o.originalStarts, &o.starts, &o.ends, &o.version, &o.canceled, &o.roomID, &o.isException); err != nil {
			rows.Close()
			return err
		}
		existing = append(existing, o)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	if len(existing) == 0 {
		return sql.ErrNoRows
	}

	inPlace := in.Scope == "all"
	var splitOrig int64
	if in.Scope == "from" {
		splitOrig = *in.OriginalStarts
		found := false
		for _, o := range existing {
			if o.originalStarts == splitOrig {
				found = true
				break
			}
		}
		if !found {
			return sql.ErrNoRows
		}
		if splitOrig <= existing[0].originalStarts {
			inPlace = true
		}
	}

	var targetExcs []existingOccurrence
	for _, o := range existing {
		if !o.isException {
			continue
		}
		if inPlace || o.originalStarts >= splitOrig {
			targetExcs = append(targetExcs, o)
		}
	}

	newOccMap := make(map[int64]recurrence.Occurrence, len(occurrences))
	for _, o := range occurrences {
		newOccMap[o.Start.Unix()] = o
	}
	excSet := make(map[int64]bool, len(targetExcs))
	for _, ex := range targetExcs {
		excSet[ex.originalStarts] = true
		if _, ok := newOccMap[ex.originalStarts]; ok {
			continue
		}
		if !unbounded {
			return errSeriesException
		}
		if ex.originalStarts < rule.Start.Unix() {
			return errSeriesException
		}
		probe, probeErr := recurrence.Expand(rule, time.Unix(ex.originalStarts, 0), time.Unix(ex.originalStarts, 0).Add(time.Second))
		if probeErr != nil || len(probe) == 0 {
			return errSeriesException
		}
		matched := false
		for _, p := range probe {
			if p.Start.Unix() == ex.originalStarts {
				matched = true
				break
			}
		}
		if !matched {
			return errSeriesException
		}
	}

	type effectiveInterval struct {
		roomID int64
		starts int64
		ends   int64
		isExc  bool
	}
	var effective []effectiveInterval
	roomsToMaterialize := map[int64]bool{in.RoomID: true}
	for _, o := range occurrences {
		s := o.Start.Unix()
		if excSet[s] {
			continue
		}
		effective = append(effective, effectiveInterval{roomID: in.RoomID, starts: s, ends: o.End.Unix(), isExc: false})
	}
	for _, ex := range targetExcs {
		if ex.canceled {
			continue
		}
		rID := in.RoomID
		if ex.roomID.Valid {
			rID = ex.roomID.Int64
		}
		roomsToMaterialize[rID] = true
		effective = append(effective, effectiveInterval{roomID: rID, starts: ex.starts, ends: ex.ends, isExc: true})
	}

	// Self-overlap check per room
	sort.Slice(effective, func(i, j int) bool {
		if effective[i].roomID != effective[j].roomID {
			return effective[i].roomID < effective[j].roomID
		}
		if effective[i].starts != effective[j].starts {
			return effective[i].starts < effective[j].starts
		}
		return effective[i].ends < effective[j].ends
	})
	for i := 1; i < len(effective); i++ {
		if effective[i].roomID == effective[i-1].roomID && effective[i].starts < effective[i-1].ends {
			return errSeriesConflict
		}
	}

	for rID := range roomsToMaterialize {
		if err = materializeUnboundedRoomTx(tx, anchor, horizon, rID); err != nil {
			return err
		}
	}
	for _, iv := range effective {
		if iv.isExc && (iv.starts < anchor.Unix() || iv.ends > horizon.Unix()) {
			if err = materializeUnboundedRoomTx(tx, time.Unix(iv.starts, 0), time.Unix(iv.ends, 0), iv.roomID); err != nil {
				return err
			}
		}
	}

	if _, err = tx.Exec("CREATE TEMP TABLE pending_series_edit(room_id INTEGER NOT NULL, starts INTEGER NOT NULL, ends INTEGER NOT NULL, is_exc INTEGER NOT NULL)"); err != nil {
		return err
	}
	for i := 0; i < len(effective); {
		var query strings.Builder
		query.WriteString("INSERT INTO pending_series_edit VALUES ")
		args := make([]any, 0, 800)
		for n := 0; i < len(effective) && n < 200; n++ {
			if n > 0 {
				query.WriteByte(',')
			}
			query.WriteString("(?,?,?,?)")
			excFlag := 0
			if effective[i].isExc {
				excFlag = 1
			}
			args = append(args, effective[i].roomID, effective[i].starts, effective[i].ends, excFlag)
			i++
		}
		if _, err = tx.Exec(query.String(), args...); err != nil {
			return err
		}
	}

	inPlaceFlag := 0
	if inPlace {
		inPlaceFlag = 1
	}
	var conflict bool
	err = tx.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM pending_series_edit p JOIN bookings b ON b.room_id=p.room_id AND b.canceled=0 AND b.starts<p.ends AND b.ends>p.starts
		UNION ALL
		SELECT 1 FROM pending_series_edit p JOIN booking_series s ON s.canceled=0 JOIN series_occurrences o ON o.series_id=s.id AND COALESCE(o.room_id,s.room_id)=p.room_id AND o.canceled=0 AND NOT (s.id=? AND (?=1 OR o.original_starts>=?)) AND o.starts<p.ends AND o.ends>p.starts
		WHERE p.is_exc=1 OR o.version>1 OR (p.starts<? AND p.ends>? AND o.starts<? AND o.ends>?) OR (p.starts<s.checked_through AND o.starts<s.checked_through) OR s.id=?
	)`, id, inPlaceFlag, splitOrig, horizon.Unix(), anchor.Unix(), horizon.Unix(), anchor.Unix(), id).Scan(&conflict)
	if err != nil {
		return err
	}
	if conflict {
		return errSeriesConflict
	}
	if _, err = tx.Exec("DROP TABLE pending_series_edit"); err != nil {
		return err
	}

	now := time.Now().Unix()
	if inPlace {
		if _, err = tx.Exec(`DELETE FROM series_occurrences WHERE series_id=? AND NOT (version>1 OR canceled=1 OR room_id IS NOT NULL OR title IS NOT NULL OR organizer IS NOT NULL OR private IS NOT NULL OR starts!=original_starts OR EXISTS(SELECT 1 FROM occurrence_audit a WHERE a.series_id=series_occurrences.series_id AND a.original_starts=series_occurrences.original_starts))`, id); err != nil {
			return err
		}
		for i := 0; i < len(occurrences); {
			var query strings.Builder
			query.WriteString("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES ")
			args := make([]any, 0, 800)
			for n := 0; i < len(occurrences) && n < 200; n++ {
				if n > 0 {
					query.WriteByte(',')
				}
				query.WriteString("(?,?,?,?)")
				s := occurrences[i].Start.Unix()
				args = append(args, id, s, s, occurrences[i].End.Unix())
				i++
			}
			query.WriteString(" ON CONFLICT(series_id,original_starts) DO NOTHING")
			if _, err = tx.Exec(query.String(), args...); err != nil {
				return err
			}
		}
		if _, err = tx.Exec("UPDATE booking_series SET room_id=?,title=?,organizer=?,private=?,rule=?,checked_through=?,version=version+1 WHERE id=?", in.RoomID, in.Title, in.Organizer, in.Private, string(encoded), horizon.Unix(), id); err != nil {
			return err
		}
		if unbounded {
			if _, err = tx.Exec("INSERT INTO materialization_coverage(series_id,starts,ends) VALUES(?,?,?) ON CONFLICT(series_id) DO UPDATE SET starts=excluded.starts,ends=excluded.ends", id, anchor.Unix(), horizon.Unix()); err != nil {
				return err
			}
		} else {
			if _, err = tx.Exec("DELETE FROM materialization_coverage WHERE series_id=?", id); err != nil {
				return err
			}
		}
		if _, err = tx.Exec("UPDATE conflict_alerts SET active=0 WHERE active=1 AND (series_a=? OR series_b=?) AND NOT EXISTS(SELECT 1 FROM series_occurrences a JOIN booking_series sa ON sa.id=a.series_id JOIN series_occurrences b ON b.series_id=conflict_alerts.series_b AND b.original_starts=conflict_alerts.original_b JOIN booking_series sb ON sb.id=b.series_id WHERE a.series_id=conflict_alerts.series_a AND a.original_starts=conflict_alerts.original_a AND sa.canceled=0 AND sb.canceled=0 AND a.canceled=0 AND b.canceled=0 AND COALESCE(a.room_id,sa.room_id)=COALESCE(b.room_id,sb.room_id) AND a.starts<b.ends AND a.ends>b.starts)", id, id); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO series_audit(actor_id,series_id,action,created_at) VALUES(?,?,'update_all',?)", actor, id, now); err != nil {
			return err
		}
		return tx.Commit()
	}

	var storedOld storedSeriesRule
	if err = json.Unmarshal([]byte(rawOldRule), &storedOld); err != nil {
		return err
	}
	storedOld.Rule.Location, err = time.LoadLocation(storedOld.Location)
	if err != nil {
		return err
	}
	truncatedOld, err := truncateRuleBefore(storedOld.Rule, splitOrig)
	if err != nil {
		return err
	}
	encodedOld, err := json.Marshal(struct {
		Rule     recurrence.Rule `json:"rule"`
		Location string          `json:"location"`
	}{truncatedOld, storedOld.Location})
	if err != nil {
		return err
	}
	res, err := tx.Exec("INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(?,?,?,?,?,?,?)", in.RoomID, in.Title, in.Organizer, in.Private, string(encoded), actor, horizon.Unix())
	if err != nil {
		return err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE series_occurrences SET series_id=? WHERE series_id=? AND original_starts>=? AND (version>1 OR canceled=1 OR room_id IS NOT NULL OR title IS NOT NULL OR organizer IS NOT NULL OR private IS NOT NULL OR starts!=original_starts OR EXISTS(SELECT 1 FROM occurrence_audit a WHERE a.series_id=series_occurrences.series_id AND a.original_starts=series_occurrences.original_starts))`, newID, id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE occurrence_audit SET series_id=? WHERE series_id=? AND original_starts>=?", newID, id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM series_occurrences WHERE series_id=? AND original_starts>=?", id, splitOrig); err != nil {
		return err
	}
	for i := 0; i < len(occurrences); {
		var query strings.Builder
		query.WriteString("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES ")
		args := make([]any, 0, 800)
		for n := 0; i < len(occurrences) && n < 200; n++ {
			if n > 0 {
				query.WriteByte(',')
			}
			query.WriteString("(?,?,?,?)")
			s := occurrences[i].Start.Unix()
			args = append(args, newID, s, s, occurrences[i].End.Unix())
			i++
		}
		query.WriteString(" ON CONFLICT(series_id,original_starts) DO NOTHING")
		if _, err = tx.Exec(query.String(), args...); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE booking_series SET rule=?,version=version+1 WHERE id=?", string(encodedOld), id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM materialization_coverage WHERE series_id=?", id); err != nil {
		return err
	}
	if unbounded {
		if _, err = tx.Exec("INSERT INTO materialization_coverage(series_id,starts,ends) VALUES(?,?,?)", newID, anchor.Unix(), horizon.Unix()); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE conflict_alerts SET active=0 WHERE active=1 AND ((series_a=? AND original_a>=?) OR (series_b=? AND original_b>=?))", id, splitOrig, id, splitOrig); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO series_audit(actor_id,series_id,action,created_at) VALUES(?,?,'split_old',?),(?,?,'split_new',?)", actor, id, now, actor, newID, now); err != nil {
		return err
	}
	return tx.Commit()
}
