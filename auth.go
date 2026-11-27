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
	if version > 7 {
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

func authHandler(db *sql.DB, origin string, trustedProxy netip.Addr) http.Handler {
	limit := &loginLimit{attempts: make(map[string][]time.Time)}
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
		http.SetCookie(w, &http.Cookie{Name: "__Host-session", Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(sessionDuration.Seconds())})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"username": input.Username, "csrf_token": csrf})
	}))
	session := func(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
		w.Header().Set("Cache-Control", "no-store")
		cookies := r.CookiesNamed("__Host-session")
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"username": username, "csrf_token": csrf})
	})
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
		http.SetCookie(w, &http.Cookie{Name: "__Host-session", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(204)
	}))
	registerUserHandlers(mux, db, post, session)
	registerRoomHandlers(mux, db, post, session)
	registerBookingHandlers(mux, db, post, session)
	registerDeviceHandlers(mux, db, post, session)
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
