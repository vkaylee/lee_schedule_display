package main

import (
	"database/sql"
	"embed"
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
	if db != nil {
		auth := authHandler(db, origin, trustedProxy)
		mux.Handle("/api/login", auth)
		mux.Handle("/api/logout", auth)
		mux.Handle("/api/session", auth)
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

func validOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))
}

func main() {
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "schedule.db"
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	if err := migrate(db); err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		if len(os.Args) != 3 || os.Args[1] != "bootstrap" {
			log.Fatal("usage: server bootstrap USERNAME (password on stdin)")
		}
		password, err := readPassword(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		if err := bootstrap(db, os.Args[2], password); err != nil {
			log.Fatal(err)
		}
		return
	}
	origin := os.Getenv("APP_ORIGIN")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	if !validOrigin(origin) {
		log.Fatal("APP_ORIGIN must be HTTPS or HTTP on localhost")
	}
	var trustedProxy netip.Addr
	if value := os.Getenv("TRUSTED_PROXY_IP"); value != "" {
		trustedProxy, err = netip.ParseAddr(value)
		if err != nil || trustedProxy.Zone() != "" {
			log.Fatal("TRUSTED_PROXY_IP must be a single IP address without a zone")
		}
	}
	server := newServer(handlerWithAuth(db, origin, trustedProxy))
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		server.Addr = addr
	}
	log.Printf("listening on HTTP %s; secure remote access requires a reverse proxy", server.Addr)
	log.Fatal(server.ListenAndServe())
}
