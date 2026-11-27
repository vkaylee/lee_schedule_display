package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func mutateOccurrence(db *sql.DB, token, csrf string, series, original int64, b booking, cancel bool) error {
	if series <= 0 || original < 0 || b.Version < 1 {
		return errSeriesInvalid
	}
	if !cancel && (b.RoomID <= 0 || b.Title == "" || strings.TrimSpace(b.Title) != b.Title || len(b.Title) > 256 || b.Organizer == "" || strings.TrimSpace(b.Organizer) != b.Organizer || len(b.Organizer) > 128 || b.Starts < 0 || b.Ends <= b.Starts || b.Ends > 253402300799) {
		return errSeriesInvalid
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
	err = tx.QueryRow("SELECT users.id FROM sessions JOIN users ON users.id=sessions.user_id WHERE sessions.token_hash=? AND sessions.csrf=? AND sessions.expires>? AND users.active=1", tokenHash(token), csrf, time.Now().Unix()).Scan(&actor)
	if errors.Is(err, sql.ErrNoRows) {
		return errSeriesUnauthorized
	}
	if err != nil {
		return err
	}
	var version int64
	var canceled, seriesCanceled bool
	err = tx.QueryRow("SELECT o.version,o.canceled,s.canceled FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE o.series_id=? AND o.original_starts=?", series, original).Scan(&version, &canceled, &seriesCanceled)
	if err != nil {
		return err
	}
	if canceled || seriesCanceled || version != b.Version {
		return errSeriesConflict
	}
	if !cancel {
		var active bool
		err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", b.RoomID).Scan(&active)
		if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
			return errSeriesInvalid
		}
		if err != nil {
			return err
		}
		if err = materializeUnboundedRoomTx(tx, time.Unix(b.Starts, 0), time.Unix(b.Ends, 0), b.RoomID); err != nil {
			return err
		}
		var conflict bool
		err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM bookings WHERE room_id=? AND canceled=0 AND starts<? AND ends>? UNION ALL SELECT 1 FROM series_occurrences o JOIN booking_series s ON s.id=o.series_id WHERE s.canceled=0 AND o.canceled=0 AND COALESCE(o.room_id,s.room_id)=? AND NOT(o.series_id=? AND o.original_starts=?) AND o.starts<? AND o.ends>?)`, b.RoomID, b.Ends, b.Starts, b.RoomID, series, original, b.Ends, b.Starts).Scan(&conflict)
		if err != nil {
			return err
		}
		if conflict {
			return errSeriesConflict
		}
		_, err = tx.Exec("UPDATE series_occurrences SET room_id=?,title=?,organizer=?,private=?,starts=?,ends=?,version=version+1 WHERE series_id=? AND original_starts=?", b.RoomID, b.Title, b.Organizer, b.Private, b.Starts, b.Ends, series, original)
	} else {
		_, err = tx.Exec("UPDATE series_occurrences SET canceled=1,version=version+1 WHERE series_id=? AND original_starts=?", series, original)
	}
	if err != nil {
		return err
	}
	action := "update"
	if cancel {
		action = "cancel"
	}
	if _, err = tx.Exec("INSERT INTO occurrence_audit(actor_id,series_id,original_starts,action,created_at) VALUES(?,?,?,?,?)", actor, series, original, action, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
