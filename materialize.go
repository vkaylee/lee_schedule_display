package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"lee_schedule_display/recurrence"
	"time"
)

type storedSeriesRule struct {
	Rule     recurrence.Rule `json:"rule"`
	Location string          `json:"location"`
}

func displayCoverage(query interface{ QueryRow(string, ...any) *sql.Row }, roomID, now int64) (sql.NullInt64, error) {
	var cutoff sql.NullInt64
	err := query.QueryRow(`SELECT MIN(CASE WHEN c.starts IS NULL THEN MAX(?,COALESCE(CAST(strftime('%s',json_extract(s.rule,'$.rule.Start')) AS INTEGER),?)) WHEN c.starts>? THEN c.starts WHEN c.ends<=? THEN ? ELSE c.ends END) FROM booking_series s LEFT JOIN materialization_coverage c ON c.series_id=s.id WHERE s.canceled=0 AND (s.room_id=? OR EXISTS (SELECT 1 FROM series_occurrences o WHERE o.series_id=s.id AND o.room_id=? AND o.canceled=0 AND o.ends>?)) AND json_extract(s.rule,'$.rule.Count')=0 AND json_extract(s.rule,'$.rule.Until')='0001-01-01T00:00:00Z'`, now, now, now, now, now, roomID, roomID, now).Scan(&cutoff)
	return cutoff, err
}

func trustedDisplayCoverage(query interface{ QueryRow(string, ...any) *sql.Row }, roomID, now int64) (sql.NullInt64, error) {
	cutoff, err := displayCoverage(query, roomID, now)
	if err != nil || !cutoff.Valid {
		return cutoff, err
	}
	var last sql.NullInt64
	var failed sql.NullBool
	err = query.QueryRow("SELECT last_success,failed FROM room_scan_status WHERE room_id=?", roomID).Scan(&last, &failed)
	if errors.Is(err, sql.ErrNoRows) {
		return cutoff, nil
	}
	if err != nil {
		return sql.NullInt64{}, err
	}
	if failed.Bool || last.Valid && (now < last.Int64 || now-last.Int64 > 7200) {
		return sql.NullInt64{Int64: now, Valid: true}, nil
	}
	return cutoff, nil
}

func materializeUnbounded(db *sql.DB, from, to time.Time) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
		return err
	}
	if err = materializeUnboundedTx(tx, from, to); err != nil {
		return err
	}
	if err = recordMaterializationCoverage(tx, from, to); err != nil {
		return err
	}
	return tx.Commit()
}
func recordMaterializationCoverage(tx *sql.Tx, from, to time.Time) error {
	if _, err := tx.Exec(`DELETE FROM materialization_coverage WHERE series_id IN (SELECT id FROM booking_series WHERE canceled=1 OR json_extract(rule,'$.rule.Count')!=0 OR json_extract(rule,'$.rule.Until')!='0001-01-01T00:00:00Z')`); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO materialization_coverage(series_id,starts,ends) SELECT id,?,? FROM booking_series WHERE canceled=0 AND json_extract(rule,'$.rule.Count')=0 AND json_extract(rule,'$.rule.Until')='0001-01-01T00:00:00Z' ON CONFLICT(series_id) DO UPDATE SET starts=excluded.starts,ends=excluded.ends`, from.Unix(), to.Unix())
	return err
}
func recordMaterializationCoverageRoom(tx *sql.Tx, from, to time.Time, room int64) error {
	if _, err := tx.Exec(`DELETE FROM materialization_coverage WHERE series_id IN (SELECT id FROM booking_series WHERE room_id=? AND (canceled=1 OR json_extract(rule,'$.rule.Count')!=0 OR json_extract(rule,'$.rule.Until')!='0001-01-01T00:00:00Z'))`, room); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO materialization_coverage(series_id,starts,ends) SELECT id,?,? FROM booking_series WHERE canceled=0 AND room_id=? AND json_extract(rule,'$.rule.Count')=0 AND json_extract(rule,'$.rule.Until')='0001-01-01T00:00:00Z' ON CONFLICT(series_id) DO UPDATE SET starts=excluded.starts,ends=excluded.ends`, from.Unix(), to.Unix(), room)
	return err
}
func materializeUnboundedTx(tx *sql.Tx, from, to time.Time) error {
	return materializeUnboundedRoomTx(tx, from, to, 0)
}
func materializeUnboundedRoomTx(tx *sql.Tx, from, to time.Time, roomID int64) error {
	if from.Year() < 1900 || to.After(time.Date(2401, 1, 1, 0, 0, 0, 0, time.UTC)) || !from.Before(to) {
		return recurrence.ErrLimit
	}
	rows, err := tx.Query("SELECT id,rule FROM booking_series WHERE canceled=0 AND (?=0 OR room_id=?)", roomID, roomID)
	if err != nil {
		return err
	}
	type entry struct {
		id  int64
		raw string
	}
	var all []entry
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.id, &e.raw); err != nil {
			rows.Close()
			return err
		}
		all = append(all, e)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, e := range all {
		var stored storedSeriesRule
		if err = json.Unmarshal([]byte(e.raw), &stored); err != nil {
			return err
		}
		if stored.Rule.Count != 0 || !stored.Rule.Until.IsZero() {
			continue
		}
		stored.Rule.Location, err = time.LoadLocation(stored.Location)
		if err != nil {
			return err
		}
		occurrences, expandErr := recurrence.Expand(stored.Rule, from, to)
		if expandErr != nil {
			return expandErr
		}
		for _, o := range occurrences {
			if _, err = tx.Exec("INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(?,?,?,?) ON CONFLICT(series_id,original_starts) DO NOTHING", e.id, o.Start.Unix(), o.Start.Unix(), o.End.Unix()); err != nil {
				return err
			}
		}
	}
	return nil
}
