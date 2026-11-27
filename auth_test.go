package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyLoginLimit(t *testing.T) {
	proxy := netip.MustParseAddr("192.0.2.10")
	for _, tc := range []struct {
		name, remote string
		trusted      netip.Addr
		headers      []string
		want         int
	}{
		{"missing", "192.0.2.10:123", proxy, nil, 400},
		{"empty", "192.0.2.10:123", proxy, []string{""}, 400},
		{"malformed", "192.0.2.10:123", proxy, []string{"192.0.2.1:42"}, 400},
		{"list", "192.0.2.10:123", proxy, []string{"192.0.2.1, 192.0.2.2"}, 400},
		{"multiple", "192.0.2.10:123", proxy, []string{"192.0.2.1", "192.0.2.2"}, 400},
		{"ipv6 zone", "[2001:db8::10]:123", netip.MustParseAddr("2001:db8::10"), []string{"fe80::1%eth0"}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := authHandler(nil, "https://example.test", tc.trusted)
			r := httptest.NewRequest("POST", "/api/login", strings.NewReader("{}"))
			r.RemoteAddr = tc.remote
			r.Header.Set("Origin", "https://example.test")
			for _, v := range tc.headers {
				r.Header.Add("X-Real-IP", v)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, remote string
		trusted      netip.Addr
		clients      []string
		want         []int
	}{
		{"proxy clients", "192.0.2.10:123", proxy, []string{"198.51.100.1", "198.51.100.1", "198.51.100.1", "198.51.100.1", "198.51.100.1", "198.51.100.1", "198.51.100.2"}, []int{400, 400, 400, 400, 400, 429, 400}},
		{"ipv6 clients", "[2001:db8::10]:123", netip.MustParseAddr("2001:db8::10"), []string{"2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4", "2001:db8::5", "2001:db8::6", "2001:db8:0:1::1"}, []int{400, 400, 400, 400, 400, 429, 400}},
		{"spoof", "192.0.2.11:123", proxy, []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "198.51.100.4", "198.51.100.5", "198.51.100.6"}, []int{400, 400, 400, 400, 400, 429}},
		{"default", "192.0.2.10:123", netip.Addr{}, []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "198.51.100.4", "198.51.100.5", "198.51.100.6"}, []int{400, 400, 400, 400, 400, 429}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := authHandler(nil, "https://example.test", tc.trusted)
			for i, client := range tc.clients {
				r := httptest.NewRequest("POST", "/api/login", strings.NewReader("{}"))
				r.RemoteAddr = tc.remote
				r.Header.Set("Origin", "https://example.test")
				r.Header.Set("X-Real-IP", client)
				r.Header.Set("X-Forwarded-For", client)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.want[i] {
					t.Fatalf("request %d: status = %d, want %d", i, w.Code, tc.want[i])
				}
			}
		})
	}
}

func TestLoginLimitCapacity(t *testing.T) {
	now := time.Now()
	limit := &loginLimit{attempts: make(map[string][]time.Time)}
	for i := 0; i < 10000; i++ {
		limit.attempts[netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}).String()] = []time.Time{now}
	}
	if limit.allow("192.0.2.1") {
		t.Fatal("new client allowed while table is full")
	}
	for i := 0; i < 4; i++ {
		if !limit.allow("10.0.0.0") {
			t.Fatal("existing client blocked below its limit")
		}
	}
	if limit.allow("10.0.0.0") {
		t.Fatal("existing client exceeded its limit")
	}
	limit.attempts["10.0.0.1"] = []time.Time{now.Add(-16 * time.Minute)}
	if !limit.allow("192.0.2.1") {
		t.Fatal("expired entry did not free capacity")
	}
}

func TestLoginLimitConcurrent(t *testing.T) {
	limit := &loginLimit{attempts: make(map[string][]time.Time)}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Go(func() {
			if limit.allow("192.0.2.1") {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 5 {
		t.Fatalf("allowed %d requests, want 5", allowed.Load())
	}
}

func TestHTTPOrigins(t *testing.T) {
	for _, tc := range []struct {
		origin string
		valid  bool
	}{
		{"http://localhost:8080", true},
		{"http://127.0.0.1:8080", true},
		{"http://example.test", false},
		{"https://example.test", true},
		{"https://example.test/path", false},
	} {
		if validOrigin(tc.origin) != tc.valid {
			t.Errorf("validOrigin(%q) != %v", tc.origin, tc.valid)
		}
	}
}

func TestAuthenticationOverLocalHTTP(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, "admin", "long-password-123"); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"http://localhost:8080", "https://example.test"} {
		h := authHandler(db, origin, netip.Addr{})
		r := httptest.NewRequest("POST", "http://localhost:8080/api/login", strings.NewReader(`{"username":"admin","password":"long-password-123"}`))
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d", origin, w.Code)
		}
		cookie := w.Result().Cookies()[0]
		if !cookie.Secure {
			t.Fatal("session cookie must be Secure")
		}
		r = httptest.NewRequest("GET", "http://localhost:8080/api/session", nil)
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s session: %d", origin, w.Code)
		}
	}
}

func TestServerReadTimeout(t *testing.T) {
	server := newServer(nil)
	if server.ReadTimeout <= 0 || server.ReadTimeout > 30*time.Second {
		t.Fatalf("ReadTimeout = %s", server.ReadTimeout)
	}
}

func TestAuthentication(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, "admin", "long-password-123"); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, "other", "long-password-123"); err == nil {
		t.Fatal("second bootstrap succeeded")
	}
	h := authHandler(db, "https://example.test", netip.Addr{})
	request := func(method, path, body, origin string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.RemoteAddr = "192.0.2.1:1234"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/session", "", "", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/login", `{}`, "", nil, ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/login", `{"username":"admin","password":"wrong"}`, "https://example.test", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request("POST", "/api/login", `{"username":"admin","password":"long-password-123"}`, "https://example.test", nil, "")
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Username string `json:"username"`
		CSRF     string `json:"csrf_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Username != "admin" || response.CSRF == "" {
		t.Fatalf("response: %s %v", w.Body.String(), err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "__Host-session" || cookies[0].Path != "/" || cookies[0].Domain != "" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal(cookies)
	}
	cookie := cookies[0]
	duplicate := httptest.NewRequest("GET", "https://example.test/api/session", nil)
	duplicate.AddCookie(cookie)
	duplicate.AddCookie(cookie)
	duplicatedResponse := httptest.NewRecorder()
	h.ServeHTTP(duplicatedResponse, duplicate)
	if duplicatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate cookie: %d", duplicatedResponse.Code)
	}
	if w := request("GET", "/api/session", "", "", cookie, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/logout", "", "https://example.test", cookie, "bad"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/logout", "", "https://example.test", cookie, response.CSRF); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/api/session", "", "", cookie, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 3; i++ {
		if w := request("POST", "/api/login", `{"username":"admin","password":"wrong"}`, "https://example.test", nil, ""); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	if w := request("POST", "/api/login", `{"username":"admin","password":"wrong"}`, "https://example.test", nil, ""); w.Code != 429 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/login", `{}`, "https://evil.test", nil, ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
}
