package main

import (
	"database/sql"
	"testing"
)

func TestWidgetMigrationPreservesSchemaObjects(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`PRAGMA foreign_keys=ON; INSERT INTO rooms(id,name) VALUES(1,'room'); INSERT INTO devices(id,room_id,name,token_hash,public_id) VALUES(1,1,'panel',X'01','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); CREATE INDEX device_name_test ON devices(name); CREATE TABLE device_audit_test(n INTEGER); CREATE TRIGGER device_trigger_test AFTER UPDATE ON devices BEGIN INSERT INTO device_audit_test VALUES(1); END; PRAGMA user_version=16`)
	if err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var objects, audits int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name IN ('device_name_test','device_trigger_test','devices_public_id')`).Scan(&objects); err != nil || objects != 3 {
		t.Fatalf("objects=%d err=%v", objects, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM device_audit_test`).Scan(&audits); err != nil || audits != 0 {
		t.Fatalf("migration fired trigger: %d %v", audits, err)
	}
	if _, err = db.Exec(`UPDATE devices SET name='changed' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM device_audit_test`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("trigger lost: %d %v", audits, err)
	}
	if _, err = db.Exec(`INSERT INTO devices(name,room_id,token_hash) VALUES('bad',999,X'02')`); err == nil {
		t.Fatal("missing room foreign key enforcement")
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key check failed")
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestWidgetMigrationRollback(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rooms(id,name) VALUES(1,'room'); INSERT INTO devices(id,room_id,name,token_hash,public_id) VALUES(1,1,'panel',X'01','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); CREATE TABLE dependent(device_id INTEGER REFERENCES devices(id)); PRAGMA user_version=16`)
	if err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err == nil {
		t.Fatal("dependent foreign key accepted")
	}
	var version int
	var raw string
	if err = db.QueryRow(`SELECT (SELECT user_version FROM pragma_user_version),display_config FROM devices WHERE id=1`).Scan(&version, &raw); err != nil || version != 16 || raw != "" {
		t.Fatalf("rollback: version=%d raw=%q err=%v", version, raw, err)
	}
	if _, err = db.Exec(`DROP TABLE dependent; CREATE TABLE devices_new(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err == nil {
		t.Fatal("conflicting table accepted")
	}
	if err = db.QueryRow(`SELECT (SELECT user_version FROM pragma_user_version),display_config FROM devices WHERE id=1`).Scan(&version, &raw); err != nil || version != 16 || raw != "" {
		t.Fatalf("rollback: version=%d raw=%q err=%v", version, raw, err)
	}
}
