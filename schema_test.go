package main

import (
	"database/sql"
	"strconv"
	"testing"
)

func TestFreshSchemaAndRejectExistingDatabases(t *testing.T) {
	for _, version := range []int{0, 1, 17, 18, 20} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err = db.Exec(`CREATE TABLE existing (value TEXT); INSERT INTO existing VALUES('keep'); PRAGMA user_version=` + strconv.Itoa(version)); err != nil {
				t.Fatal(err)
			}
			if err = migrate(db); err == nil {
				t.Fatal("existing database accepted")
			}
			var value string
			var gotVersion int
			if err = db.QueryRow(`SELECT value FROM existing`).Scan(&value); err != nil || value != "keep" {
				t.Fatalf("existing data changed: %q %v", value, err)
			}
			if err = db.QueryRow(`PRAGMA user_version`).Scan(&gotVersion); err != nil || gotVersion != version {
				t.Fatalf("version changed: %d %v", gotVersion, err)
			}
		})
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO devices(name,public_id) VALUES('screen','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var count, version, legacy int
	if err = db.QueryRow(`SELECT count(*) FROM devices`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("device count %d: %v", count, err)
	}
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 19 {
		t.Fatalf("version %d: %v", version, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM pragma_table_info('devices') WHERE name IN ('room_id','token_hash','last_seen')`).Scan(&legacy); err != nil || legacy != 0 {
		t.Fatalf("legacy columns %d: %v", legacy, err)
	}
}
