package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestDisplayHasNoLegacyContract(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO devices(name,public_id) VALUES('panel','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`); err != nil {
		t.Fatal(err)
	}
	handler := handlerWithAuth(db, "", netip.Addr{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/display/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil))
	var payload map[string]json.RawMessage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &payload) != nil {
		t.Fatalf("display: %d %s", w.Code, w.Body.String())
	}
	for _, field := range []string{"room", "entries", "valid_until"} {
		if _, ok := payload[field]; ok {
			t.Fatalf("unexpected legacy field %s", field)
		}
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/kiosk", nil))
	if w.Code != 404 {
		t.Fatalf("kiosk status: %d", w.Code)
	}
}
