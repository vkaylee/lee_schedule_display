package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	for _, tc := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/", 200, "<div id=\"app\">"},
		{"/schedule", 200, "<div id=\"app\">"},
		{"/missing.js", 404, "404"},
		{"/api/health", 200, `"status":"ok"`},
		{"/api/missing", 404, "404"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("got status %d and body %q", w.Code, w.Body.String())
			}
		})
	}
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodHead, "/api/health", http.StatusOK},
		{http.MethodPost, "/api/health", http.StatusNotFound},
		{http.MethodGet, "/api/missing/nested", http.StatusNotFound},
		{http.MethodPost, "/schedule", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
		})
	}
}
