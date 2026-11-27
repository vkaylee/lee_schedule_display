//go:build integration

package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSQLitePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE test (value TEXT NOT NULL); INSERT INTO test VALUES ('ok')"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow("SELECT value FROM test").Scan(&value); err != nil || value != "ok" {
		t.Fatalf("value = %q, error = %v", value, err)
	}
}
