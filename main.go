package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

//go:embed all:web/dist
var assets embed.FS

func handler() http.Handler { return handlerWithAuth(nil, "", netip.Addr{}) }

func handlerWithAuth(db *sql.DB, origin string, trustedProxy netip.Addr) http.Handler {
	return handlerWithTimezone(db, origin, trustedProxy, time.UTC)
}

func handlerWithTimezone(db *sql.DB, origin string, trustedProxy netip.Addr, timezone *time.Location) http.Handler {
	dist, err := fs.Sub(assets, "web/dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	mux := chi.NewRouter()
	mux.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.MethodFunc(http.MethodHead, "/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
	})
	mux.Get("/api/timezone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"timezone": timezone.String()})
	})
	if db != nil {
		auth := authHandler(db, origin, trustedProxy, timezone)
		mux.Handle("/api/login", auth)
		mux.Handle("/api/logout", auth)
		mux.Handle("/api/session", auth)
		mux.Handle("/api/session/language", auth)
		mux.Handle("/api/users", auth)
		mux.Handle("/api/users/*", auth)
		mux.Handle("/api/rooms", auth)
		mux.Handle("/api/rooms/*", auth)
		mux.Handle("/api/bookings", auth)
		mux.Handle("/api/bookings/*", auth)
		mux.Handle("/api/series", auth)
		mux.Handle("/api/series/*", auth)
		mux.Handle("/api/alerts", auth)
		mux.Handle("/api/alerts/*", auth)
		mux.Handle("/api/devices", auth)
		mux.Handle("/api/devices/*", auth)
		mux.Handle("/api/kiosk", auth)
		mux.Handle("/api/display/*", auth)
	}
	mux.HandleFunc("/api/*", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if f, err := dist.Open(name); err == nil {
				f.Close()
			} else if strings.Contains(name, ".") {
				http.NotFound(w, r)
				return
			} else {
				r.URL.Path = "/"
			}
		}
		files.ServeHTTP(w, r)
	})
	return mux
}

func newServer(h http.Handler) *http.Server {
	return &http.Server{Addr: "127.0.0.1:8080", Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second}
}

func validOrigin(origin string, dev bool) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" {
		return true
	}
	ip, err := netip.ParseAddr(u.Hostname())
	return dev && err == nil && ip.IsPrivate()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		if len(os.Args) != 4 {
			log.Fatal("usage: server backup SOURCE DESTINATION")
		}
		if err := backupSQLite(os.Args[2], os.Args[3]); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) > 1 && (len(os.Args) != 3 || os.Args[1] != "bootstrap") {
		log.Fatal("usage: server bootstrap USERNAME (password on stdin) or server backup SOURCE DESTINATION")
	}
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "schedule.db"
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// ponytail: one process serializes SQLite writes; use a shared busy timeout for multi-process deployments.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	if err := migrate(db); err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		password, err := readPassword(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		if err := bootstrap(db, os.Args[2], password); err != nil {
			log.Fatal(err)
		}
		return
	}
	dev := os.Getenv("ENV_DEV") == "true"
	origin := os.Getenv("APP_ORIGIN")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	if !validOrigin(origin, dev) {
		log.Fatal("APP_ORIGIN must be HTTPS or HTTP on localhost; private-network HTTP requires ENV_DEV=true")
	}
	if err := resetDevAccount(db, dev, os.Getenv("DEV_USERNAME"), os.Getenv("DEV_PASSWORD")); err != nil {
		log.Fatal(err)
	}
	var trustedProxy netip.Addr
	if value := os.Getenv("TRUSTED_PROXY_IP"); value != "" {
		trustedProxy, err = netip.ParseAddr(value)
		if err != nil || trustedProxy.Zone() != "" {
			log.Fatal("TRUSTED_PROXY_IP must be a single IP address without a zone")
		}
	}
	zone := os.Getenv("APP_TIMEZONE")
	if zone == "" {
		zone = "UTC"
	}
	timezone, err := time.LoadLocation(zone)
	if err != nil || zone == "Local" {
		log.Fatal("APP_TIMEZONE must be a valid IANA timezone")
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	go conflictWorker(context.Background(), db, func() time.Time { return time.Now().In(timezone) }, ticker.C)
	server := newServer(handlerWithTimezone(db, origin, trustedProxy, timezone))
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		server.Addr = addr
	}
	log.Printf("listening on HTTP %s; secure remote access requires a reverse proxy", server.Addr)
	log.Fatal(server.ListenAndServe())
}
