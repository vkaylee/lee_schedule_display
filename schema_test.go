package main

import (
	"database/sql"
	"strconv"
	"testing"
)

func TestVersion19DatabaseMigratesWithoutDataLoss(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema19Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,salt,password_hash) VALUES(7,'admin',X'01',X'02');
INSERT INTO sessions(token_hash,user_id,csrf,expires) VALUES(X'03',7,'csrf',1234);
INSERT INTO rooms(id,name) VALUES(8,'Room');
INSERT INTO booking_series(id,room_id,title,organizer,private,rule,actor_id) VALUES(9,8,'Title','Organizer',0,'rule',7);
INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(9,100,100,200);
INSERT INTO devices(id,name,language,display_config,public_id) VALUES(10,'Display','en','{}','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
PRAGMA user_version=19`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var userCount, sessionCount, seriesCount, occurrenceCount, version int
	var username, language string
	if err := db.QueryRow(`SELECT username FROM users WHERE id=7`).Scan(&username); err != nil || username != "admin" {
		t.Fatalf("user data lost: username=%q err=%v", username, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sessions WHERE user_id=7`).Scan(&sessionCount); err != nil || sessionCount != 1 {
		t.Fatalf("session data lost: count=%d err=%v", sessionCount, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM booking_series WHERE id=9 AND actor_id=7`).Scan(&seriesCount); err != nil || seriesCount != 1 {
		t.Fatalf("series data lost: count=%d err=%v", seriesCount, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM series_occurrences WHERE series_id=9 AND original_starts=100`).Scan(&occurrenceCount); err != nil || occurrenceCount != 1 {
		t.Fatalf("occurrence data lost: count=%d err=%v", occurrenceCount, err)
	}
	if err := db.QueryRow(`SELECT language FROM devices WHERE id=10`).Scan(&language); err != nil || language != "en" {
		t.Fatalf("device data lost: language=%q err=%v", language, err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,salt,password_hash,language) VALUES(11,'zh',X'01',X'02','zh-CN')`); err != nil {
		t.Fatalf("zh-CN rejected for user: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE id=7`).Scan(&userCount); err != nil || userCount != 1 {
		t.Fatalf("user count=%d err=%v", userCount, err)
	}
	if _, err := db.Exec(`UPDATE devices SET language='zh-CN' WHERE id=10`); err != nil {
		t.Fatalf("zh-CN rejected for device: %v", err)
	}
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 20 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	var violations int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations=%d err=%v", violations, err)
	}
}

func TestVersion19MigrationFailureRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema19Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,salt,password_hash) VALUES(7,'admin',X'01',X'02');
DROP TABLE devices;
PRAGMA user_version=19`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err == nil {
		t.Fatal("migration accepted missing devices table")
	}
	var version, migrationTables, users int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 19 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name LIKE '%_migration'`).Scan(&migrationTables); err != nil || migrationTables != 0 {
		t.Fatalf("migration tables=%d err=%v", migrationTables, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE id=7`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("user count=%d err=%v", users, err)
	}
}

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
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 20 {
		t.Fatalf("version %d: %v", version, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM pragma_table_info('devices') WHERE name IN ('room_id','token_hash','last_seen')`).Scan(&legacy); err != nil || legacy != 0 {
		t.Fatalf("legacy columns %d: %v", legacy, err)
	}
}
