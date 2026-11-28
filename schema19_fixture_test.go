package main

const schema19Fixture = `CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, salt BLOB NOT NULL, password_hash BLOB NOT NULL, active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)), language TEXT NOT NULL DEFAULT 'vi' CHECK(language IN ('vi','en')));
CREATE TABLE sessions (token_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), csrf TEXT NOT NULL, expires INTEGER NOT NULL);
CREATE TABLE account_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, target_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE rooms (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)), show_title INTEGER NOT NULL DEFAULT 0 CHECK (show_title IN (0,1)), show_organizer INTEGER NOT NULL DEFAULT 0 CHECK (show_organizer IN (0,1)));
CREATE TABLE bookings (id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), title TEXT NOT NULL, organizer TEXT NOT NULL, private INTEGER NOT NULL CHECK(private IN (0,1)), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1)), version INTEGER NOT NULL DEFAULT 1);
CREATE INDEX bookings_room_time ON bookings(room_id, starts, ends) WHERE canceled = 0;
CREATE TABLE booking_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, booking_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE devices (id INTEGER PRIMARY KEY, name TEXT NOT NULL, revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN (0,1)), language TEXT NOT NULL DEFAULT 'vi' CHECK(language IN ('vi','en')), display_config TEXT NOT NULL DEFAULT '', public_id TEXT NOT NULL DEFAULT '');
CREATE UNIQUE INDEX devices_public_id ON devices(public_id) WHERE public_id<>'';
CREATE TABLE booking_series (id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), title TEXT NOT NULL, organizer TEXT NOT NULL, private INTEGER NOT NULL CHECK(private IN (0,1)), rule TEXT NOT NULL, actor_id INTEGER NOT NULL REFERENCES users(id), checked_through INTEGER, version INTEGER NOT NULL DEFAULT 1, canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1)));
CREATE TABLE series_occurrences (series_id INTEGER NOT NULL REFERENCES booking_series(id), original_starts INTEGER NOT NULL, starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), version INTEGER NOT NULL DEFAULT 1, canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1)), room_id INTEGER REFERENCES rooms(id), title TEXT, organizer TEXT, private INTEGER CHECK(private IN (0,1)), PRIMARY KEY(series_id,original_starts));
CREATE INDEX series_occurrences_time ON series_occurrences(starts,ends);
CREATE TABLE occurrence_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, series_id INTEGER NOT NULL, original_starts INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE conflict_alerts (id INTEGER PRIMARY KEY, series_a INTEGER NOT NULL, original_a INTEGER NOT NULL, series_b INTEGER NOT NULL, original_b INTEGER NOT NULL, active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)), acknowledged INTEGER NOT NULL DEFAULT 0 CHECK(acknowledged IN (0,1)), UNIQUE(series_a,original_a,series_b,original_b));
CREATE TABLE conflict_scan_status(id INTEGER PRIMARY KEY CHECK(id=1),last_success INTEGER,checked_from INTEGER,checked_through INTEGER,attempt INTEGER NOT NULL DEFAULT 0,failed INTEGER NOT NULL DEFAULT 0 CHECK(failed IN (0,1)));
INSERT INTO conflict_scan_status(id) VALUES(1);
CREATE TABLE room_scan_status(room_id INTEGER PRIMARY KEY REFERENCES rooms(id),last_success INTEGER,checked_from INTEGER,checked_through INTEGER,attempt INTEGER NOT NULL DEFAULT 0,failed INTEGER NOT NULL DEFAULT 0 CHECK(failed IN (0,1)));
CREATE TABLE materialization_coverage(series_id INTEGER PRIMARY KEY REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends>starts));
CREATE TABLE series_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, series_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
PRAGMA user_version=19`
