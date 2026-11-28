package main

import (
	"database/sql"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestDevPasswordReset(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, "existing", "existing-password"); err != nil {
		t.Fatal(err)
	}
	login := func(username, password string) int {
		h := authHandler(db, "https://example.test", netip.Addr{}, time.UTC)
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
		r.Header.Set("Origin", "https://example.test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if err := resetDevAccount(db, false, "existing", "changed-password"); err != nil {
		t.Fatal(err)
	}
	if login("existing", "existing-password") != 200 {
		t.Fatal("non-dev changed password")
	}
	if err := resetDevAccount(db, true, "playwright-dev", "dev-password-123"); err != nil {
		t.Fatal(err)
	}
	if login("playwright-dev", "dev-password-123") != 200 {
		t.Fatal("dev user not created")
	}
	if err := resetDevAccount(db, true, "playwright-dev", "new-password-123"); err != nil {
		t.Fatal(err)
	}
	if login("playwright-dev", "dev-password-123") != 401 || login("playwright-dev", "new-password-123") != 200 {
		t.Fatal("password not reset")
	}
	if login("existing", "existing-password") != 200 {
		t.Fatal("other user changed")
	}
	if err := resetDevAccount(db, true, "playwright-dev", "short"); err == nil {
		t.Fatal("invalid password accepted")
	}
	if login("playwright-dev", "new-password-123") != 200 {
		t.Fatal("invalid configuration changed password")
	}
}
