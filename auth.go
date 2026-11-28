package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

const passwordIterations = 600000
const sessionDuration = 12 * time.Hour

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 17 {
		return errors.New("unsupported database version")
	}
	if version == 0 {
		_, err = tx.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, salt BLOB NOT NULL, password_hash BLOB NOT NULL);
CREATE TABLE sessions (token_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), csrf TEXT NOT NULL, expires INTEGER NOT NULL);
PRAGMA user_version = 1`)
		if err != nil {
			return err
		}
	}
	if version <= 1 {
		if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1));
CREATE TABLE account_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, target_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
PRAGMA user_version = 2`); err != nil {
			return err
		}
	}
	if version <= 2 {
		if _, err := tx.Exec(`CREATE TABLE rooms (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)), show_title INTEGER NOT NULL DEFAULT 0 CHECK (show_title IN (0,1)), show_organizer INTEGER NOT NULL DEFAULT 0 CHECK (show_organizer IN (0,1)));
PRAGMA user_version = 3`); err != nil {
			return err
		}
	}
	if version <= 3 {
		if _, err := tx.Exec(`CREATE TABLE bookings (id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), title TEXT NOT NULL, organizer TEXT NOT NULL, private INTEGER NOT NULL CHECK(private IN (0,1)), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1)));
CREATE INDEX bookings_room_time ON bookings(room_id, starts, ends) WHERE canceled = 0;
CREATE TABLE booking_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, booking_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
PRAGMA user_version = 4`); err != nil {
			return err
		}
	}
	if version <= 4 {
		if _, err := tx.Exec(`CREATE TABLE devices (id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), name TEXT NOT NULL, token_hash BLOB NOT NULL UNIQUE, revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN (0,1)), last_seen INTEGER);
PRAGMA user_version = 5`); err != nil {
			return err
		}
	}
	if version <= 5 {
		if _, err := tx.Exec(`ALTER TABLE bookings ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
PRAGMA user_version = 6`); err != nil {
			return err
		}
	}
	if version <= 6 {
		if _, err := tx.Exec(`ALTER TABLE devices ADD COLUMN language TEXT NOT NULL DEFAULT 'vi' CHECK(language IN ('vi','en'));
PRAGMA user_version = 7`); err != nil {
			return err
		}
	}
	if version <= 7 {
		if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT 'vi' CHECK(language IN ('vi','en'));
PRAGMA user_version = 8`); err != nil {
			return err
		}
	}
	if version <= 8 {
		if _, err := tx.Exec(`ALTER TABLE devices ADD COLUMN display_config TEXT NOT NULL DEFAULT '';
PRAGMA user_version = 9`); err != nil {
			return err
		}
	}
	if version <= 9 {
		if _, err := tx.Exec(`CREATE TABLE booking_series (id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), title TEXT NOT NULL, organizer TEXT NOT NULL, private INTEGER NOT NULL CHECK(private IN (0,1)), rule TEXT NOT NULL, actor_id INTEGER NOT NULL REFERENCES users(id));
CREATE TABLE series_occurrences (series_id INTEGER NOT NULL REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), PRIMARY KEY(series_id, starts));
CREATE INDEX series_occurrences_time ON series_occurrences(starts, ends);
PRAGMA user_version = 10`); err != nil {
			return err
		}
	}
	if version <= 10 {
		if _, err := tx.Exec(`ALTER TABLE devices ADD COLUMN public_id TEXT NOT NULL DEFAULT '';
UPDATE devices SET public_id=lower(hex(randomblob(16))) WHERE public_id='';
CREATE UNIQUE INDEX devices_public_id ON devices(public_id) WHERE public_id<>'';
PRAGMA user_version=11`); err != nil {
			return err
		}
	}
	if version >= 11 {
		var hasPublicID int
		if err := tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info('devices') WHERE name='public_id'").Scan(&hasPublicID); err != nil {
			return err
		}
		if hasPublicID != 1 {
			return errors.New("unsupported experimental database schema")
		}
	}
	if version <= 11 {
		if _, err := tx.Exec(`ALTER TABLE booking_series ADD COLUMN checked_through INTEGER;
UPDATE booking_series SET checked_through=(SELECT MAX(ends) FROM series_occurrences WHERE series_id=booking_series.id);
PRAGMA user_version=12`); err != nil {
			return err
		}
	}
	if version <= 12 {
		if _, err := tx.Exec(`CREATE TABLE new_series_occurrences (series_id INTEGER NOT NULL REFERENCES booking_series(id), original_starts INTEGER NOT NULL, starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends > starts), version INTEGER NOT NULL DEFAULT 1, canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1)), room_id INTEGER REFERENCES rooms(id), title TEXT, organizer TEXT, private INTEGER CHECK(private IN (0,1)), PRIMARY KEY(series_id,original_starts));
INSERT INTO new_series_occurrences(series_id,original_starts,starts,ends) SELECT series_id,starts,starts,ends FROM series_occurrences;
DROP TABLE series_occurrences;
ALTER TABLE new_series_occurrences RENAME TO series_occurrences;
CREATE INDEX series_occurrences_time ON series_occurrences(starts,ends);
CREATE TABLE occurrence_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, series_id INTEGER NOT NULL, original_starts INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
PRAGMA user_version = 13`); err != nil {
			return err
		}
	}
	if version <= 13 {
		if _, err := tx.Exec(`CREATE TABLE conflict_alerts (id INTEGER PRIMARY KEY, series_a INTEGER NOT NULL, original_a INTEGER NOT NULL, series_b INTEGER NOT NULL, original_b INTEGER NOT NULL, active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)), acknowledged INTEGER NOT NULL DEFAULT 0 CHECK(acknowledged IN (0,1)), UNIQUE(series_a,original_a,series_b,original_b)); CREATE TABLE conflict_scan_status(id INTEGER PRIMARY KEY CHECK(id=1),last_success INTEGER,checked_from INTEGER,checked_through INTEGER,attempt INTEGER NOT NULL DEFAULT 0,failed INTEGER NOT NULL DEFAULT 0 CHECK(failed IN (0,1))); INSERT INTO conflict_scan_status(id) VALUES(1); CREATE TABLE room_scan_status(room_id INTEGER PRIMARY KEY REFERENCES rooms(id),last_success INTEGER,checked_from INTEGER,checked_through INTEGER,attempt INTEGER NOT NULL DEFAULT 0,failed INTEGER NOT NULL DEFAULT 0 CHECK(failed IN (0,1))); INSERT INTO room_scan_status(room_id) SELECT id FROM rooms; PRAGMA user_version=14`); err != nil {
			return err
		}
	}
	if version <= 14 {
		if _, err := tx.Exec(`CREATE TABLE materialization_coverage(series_id INTEGER PRIMARY KEY REFERENCES booking_series(id), starts INTEGER NOT NULL, ends INTEGER NOT NULL CHECK(ends>starts)); PRAGMA user_version=15`); err != nil {
			return err
		}
	}
	if version <= 15 {
		if _, err := tx.Exec(`ALTER TABLE booking_series ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE booking_series ADD COLUMN canceled INTEGER NOT NULL DEFAULT 0 CHECK(canceled IN (0,1));
CREATE TABLE series_audit (id INTEGER PRIMARY KEY, actor_id INTEGER NOT NULL, series_id INTEGER NOT NULL, action TEXT NOT NULL, created_at INTEGER NOT NULL);
PRAGMA user_version = 16`); err != nil {
			return err
		}
	}
	if version <= 16 {
		if err := migrateDisplayRooms(tx); err != nil {
			return err
		}
	}
	var compatible bool
	if err := tx.QueryRow(`SELECT
(SELECT COUNT(*) FROM pragma_table_info('devices') WHERE name='public_id')=1 AND
(SELECT COUNT(*) FROM pragma_table_info('booking_series') WHERE name IN ('id','room_id','title','organizer','private','rule','actor_id','checked_through','version','canceled'))=10 AND
(SELECT COUNT(*) FROM pragma_table_info('booking_series') WHERE name='id' AND pk=1)=1 AND
(SELECT COUNT(*) FROM pragma_table_info('booking_series') WHERE (name='version' AND dflt_value='1' AND "notnull"=1) OR (name='canceled' AND dflt_value='0' AND "notnull"=1))=2 AND
(SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='booking_series' AND lower(replace(replace(sql,' ',''),char(10),'')) LIKE '%check(canceledin(0,1))%')=1 AND
(SELECT COUNT(*) FROM pragma_table_info('series_audit') WHERE name IN ('id','actor_id','series_id','action','created_at'))=5 AND
(SELECT COUNT(*) FROM pragma_index_list('devices') WHERE name='devices_public_id' AND "unique"=1 AND partial=1)=1 AND
(SELECT COUNT(*) FROM pragma_index_info('devices_public_id') WHERE seqno=0 AND name='public_id')=1 AND
(SELECT COUNT(*) FROM pragma_index_info('devices_public_id'))=1 AND
(SELECT COUNT(*) FROM sqlite_schema WHERE type='index' AND name='devices_public_id' AND lower(replace(replace(sql,' ',''),char(10),'')) LIKE '%wherepublic_id<>''''')=1 AND
(SELECT COUNT(*) FROM pragma_table_info('series_occurrences') WHERE name IN ('series_id','original_starts','starts','ends','version','canceled','room_id','title','organizer','private'))=10 AND
(SELECT COUNT(*) FROM pragma_table_info('series_occurrences') WHERE (name='series_id' AND pk=1) OR (name='original_starts' AND pk=2))=2 AND
(SELECT COUNT(*) FROM pragma_table_info('series_occurrences') WHERE name IN ('series_id','original_starts','starts','ends','version','canceled') AND "notnull"=1)=6 AND
(SELECT COUNT(*) FROM pragma_table_info('series_occurrences') WHERE (name='version' AND dflt_value='1') OR (name='canceled' AND dflt_value='0'))=2 AND
(SELECT COUNT(*) FROM pragma_foreign_key_list('series_occurrences') WHERE "from"='series_id' AND "table"='booking_series' AND "to"='id')=1 AND
(SELECT COUNT(*) FROM pragma_foreign_key_list('series_occurrences') WHERE "from"='room_id' AND "table"='rooms' AND "to"='id')=1 AND
(SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='series_occurrences' AND lower(replace(replace(sql,' ',''),char(10),'')) LIKE '%check(ends>starts)%' AND lower(replace(replace(sql,' ',''),char(10),'')) LIKE '%check(canceledin(0,1))%' AND lower(replace(replace(sql,' ',''),char(10),'')) LIKE '%check(privatein(0,1))%')=1 AND
(SELECT COUNT(*) FROM pragma_table_info('occurrence_audit') WHERE name IN ('id','actor_id','series_id','original_starts','action','created_at'))=6`).Scan(&compatible); err != nil {
		return err
	}
	if !compatible {
		return errors.New("unsupported or incomplete database schema")
	}
	return tx.Commit()
}

func bootstrap(db *sql.DB, username, password string) error {
	if username == "" || len(username) > 128 || strings.TrimSpace(username) != username || len(password) < 12 || len(password) > 1024 {
		return errors.New("invalid username or password; password must have 12–1024 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return err
	}
	result, err := db.Exec("INSERT INTO users (username, salt, password_hash) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)", username, salt, hash)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("bootstrap already completed")
	}
	return nil
}

// resetDevAccount is only called with explicitly configured development credentials.
func resetDevAccount(db *sql.DB, dev bool, username, password string) error {
	if !dev {
		return nil
	}
	if username == "" || len(username) > 128 || strings.TrimSpace(username) != username || len(password) < 12 || len(password) > 1024 {
		return errors.New("invalid DEV_USERNAME or DEV_PASSWORD; password must have 12–1024 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO users (username, salt, password_hash) VALUES (?, ?, ?) ON CONFLICT(username) DO UPDATE SET salt=excluded.salt, password_hash=excluded.password_hash`, username, salt, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM sessions WHERE user_id = (SELECT id FROM users WHERE username = ?)`, username); err != nil {
		return err
	}
	return tx.Commit()
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func tokenHash(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }

type loginLimit struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func (l *loginLimit) allow(host string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for key, times := range l.attempts {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= 15*time.Minute {
			delete(l.attempts, key)
		}
	}
	times := l.attempts[host]
	keep := times[:0]
	for _, t := range times {
		if now.Sub(t) < 15*time.Minute {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 5 || (len(l.attempts) >= 10000 && len(keep) == 0) {
		return false
	}
	l.attempts[host] = append(keep, now)
	return true
}

func authHandler(db *sql.DB, origin string, trustedProxy netip.Addr, timezone *time.Location) http.Handler {
	limit := &loginLimit{attempts: make(map[string][]time.Time)}
	cookieName := "__Host-session"
	secureCookie := true
	if u, err := url.Parse(origin); err == nil && u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		cookieName = "dev-session"
		secureCookie = false
	}
	mux := http.NewServeMux()
	post := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if r.Header.Get("Origin") != origin {
				http.Error(w, "forbidden", 403)
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("POST /api/login", post(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid client address", http.StatusBadRequest)
			return
		}
		peer, err := netip.ParseAddr(host)
		if err != nil || peer.Zone() != "" {
			http.Error(w, "invalid client address", http.StatusBadRequest)
			return
		}
		client := peer.Unmap()
		if trustedProxy.IsValid() && client == trustedProxy.Unmap() {
			values := r.Header.Values("X-Real-IP")
			if len(values) != 1 {
				http.Error(w, "invalid client address", http.StatusBadRequest)
				return
			}
			client, err = netip.ParseAddr(values[0])
			if err != nil || client.Zone() != "" {
				http.Error(w, "invalid client address", http.StatusBadRequest)
				return
			}
			client = client.Unmap()
		}
		// ponytail: /64 groups common IPv6 subnets; add a deployment-specific policy if prefixes vary.
		if client.Is6() {
			client = netip.PrefixFrom(client, 64).Masked().Addr()
		}
		if !limit.allow(client.String()) {
			http.Error(w, "too many requests", 429)
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.Username == "" || len(input.Username) > 128 || len(input.Password) > 1024 {
			http.Error(w, "invalid request", 400)
			return
		}
		var id int64
		var salt, hash []byte
		err = db.QueryRow("SELECT id, salt, password_hash FROM users WHERE username = ? AND active = 1", input.Username).Scan(&id, &salt, &hash)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "internal error", 500)
			return
		}
		if err == sql.ErrNoRows {
			salt = make([]byte, 16)
			hash = make([]byte, 32)
		}
		candidate, err := pbkdf2.Key(sha256.New, input.Password, salt, passwordIterations, 32)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		if subtle.ConstantTimeCompare(hash, candidate) != 1 || id == 0 {
			http.Error(w, "invalid credentials", 401)
			return
		}
		token, err := randomToken()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		csrf, err := randomToken()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		expires := time.Now().Add(sessionDuration)
		result, err := db.Exec("INSERT INTO sessions (token_hash, user_id, csrf, expires) SELECT ?, id, ?, ? FROM users WHERE id = ? AND active = 1 AND password_hash = ? AND salt = ?", tokenHash(token), csrf, expires.Unix(), id, hash, salt)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			http.Error(w, "invalid credentials", 401)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: secureCookie, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(sessionDuration.Seconds())})
		var language string
		if err := db.QueryRow("SELECT language FROM users WHERE id = ?", id).Scan(&language); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"username": input.Username, "csrf_token": csrf, "language": language})
	}))
	session := func(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
		w.Header().Set("Cache-Control", "no-store")
		cookies := r.CookiesNamed(cookieName)
		if len(cookies) != 1 || len(cookies[0].Value) != 43 {
			http.Error(w, "unauthorized", 401)
			return "", "", "", false
		}
		c := cookies[0]
		var username, csrf string
		err := db.QueryRow("SELECT users.username, sessions.csrf FROM sessions JOIN users ON users.id = sessions.user_id WHERE sessions.token_hash = ? AND sessions.expires > ? AND users.active = 1", tokenHash(c.Value), time.Now().Unix()).Scan(&username, &csrf)
		if err == sql.ErrNoRows {
			http.Error(w, "unauthorized", 401)
			return "", "", "", false
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return "", "", "", false
		}
		return c.Value, username, csrf, true
	}
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		_, username, csrf, ok := session(w, r)
		if !ok {
			return
		}
		var language string
		if err := db.QueryRow("SELECT language FROM users WHERE username = ? AND active = 1", username).Scan(&language); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"username": username, "csrf_token": csrf, "language": language})
	})
	mux.HandleFunc("PUT /api/session/language", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		var input struct {
			Language string `json:"language"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || (input.Language != "vi" && input.Language != "en") {
			http.Error(w, "invalid request", 400)
			return
		}
		result, err := db.Exec("UPDATE users SET language = ? WHERE active = 1 AND id = (SELECT user_id FROM sessions WHERE token_hash = ? AND expires > ?)", input.Language, tokenHash(token), time.Now().Unix())
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("POST /api/logout", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		if _, err := db.Exec("DELETE FROM sessions WHERE token_hash = ?", tokenHash(token)); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", Secure: secureCookie, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(204)
	}))
	registerUserHandlers(mux, db, post, session)
	registerRoomHandlers(mux, db, post, session)
	registerBookingHandlers(mux, db, post, session)
	registerSeriesHandlers(mux, db, post, session, timezone)
	registerAlertHandlers(mux, db, post, session)
	registerDeviceHandlers(mux, db, post, session)
	registerDisplayHandlers(mux, db, post, session)
	return mux
}

func readPassword(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1026))
	if err != nil {
		return "", err
	}
	if len(data) > 1025 {
		return "", fmt.Errorf("password too long")
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r"), nil
}
