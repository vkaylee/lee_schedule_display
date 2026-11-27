package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"
)

func scanConflicts(db *sql.DB, now time.Time) error { return scanConflictsAttempt(db, now, 0) }

func scanConflictRoom(db *sql.DB, now time.Time, room int64, attempt int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
		return err
	}
	if attempt != 0 {
		var current int64
		if err = tx.QueryRow("SELECT attempt FROM room_scan_status WHERE room_id=?", room).Scan(&current); err != nil {
			return err
		}
		if current != attempt {
			return nil
		}
	}
	horizon := now.AddDate(1, 0, 0)
	if err = materializeUnboundedRoomTx(tx, now, horizon, room); err != nil {
		return err
	}
	if err = recordMaterializationCoverageRoom(tx, now, horizon, room); err != nil {
		return err
	}
	if _, err = tx.Exec(`DROP TABLE IF EXISTS room_conflicts; CREATE TEMP TABLE room_conflicts AS SELECT a.series_id series_a,a.original_starts original_a,b.series_id series_b,b.original_starts original_b FROM series_occurrences a JOIN booking_series sa ON sa.id=a.series_id JOIN series_occurrences b ON b.series_id>a.series_id JOIN booking_series sb ON sb.id=b.series_id WHERE a.canceled=0 AND b.canceled=0 AND COALESCE(a.room_id,sa.room_id)=? AND COALESCE(b.room_id,sb.room_id)=? AND a.starts<b.ends AND a.ends>b.starts AND a.starts<? AND a.ends>? AND b.starts<? AND b.ends>?`, room, room, horizon.Unix(), now.Unix(), horizon.Unix(), now.Unix()); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO conflict_alerts(series_a,original_a,series_b,original_b) SELECT series_a,original_a,series_b,original_b FROM room_conflicts WHERE 1 ON CONFLICT(series_a,original_a,series_b,original_b) DO UPDATE SET acknowledged=CASE WHEN active=0 THEN 0 ELSE acknowledged END,active=1`); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE conflict_alerts SET active=0 WHERE active=1 AND (SELECT COALESCE(o.room_id,s.room_id) FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE o.series_id=conflict_alerts.series_a AND o.original_starts=conflict_alerts.original_a)=? AND NOT EXISTS(SELECT 1 FROM series_occurrences a JOIN booking_series sa ON sa.id=a.series_id JOIN series_occurrences b ON b.series_id=conflict_alerts.series_b AND b.original_starts=conflict_alerts.original_b JOIN booking_series sb ON sb.id=b.series_id WHERE a.series_id=conflict_alerts.series_a AND a.original_starts=conflict_alerts.original_a AND a.canceled=0 AND b.canceled=0 AND a.starts<b.ends AND a.ends>b.starts)`, room); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE room_scan_status SET last_success=?,checked_from=?,checked_through=?,failed=0 WHERE room_id=?", now.Unix(), now.Unix(), horizon.Unix(), room); err != nil {
		return err
	}
	return tx.Commit()
}

func scanConflictsAttempt(db *sql.DB, now time.Time, attempt int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
		return err
	}
	if attempt != 0 {
		var current int64
		if err = tx.QueryRow("SELECT attempt FROM conflict_scan_status WHERE id=1").Scan(&current); err != nil {
			return err
		}
		if current != attempt {
			return nil
		}
	}
	horizon := now.AddDate(1, 0, 0).Unix()
	if err = materializeUnboundedTx(tx, now, time.Unix(horizon, 0)); err != nil {
		return err
	}
	if err = recordMaterializationCoverage(tx, now, time.Unix(horizon, 0)); err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE TEMP TABLE current_conflicts AS SELECT a.series_id AS series_a,a.original_starts AS original_a,b.series_id AS series_b,b.original_starts AS original_b FROM series_occurrences a JOIN booking_series sa ON sa.id=a.series_id JOIN series_occurrences b ON b.series_id>a.series_id JOIN booking_series sb ON sb.id=b.series_id WHERE a.canceled=0 AND b.canceled=0 AND COALESCE(a.room_id,sa.room_id)=COALESCE(b.room_id,sb.room_id) AND a.starts<b.ends AND a.ends>b.starts AND a.starts<? AND a.ends>? AND b.starts<? AND b.ends>?`, horizon, now.Unix(), horizon, now.Unix()); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO conflict_alerts(series_a,original_a,series_b,original_b) SELECT series_a,original_a,series_b,original_b FROM current_conflicts WHERE 1 ON CONFLICT(series_a,original_a,series_b,original_b) DO UPDATE SET acknowledged=CASE WHEN active=0 THEN 0 ELSE acknowledged END,active=1`); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE conflict_alerts SET active=0 WHERE active=1 AND NOT EXISTS(SELECT 1 FROM series_occurrences a JOIN booking_series sa ON sa.id=a.series_id JOIN series_occurrences b ON b.series_id=conflict_alerts.series_b AND b.original_starts=conflict_alerts.original_b JOIN booking_series sb ON sb.id=b.series_id WHERE a.series_id=conflict_alerts.series_a AND a.original_starts=conflict_alerts.original_a AND a.canceled=0 AND b.canceled=0 AND COALESCE(a.room_id,sa.room_id)=COALESCE(b.room_id,sb.room_id) AND a.starts<b.ends AND a.ends>b.starts)`); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE conflict_scan_status SET last_success=?,checked_from=?,checked_through=?,failed=0 WHERE id=1", now.Unix(), now.Unix(), horizon); err != nil {
		return err
	}
	if _, err = tx.Exec("DROP TABLE current_conflicts"); err != nil {
		return err
	}
	return tx.Commit()
}

func runConflictScan(db *sql.DB, now time.Time) error {
	var attempt int64
	if err := db.QueryRow("UPDATE conflict_scan_status SET attempt=attempt+1 WHERE id=1 RETURNING attempt").Scan(&attempt); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT INTO room_scan_status(room_id) SELECT id FROM rooms WHERE 1 ON CONFLICT(room_id) DO NOTHING"); err != nil {
		_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		return errors.Join(err, statusErr)
	}
	rows, err := db.Query("SELECT id FROM rooms ORDER BY id")
	if err != nil {
		_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		return errors.Join(err, statusErr)
	}
	var rooms []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
			return errors.Join(err, statusErr)
		}
		rooms = append(rooms, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		return errors.Join(err, statusErr)
	}
	if err = rows.Close(); err != nil {
		_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		return errors.Join(err, statusErr)
	}
	if _, err = db.Exec("INSERT INTO room_scan_status(room_id) SELECT id FROM rooms WHERE 1 ON CONFLICT(room_id) DO NOTHING"); err != nil {
		_, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		return errors.Join(err, statusErr)
	}
	var failures []error
	for _, room := range rooms {
		var attempt int64
		if err = db.QueryRow("UPDATE room_scan_status SET attempt=attempt+1 WHERE room_id=? RETURNING attempt", room).Scan(&attempt); err != nil {
			failures = append(failures, err)
			continue
		}
		if err = scanConflictRoom(db, now, room, attempt); err != nil {
			_, statusErr := db.Exec("UPDATE room_scan_status SET failed=1 WHERE room_id=? AND attempt=?", room, attempt)
			if statusErr != nil {
				failures = append(failures, errors.Join(err, statusErr))
			} else {
				failures = append(failures, err)
			}
		}
	}
	if len(failures) > 0 {
		_, err = db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt)
		if err != nil {
			failures = append(failures, err)
		}
		return errors.Join(failures...)
	}
	horizon := now.AddDate(1, 0, 0).Unix()
	if _, err = db.Exec("UPDATE conflict_scan_status SET last_success=?,checked_from=?,checked_through=?,failed=0 WHERE id=1 AND attempt=?", now.Unix(), now.Unix(), horizon, attempt); err != nil {
		return err
	}
	return nil
}

func runConflictScanWith(db *sql.DB, now time.Time, scan func(*sql.DB, time.Time, int64) error) error {
	var attempt int64
	if err := db.QueryRow("UPDATE conflict_scan_status SET attempt=attempt+1 WHERE id=1 RETURNING attempt").Scan(&attempt); err != nil {
		return err
	}
	err := scan(db, now, attempt)
	if err != nil {
		if _, statusErr := db.Exec("UPDATE conflict_scan_status SET failed=1 WHERE id=1 AND attempt=?", attempt); statusErr != nil {
			return errors.Join(err, statusErr)
		}
	}
	return err
}

func conflictWorker(ctx context.Context, db *sql.DB, clock func() time.Time, ticks <-chan time.Time) {
	for {
		if err := runConflictScan(db, clock()); err != nil {
			log.Printf("conflict scan failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
		}
	}
}

func registerAlertHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	mux.HandleFunc("GET /api/alerts/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		var last, from, through sql.NullInt64
		var failed bool
		if err := db.QueryRow("SELECT last_success,failed,checked_from,checked_through FROM conflict_scan_status WHERE id=1").Scan(&last, &failed, &from, &through); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		stale := !last.Valid || time.Now().Unix() < last.Int64 || time.Now().Unix()-last.Int64 > 7200
		rows, err := db.Query(`SELECT r.id,r.name,s.last_success,s.checked_from,s.checked_through,s.failed FROM rooms r LEFT JOIN room_scan_status s ON s.room_id=r.id ORDER BY r.name,r.id`)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		type roomStatus struct {
			RoomID         int64  `json:"room_id"`
			Room           string `json:"room"`
			LastSuccess    *int64 `json:"last_success"`
			CheckedFrom    int64  `json:"checked_from"`
			CheckedThrough int64  `json:"checked_through"`
			Failed         bool   `json:"failed"`
			Stale          bool   `json:"stale"`
		}
		rooms := []roomStatus{}
		allRoomsOK := true
		current := time.Now().Unix()
		for rows.Next() {
			var room roomStatus
			var roomLast, roomFrom, roomThrough sql.NullInt64
			var roomFailed sql.NullBool
			if err := rows.Scan(&room.RoomID, &room.Room, &roomLast, &roomFrom, &roomThrough, &roomFailed); err != nil {
				rows.Close()
				http.Error(w, "internal error", 500)
				return
			}
			room.Stale = !roomLast.Valid || current < roomLast.Int64 || current-roomLast.Int64 > 7200
			room.Failed = roomFailed.Valid && roomFailed.Bool
			if roomLast.Valid {
				room.LastSuccess = &roomLast.Int64
			}
			room.CheckedFrom, room.CheckedThrough = roomFrom.Int64, roomThrough.Int64
			allRoomsOK = allRoomsOK && !room.Failed && !room.Stale
			rooms = append(rooms, room)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			http.Error(w, "internal error", 500)
			return
		}
		if err := rows.Close(); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		failed = failed || !allRoomsOK
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			LastSuccess    *int64       `json:"last_success"`
			CheckedFrom    int64        `json:"checked_from"`
			CheckedThrough int64        `json:"checked_through"`
			Failed         bool         `json:"failed"`
			Stale          bool         `json:"stale"`
			Rooms          []roomStatus `json:"rooms"`
		}{func() *int64 {
			if last.Valid {
				return &last.Int64
			}
			return nil
		}(), from.Int64, through.Int64, failed, stale || !allRoomsOK, rooms})
	})
	mux.HandleFunc("GET /api/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		rows, err := db.Query(`SELECT id,series_a,original_a,series_b,original_b,acknowledged FROM conflict_alerts WHERE active=1 ORDER BY id`)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer rows.Close()
		type alert struct {
			ID           int64 `json:"id"`
			SeriesA      int64 `json:"series_a"`
			OriginalA    int64 `json:"original_a"`
			SeriesB      int64 `json:"series_b"`
			OriginalB    int64 `json:"original_b"`
			Acknowledged bool  `json:"acknowledged"`
		}
		items := []alert{}
		for rows.Next() {
			var a alert
			if rows.Scan(&a.ID, &a.SeriesA, &a.OriginalA, &a.SeriesB, &a.OriginalB, &a.Acknowledged) != nil {
				http.Error(w, "internal error", 500)
				return
			}
			items = append(items, a)
		}
		if rows.Err() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})
	mux.HandleFunc("POST /api/alerts/{id}/acknowledge", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
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
		result, err := tx.Exec("UPDATE conflict_alerts SET acknowledged=1 WHERE id=? AND active=1", id)
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
		if err = tx.Commit(); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.WriteHeader(204)
	}))
}
