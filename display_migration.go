package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

func migrateDisplayRooms(tx *sql.Tx) error {
	var columns, references int
	if err := tx.QueryRow(`SELECT count(*) FROM pragma_table_info('devices') WHERE name IN ('id','room_id','name','token_hash','revoked','last_seen','language','display_config','public_id')`).Scan(&columns); err != nil {
		return err
	}
	var total int
	if err := tx.QueryRow(`SELECT count(*) FROM pragma_table_info('devices')`).Scan(&total); err != nil {
		return err
	}
	if columns != 9 || total != 9 {
		return fmt.Errorf("unsupported devices schema")
	}
	if err := tx.QueryRow(`SELECT count(*) FROM sqlite_schema s, pragma_foreign_key_list(s.name) f WHERE s.type='table' AND s.name NOT LIKE 'sqlite_%' AND f."table"='devices'`).Scan(&references); err != nil {
		return err
	}
	if references != 0 {
		return fmt.Errorf("devices has dependent foreign keys")
	}
	objects, err := tx.Query(`SELECT sql FROM sqlite_schema WHERE tbl_name='devices' AND type IN ('index','trigger') AND sql IS NOT NULL ORDER BY type,name`)
	if err != nil {
		return err
	}
	var definitions []string
	for objects.Next() {
		var definition string
		if err := objects.Scan(&definition); err != nil {
			objects.Close()
			return err
		}
		definitions = append(definitions, definition)
	}
	if err := objects.Err(); err != nil {
		objects.Close()
		return err
	}
	if err := objects.Close(); err != nil {
		return err
	}
	rows, err := tx.Query("SELECT id,room_id,display_config FROM devices ORDER BY id")
	if err != nil {
		return err
	}
	type update struct {
		id  int64
		raw string
	}
	var updates []update
	for rows.Next() {
		var id, roomID int64
		var raw string
		if err := rows.Scan(&id, &roomID, &raw); err != nil {
			rows.Close()
			return err
		}
		c := defaultDisplayConfig()
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &c); err != nil {
				rows.Close()
				return fmt.Errorf("device %d: invalid display config: %w", id, err)
			}
		}
		for i := range c.Blocks {
			b := &c.Blocks[i]
			if b.Type == "room" || b.Type == "status" || b.Type == "upcoming" {
				b.ID = b.Type
				b.RoomID = roomID
			}
		}
		if !validDisplayConfig(c) {
			rows.Close()
			return fmt.Errorf("device %d: invalid display config", id)
		}
		encoded, err := json.Marshal(c)
		if err != nil {
			rows.Close()
			return err
		}
		updates = append(updates, update{id, string(encoded)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE devices_new (id INTEGER PRIMARY KEY, room_id INTEGER REFERENCES rooms(id), name TEXT NOT NULL, token_hash BLOB NOT NULL UNIQUE, revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN (0,1)), last_seen INTEGER, language TEXT NOT NULL DEFAULT 'vi' CHECK(language IN ('vi','en')), display_config TEXT NOT NULL DEFAULT '', public_id TEXT NOT NULL DEFAULT '');
INSERT INTO devices_new(id,room_id,name,token_hash,revoked,last_seen,language,display_config,public_id) SELECT id,room_id,name,token_hash,revoked,last_seen,language,display_config,public_id FROM devices`)
	if err != nil {
		return err
	}
	for _, u := range updates {
		if _, err := tx.Exec("UPDATE devices_new SET display_config=? WHERE id=?", u.raw, u.id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DROP TABLE devices; ALTER TABLE devices_new RENAME TO devices`); err != nil {
		return err
	}
	for _, definition := range definitions {
		if _, err := tx.Exec(definition); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`PRAGMA user_version=17`)
	return err
}
